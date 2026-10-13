package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	geo "github.com/lsbjordao/go-iucn-eoo-aoo"
	"github.com/lsbjordao/go-iucn-eoo-aoo/internal/input"
	"github.com/lsbjordao/go-iucn-eoo-aoo/internal/server"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	if e := run(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, "eoo-aoo:", e)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return nil
	}
	switch args[0] {
	case "version":
		return output("-", map[string]string{"version": geo.Version, "gdal": geo.GDALVersion(), "proj": geo.PROJVersion(), "go": runtime.Version(), "binding": "github.com/airbusgeo/godal v0.0.18"})
	case "calc", "batch", "compare":
		return calculate(args[0], args[1:])
	case "audit":
		return audit(args[1:])
	case "curate":
		return curate(args[1:])
	case "influence":
		return influence(args[1:])
	case "serve":
		return serve(args[1:])
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		return fmt.Errorf("unknown command %q; use help", args[0])
	}
}

func usage() {
	fmt.Println(`eoo-aoo: Go + GDAL/PROJ spatial range metrics

  eoo-aoo calc --in occurrences.csv --out result.json
  eoo-aoo calc --in occurrences.csv --projection iucn-cea --html map.html
  eoo-aoo compare --in occurrences.csv --out projection-comparison.json
  eoo-aoo batch --in taxa.csv --out batch.json --workers 4
  eoo-aoo audit --in occurrences.csv --source-flag-field issue --out audit.json
  eoo-aoo curate --in occurrences.csv --manifest manifest.json --derived curated.csv --out curation-log.json
  eoo-aoo influence --in occurrences.csv --top-k 10 --out influence.json
  eoo-aoo calc --in occurrences.csv --protocol-out protocol.json
  eoo-aoo calc --in later.csv --protocol protocol.json
  eoo-aoo serve --listen 127.0.0.1:8080
  eoo-aoo version

Canonical record fields: id, taxon, lon, lat.
Projection strategies: local-laea (default), iucn-cea.
Use <command> --help for options. Read docs/METHODOLOGY.md and docs/PROJECTIONS.md before assessment use.`)
}

func pair(s string) ([2]float64, error) {
	a := strings.Split(s, ",")
	if len(a) != 2 {
		return [2]float64{}, fmt.Errorf("expected two comma-separated numbers")
	}
	x, e := strconv.ParseFloat(strings.TrimSpace(a[0]), 64)
	if e != nil {
		return [2]float64{}, e
	}
	y, e := strconv.ParseFloat(strings.TrimSpace(a[1]), 64)
	return [2]float64{x, y}, e
}

// common holds the flags shared by every command that reads occurrences.
type common struct {
	fs                                    *flag.FlagSet
	file, out, format, configFile         *string
	crs, projection, center, mode, origin *string
	xcol, ycol, tcol, idcol               *string
	cell, extentWarn                      *float64
	steps                                 *int
}

func newCommon(name string) *common {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	return &common{
		fs:         fs,
		file:       fs.String("in", "", "CSV, TSV, JSON or GeoJSON path; - for stdin"),
		out:        fs.String("out", "-", "JSON output; - for stdout"),
		format:     fs.String("format", "", "csv, tsv, json; auto from extension"),
		configFile: fs.String("config", "", "JSON Config file (flags override its fields)"),
		crs:        fs.String("input-crs", "EPSG:4326", "input CRS; traditional GIS X,Y order"),
		projection: fs.String("projection", geo.ProjectionLocalLAEA, "analysis projection: local-laea or iucn-cea"),
		center:     fs.String("center", "", "fixed WGS84 lon,lat center for local-laea"),
		cell:       fs.Float64("cell-size", 2000, "grid side in metres; IUCN reference=2000"),
		mode:       fs.String("grid-mode", "auto", "auto, exact, sampled or fixed"),
		steps:      fs.Int("grid-steps", 10, "sampled offsets per axis"),
		origin:     fs.String("origin", "0,0", "grid base origin in analysis metres: x,y"),
		extentWarn: fs.Float64("extent-warning-deg", 20, "warn when the lon or lat span exceeds this many degrees"),
		xcol:       fs.String("x-col", "", "coordinate column; autodetect lon/x/decimalLongitude"),
		ycol:       fs.String("y-col", "", "coordinate column; autodetect lat/y/decimalLatitude"),
		tcol:       fs.String("taxon-col", "", "taxon column; autodetect taxon/scientificName/species"),
		idcol:      fs.String("id-col", "", "record ID column; autodetect id/occurrenceID"),
	}
}

