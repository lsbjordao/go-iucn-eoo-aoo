package eooaoo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/airbusgeo/godal"
	"math"
	"sort"
	"strings"
)

func normalizeConfig(c Config) (Config, error) {
	if c.InputCRS == "" {
		c.InputCRS = "EPSG:4326"
	}
	if c.ProjectionStrategy == "" {
		c.ProjectionStrategy = ProjectionLocalLAEA
	}
	if c.ProjectionStrategy != ProjectionLocalLAEA && c.ProjectionStrategy != ProjectionIUCNCEA {
		return c, fmt.Errorf("projection_strategy must be %s or %s", ProjectionLocalLAEA, ProjectionIUCNCEA)
	}
	if c.CellSizeM == 0 {
		c.CellSizeM = 2000
	}
	if c.GridMode == "" {
		c.GridMode = "auto"
	}
	if c.GridSteps == 0 {
		c.GridSteps = 10
	}
	if !finite(c.CellSizeM) || c.CellSizeM < 1 || c.CellSizeM > 100000 {
		return c, fmt.Errorf("cell_size_m must be between 1 and 100000 metres")
	}
	if c.GridSteps < 1 || c.GridSteps > 100 {
		return c, fmt.Errorf("grid_steps must be between 1 and 100")
	}
	if c.GridMode != "auto" && c.GridMode != "exact" && c.GridMode != "sampled" && c.GridMode != "fixed" {
		return c, fmt.Errorf("grid_mode must be auto, exact, sampled, or fixed")
	}
	for _, v := range c.Origin {
		if !finite(v) || math.Abs(v) > 1e9 {
			return c, fmt.Errorf("grid origin must be finite and within +/- 1e9 metres")
		}
	}
	if c.Center != nil {
		if c.ProjectionStrategy != ProjectionLocalLAEA {
			return c, fmt.Errorf("center is only valid with projection_strategy=%s", ProjectionLocalLAEA)
		}
		v := *c.Center
		if !finite(v[0]) || !finite(v[1]) || v[0] < -180 || v[0] > 180 || v[1] < -90 || v[1] > 90 {
			return c, fmt.Errorf("center must be WGS84 [longitude,latitude]")
		}
		c.Center = &v
	}
	// Only CRS definitions are accepted, never filenames or remote references.
	if len(c.InputCRS) > 16384 || !(strings.HasPrefix(c.InputCRS, "EPSG:") || strings.HasPrefix(c.InputCRS, "+proj=") || strings.HasPrefix(c.InputCRS, "GEOGCS[") || strings.HasPrefix(c.InputCRS, "PROJCS[") || strings.HasPrefix(c.InputCRS, "GEOGCRS[") || strings.HasPrefix(c.InputCRS, "PROJCRS[")) {
		return c, fmt.Errorf("input_crs must be an EPSG code, PROJ definition, or WKT CRS")
	}
	return c, nil
}

