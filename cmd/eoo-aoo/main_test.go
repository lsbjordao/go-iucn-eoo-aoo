package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	geo "github.com/lsbjordao/go-iucn-eoo-aoo"
)

func TestBatchSeparatesTaxaAndErrors(t *testing.T) {
	p := []geo.Point{{X: -43, Y: -22, Taxon: "B"}, {X: 0, Y: 0, Taxon: "A"}, {X: 181, Y: 0, Taxon: "C"}}
	r, e := batch(context.Background(), p, geo.Config{}, 3, false)
	if e != nil {
		t.Fatal(e)
	}
	if len(r) != 3 || r[0].Taxon != "A" || r[1].Taxon != "B" || r[2].Error == "" || r[0].Result.AOO.AreaKM2 != 4 {
		t.Fatal(r)
	}
	if _, e = batch(context.Background(), []geo.Point{{X: 0, Y: 0}}, geo.Config{}, 1, false); e == nil {
		t.Fatal("unlabelled batch record accepted")
	}
}

func TestRunCalcCSVAndGeoJSON(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "occurrences.csv")
	csvOut := filepath.Join(dir, "csv-result.json")
	if err := os.WriteFile(csvPath, []byte("id,taxon,lon,lat\na,Mimosa test,-43.2,-22.9\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"calc", "--in", csvPath, "--out", csvOut}); err != nil {
		t.Fatal(err)
	}
	var csvResult geo.Result
	readJSON(t, csvOut, &csvResult)
	if csvResult.SchemaVersion != "1" || csvResult.SoftwareVersion != geo.Version || csvResult.Taxon != "Mimosa test" || csvResult.AOO.AreaKM2 != 4 {
		t.Fatalf("unexpected CSV result: %+v", csvResult)
	}

	geojsonPath := filepath.Join(dir, "occurrences.geojson")
	geojsonOut := filepath.Join(dir, "geojson-result.json")
	geojson := `{"type":"FeatureCollection","features":[{"type":"Feature","id":"a","properties":{"taxon":"Mimosa test"},"geometry":{"type":"Point","coordinates":[-43.2,-22.9]}}]}`
	if err := os.WriteFile(geojsonPath, []byte(geojson), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"calc", "--in", geojsonPath, "--out", geojsonOut}); err != nil {
		t.Fatal(err)
	}
	var geojsonResult geo.Result
	readJSON(t, geojsonOut, &geojsonResult)
	if geojsonResult.SchemaVersion != "1" || geojsonResult.Taxon != csvResult.Taxon || geojsonResult.AOO.AreaKM2 != csvResult.AOO.AreaKM2 || geojsonResult.EOO.Status != csvResult.EOO.Status {
		t.Fatalf("equivalent CSV/GeoJSON metrics diverged: csv=%+v geojson=%+v", csvResult, geojsonResult)
	}
}

func TestRunCompareAndInputOutputGuard(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "occurrences.json")
	out := filepath.Join(dir, "comparison.json")
	data := `[{"id":"a","taxon":"Mimosa test","lon":-43.2,"lat":-22.9},{"id":"b","taxon":"Mimosa test","lon":-44.3,"lat":-21.5},{"id":"c","taxon":"Mimosa test","lon":-42.9,"lat":-20.8}]`
	if err := os.WriteFile(input, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"compare", "--in", input, "--out", out}); err != nil {
		t.Fatal(err)
	}
	var comparison geo.ProjectionComparison
	readJSON(t, out, &comparison)
	if comparison.SchemaVersion != "1" || comparison.LocalLAEA.Projection.Strategy != geo.ProjectionLocalLAEA || comparison.IUCNCEA.Projection.Strategy != geo.ProjectionIUCNCEA {
		t.Fatalf("unexpected comparison: %+v", comparison)
	}
	if err := run([]string{"calc", "--in", input, "--out", input}); err == nil {
		t.Fatal("accepted identical input and output path")
	}
}

func readJSON(t *testing.T, path string, dst any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, dst); err != nil {
		t.Fatal(err)
	}
}