// parse returns false (with nil error) when help was requested.
func (c *common) parse(args []string) (bool, error) {
	if e := c.fs.Parse(args); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return false, nil
		}
		return false, e
	}
	if c.fs.NArg() != 0 {
		return false, fmt.Errorf("unexpected positional arguments")
	}
	if *c.file == "" {
		return false, fmt.Errorf("--in is required")
	}
	if *c.file != "-" && *c.out != "-" {
		a, _ := filepath.Abs(*c.file)
		b, _ := filepath.Abs(*c.out)
		if a == b {
			return false, fmt.Errorf("input and output must differ")
		}
	}
	return true, nil
}

func (c *common) config() (geo.Config, error) {
	cfg := geo.Config{}
	if *c.configFile != "" {
		if e := readStrictJSON(*c.configFile, &cfg); e != nil {
			return cfg, e
		}
	}
	visited := map[string]bool{}
	c.fs.Visit(func(f *flag.Flag) { visited[f.Name] = true })
	if visited["input-crs"] {
		cfg.InputCRS = *c.crs
	}
	if visited["projection"] {
		cfg.ProjectionStrategy = *c.projection
	}
	if visited["cell-size"] {
		cfg.CellSizeM = *c.cell
	}
	if visited["grid-mode"] {
		cfg.GridMode = *c.mode
	}
	if visited["grid-steps"] {
		cfg.GridSteps = *c.steps
	}
	if visited["extent-warning-deg"] {
		cfg.ExtentWarningDeg = *c.extentWarn
	}
	if visited["center"] {
		p, e := pair(*c.center)
		if e != nil {
			return cfg, e
		}
		cfg.Center = &p
	}
	if visited["origin"] {
		p, e := pair(*c.origin)
		if e != nil {
			return cfg, e
		}
		cfg.Origin = p
	}
	return cfg, nil
}

func readStrictJSON(path string, dst any) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if e = d.Decode(dst); e != nil {
		return fmt.Errorf("%s: %w", path, e)
	}
	var extra any
	if e = d.Decode(&extra); e != io.EOF {
		return fmt.Errorf("%s must contain one JSON value", path)
	}
	return nil
}

// raw reads the input bytes and resolves the format.
func (c *common) raw() ([]byte, string, error) {
	var r io.Reader = os.Stdin
	if *c.file != "-" {
		f, e := os.Open(*c.file)
		if e != nil {
			return nil, "", e
		}
		defer f.Close()
		r = f
	}
	b, e := io.ReadAll(io.LimitReader(r, (64<<20)+1))
	if e != nil {
		return nil, "", e
	}
	if len(b) > 64<<20 {
		return nil, "", fmt.Errorf("input exceeds 64 MiB")
	}
	format := *c.format
	if format == "" {
		format = strings.TrimPrefix(strings.ToLower(filepath.Ext(*c.file)), ".")
		if format == "" || format == "geojson" {
			format = "json"
		}
	}
	if format != "json" && format != "csv" && format != "tsv" {
		return nil, "", fmt.Errorf("unsupported input format; use --format csv, tsv or json")
	}
	return b, format, nil
}

func (c *common) options() input.Options {
	return input.Options{XColumn: *c.xcol, YColumn: *c.ycol, TaxonColumn: *c.tcol, IDColumn: *c.idcol}
}

func (c *common) load() (input.Data, geo.Config, string, error) {
	cfg, e := c.config()
	if e != nil {
		return input.Data{}, cfg, "", e
	}
	b, format, e := c.raw()
	if e != nil {
		return input.Data{}, cfg, "", e
	}
	data, e := input.Read(b, format, c.options())
	if e != nil {
		return data, cfg, "", e
	}
	if data.GeoJSON && cfg.InputCRS != "" && cfg.InputCRS != "EPSG:4326" {
		return data, cfg, "", fmt.Errorf("RFC7946 GeoJSON requires EPSG:4326 input")
	}
	return data, cfg, format, nil
}

