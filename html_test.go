package eooaoo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteMapHTML(t *testing.T) {
	dir := t.TempDir()
	geojsonDir := filepath.Join(dir, "layers")
	if err := os.Mkdir(geojsonDir, 0755); err != nil {
		t.Fatal(err)
	}
	fc := `{"type":"FeatureCollection","features":[]}`
	fc = strings.ReplaceAll(fc, `\"`, `"`)
	for _, name := range []string{"occurrences", "aoo", "eoo"} {
		if err := os.WriteFile(filepath.Join(geojsonDir, name+".geojson"), []byte(fc), 0644); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, "map.html")
	r := Result{
		SoftwareVersion:   "test",
		GDALVersion:       "3.x",
		PROJVersion:       "9.x",
		Taxon:             "Example species",
		InputSHA256:       "abc",
		InputRecords:      3,
		UniqueCoordinates: 3,
		Projection:        Projection{Strategy: ProjectionLocalLAEA, Reference: "test projection"},
		EOO:               EOOResult{AssessmentAreaKM2: 12},
		AOO:               AOOResult{AreaKM2: 8, OccupiedCells: 2, CellSizeM: 2000, GridMode: "exact"},
	}
	if err := WriteMapHTML(path, geojsonDir, r); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"maplibre-gl@6.10.0", "Example species", "occurrences", "aoo-fill", "eoo-fill"} {
		if !strings.Contains(s, want) {
			t.Fatalf("HTML missing %q", want)
		}
	}
	if err := WriteMapHTML(path, geojsonDir, r); err == nil {
		t.Fatal("expected overwrite refusal")
	}
}
