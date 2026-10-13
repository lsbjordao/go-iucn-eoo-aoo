package eooaoo

import (
	"context"
	"fmt"
	"sort"

	"github.com/airbusgeo/godal"
)

// InfluenceConfig bounds an EOO influence analysis.
type InfluenceConfig struct {
	// MaxCandidates limits how many hull-vertex groups are evaluated, in
	// longitude/latitude order. Zero selects 1000. When more vertices exist
	// the report is partial and its ranking is not global.
	MaxCandidates int `json:"max_candidates,omitempty"`
	// TopK limits how many evaluated groups are listed. Zero lists all.
	TopK int `json:"top_k,omitempty"`
	// RecomputeProjection additionally recalculates raw EOO for each removal
	// with a freshly chosen automatic local-laea centre, to separate geometric
	// influence from the effect of moving the projection centre.
	RecomputeProjection bool `json:"recompute_projection,omitempty"`
}

// InfluenceGroup is one unique coordinate that is a vertex of the original
// hull, removed as a geometric unit together with every record at it.
type InfluenceGroup struct {
	Longitude    float64  `json:"lon"`
	Latitude     float64  `json:"lat"`
	X            float64  `json:"x_m"`
	Y            float64  `json:"y_m"`
	InputIndices []int    `json:"input_indices"`
	RecordIDs    []string `json:"record_ids"`
	// RawEOOWithoutKM2 is the raw hull area of all remaining points in the
	// frozen analysis CRS; null when the remaining hull is degenerate.
	RawEOOWithoutKM2 *float64 `json:"raw_eoo_without_km2"`
	StatusWithout    string   `json:"eoo_status_without"`
	// ReductionKM2 is original raw EOO minus RawEOOWithoutKM2.
	ReductionKM2            *float64 `json:"reduction_km2"`
	RelativeReductionPct    *float64 `json:"relative_reduction_percent"`
	RecomputedRawEOOKM2     *float64 `json:"recomputed_projection_raw_eoo_km2,omitempty"`
	RecomputedStatus        string   `json:"recomputed_projection_status,omitempty"`
	RecomputedProjectionErr string   `json:"recomputed_projection_error,omitempty"`
}

// EOOInfluence reports how much raw EOO depends on each hull vertex. It is a
// sensitivity diagnostic: it never classifies a record as an error and never
// excludes records. The AOO floor is not applied to any value here.
type EOOInfluence struct {
	SchemaVersion       string           `json:"schema_version"`
	SoftwareVersion     string           `json:"software_version"`
	InputSHA256         string           `json:"input_sha256"`
	GeometrySHA256      string           `json:"geometry_sha256"`
	ProtocolSHA256      string           `json:"protocol_sha256"`
	Projection          Projection       `json:"projection"`
	CRSPolicy           string           `json:"crs_policy"`
	OriginalRawEOOKM2   *float64         `json:"original_raw_eoo_km2"`
	OriginalStatus      string           `json:"original_eoo_status"`
	Status              string           `json:"status"`
	HullVertexGroups    int              `json:"hull_vertex_groups"`
	EvaluatedGroups     int              `json:"evaluated_groups"`
	RankingComplete     bool             `json:"ranking_complete"`
	Groups              []InfluenceGroup `json:"groups"`
	RecomputeProjection bool             `json:"recompute_projection"`
	Note                string           `json:"note"`
}

