package eooaoo

import (
	"fmt"
	"github.com/airbusgeo/godal"
	"os"
	"path/filepath"
	"sync"
)

var registerDrivers sync.Once

// WriteGeoPackage stores exact analysis-plane geometries with their CRS.
// It refuses to overwrite an existing destination.
func WriteGeoPackage(path string, r Result) (err error) {
	if len(r.Points) == 0 || len(r.Cells) == 0 {
		return fmt.Errorf("export needs a full result, not Summary()")
	}
	if _, e := os.Lstat(path); !os.IsNotExist(e) {
		return fmt.Errorf("output already exists or is inaccessible: %s", path)
	}
	registerDrivers.Do(godal.RegisterAll)
	sr, e := godal.NewSpatialRef(r.Projection.AnalysisWKT)
	if e != nil {
		return e
	}
	defer sr.Close()
	ds, e := godal.CreateVector(godal.DriverName("GPKG"), path)
	if e != nil {
		return e
	}
	defer func() {
		ce := ds.Close()
		if err == nil {
			err = ce
		}
		if err != nil {
			_ = os.Remove(path)
		}
	}()
	points, e := ds.CreateLayer("occurrences", sr, godal.GTPoint,
		godal.NewFieldDefinition("input_indices", godal.FTString), godal.NewFieldDefinition("records", godal.FTInt), godal.NewFieldDefinition("taxon", godal.FTString))
	if e != nil {
		return e
	}
	cells, e := ds.CreateLayer("aoo", sr, godal.GTPolygon,
		godal.NewFieldDefinition("cell_col", godal.FTInt64), godal.NewFieldDefinition("cell_row", godal.FTInt64), godal.NewFieldDefinition("points", godal.FTInt), godal.NewFieldDefinition("area_km2", godal.FTReal))
	if e != nil {
		return e
	}
	hull, e := ds.CreateLayer("eoo", sr, godal.GTPolygon, godal.NewFieldDefinition("raw_km2", godal.FTReal), godal.NewFieldDefinition("assessment_km2", godal.FTReal))
	if e != nil {
		return e
	}
	if e = ds.StartTransaction(); e != nil {
		return e
	}
	for _, p := range r.Points {
		if e = addFeature(points, sr, fmt.Sprintf("POINT (%.17g %.17g)", p.X, p.Y), map[string]any{"input_indices": fmt.Sprint(p.InputIndices), "records": len(p.Records), "taxon": r.Taxon}); e != nil {
			return e
		}
	}
	for _, c := range r.Cells {
		b := c.Bounds
		wkt := polygonWKT([]xy{{b[0], b[1]}, {b[2], b[1]}, {b[2], b[3]}, {b[0], b[3]}})
		if e = addFeature(cells, sr, wkt, map[string]any{"cell_col": c.Column, "cell_row": c.Row, "points": c.UniquePoints, "area_km2": r.AOO.CellSizeM * r.AOO.CellSizeM / 1e6}); e != nil {
			return e
		}
	}
	if r.EOO.HullWKT != "" {
		if e = addFeature(hull, sr, r.EOO.HullWKT, map[string]any{"raw_km2": *r.EOO.RawAreaKM2, "assessment_km2": r.EOO.AssessmentAreaKM2}); e != nil {
			return e
		}
	}
	return ds.CommitTransaction()
}

func addFeature(layer godal.Layer, sr *godal.SpatialRef, wkt string, values map[string]any) error {
	g, e := godal.NewGeometryFromWKT(wkt, sr)
	if e != nil {
		return e
	}
	defer g.Close()
	f, e := layer.NewFeature(nil)
	if e != nil {
		return e
	}
	defer f.Close()
	if e = f.SetGeometry(g); e != nil {
		return e
	}
	fields := f.Fields()
	for name, v := range values {
		if e = f.SetFieldValue(fields[name], v); e != nil {
			return e
		}
	}
	return layer.CreateFeature(f)
}

// ExportGeoJSON uses GDAL RFC7946 conversion (including antimeridian splitting).
// Splitting requires GDAL 3.8 or newer; earlier versions emit a single ring that
// spans the antimeridian instead of a MultiPolygon.
// Segmentization at 250 m is for display; authoritative areas are in the report
// and GeoPackage, never recomputed from the densified WGS84 display geometries.
func ExportGeoJSON(gpkg, dir string) error {
	registerDrivers.Do(godal.RegisterAll)
	if _, e := os.Lstat(dir); !os.IsNotExist(e) {
		return fmt.Errorf("GeoJSON output directory already exists or is inaccessible")
	}
	if e := os.MkdirAll(dir, 0755); e != nil {
		return e
	}
	ds, e := godal.Open(gpkg, godal.VectorOnly())
	if e != nil {
		return e
	}
	defer ds.Close()
	for _, layer := range []string{"occurrences", "aoo", "eoo"} {
		d, e := ds.VectorTranslate(filepath.Join(dir, layer+".geojson"), []string{"-f", "GeoJSON", layer, "-t_srs", "EPSG:4326", "-segmentize", "250", "-lco", "RFC7946=YES", "-lco", "COORDINATE_PRECISION=9"})
		if e != nil {
			return fmt.Errorf("export %s: %w", layer, e)
		}
		if e = d.Close(); e != nil {
			return e
		}
	}
	return nil
}
