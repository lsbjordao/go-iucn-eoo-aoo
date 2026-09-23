package main

import (
	"context"
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

func calculate(command string, args []string) error {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	file := fs.String("in", "", "CSV, TSV, JSON or GeoJSON path; - for stdin")
	out := fs.String("out", "-", "JSON output; - for stdout")
	format := fs.String("format", "", "csv, tsv, json; auto from extension")
	configFile := fs.String("config", "", "JSON Config file (flags override its fields)")
	crs := fs.String("input-crs", "EPSG:4326", "input CRS; traditional GIS X,Y order")
	projection := fs.String("projection", geo.ProjectionLocalLAEA, "analysis projection: local-laea or iucn-cea")
	center := fs.String("center", "", "fixed WGS84 lon,lat center for local-laea")
	cell := fs.Float64("cell-size", 2000, "grid side in metres; IUCN reference=2000")
	mode := fs.String("grid-mode", "auto", "auto, exact, sampled or fixed")
	steps := fs.Int("grid-steps", 10, "sampled offsets per axis")
	origin := fs.String("origin", "0,0", "grid base origin in analysis metres: x,y")
	xcol := fs.String("x-col", "", "coordinate column; autodetect lon/x/decimalLongitude")
	ycol := fs.String("y-col", "", "coordinate column; autodetect lat/y/decimalLatitude")
	tcol := fs.String("taxon-col", "", "taxon column; autodetect taxon/scientificName/species")
	gpkg := fs.String("gpkg", "", "new GeoPackage output (calc only)")
	geojsonDir := fs.String("geojson-dir", "", "new directory with three WGS84 GeoJSON layers (calc only)")
	html := fs.String("html", "", "new interactive MapLibre HTML map (calc only)")
	workers := fs.Int("workers", 4, "batch workers, 1..32")
	details := fs.Bool("details", false, "include occupied cells and full record provenance in JSON")
	if e := fs.Parse(args); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return nil
		}
		return e
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	if *file == "" {
		return fmt.Errorf("--in is required")
	}
	if *file != "-" && *out != "-" {
		a, _ := filepath.Abs(*file)
		b, _ := filepath.Abs(*out)
		if a == b {
			return fmt.Errorf("input and output must differ")
		}
	}
	if *workers < 1 || *workers > 32 {
		return fmt.Errorf("workers must be 1..32")
	}
	cfg := geo.Config{}
	if *configFile != "" {
		f, e := os.Open(*configFile)
		if e != nil {
			return e
		}
		defer f.Close()
		d := json.NewDecoder(f)
		d.DisallowUnknownFields()
		if e = d.Decode(&cfg); e != nil {
			return e
		}
		var extra any
		if e = d.Decode(&extra); e != io.EOF {
			return fmt.Errorf("config must contain one JSON value")
		}
	}
	visited := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { visited[f.Name] = true })
	if visited["input-crs"] {
		cfg.InputCRS = *crs
	}
	if visited["projection"] {
		cfg.ProjectionStrategy = *projection
	}
	if visited["cell-size"] {
		cfg.CellSizeM = *cell
	}
	if visited["grid-mode"] {
		cfg.GridMode = *mode
	}
	if visited["grid-steps"] {
		cfg.GridSteps = *steps
	}
	if visited["center"] {
		p, e := pair(*center)
		if e != nil {
			return e
		}
		cfg.Center = &p
	}
	if visited["origin"] {
		p, e := pair(*origin)
		if e != nil {
			return e
		}
		cfg.Origin = p
	}
	var r io.Reader = os.Stdin
	if *file != "-" {
		f, e := os.Open(*file)
		if e != nil {
			return e
		}
		defer f.Close()
		r = f
	}
	b, e := io.ReadAll(io.LimitReader(r, (64<<20)+1))
	if e != nil {
		return e
	}
	if len(b) > 64<<20 {
		return fmt.Errorf("input exceeds 64 MiB")
	}
	if *format == "" {
		*format = strings.TrimPrefix(strings.ToLower(filepath.Ext(*file)), ".")
		if *format == "" || *format == "geojson" {
			*format = "json"
		}
	}
	if *format != "json" && *format != "csv" && *format != "tsv" {
		return fmt.Errorf("unsupported input format; use --format csv, tsv or json")
	}
	data, e := input.Read(b, *format, input.Options{XColumn: *xcol, YColumn: *ycol, TaxonColumn: *tcol})
	if e != nil {
		return e
	}
	if data.GeoJSON && cfg.InputCRS != "" && cfg.InputCRS != "EPSG:4326" {
		return fmt.Errorf("RFC7946 GeoJSON requires EPSG:4326 input")
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
		if e = output(*out, items); e != nil {
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
		return output(*out, comparison)
	}

	result, e := geo.Calculate(ctx, data.Points, cfg)
	if e != nil {
		return e
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
	return output(*out, result)
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
