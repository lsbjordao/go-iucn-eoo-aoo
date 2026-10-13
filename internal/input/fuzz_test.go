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

// The permissive audit reader must agree with the strict reader whenever the
// strict reader succeeds, and must never hand out a non-finite coordinate
// without a ParseError.
func FuzzReadAuditAgreesWithRead(f *testing.F) {
	for _, seed := range []string{
		"id,taxon,lon,lat\na,Mimosa test,-43.2,-22.9\n",
		"lon,lat\n0,0\n,1\n2\n",
		"lon,lat\n\"1\",\"2\nx\"\n",
		`[{"lon":1,"lat":2},3]`,
		`{"type":"FeatureCollection","features":[{"type":"Feature","geometry":null}]}`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		for _, format := range []string{"csv", "json"} {
			recs, err := ReadAudit([]byte(s), format, Options{})
			if err != nil {
				continue
			}
			for _, r := range recs {
				if r.ParseError == "" && (math.IsNaN(r.X) || math.IsInf(r.X, 0) || math.IsNaN(r.Y) || math.IsInf(r.Y, 0)) {
					t.Fatalf("record without ParseError has non-finite coordinate: %+v", r)
				}
			}
			d, err := Read([]byte(s), format, Options{})
			if err != nil {
				continue
			}
			if len(d.Points) != len(recs) {
				t.Fatalf("%s: strict %d records, permissive %d", format, len(d.Points), len(recs))
			}
			for i := range recs {
				if recs[i].ParseError != "" || recs[i].X != d.Points[i].X || recs[i].Y != d.Points[i].Y || recs[i].ID != d.Points[i].ID {
					t.Fatalf("%s record %d: strict %+v permissive %+v", format, i, d.Points[i], recs[i])
				}
			}
		}
	})
}