func calculate(command string, args []string) error {
	c := newCommon(command)
	gpkg := c.fs.String("gpkg", "", "new GeoPackage output (calc only)")
	geojsonDir := c.fs.String("geojson-dir", "", "new directory with three WGS84 GeoJSON layers (calc only)")
	html := c.fs.String("html", "", "new interactive MapLibre HTML map (calc only)")
	protocolIn := c.fs.String("protocol", "", "reproduce a frozen analysis protocol JSON (fixed grid; calc only)")
	protocolOut := c.fs.String("protocol-out", "", "write the frozen analysis protocol of this run (calc only)")
	workers := c.fs.Int("workers", 4, "batch workers, 1..32")
	details := c.fs.Bool("details", false, "include occupied cells and full record provenance in JSON")
	if ok, e := c.parse(args); !ok {
		return e
	}
	if *workers < 1 || *workers > 32 {
		return fmt.Errorf("workers must be 1..32")
	}
	if command != "calc" && (*protocolIn != "" || *protocolOut != "") {
		return fmt.Errorf("--protocol and --protocol-out are available in calc")
	}
	data, cfg, _, e := c.load()
	if e != nil {
		return e
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if command == "batch" {
		if *gpkg != "" || *geojsonDir != "" || *html != "" {
			return fmt.Errorf("GIS and HTML exports are available in calc; batch returns per-taxon JSON")
		}
		items, e := batch(ctx, data.Points, cfg, *workers, *details)
		if e != nil {
			return e
		}
		if e = output(*c.out, items); e != nil {
			return e
		}
		for _, item := range items {
			if item.Error != "" {
				return fmt.Errorf("one or more taxa failed; inspect batch report")
			}
		}
		return nil
	}

	if command == "compare" {
		if *gpkg != "" || *geojsonDir != "" || *html != "" {
			return fmt.Errorf("compare returns a JSON comparison; use calc to export maps")
		}
		comparison, e := geo.CompareProjections(ctx, data.Points, cfg)
		if e != nil {
			return e
		}
		for _, w := range comparison.Warnings {
			fmt.Fprintln(os.Stderr, "eoo-aoo: compare:", w.Code+":", w.Message)
		}
		return output(*c.out, comparison)
	}

	var result geo.Result
	if *protocolIn != "" {
		var p geo.AnalysisProtocol
		if e = readStrictJSON(*protocolIn, &p); e != nil {
			return e
		}
		result, e = geo.CalculateWithProtocol(ctx, data.Points, p, cfg)
	} else {
		result, e = geo.Calculate(ctx, data.Points, cfg)
	}
	if e != nil {
		return e
	}
	reportAOO(os.Stderr, result.AOO)
	if *protocolOut != "" {
		if e = output(*protocolOut, result.Protocol); e != nil {
			return e
		}
	}
	if *gpkg != "" || *geojsonDir != "" || *html != "" {
		path := *gpkg
		var tempRoot string
		if path == "" {
			tempRoot, e = os.MkdirTemp("", "eoo-aoo-")
			if e != nil {
				return e
			}
			defer os.RemoveAll(tempRoot)
			path = filepath.Join(tempRoot, "analysis.gpkg")
		}
		if e = geo.WriteGeoPackage(path, result); e != nil {
			return e
		}

		displayDir := *geojsonDir
		if *html != "" && displayDir == "" {
			if tempRoot == "" {
				tempRoot, e = os.MkdirTemp("", "eoo-aoo-")
				if e != nil {
					return e
				}
				defer os.RemoveAll(tempRoot)
			}
			displayDir = filepath.Join(tempRoot, "display")
		}
		if displayDir != "" {
			if e = geo.ExportGeoJSON(path, displayDir); e != nil {
				return e
			}
		}
		if *html != "" {
			if e = geo.WriteMapHTML(*html, displayDir, result); e != nil {
				return e
			}
		}
	}
	if !*details {
		result = result.Summary()
	}
	return output(*c.out, result)
}

// reportAOO puts the effective search and its guarantee next to the AOO, so
// a fallback is visible even when only the exit status and stderr are read.
func reportAOO(w io.Writer, a geo.AOOResult) {
	fmt.Fprintf(w, "eoo-aoo: AOO %g km² (%d cells); grid requested=%s effective=%s guarantee=%s; tested origins %d..%d cells", a.AreaKM2, a.OccupiedCells, a.RequestedGridMode, a.EffectiveGridMode, a.SearchGuarantee, a.TestedMinCells, a.TestedMaxCells)
	if a.FallbackReason != "" {
		fmt.Fprintf(w, "; fallback=%s", a.FallbackReason)
	}
	fmt.Fprintln(w)
}

func audit(args []string) error {
	c := newCommon("audit")
	coarse := c.fs.Int("coarse-decimals", -1, "flag records whose X and Y both have at most this many decimals; -1 disables")
	uncertainty := c.fs.String("uncertainty-field", "", "property with coordinate uncertainty in metres, e.g. coordinateUncertaintyInMeters")
	flags := c.fs.String("source-flag-field", "", "property with source quality flags, e.g. issue")
	contextFields := c.fs.String("context-fields", "", "comma-separated properties copied into each finding, e.g. locality,countryCode")
	if ok, e := c.parse(args); !ok {
		return e
	}
	cfg, e := c.config()
	if e != nil {
		return e
	}
	b, format, e := c.raw()
	if e != nil {
		return e
	}
	records, e := input.ReadAudit(b, format, c.options())
	if e != nil {
		return e
	}
	ac := geo.AuditConfig{InputCRS: cfg.InputCRS, CellSizeM: cfg.CellSizeM, UncertaintyField: *uncertainty, SourceFlagField: *flags}
	for _, f := range strings.Split(*contextFields, ",") {
		if f = strings.TrimSpace(f); f != "" {
			ac.ContextFields = append(ac.ContextFields, f)
		}
	}
	if *coarse >= 0 {
		ac.CoarseDecimals = coarse
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	report, e := geo.AuditOccurrences(ctx, records, ac)
	if e != nil {
		return e
	}
	sum := sha256.Sum256(b)
	report.SourceSHA256 = hex.EncodeToString(sum[:])
	fmt.Fprintf(os.Stderr, "eoo-aoo: audit: %d records, %d invalid, %d with findings %v\n", report.InputRecords, report.InvalidRecords, report.RecordsWithIssues, report.CountsByCode)
	return output(*c.out, report)
}

func curate(args []string) error {
	c := newCommon("curate")
	manifest := c.fs.String("manifest", "", "curation manifest JSON (required)")
	derived := c.fs.String("derived", "", "new derived input file, same format as --in (required)")
	if ok, e := c.parse(args); !ok {
		return e
	}
	if *manifest == "" || *derived == "" {
		return fmt.Errorf("--manifest and --derived are required")
	}
	if _, e := os.Lstat(*derived); !os.IsNotExist(e) {
		return fmt.Errorf("derived output already exists or is inaccessible: %s", *derived)
	}
	data, _, format, e := c.load()
	if e != nil {
		return e
	}
	var m geo.CurationManifest
	if e = readStrictJSON(*manifest, &m); e != nil {
		return e
	}
	points, log, e := geo.ApplyCuration(data.Points, m)
	if e != nil {
		return e
	}
	b, e := input.Write(points, data, format)
	if e != nil {
		return e
	}
	check, e := input.Read(b, format, c.options())
	if e != nil {
		return fmt.Errorf("derived file does not read back: %w", e)
	}
	if h, _ := geo.InputHash(check.Points); h != log.OutputSHA256 {
		return fmt.Errorf("derived file reads back with hash %s, expected %s", h, log.OutputSHA256)
	}
	if e = writeNew(*derived, b); e != nil {
		return e
	}
	fmt.Fprintf(os.Stderr, "eoo-aoo: curate: %d -> %d records; derived input_sha256 %s\n", log.InputRecords, log.OutputRecords, log.OutputSHA256)
	return output(*c.out, log)
}

func influence(args []string) error {
	c := newCommon("influence")
	maxCandidates := c.fs.Int("max-candidates", 1000, "evaluate at most this many hull-vertex groups")
	topK := c.fs.Int("top-k", 0, "list only the k most influential groups; 0 lists all")
	recompute := c.fs.Bool("recompute-projection", false, "also recompute each removal with a new automatic local-laea centre")
	if ok, e := c.parse(args); !ok {
		return e
	}
	data, cfg, _, e := c.load()
	if e != nil {
		return e
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	r, e := geo.AnalyzeEOOInfluence(ctx, data.Points, cfg, geo.InfluenceConfig{MaxCandidates: *maxCandidates, TopK: *topK, RecomputeProjection: *recompute})
	if e != nil {
		return e
	}
	return output(*c.out, r)
}

func writeNew(path string, b []byte) error {
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if e != nil {
		return e
	}
	if _, e = f.Write(b); e != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return e
	}
	return f.Close()
}

type batchItem struct {
	Taxon  string      `json:"taxon"`
	Result *geo.Result `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func batch(ctx context.Context, p []geo.Point, c geo.Config, workers int, details bool) ([]batchItem, error) {
	groups := map[string][]geo.Point{}
	for _, v := range p {
		if v.Taxon == "" {
			return nil, fmt.Errorf("batch requires a nonempty taxon for every record")
		}
		groups[v.Taxon] = append(groups[v.Taxon], v)
	}
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]batchItem, len(names))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				name := names[i]
				out[i].Taxon = name
				r, e := geo.Calculate(ctx, groups[name], c)
				if e != nil {
					out[i].Error = e.Error()
				} else {
					if !details {
						r = r.Summary()
					}
					out[i].Result = &r
				}
			}
		}()
	}
	for i := range names {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return out, nil
}

func output(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	b = append(b, '\n')
	if path == "-" {
		_, e = os.Stdout.Write(b)
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".eoo-aoo-*.json")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(b); e != nil {
		_ = f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(f.Name(), path)
}

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	listen := fs.String("listen", "127.0.0.1:8080", "listen address")
	workers := fs.Int("workers", 4, "max concurrent calculations (1..32)")
	if e := fs.Parse(args); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return nil
		}
		return e
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	if *workers < 1 || *workers > 32 {
		return fmt.Errorf("workers must be 1..32")
	}
	srv := &http.Server{Addr: *listen, Handler: server.NewHandler(*workers, os.Getenv("EOO_AOO_TOKEN")), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 8192}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe() }()
	fmt.Fprintln(os.Stderr, "eoo-aoo listening on", *listen)
	select {
	case e := <-done:
		if errors.Is(e, http.ErrServerClosed) {
			return nil
		}
		return e
	case <-ctx.Done():
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(c)
	}
}
