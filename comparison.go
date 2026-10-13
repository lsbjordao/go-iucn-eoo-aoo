package eooaoo

import (
	"context"
	"fmt"
)

// ProjectionDelta reports iucn-cea minus local-laea differences for the same
// normalized records and grid configuration. Percentage fields are nil when a
// baseline is zero or the corresponding raw EOO is unavailable.
type ProjectionDelta struct {
	EOORawKM2            *float64 `json:"eoo_raw_km2"`
	EOORawPercent        *float64 `json:"eoo_raw_percent"`
	EOOAssessmentKM2     float64  `json:"eoo_assessment_km2"`
	EOOAssessmentPercent *float64 `json:"eoo_assessment_percent"`
	AOOKM2               float64  `json:"aoo_km2"`
	AOOPercent           *float64 `json:"aoo_percent"`
	AOOOccupiedCells     int      `json:"aoo_occupied_cells"`
}

// ComparisonWarning is a structured caveat about what a delta can mean.
type ComparisonWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ProjectionComparison contains summary results for both supported projection
// strategies plus their signed differences. It exposes projection sensitivity;
// it does not select a universally preferred strategy. Warnings flag when the
// two AOO values were not obtained with the same search coverage, in which
// case the AOO delta mixes projection with grid-search effects.
type ProjectionComparison struct {
	SchemaVersion   string              `json:"schema_version"`
	SoftwareVersion string              `json:"software_version"`
	InputSHA256     string              `json:"input_sha256"`
	LocalLAEA       Result              `json:"local_laea"`
	IUCNCEA         Result              `json:"iucn_cea"`
	Delta           ProjectionDelta     `json:"delta_iucn_cea_minus_local_laea"`
	Warnings        []ComparisonWarning `json:"warnings"`
	Note            string              `json:"note"`
}

func comparisonWarnings(local, iucn Result) []ComparisonWarning {
	w := []ComparisonWarning{}
	a, b := local.AOO, iucn.AOO
	if a.EffectiveGridMode != b.EffectiveGridMode || a.SearchGuarantee != b.SearchGuarantee {
		w = append(w, ComparisonWarning{"GRID_SEARCH_COVERAGE_DIFFERS", fmt.Sprintf("local-laea AOO used %s search (%s) and iucn-cea used %s search (%s); the AOO delta mixes projection with search coverage", a.EffectiveGridMode, a.SearchGuarantee, b.EffectiveGridMode, b.SearchGuarantee)})
	}
	for _, side := range []struct {
		name string
		a    AOOResult
	}{{ProjectionLocalLAEA, a}, {ProjectionIUCNCEA, b}} {
		switch side.a.EffectiveGridMode {
		case "sampled":
			msg := side.name + " AOO is the minimum among tested origins; not a proven translation minimum"
			if side.a.FallbackReason != "" {
				msg += " (auto fallback: " + side.a.FallbackReason + ")"
			}
			w = append(w, ComparisonWarning{"SAMPLED_GRID", msg})
		case "fixed":
			w = append(w, ComparisonWarning{"FIXED_GRID", side.name + " AOO uses one fixed origin; origin sensitivity was not searched"})
		}
	}
	return w
}

func relativePercent(delta, baseline float64) *float64 {
	if baseline == 0 {
		return nil
	}
	v := delta / baseline * 100
	return &v
}

// CompareProjections runs the same normalized records and grid configuration
// through the local LAEA and IUCN-compatible World Cylindrical Equal Area
// strategies. The delta is always iucn-cea minus local-laea.
func CompareProjections(ctx context.Context, points []Point, config Config) (ProjectionComparison, error) {
	var out ProjectionComparison
	if config.Center != nil {
		return out, fmt.Errorf("projection comparison requires an automatic local-laea center; remove center")
	}

	localCfg := config
	localCfg.ProjectionStrategy = ProjectionLocalLAEA
	local, err := Calculate(ctx, points, localCfg)
	if err != nil {
		return out, fmt.Errorf("local-laea: %w", err)
	}

	iucnCfg := config
	iucnCfg.ProjectionStrategy = ProjectionIUCNCEA
	iucnCfg.Center = nil
	iucn, err := Calculate(ctx, points, iucnCfg)
	if err != nil {
		return out, fmt.Errorf("iucn-cea: %w", err)
	}

	if local.InputSHA256 != iucn.InputSHA256 {
		return out, fmt.Errorf("internal comparison error: input hashes differ")
	}

	d := ProjectionDelta{
		EOOAssessmentKM2: iucn.EOO.AssessmentAreaKM2 - local.EOO.AssessmentAreaKM2,
		AOOKM2:           iucn.AOO.AreaKM2 - local.AOO.AreaKM2,
		AOOOccupiedCells: iucn.AOO.OccupiedCells - local.AOO.OccupiedCells,
	}
	d.EOOAssessmentPercent = relativePercent(d.EOOAssessmentKM2, local.EOO.AssessmentAreaKM2)
	d.AOOPercent = relativePercent(d.AOOKM2, local.AOO.AreaKM2)
	if local.EOO.RawAreaKM2 != nil && iucn.EOO.RawAreaKM2 != nil {
		v := *iucn.EOO.RawAreaKM2 - *local.EOO.RawAreaKM2
		d.EOORawKM2 = &v
		d.EOORawPercent = relativePercent(v, *local.EOO.RawAreaKM2)
	}

	out = ProjectionComparison{
		SchemaVersion:   "1",
		SoftwareVersion: Version,
		InputSHA256:     local.InputSHA256,
		LocalLAEA:       local.Summary(),
		IUCNCEA:         iucn.Summary(),
		Delta:           d,
		Warnings:        comparisonWarnings(local, iucn),
		Note:            "Projection choice can change planar hull shape and 2 km grid orientation. iucn-cea uses the World Cylindrical Equal Area projection documented by the IUCN EOO Calculator; it is a compatibility-oriented comparison, not an IUCN certification.",
	}
	return out, nil
}
