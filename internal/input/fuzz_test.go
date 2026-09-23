package input

import (
	"math"
	"testing"

	geo "github.com/lsbjordao/go-iucn-eoo-aoo"
)

func FuzzReadJSON(f *testing.F) {
	for _, seed := range []string{
		`[{"id":"a","taxon":"Mimosa test","lon":-43.2,"lat":-22.9}]`,
		`{"type":"FeatureCollection","features":[{"type":"Feature","id":"a","properties":{"taxon":"Mimosa test"},"geometry":{"type":"Point","coordinates":[-43.2,-22.9]}}]}`,
		`[]`,
		`{"type":"FeatureCollection","features":[]}`,
		`[{"lon":null,"lat":0}]`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		d, err := Read([]byte(s), "json", Options{})
		if err != nil {
			return
		}
		if len(d.Points) < 1 || len(d.Points) > geo.MaxPoints {
			t.Fatalf("successful parse returned %d points", len(d.Points))
		}
		for _, p := range d.Points {
			if math.IsNaN(p.X) || math.IsInf(p.X, 0) || math.IsNaN(p.Y) || math.IsInf(p.Y, 0) {
				t.Fatalf("successful parse returned non-finite coordinate: %+v", p)
			}
		}
	})
}

func FuzzReadCSV(f *testing.F) {
	for _, seed := range []string{
		"id,taxon,lon,lat\na,Mimosa test,-43.2,-22.9\n",
		"lon,lat\n0,0\n",
		"lon,lat\n,0\n",
		"lon,lon,lat\n0,0,0\n",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		d, err := Read([]byte(s), "csv", Options{})
		if err != nil {
			return
		}
		if len(d.Points) < 1 || len(d.Points) > geo.MaxPoints {
			t.Fatalf("successful parse returned %d points", len(d.Points))
		}
	})
}
