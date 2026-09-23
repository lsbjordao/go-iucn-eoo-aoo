package input

import "testing"

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
