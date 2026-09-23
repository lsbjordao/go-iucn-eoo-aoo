package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