func TestRunAuditCurateInfluenceProtocol(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join("..", "..", "testdata", "diagnostics", "zero-zero.csv")
	cols := []string{"--x-col", "decimalLongitude", "--y-col", "decimalLatitude", "--id-col", "ID"}

	auditOut := filepath.Join(dir, "audit.json")
	if err := run(append([]string{"audit", "--in", fixture, "--out", auditOut, "--source-flag-field", "issue", "--context-fields", "locality"}, cols...)); err != nil {
		t.Fatal(err)
	}
	var report geo.AuditReport
	readJSON(t, auditOut, &report)
	if report.InputRecords != 11 || report.InvalidRecords != 2 || report.CountsByCode[geo.IssueZeroZero] != 1 || report.SourceSHA256 == "" {
		t.Fatalf("%+v", report)
	}
	for _, is := range report.Issues {
		if is.Code == geo.IssueZeroZero && (is.RecordID != "r6" || *is.SourceRow != 7) {
			t.Fatalf("%+v", is)
		}
	}

	// calc is strict: the invalid rows must be curated upstream first.
	if err := run(append([]string{"calc", "--in", fixture, "--out", filepath.Join(dir, "x.json")}, cols...)); err == nil {
		t.Fatal("strict calc accepted invalid rows")
	}
	valid := filepath.Join(dir, "valid.csv")
	b, _ := os.ReadFile(fixture)
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	clean := append(lines[:9:9], lines[11])
	if err := os.WriteFile(valid, []byte(strings.Join(clean, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	calcOut, protoOut := filepath.Join(dir, "calc.json"), filepath.Join(dir, "protocol.json")
	if err := run(append([]string{"calc", "--in", valid, "--out", calcOut, "--protocol-out", protoOut}, cols...)); err != nil {
		t.Fatal(err)
	}
	var res geo.Result
	readJSON(t, calcOut, &res)

	manifest := geo.CurationManifest{ManifestVersion: "1", InputSHA256: res.InputSHA256, Decisions: []geo.CurationDecision{{InputIndex: 5, RecordID: ptr("r6"), Action: "exclude", Justification: "(0,0) placeholder; locality in Argentina"}}}
	mPath := filepath.Join(dir, "manifest.json")
	mb, _ := json.Marshal(manifest)
	if err := os.WriteFile(mPath, mb, 0o600); err != nil {
		t.Fatal(err)
	}
	derived, logOut := filepath.Join(dir, "curated.csv"), filepath.Join(dir, "log.json")
	if err := run(append([]string{"curate", "--in", valid, "--manifest", mPath, "--derived", derived, "--out", logOut}, cols...)); err != nil {
		t.Fatal(err)
	}
	var log geo.CurationLog
	readJSON(t, logOut, &log)
	curatedOut := filepath.Join(dir, "curated.json")
	if err := run(append([]string{"calc", "--in", derived, "--out", curatedOut}, cols...)); err != nil {
		t.Fatal(err)
	}
	var curated geo.Result
	readJSON(t, curatedOut, &curated)
	if curated.InputSHA256 != log.OutputSHA256 || curated.InputRecords != res.InputRecords-1 {
		t.Fatalf("derived file does not carry the logged hash: %s vs %s", curated.InputSHA256, log.OutputSHA256)
	}
	if err := run(append([]string{"curate", "--in", valid, "--manifest", mPath, "--derived", derived, "--out", logOut}, cols...)); err == nil {
		t.Fatal("curate overwrote an existing derived file")
	}

	infOut := filepath.Join(dir, "influence.json")
	if err := run(append([]string{"influence", "--in", valid, "--out", infOut}, cols...)); err != nil {
		t.Fatal(err)
	}
	var inf geo.EOOInfluence
	readJSON(t, infOut, &inf)
	zeroIsVertex := false
	for _, g := range inf.Groups {
		zeroIsVertex = zeroIsVertex || (g.RecordIDs[0] == "r6" && g.RelativeReductionPct != nil && *g.RelativeReductionPct > 10)
	}
	if !zeroIsVertex || inf.Status != "complete" {
		t.Fatalf("(0,0) not reported as an influential vertex: %+v", inf.Groups)
	}

	reproOut := filepath.Join(dir, "repro.json")
	if err := run(append([]string{"calc", "--in", derived, "--protocol", protoOut, "--out", reproOut}, cols...)); err != nil {
		t.Fatal(err)
	}
	var repro geo.Result
	readJSON(t, reproOut, &repro)
	if repro.ProtocolSHA256 != res.ProtocolSHA256 || repro.Projection.AnalysisPROJ != res.Projection.AnalysisPROJ || repro.AOO.EffectiveGridMode != "fixed" {
		t.Fatalf("protocol not frozen: %+v", repro.Projection)
	}
	if err := run(append([]string{"calc", "--in", derived, "--protocol", protoOut, "--cell-size", "1000", "--out", filepath.Join(dir, "bad.json")}, cols...)); err == nil {
		t.Fatal("conflicting cell size accepted with a protocol")
	}
}

func ptr(s string) *string { return &s }
