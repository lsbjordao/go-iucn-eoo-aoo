package input

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"

	geo "github.com/lsbjordao/go-iucn-eoo-aoo"
)

// ReadAudit parses like Read but keeps going past bad records, returning each
// one with its ParseError so audit can report it. A ParseError starting with
// "record:" is a malformed record; any other is a coordinate problem. Only
// unreadable files (bad JSON, bad CSV header) fail as a whole.
func ReadAudit(data []byte, format string, o Options) ([]geo.AuditRecord, error) {
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("empty input")
	}
	var out []geo.AuditRecord
	bad := func(p geo.Point, row int, err string) {
		p.X, p.Y = math.NaN(), math.NaN()
		out = append(out, geo.AuditRecord{Point: p, SourceRow: row, ParseError: err})
	}
	fromMap := func(m map[string]any, row int) {
		p, e := row2(m, o)
		if e != nil {
			bad(p, row, e.Error())
			return
		}
		out = append(out, geo.AuditRecord{Point: p, SourceRow: row})
	}
	if format == "csv" || format == "tsv" {
		r := csv.NewReader(bytes.NewReader(data))
		r.FieldsPerRecord = -1
		if format == "tsv" {
			r.Comma = '\t'
		}
		h, e := r.Read()
		if e != nil {
			return nil, e
		}
		seen := map[string]bool{}
		for i := range h {
			h[i] = strings.TrimSpace(h[i])
			if h[i] == "" || seen[h[i]] {
				return nil, fmt.Errorf("CSV headers must be nonempty and unique")
			}
			seen[h[i]] = true
		}
		for {
			v, e := r.Read()
			if e == io.EOF {
				break
			}
			if e != nil {
				// With FieldsPerRecord=-1 only syntax errors remain; they
				// make the rest of the file unreadable.
				return nil, fmt.Errorf("CSV: %w", e)
			}
			line, _ := r.FieldPos(0)
			m := map[string]any{}
			for i, k := range h {
				if i < len(v) {
					m[k] = v[i]
				}
			}
			if len(v) != len(h) {
				p, _ := row2(m, o)
				bad(p, line, fmt.Sprintf("record: expected %d fields, found %d", len(h), len(v)))
			} else {
				fromMap(m, line)
			}
			if len(out) > geo.MaxPoints {
				return nil, fmt.Errorf("input exceeds %d records", geo.MaxPoints)
			}
		}
	} else {
		d := json.NewDecoder(bytes.NewReader(data))
		d.UseNumber()
		var v any
		if e := d.Decode(&v); e != nil {
			return nil, e
		}
		var extra any
		if e := d.Decode(&extra); e != io.EOF {
			return nil, fmt.Errorf("input must contain exactly one JSON value")
		}
		if arr, ok := v.([]any); ok {
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					fromMap(m, 0)
				} else {
					bad(geo.Point{}, 0, "record: not a JSON object")
				}
			}
		} else if obj, ok := v.(map[string]any); ok && obj["type"] == "FeatureCollection" {
			if _, ok := obj["crs"]; ok {
				return nil, fmt.Errorf("GeoJSON crs member is unsupported: convert to RFC 7946 WGS84 first")
			}
			features, ok := obj["features"].([]any)
			if !ok {
				return nil, fmt.Errorf("missing GeoJSON features")
			}
			for _, item := range features {
				f, _ := item.(map[string]any)
				props, _ := f["properties"].(map[string]any)
				p := geo.Point{ID: str(f["id"]), Taxon: strings.TrimSpace(str(value(props, o.TaxonColumn, "taxon", "scientificName", "species"))), Properties: props}
				if p.ID == "" {
					p.ID = str(value(props, o.IDColumn, "id", "occurrenceID"))
				}
				g, _ := f["geometry"].(map[string]any)
				c, _ := g["coordinates"].([]any)
				switch {
				case f == nil || f["type"] != "Feature":
					bad(p, 0, "record: expected Feature")
				case g == nil || g["type"] != "Point":
					bad(p, 0, "record: only non-null Point geometry is supported")
				case len(c) < 2:
					bad(p, 0, "coordinates required")
				default:
					x, e1 := number(c[0])
					y, e2 := number(c[1])
					if e1 != nil || e2 != nil {
						bad(p, 0, "invalid coordinate")
						continue
					}
					p.X, p.Y = x, y
					out = append(out, geo.AuditRecord{Point: p})
				}
			}
		} else {
			return nil, fmt.Errorf("expected JSON array or GeoJSON FeatureCollection")
		}
	}
	if len(out) == 0 || len(out) > geo.MaxPoints {
		return nil, fmt.Errorf("expected 1..%d records", geo.MaxPoints)
	}
	return out, nil
}

// row2 is row but also returns the partially filled point on error, so the
// record keeps its ID and properties in the audit report.
func row2(m map[string]any, o Options) (geo.Point, error) {
	p, e := row(m, o)
	if e != nil {
		p = geo.Point{ID: str(value(m, o.IDColumn, "id", "occurrenceID")), Taxon: strings.TrimSpace(str(value(m, o.TaxonColumn, "taxon", "scientificName", "species"))), Properties: m}
	}
	return p, e
}

// Write serializes points in the layout they were read from: CSV/TSV with the
// original column order, a JSON record array, or an RFC 7946 FeatureCollection.
// Reading the output back with the same options yields identical points, so
// derived files keep the hashes recorded in curation logs.
func Write(points []geo.Point, d Data, format string) ([]byte, error) {
	var b bytes.Buffer
	switch {
	case format == "csv" || format == "tsv":
		if len(d.Header) == 0 {
			return nil, fmt.Errorf("CSV output needs the original header")
		}
		w := csv.NewWriter(&b)
		if format == "tsv" {
			w.Comma = '\t'
		}
		_ = w.Write(d.Header)
		for i, p := range points {
			rec := make([]string, len(d.Header))
			for k, h := range d.Header {
				v, ok := p.Properties[h]
				if !ok {
					return nil, fmt.Errorf("record %d has no %q property; it was not read from this CSV", i, h)
				}
				rec[k] = fmt.Sprint(v)
			}
			_ = w.Write(rec)
		}
		w.Flush()
		return b.Bytes(), w.Error()
	case d.GeoJSON:
		type feature struct {
			Type       string         `json:"type"`
			ID         string         `json:"id,omitempty"`
			Properties map[string]any `json:"properties"`
			Geometry   map[string]any `json:"geometry"`
		}
		fc := struct {
			Type     string    `json:"type"`
			Features []feature `json:"features"`
		}{Type: "FeatureCollection", Features: []feature{}}
		for _, p := range points {
			fc.Features = append(fc.Features, feature{"Feature", p.ID, p.Properties, map[string]any{"type": "Point", "coordinates": []float64{p.X, p.Y}}})
		}
		e := json.NewEncoder(&b)
		e.SetIndent("", "  ")
		if err := e.Encode(fc); err != nil {
			return nil, err
		}
		return b.Bytes(), nil
	default:
		arr := make([]map[string]any, 0, len(points))
		for i, p := range points {
			if p.Properties == nil {
				return nil, fmt.Errorf("record %d has no source properties", i)
			}
			arr = append(arr, p.Properties)
		}
		e := json.NewEncoder(&b)
		e.SetIndent("", "  ")
		if err := e.Encode(arr); err != nil {
			return nil, err
		}
		return b.Bytes(), nil
	}
}
