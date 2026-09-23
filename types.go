package eooaoo

import "context"

// Version is the public software version reported in calculation results.
const Version = "0.1.0"

// MaxPoints is the maximum number of occurrence records accepted by one calculation.
const MaxPoints = 100000

const gridWorkLimit = 25000000

const (
	// ProjectionLocalLAEA selects a local Lambert Azimuthal Equal Area projection
	// centred on the assessment distribution.
	ProjectionLocalLAEA = "local-laea"
	// ProjectionIUCNCEA selects the World Cylindrical Equal Area strategy used for
	// methodological comparison with the projection documented by the IUCN EOO Calculator.
	ProjectionIUCNCEA = "iucn-cea"
)

// Point is one occurrence record. X/Y use traditional GIS axis order:
// longitude/easting and latitude/northing. JSON uses the canonical public
// field names lon/lat. Properties are retained for provenance; records at
// identical coordinates are merged only geometrically, never discarded.
type Point struct {
	X          float64        `json:"lon"`
	Y          float64        `json:"lat"`
	ID         string         `json:"id,omitempty"`
	Taxon      string         `json:"taxon,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
}

// Config controls coordinate interpretation, analysis projection and AOO grid search.
// Zero values select the documented defaults: EPSG:4326 input, local-laea,
// 2,000 m cells, auto grid search and 10 sampled steps when fallback is needed.
type Config struct {
	InputCRS string `json:"input_crs,omitempty"`
	// ProjectionStrategy is local-laea (default) or iucn-cea.
	ProjectionStrategy string `json:"projection_strategy,omitempty"`
	// Center is [longitude, latitude] in WGS84. Nil selects a spherical mean.
	// It is only valid with local-laea.
	Center    *[2]float64 `json:"center,omitempty"`
	CellSizeM float64     `json:"cell_size_m,omitempty"`
	// GridMode is auto, exact, sampled, or fixed. Orientation is never rotated.
	GridMode  string     `json:"grid_mode,omitempty"`
	GridSteps int        `json:"grid_steps,omitempty"`
	Origin    [2]float64 `json:"origin_m"`
}

// Projection records the effective input and analysis CRS definitions and the
// information needed to reproduce the projection choice.
type Projection struct {
	Strategy              string     `json:"strategy"`
	Reference             string     `json:"reference"`
	InputCRS              string     `json:"input_crs"`
	InputWKT              string     `json:"input_wkt"`
	AnalysisPROJ          string     `json:"analysis_proj"`
	AnalysisWKT           string     `json:"analysis_wkt"`
	Center                [2]float64 `json:"center_lon_lat"`
	CenterMethod          string     `json:"center_method"`
	MaxAngularDistanceDeg float64    `json:"max_angular_distance_deg"`
	AxisOrder             string     `json:"axis_order"`
}

// EOOResult contains raw convex-hull EOO and the assessment value after the
// explicit rule that assessment EOO must not be smaller than AOO.
type EOOResult struct {
	RawAreaKM2        *float64 `json:"raw_area_km2"`
	AssessmentAreaKM2 float64  `json:"assessment_area_km2"`
	Status            string   `json:"status"`
	AdjustedToAOO     bool     `json:"adjusted_to_aoo"`
	HullWKT           string   `json:"hull_wkt,omitempty"`
}

// AOOResult contains the occupied-cell result and the grid-search metadata used
// to obtain it for the fixed orientation of the analysis CRS axes.
type AOOResult struct {
	AreaKM2                   float64    `json:"area_km2"`
	OccupiedCells             int        `json:"occupied_cells"`
	CellSizeM                 float64    `json:"cell_size_m"`
	ReferenceScale            bool       `json:"iucn_2km_scale"`
	GridMode                  string     `json:"grid_mode"`
	Origin                    [2]float64 `json:"origin_m"`
	Candidates                int        `json:"candidate_origins"`
	MinCells                  int        `json:"min_cells"`
	MaxCells                  int        `json:"max_cells"`
	CompleteTranslationSearch bool       `json:"complete_translation_search"`
	Orientation               string     `json:"orientation"`
	BoundaryRule              string     `json:"boundary_rule"`
}

// Cell describes one occupied AOO cell in analysis-CRS metres.
type Cell struct {
	Column       int64      `json:"column"`
	Row          int64      `json:"row"`
	UniquePoints int        `json:"unique_points"`
	Bounds       [4]float64 `json:"bounds_m"`
}

// UsedPoint is one unique normalized WGS84 coordinate together with its
// analysis-plane coordinate and complete source-record provenance.
type UsedPoint struct {
	Longitude float64 `json:"lon"`
	Latitude  float64 `json:"lat"`
	X         float64 `json:"x_m"`
	Y         float64 `json:"y_m"`
	// InputIndices are zero-based positions in the input slice.
	InputIndices []int   `json:"input_indices"`
	Records      []Point `json:"records"`
}

// Result is the reproducible calculation record returned by Calculate.
// Cells and Points are omitted by Summary and by default CLI/API responses.
type Result struct {
	SchemaVersion     string      `json:"schema_version"`
	SoftwareVersion   string      `json:"software_version"`
	GDALVersion       string      `json:"gdal_version"`
	PROJVersion       string      `json:"proj_version"`
	Taxon             string      `json:"taxon,omitempty"`
	InputSHA256       string      `json:"input_sha256"`
	InputRecords      int         `json:"input_records"`
	UniqueCoordinates int         `json:"unique_coordinates"`
	DuplicateRecords  int         `json:"duplicate_records"`
	Config            Config      `json:"config"`
	Projection        Projection  `json:"projection"`
	EOO               EOOResult   `json:"eoo"`
	AOO               AOOResult   `json:"aoo"`
	Warnings          []string    `json:"warnings"`
	Cells             []Cell      `json:"cells,omitempty"`
	Points            []UsedPoint `json:"points,omitempty"`
}

// Summary returns a copy with the potentially large cell and point provenance
// arrays removed. It does not mutate the receiver.
func (r Result) Summary() Result { r.Cells = nil; r.Points = nil; return r }

func checkContext(ctx context.Context, i int) error {
	if i%1024 == 0 {
		return ctx.Err()
	}
	return nil
}