// Calculate accepts exactly one assessment unit (taxon). Context cancellation
// is checked between transforms and inside the grid search. All input is strict:
// no silent dropping, imputing missing coordinates to zero, or auto-swapping axes.
func Calculate(ctx context.Context, points []Point, config Config) (Result, error) {
	r := Result{SchemaVersion: "1", SoftwareVersion: Version, GDALVersion: GDALVersion(), PROJVersion: PROJVersion(), Warnings: []string{}}
	c, err := normalizeConfig(config)
	if err != nil {
		return r, err
	}
	r.Config = c
	if err = ctx.Err(); err != nil {
		return r, err
	}
	if len(points) == 0 || len(points) > MaxPoints {
		return r, fmt.Errorf("expected 1..%d points", MaxPoints)
	}
	r.InputRecords = len(points)
	x, y := make([]float64, len(points)), make([]float64, len(points))
	for i, p := range points {
		if err = checkContext(ctx, i); err != nil {
			return r, err
		}
		if !finite(p.X) || !finite(p.Y) {
			return r, fmt.Errorf("point %d: non-finite coordinates", i)
		}
		if p.Taxon != "" {
			if r.Taxon != "" && r.Taxon != p.Taxon {
				return r, fmt.Errorf("mixed taxa: use batch mode to split assessment units")
			}
			r.Taxon = p.Taxon
		}
		x[i], y[i] = p.X, p.Y
	}
	if r.Taxon != "" {
		for i, p := range points {
			if p.Taxon == "" {
				return r, fmt.Errorf("point %d has no taxon while other records do; curate labels before calculation", i)
			}
		}
	}
	b, err := json.Marshal(points)
	if err != nil {
		return r, fmt.Errorf("input metadata must be JSON-compatible: %w", err)
	}
	sum := sha256.Sum256(b)
	r.InputSHA256 = hex.EncodeToString(sum[:])
	src, err := godal.NewSpatialRef(c.InputCRS)
	if err != nil {
		return r, fmt.Errorf("input CRS: %w", err)
	}
	defer src.Close()
	if err = src.Validate(); err != nil {
		return r, fmt.Errorf("input CRS validation: %w", err)
	}
	wgs, err := godal.NewSpatialRefFromEPSG(4326)
	if err != nil {
		return r, err
	}
	defer wgs.Close()
	// Reject geographic out-of-domain values before PROJ can wrap longitudes.
	if src.Geographic() {
		for i := range x {
			if x[i] < -180 || x[i] > 180 || y[i] < -90 || y[i] > 90 {
				return r, fmt.Errorf("point %d: geographic coordinates out of range", i)
			}
		}
	}
	if err = transform(src, wgs, x, y); err != nil {
		return r, fmt.Errorf("input to WGS84: %w", err)
	}
	unique := map[[2]float64]int{}
	for i := range x {
		if x[i] < -180 || x[i] > 180 || y[i] < -90 || y[i] > 90 {
			return r, fmt.Errorf("point %d: transformed WGS84 coordinate out of range", i)
		}
		if x[i] == 180 {
			x[i] = -180
		}
		if math.Abs(y[i]) == 90 {
			x[i] = 0
		}
		key := [2]float64{x[i], y[i]}
		if j, ok := unique[key]; ok {
			r.Points[j].InputIndices = append(r.Points[j].InputIndices, i)
			r.Points[j].Records = append(r.Points[j].Records, points[i])
		} else {
			unique[key] = len(r.Points)
			r.Points = append(r.Points, UsedPoint{Longitude: x[i], Latitude: y[i], InputIndices: []int{i}, Records: []Point{points[i]}})
		}
	}
	sort.Slice(r.Points, func(i, j int) bool {
		a, b := r.Points[i], r.Points[j]
		if a.Longitude != b.Longitude {
			return a.Longitude < b.Longitude
		}
		return a.Latitude < b.Latitude
	})
	r.UniqueCoordinates = len(r.Points)
	r.DuplicateRecords = len(points) - len(r.Points)

	center := [2]float64{}
	centerMethod := "fixed projection origin at 0,0"
	maxDistance := 0.0
	if c.ProjectionStrategy == ProjectionLocalLAEA {
		center, err = sphericalCenter(r.Points)
		if err != nil && c.Center == nil {
			return r, err
		}
		centerMethod = "spherical mean of sorted unique WGS84 coordinates"
		if c.Center != nil {
			center = *c.Center
			centerMethod = "explicit fixed center"
		}
		for _, p := range r.Points {
			d := angularDistance(p, center)
			if d > maxDistance {
				maxDistance = d
			}
		}
		if maxDistance > 80 {
			return r, fmt.Errorf("point is %.2f degrees from LAEA center; local-laea supports at most 80 degrees (use iucn-cea or another method)", maxDistance)
		}
		if maxDistance > 30 {
			r.Warnings = append(r.Warnings, "WIDE_EXTENT: equal-area preserves area, not shape; convex hull and grid shape are projection-dependent")
		}
	} else {
		centerMethod = "World Cylindrical Equal Area fixed origin and standard parallel"
		if len(r.Points) > 1 && r.Points[len(r.Points)-1].Longitude-r.Points[0].Longitude > 180 {
			r.Warnings = append(r.Warnings, "ANTIMERIDIAN_CEA: World Cylindrical Equal Area centered on Greenwich may produce a world-spanning planar hull for antimeridian distributions; compare with local-laea")
		}
		r.Warnings = append(r.Warnings, "IUCN_CEA_STRATEGY: uses World Cylindrical Equal Area (ESRI:54034), matching the projection documented by the IUCN EOO Calculator; this software is not an IUCN-certified replica")
	}

	dst, def, reference, err := analysisRef(c.ProjectionStrategy, center)
	if err != nil {
		return r, err
	}
	defer dst.Close()
	inputWKT, err := src.WKT()
	if err != nil {
		return r, err
	}
	outputWKT, err := dst.WKT()
	if err != nil {
		return r, err
	}
	r.Projection = Projection{Strategy: c.ProjectionStrategy, Reference: reference, InputCRS: c.InputCRS, InputWKT: inputWKT, AnalysisPROJ: def, AnalysisWKT: outputWKT, Center: center, CenterMethod: centerMethod, MaxAngularDistanceDeg: maxDistance, AxisOrder: "traditional GIS: longitude/easting, latitude/northing"}
	x, y = make([]float64, len(r.Points)), make([]float64, len(r.Points))
	for i, p := range r.Points {
		x[i], y[i] = p.Longitude, p.Latitude
	}
	if err = ctx.Err(); err != nil {
		return r, err
	}
	if err = transform(wgs, dst, x, y); err != nil {
		return r, fmt.Errorf("WGS84 to analysis CRS: %w", err)
	}
	projected := make([]xy, len(x))
	for i := range x {
		projected[i] = xy{x[i], y[i]}
		r.Points[i].X = x[i]
		r.Points[i].Y = y[i]
	}
	r.AOO, r.Cells, err = calculateGrid(ctx, projected, c)
	if err != nil {
		return r, err
	}
	r.EOO, err = calculateEOO(projected, dst, r.AOO.AreaKM2)
	if err != nil {
		return r, err
	}
	if r.EOO.Status != "ok" {
		r.Warnings = append(r.Warnings, "EOO_UNAVAILABLE: no non-degenerate polygon; raw EOO is null, assessment value uses AOO by explicit convention")
	}
	if r.EOO.AdjustedToAOO {
		r.Warnings = append(r.Warnings, "EOO_FLOORED_TO_AOO: assessment EOO equals AOO; no replacement hull is invented")
	}
	if r.AOO.GridMode == "sampled" {
		r.Warnings = append(r.Warnings, "SAMPLED_GRID: minimum among tested origins; not a proven translation minimum")
	}
	if r.AOO.GridMode == "fixed" {
		r.Warnings = append(r.Warnings, "FIXED_GRID: origin sensitivity was not searched")
	}
	if !r.AOO.ReferenceScale {
		r.Warnings = append(r.Warnings, "NONSTANDARD_SCALE: cell size differs from the IUCN 2 km reference")
	}
	if c.ProjectionStrategy == ProjectionLocalLAEA && c.Center == nil {
		r.Warnings = append(r.Warnings, "AUTO_CENTER: freeze center and grid origin for temporal comparisons")
	}
	if c.InputCRS != "EPSG:4326" {
		r.Warnings = append(r.Warnings, "DATUM_TRANSFORM: operation accuracy depends on installed PROJ resources; this version does not certify datum-operation accuracy")
	}
	r.Warnings = append(r.Warnings, "POINT_BASED_ESTIMATE: records alone may underestimate occupancy; occurrence suitability and other Red List subcriteria require assessment")
	return r, ctx.Err()
}
