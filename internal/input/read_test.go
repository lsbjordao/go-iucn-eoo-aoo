package input

import (
	"testing"

	geo "github.com/lsbjordao/go-iucn-eoo-aoo"
)

func TestReadFormats(t *testing.T) {
	for _, tc := range []struct{ format, s string }{
		{"csv", "id,taxon,lon,lat\nocc-1,Mimosa test,-43.2,-22.9\n"},
		{"json", "[{\"id\":\"occ-1\",\"decimalLongitude\":\"-43.2\",\"decimalLatitude\":-22.9,\"scientificName\":\"Mimosa test\"}]"},
		{"tsv", "id\tx\ty\ttaxon\nocc-1\t-43.2\t-22.9\tMimosa test\n"},
		{"json", `{"type":"FeatureCollection","features":[{"type":"Feature","id":"occ-1","properties":{"taxon":"Mimosa test"},"geometry":{"type":"Point","coordinates":[-43.2,-22.9]}}]}`},
	} {
		d, e := Read([]byte(tc.s), tc.format, Options{})
		if e != nil {
			t.Fatal(e)
		}
		if len(d.Points) != 1 || d.Points[0].X != -43.2 || d.Points[0].Taxon != "Mimosa test" || d.Points[0].ID != "occ-1" {
			t.Fatal(d)
		}
	}
}

func TestRejectMissingNullAndMalformed(t *testing.T) {
	for _, s := range []string{`[{"x":null,"y":1}]`, `[{"x":"","y":1}]`, `[{"y":0}]`, `[{"x":"NaN","y":1}]`, `[]`, `[{"x":1,"y":2}] {}`, `{"type":"FeatureCollection","features":[{"type":"Feature","geometry":null}]}`} {
		if _, e := Read([]byte(s), "json", Options{}); e == nil {
			t.Fatalf("accepted %s", s)
		}
	}
	for _, s := range []string{"x,y\n,1\n", "x,x,y\n1,1,1\n", "x,y\n1,2,3\n"} {
		if _, e := Read([]byte(s), "csv", Options{}); e == nil {
			t.Fatalf("accepted %s", s)
		}
	}
	d, e := Read([]byte(`[{"x":0,"y":0}]`), "json", Options{})
	if e != nil || len(d.Points) != 1 {
		t.Fatal("valid zero coordinate rejected")
	}
}

func TestReadAuditKeepsBadRows(t *testing.T) {
	s := "ID,decimalLongitude,decimalLatitude,locality\n" +
		"a,-43.2,-22.9,ok\n" +
		"b,,-22.9,missing lon\n" +
		"c,-43.2\n" +
		"\"d\",\"-44\",\"-21\",\"multi\nline\"\n" +
		"e,0,0,zero\n"
	recs, e := ReadAudit([]byte(s), "csv", Options{IDColumn: "ID"})
	if e != nil {
		t.Fatal(e)
	}
	if len(recs) != 5 {
		t.Fatalf("%+v", recs)
	}
	want := []struct {
		id, errPrefix string
		row           int
	}{{"a", "", 2}, {"b", "lon/X", 3}, {"c", "record:", 4}, {"d", "", 5}, {"e", "", 7}}
	for i, w := range want {
		r := recs[i]
		if r.ID != w.id || r.SourceRow != w.row || (w.errPrefix == "") != (r.ParseError == "") || (w.errPrefix != "" && r.ParseError[:len(w.errPrefix)] != w.errPrefix) {
			t.Fatalf("record %d: %+v", i, r)
		}
	}
	if recs[1].Properties["locality"] != "missing lon" {
		t.Fatal("bad row lost its properties")
	}
	if _, e := ReadAudit([]byte(`[1,{"lon":1,"lat":2}]`), "json", Options{}); e != nil {
		t.Fatal(e)
	}
}

// Derived files must read back to the same points, hence the same hash.
func TestWriteRoundTrip(t *testing.T) {
	for _, tc := range []struct{ format, s string }{
		{"csv", "occurrenceID,scientificName,decimalLongitude,decimalLatitude,note\n1,A b,-43.2,-22.9,\"x, y\"\n2,A b,-44.000001,-21.5,\n"},
		{"tsv", "id\tlon\tlat\n1\t-43.2\t-22.9\n"},
		{"json", `[{"id":"1","lon":-43.2,"lat":-22.9,"extra":{"k":[1,2]}},{"id":"2","lon":"-44","lat":"-21"}]`},
		{"json", `{"type":"FeatureCollection","features":[{"type":"Feature","id":7,"properties":{"taxon":"A b"},"geometry":{"type":"Point","coordinates":[-43.2,-22.9]}},{"type":"Feature","properties":null,"geometry":{"type":"Point","coordinates":[0.1,0.2]}}]}`},
	} {
		d, e := Read([]byte(tc.s), tc.format, Options{})
		if e != nil {
			t.Fatal(e)
		}
		b, e := Write(d.Points, d, tc.format)
		if e != nil {
			t.Fatal(e)
		}
		d2, e := Read(b, tc.format, Options{})
		if e != nil {
			t.Fatalf("%s: %v\n%s", tc.format, e, b)
		}
		h1, _ := geo.InputHash(d.Points)
		h2, _ := geo.InputHash(d2.Points)
		if h1 != h2 {
			t.Fatalf("%s: round trip changed points\n%s", tc.format, b)
		}
	}
}
