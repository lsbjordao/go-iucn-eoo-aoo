// Package input reads strict coordinate tables and WGS84 point GeoJSON.
package input

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	geo "github.com/lsbjordao/go-iucn-eoo-aoo"
	"io"
	"math"
	"strconv"
	"strings"
)

type Options struct{ XColumn, YColumn, TaxonColumn string }
type Data struct {
	Points  []geo.Point
	GeoJSON bool
}

func number(v any) (float64, error) {
	var s string
	switch n := v.(type) {
	case json.Number:
		s = string(n)
	case string:
		s = strings.TrimSpace(n)
	case float64:
		return n, nil
	default:
		return 0, fmt.Errorf("missing or non-numeric coordinate")
	}
	f, e := strconv.ParseFloat(s, 64)
	if e != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("invalid coordinate %q", s)
	}
	return f, nil
}
func value(m map[string]any, explicit string, aliases ...string) any {
	if explicit != "" {
		return m[explicit]
	}
	for _, a := range aliases {
		if v, ok := m[a]; ok {
			return v
		}
	}
	return nil
}
func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func row(m map[string]any, o Options) (geo.Point, error) {
	x, e := number(value(m, o.XColumn, "lon", "x", "decimalLongitude"))
	if e != nil {
		return geo.Point{}, fmt.Errorf("lon/X: %w", e)
	}
	y, e := number(value(m, o.YColumn, "lat", "y", "decimalLatitude"))
	if e != nil {
		return geo.Point{}, fmt.Errorf("lat/Y: %w", e)
	}
	taxon := str(value(m, o.TaxonColumn, "taxon", "scientificName", "species"))
	return geo.Point{X: x, Y: y, ID: str(value(m, "", "id", "occurrenceID")), Taxon: strings.TrimSpace(taxon), Properties: m}, nil
}

func Read(data []byte, format string, o Options) (Data, error) {
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	if len(bytes.TrimSpace(data)) == 0 {
		return Data{}, fmt.Errorf("empty input")
	}
	if format == "csv" || format == "tsv" {
		return readCSV(data, format, o)
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var v any
	if e := d.Decode(&v); e != nil {
		return Data{}, e
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return Data{}, fmt.Errorf("input must contain exactly one JSON value")
	}
	out := Data{}
	if arr, ok := v.([]any); ok {
		for i, item := range arr {
			m, ok := item.(map[string]any)
			if !ok {
				return out, fmt.Errorf("record %d is not an object", i+1)
			}
			p, e := row(m, o)
			if e != nil {
				return out, fmt.Errorf("record %d: %w", i+1, e)
			}
			out.Points = append(out.Points, p)
		}
	} else if obj, ok := v.(map[string]any); ok {
		if obj["type"] != "FeatureCollection" {
			return out, fmt.Errorf("expected an array of records or a GeoJSON FeatureCollection")
		}
		if _, ok := obj["crs"]; ok {
			return out, fmt.Errorf("GeoJSON crs member is unsupported: convert to RFC 7946 WGS84 first")
		}
		out.GeoJSON = true
		features, ok := obj["features"].([]any)
		if !ok {
			return out, fmt.Errorf("missing GeoJSON features")
		}
		for i, item := range features {
			f, ok := item.(map[string]any)
			if !ok || f["type"] != "Feature" {
				return out, fmt.Errorf("feature %d: expected Feature", i+1)
			}
			g, ok := f["geometry"].(map[string]any)
			if !ok || g["type"] != "Point" {
				return out, fmt.Errorf("feature %d: only non-null Point geometry is supported", i+1)
			}
			c, ok := g["coordinates"].([]any)
			if !ok || len(c) < 2 {
				return out, fmt.Errorf("feature %d: coordinates required", i+1)
			}
			x, e := number(c[0])
			if e != nil {
				return out, e
			}
			y, e := number(c[1])
			if e != nil {
				return out, e
			}
			props, _ := f["properties"].(map[string]any)
			p := geo.Point{X: x, Y: y, ID: str(f["id"]), Taxon: strings.TrimSpace(str(value(props, o.TaxonColumn, "taxon", "scientificName", "species"))), Properties: props}
			if p.ID == "" {
				p.ID = str(value(props, "", "id", "occurrenceID"))
			}
			out.Points = append(out.Points, p)
		}
	} else {
		return out, fmt.Errorf("expected JSON array or GeoJSON FeatureCollection")
	}
	if len(out.Points) == 0 || len(out.Points) > geo.MaxPoints {
		return out, fmt.Errorf("expected 1..%d records", geo.MaxPoints)
	}
	return out, nil
}

func readCSV(data []byte, format string, o Options) (Data, error) {
	r := csv.NewReader(bytes.NewReader(data))
	if format == "tsv" {
		r.Comma = '\t'
	}
	h, e := r.Read()
	if e != nil {
		return Data{}, e
	}
	seen := map[string]bool{}
	for i := range h {
		h[i] = strings.TrimSpace(h[i])
		if h[i] == "" || seen[h[i]] {
			return Data{}, fmt.Errorf("CSV headers must be nonempty and unique")
		}
		seen[h[i]] = true
	}
	out := Data{}
	for line := 2; ; line++ {
		v, e := r.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return out, fmt.Errorf("CSV line %d: %w", line, e)
		}
		m := map[string]any{}
		for i, k := range h {
			m[k] = v[i]
		}
		p, e := row(m, o)
		if e != nil {
			return out, fmt.Errorf("CSV line %d: %w", line, e)
		}
		out.Points = append(out.Points, p)
		if len(out.Points) > geo.MaxPoints {
			return out, fmt.Errorf("input exceeds %d records", geo.MaxPoints)
		}
	}
	if len(out.Points) == 0 {
		return out, fmt.Errorf("CSV has no records")
	}
	return out, nil
}
