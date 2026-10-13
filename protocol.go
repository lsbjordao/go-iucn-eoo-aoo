package eooaoo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Method identifiers recorded in results and protocols. Changing what a
// calculation does under one of these names requires a new identifier.
const (
	ProtocolVersion       = "1"
	InclusionRulesVersion = "strict-v1: finite, in-domain coordinates; exact WGS84 equality merges records; lon 180 -> -180; lon 0 at the poles"
	HullMethod            = "planar convex hull in the analysis CRS (monotone chain, exact orientation predicate)"
	AreaMethod            = "GDAL planar polygon area in the analysis CRS"
	GridOrientation       = "fixed analysis CRS axes; no rotation"
	GridBoundaryRule      = "[left,right) x [bottom,top); no snapping"
)

// ProtocolEnvironment records the native stack that produced a result. It is
// reported but excluded from the protocol hash: a protocol fixes method and
// parameters, while bit-identical output across native versions is not promised.
type ProtocolEnvironment struct {
	SoftwareVersion string `json:"software_version"`
	GDALVersion     string `json:"gdal_version"`
	PROJVersion     string `json:"proj_version"`
}

// AnalysisProtocol freezes every effective parameter needed to reproduce a
// calculation on a changed record set: full analysis CRS definition, projection
// centre, cell size, grid origin and orientation, boundary rule, hull and area
// methods, and the record-inclusion rules. Reproduction always uses a fixed
// grid, so added records cannot move the centre or the origin.
type AnalysisProtocol struct {
	ProtocolVersion    string              `json:"protocol_version"`
	InputCRS           string              `json:"input_crs"`
	ProjectionStrategy string              `json:"projection_strategy"`
	AnalysisPROJ       string              `json:"analysis_proj"`
	Center             *[2]float64         `json:"center_lon_lat,omitempty"`
	CellSizeM          float64             `json:"cell_size_m"`
	Origin             [2]float64          `json:"origin_m"`
	GridMode           string              `json:"grid_mode"`
	Orientation        string              `json:"orientation"`
	BoundaryRule       string              `json:"boundary_rule"`
	HullMethod         string              `json:"hull_method"`
	AreaMethod         string              `json:"area_method"`
	InclusionRules     string              `json:"inclusion_rules_version"`
	Environment        ProtocolEnvironment `json:"environment"`
}

// deriveProtocol returns the frozen protocol describing how r was computed.
func (r Result) deriveProtocol() AnalysisProtocol {
	p := AnalysisProtocol{
		ProtocolVersion:    ProtocolVersion,
		InputCRS:           r.Config.InputCRS,
		ProjectionStrategy: r.Projection.Strategy,
		AnalysisPROJ:       r.Projection.AnalysisPROJ,
		CellSizeM:          r.AOO.CellSizeM,
		Origin:             r.AOO.Origin,
		GridMode:           "fixed",
		Orientation:        r.AOO.Orientation,
		BoundaryRule:       r.AOO.BoundaryRule,
		HullMethod:         HullMethod,
		AreaMethod:         AreaMethod,
		InclusionRules:     InclusionRulesVersion,
		Environment:        ProtocolEnvironment{SoftwareVersion: r.SoftwareVersion, GDALVersion: r.GDALVersion, PROJVersion: r.PROJVersion},
	}
	if r.Projection.Strategy == ProjectionLocalLAEA {
		c := r.Projection.Center
		p.Center = &c
	}
	return p
}