// AnalyzeEOOInfluence computes the original hull, freezes its complete
// analysis CRS and removes each hull-vertex coordinate group in turn,
// rebuilding the hull from all remaining points (interior points may become
// vertices). Groups are ordered by relative reduction, largest first; groups
// whose removal leaves no valid hull come first because they have no numeric
// delta. Remaining ties are broken by longitude then latitude.
func AnalyzeEOOInfluence(ctx context.Context, points []Point, config Config, ic InfluenceConfig) (EOOInfluence, error) {
	out := EOOInfluence{SchemaVersion: "1", SoftwareVersion: Version, Groups: []InfluenceGroup{}, RecomputeProjection: ic.RecomputeProjection}
	if ic.MaxCandidates == 0 {
		ic.MaxCandidates = 1000
	}
	if ic.MaxCandidates < 1 || ic.TopK < 0 {
		return out, fmt.Errorf("max_candidates must be positive and top_k non-negative")
	}
	if ic.RecomputeProjection && (config.Center != nil || (config.ProjectionStrategy != "" && config.ProjectionStrategy != ProjectionLocalLAEA)) {
		return out, fmt.Errorf("recompute_projection compares against an automatic local-laea centre; it needs local-laea without a fixed center")
	}
	r, err := Calculate(ctx, points, config)
	if err != nil {
		return out, err
	}
	out.InputSHA256, out.GeometrySHA256, out.ProtocolSHA256 = r.InputSHA256, r.GeometrySHA256, r.ProtocolSHA256
	out.Projection = r.Projection
	out.CRSPolicy = "frozen: every removal reuses the original analysis CRS, including any automatic local-laea centre"
	out.OriginalRawEOOKM2, out.OriginalStatus = r.EOO.RawAreaKM2, r.EOO.Status
	out.Note = "Sensitivity of raw EOO to individual hull vertices. A large reduction does not make a record wrong; records are never excluded by this analysis."
	if r.EOO.Status != "ok" {
		out.Status = "unavailable"
		return out, nil
	}
	sr, err := godal.NewSpatialRef(r.Projection.AnalysisPROJ)
	if err != nil {
		return out, err
	}
	defer sr.Close()
	projected := make([]xy, len(r.Points))
	for i, p := range r.Points {
		projected[i] = xy{p.X, p.Y}
	}
	_, hull, err := calculateEOO(projected, sr, 0)
	if err != nil {
		return out, err
	}
	vertex := make(map[xy]bool, len(hull))
	for _, v := range hull {
		vertex[v] = true
	}
	var candidates []int // indices into r.Points, already in lon/lat order
	for i, v := range projected {
		if vertex[v] {
			candidates = append(candidates, i)
		}
	}
	out.HullVertexGroups = len(candidates)
	if len(candidates) > ic.MaxCandidates {
		candidates = candidates[:ic.MaxCandidates]
	}
	rest := make([]xy, 0, len(projected))
	for _, ci := range candidates {
		if err = ctx.Err(); err != nil {
			return out, err
		}
		rest = rest[:0]
		for i, v := range projected {
			if i != ci {
				rest = append(rest, v)
			}
		}
		e, _, err := calculateEOO(rest, sr, 0)
		if err != nil {
			return out, err
		}
		up := r.Points[ci]
		g := InfluenceGroup{Longitude: up.Longitude, Latitude: up.Latitude, X: up.X, Y: up.Y, InputIndices: up.InputIndices, RawEOOWithoutKM2: e.RawAreaKM2, StatusWithout: e.Status}
		for _, rec := range up.Records {
			g.RecordIDs = append(g.RecordIDs, rec.ID)
		}
		if e.RawAreaKM2 != nil {
			d := *r.EOO.RawAreaKM2 - *e.RawAreaKM2
			g.ReductionKM2 = &d
			g.RelativeReductionPct = relativePercent(d, *r.EOO.RawAreaKM2)
		}
		if ic.RecomputeProjection {
			recomputeWithout(ctx, points, config, up.InputIndices, &g)
		}
		out.Groups = append(out.Groups, g)
	}
	out.EvaluatedGroups = len(out.Groups)
	out.RankingComplete = out.EvaluatedGroups == out.HullVertexGroups
	out.Status = "complete"
	if !out.RankingComplete {
		out.Status = "partial"
	}
	sort.SliceStable(out.Groups, func(i, j int) bool {
		a, b := out.Groups[i].RelativeReductionPct, out.Groups[j].RelativeReductionPct
		if (a == nil) != (b == nil) {
			return a == nil
		}
		if a != nil && *a != *b {
			return *a > *b
		}
		if out.Groups[i].Longitude != out.Groups[j].Longitude {
			return out.Groups[i].Longitude < out.Groups[j].Longitude
		}
		return out.Groups[i].Latitude < out.Groups[j].Latitude
	})
	if ic.TopK > 0 && len(out.Groups) > ic.TopK {
		out.Groups = out.Groups[:ic.TopK]
	}
	return out, ctx.Err()
}

// recomputeWithout reruns the full calculation without the group's records,
// letting local-laea choose a new centre. AOO is irrelevant here, so the grid
// is fixed to avoid a search.
func recomputeWithout(ctx context.Context, points []Point, config Config, drop []int, g *InfluenceGroup) {
	skip := make(map[int]bool, len(drop))
	for _, i := range drop {
		skip[i] = true
	}
	subset := make([]Point, 0, len(points)-len(drop))
	for i, p := range points {
		if !skip[i] {
			subset = append(subset, p)
		}
	}
	c := config
	c.GridMode = "fixed"
	rr, err := Calculate(ctx, subset, c)
	if err != nil {
		g.RecomputedProjectionErr = err.Error()
		return
	}
	g.RecomputedRawEOOKM2, g.RecomputedStatus = rr.EOO.RawAreaKM2, rr.EOO.Status
}
