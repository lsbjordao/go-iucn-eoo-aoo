package eooaoo

import (
	"encoding/json"
	"github.com/airbusgeo/godal"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestGeoPackageAndDatelineGeoJSON(t *testing.T) {
	r := mustCalc(t, []Point{{X: 179.9, Y: 10}, {X: -179.9, Y: 10}, {X: -179.9, Y: 10.1}, {X: 179.9, Y: 10.1}}, Config{})
	dir := t.TempDir()
	path := filepath.Join(dir, "result.gpkg")
	if e := WriteGeoPackage(path, r); e != nil {
		t.Fatal(e)
	}
	if e := WriteGeoPackage(path, r); e == nil {
		t.Fatal("existing GPKG overwritten")
	}
	ds, e := godal.Open(path, godal.VectorOnly())
	if e != nil {
		t.Fatal(e)
	}
	for _, layer := range ds.Layers() {
		count, e := layer.FeatureCount()
		if e != nil {
			t.Fatal(e)
		}
		switch layer.Name() {
		case "occurrences":
			if count != 4 {
				t.Fatal("wrong point count", count)
			}
		case "aoo":
			if count != r.AOO.OccupiedCells {
				t.Fatal("wrong cell count", count)
			}
			for f := layer.NextFeature(); f != nil; f = layer.NextFeature() {
				g := f.Geometry()
				if math.Abs(g.Area()-4e6) > 0.001 {
					t.Fatal("cell area is not 4 km²")
				}
				g.Close()
				f.Close()
			}
		case "eoo":
			if count != 1 {
				t.Fatal("wrong hull count", count)
			}
		}
	}
	if e = ds.Close(); e != nil {
		t.Fatal(e)
	}
	if e = ExportGeoJSON(path, filepath.Join(dir, "maps")); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"eoo", "aoo", "occurrences"} {
		b, e := os.ReadFile(filepath.Join(dir, "maps", name+".geojson"))
		if e != nil {
			t.Fatal(e)
		}
		var fc struct {
			Type     string `json:"type"`
			Features []struct {
				Geometry struct {
					Type        string          `json:"type"`
					Coordinates json.RawMessage `json:"coordinates"`
				} `json:"geometry"`
			} `json:"features"`
		}
		if e = json.Unmarshal(b, &fc); e != nil {
			t.Fatal(e)
		}
		if fc.Type != "FeatureCollection" {
			t.Fatal("not GeoJSON")
		}
		for _, f := range fc.Features {
			var polys [][][][]float64
			if f.Geometry.Type == "Polygon" {
				var p [][][]float64
				if e = json.Unmarshal(f.Geometry.Coordinates, &p); e != nil {
					t.Fatal(e)
				}
				polys = append(polys, p)
			} else if f.Geometry.Type == "MultiPolygon" {
				if e = json.Unmarshal(f.Geometry.Coordinates, &polys); e != nil {
					t.Fatal(e)
				}
			}
			for _, p := range polys {
				for _, ring := range p {
					for i, c := range ring {
						if math.Abs(c[0]) > 180 || math.Abs(c[1]) > 90 {
							t.Fatal("invalid WGS84 coordinate")
						}
						if i > 0 && math.Abs(c[0]-ring[i-1][0]) > 180 {
							t.Fatal("unsplit antimeridian edge")
						}
					}
				}
			}
		}
	}
}