// SHA256 hashes the canonical JSON serialization of the protocol without its
// Environment, so the hash identifies method and effective parameters only.
func (p AnalysisProtocol) SHA256() string {
	p.Environment = ProtocolEnvironment{}
	b, _ := json.Marshal(p) // struct field order and shortest float formatting are deterministic
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// ApplyProtocol returns the fixed-grid Config that reproduces p. Fields already
// set in c must agree with the protocol; any disagreement, or a protocol whose
// methods differ from this software's, is an error raised before calculation.
func ApplyProtocol(c Config, p AnalysisProtocol) (Config, error) {
	var out Config
	if p.ProtocolVersion != ProtocolVersion {
		return out, fmt.Errorf("protocol_version %q is not supported (want %q)", p.ProtocolVersion, ProtocolVersion)
	}
	for _, f := range []struct{ name, got, want string }{
		{"grid_mode", p.GridMode, "fixed"},
		{"orientation", p.Orientation, GridOrientation},
		{"boundary_rule", p.BoundaryRule, GridBoundaryRule},
		{"hull_method", p.HullMethod, HullMethod},
		{"area_method", p.AreaMethod, AreaMethod},
		{"inclusion_rules_version", p.InclusionRules, InclusionRulesVersion},
	} {
		if f.got != f.want {
			return out, fmt.Errorf("protocol %s %q is incompatible with this software (%q)", f.name, f.got, f.want)
		}
	}
	mismatch := func(name string) error {
		return fmt.Errorf("configuration %s conflicts with the frozen protocol; remove it or use the protocol value", name)
	}
	if c.InputCRS != "" && c.InputCRS != p.InputCRS {
		return out, mismatch("input_crs")
	}
	if c.ProjectionStrategy != "" && c.ProjectionStrategy != p.ProjectionStrategy {
		return out, mismatch("projection_strategy")
	}
	if c.CellSizeM != 0 && c.CellSizeM != p.CellSizeM {
		return out, mismatch("cell_size_m")
	}
	if c.GridMode != "" && c.GridMode != "fixed" {
		return out, mismatch("grid_mode")
	}
	if c.Origin != ([2]float64{}) && c.Origin != p.Origin {
		return out, mismatch("origin_m")
	}
	if c.Center != nil && (p.Center == nil || *c.Center != *p.Center) {
		return out, mismatch("center")
	}
	out = Config{InputCRS: p.InputCRS, ProjectionStrategy: p.ProjectionStrategy, CellSizeM: p.CellSizeM, GridMode: "fixed", GridSteps: c.GridSteps, Origin: p.Origin, ExtentWarningDeg: c.ExtentWarningDeg}
	switch p.ProjectionStrategy {
	case ProjectionLocalLAEA:
		if p.Center == nil {
			return out, fmt.Errorf("local-laea protocol requires center_lon_lat")
		}
		v := *p.Center
		out.Center = &v
	case ProjectionIUCNCEA:
		if p.Center != nil {
			return out, fmt.Errorf("iucn-cea protocol must not define center_lon_lat")
		}
	default:
		return out, fmt.Errorf("unknown projection_strategy %q", p.ProjectionStrategy)
	}
	out, err := normalizeConfig(out)
	if err != nil {
		return out, err
	}
	center := [2]float64{}
	if out.Center != nil {
		center = *out.Center
	}
	sr, def, _, err := analysisRef(out.ProjectionStrategy, center)
	if err != nil {
		return out, err
	}
	sr.Close()
	if def != p.AnalysisPROJ {
		return out, fmt.Errorf("protocol analysis_proj %q does not match the definition derived from its strategy and centre (%q)", p.AnalysisPROJ, def)
	}
	return out, nil
}

// CalculateWithProtocol reproduces a frozen protocol on a (possibly changed)
// record set. Differences in software, GDAL or PROJ versions are reported as a
// warning, not hidden; identical output across native versions is not promised.
func CalculateWithProtocol(ctx context.Context, points []Point, p AnalysisProtocol, c Config) (Result, error) {
	cfg, err := ApplyProtocol(c, p)
	if err != nil {
		return Result{}, err
	}
	r, err := Calculate(ctx, points, cfg)
	if err != nil {
		return r, err
	}
	if r.ProtocolSHA256 != p.SHA256() {
		return r, fmt.Errorf("internal protocol error: reproduction hash %s differs from protocol %s", r.ProtocolSHA256, p.SHA256())
	}
	r.Warnings = append(r.Warnings, "FROZEN_PROTOCOL: projection, centre, grid origin, orientation and cell size were imported from protocol "+p.SHA256()[:12]+"; no default was recalculated")
	if env := r.Protocol.Environment; env != p.Environment {
		r.Warnings = append(r.Warnings, fmt.Sprintf("PROTOCOL_ENVIRONMENT_CHANGED: protocol recorded software %s, GDAL %s, PROJ %s; this run used %s, %s, %s; values may differ in the last digits", p.Environment.SoftwareVersion, p.Environment.GDALVersion, p.Environment.PROJVersion, env.SoftwareVersion, env.GDALVersion, env.PROJVersion))
	}
	return r, nil
}

// geometryHash hashes the sorted unique normalized WGS84 coordinates, so it is
// independent of record order, duplicates and non-spatial metadata. -0 is
// written as 0; ±180 and polar longitudes are already normalized by Calculate.
func geometryHash(p []UsedPoint) string {
	var b strings.Builder
	num := func(v float64) string {
		if v == 0 {
			v = 0 // folds -0
		}
		return strconv.FormatFloat(v, 'g', -1, 64)
	}
	for _, v := range p {
		b.WriteString(num(v.Longitude))
		b.WriteByte(',')
		b.WriteString(num(v.Latitude))
		b.WriteByte('\n')
	}
	s := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(s[:])
}

// lonLatExtent returns the smallest longitude arc covering every point (so an
// antimeridian-crossing set is measured across ±180) and the latitude span.
func lonLatExtent(p []UsedPoint) (lonSpan, latSpan float64) {
	if len(p) == 0 {
		return 0, 0
	}
	lons := make([]float64, len(p))
	minLat, maxLat := math.Inf(1), math.Inf(-1)
	for i, v := range p {
		lons[i] = v.Longitude
		minLat, maxLat = math.Min(minLat, v.Latitude), math.Max(maxLat, v.Latitude)
	}
	// r.Points is sorted by longitude; the largest gap between neighbours
	// (including the wrap-around gap) is the part of the circle left uncovered.
	gap := lons[0] + 360 - lons[len(lons)-1]
	for i := 1; i < len(lons); i++ {
		gap = math.Max(gap, lons[i]-lons[i-1])
	}
	return 360 - gap, maxLat - minLat
}
