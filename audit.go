package eooaoo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/airbusgeo/godal"
)

// AuditRulesVersion identifies the rule set; change it whenever a rule changes.
const AuditRulesVersion = "1"

// Audit issue codes. Codes are stable identifiers; messages may change.
const (
	IssueInvalidRecord         = "INVALID_RECORD"
	IssueInvalidCoordinate     = "INVALID_COORDINATE"
	IssueZeroZero              = "ZERO_ZERO_COORDINATE"
	IssueCoarsePrecision       = "COARSE_COORDINATE_PRECISION"
	IssueUncertaintyExceedCell = "UNCERTAINTY_EXCEEDS_CELL_SIZE"
	IssueSourceQualityFlag     = "SOURCE_QUALITY_FLAG"
)

// AuditRecord is one input record as read by a permissive parser. ParseError
// is set when the record could not be turned into numeric coordinates (the
// strict calc parser would reject it); X/Y are then meaningless.
type AuditRecord struct {
	Point
	// SourceRow is the 1-based CSV line number including the header, or 0.
	SourceRow  int
	ParseError string
}

// AuditConfig enables and parameterizes the optional rules. Every threshold is
// explicit: nothing is inferred from field names unless a field is named here.
type AuditConfig struct {
	InputCRS string `json:"input_crs,omitempty"`
	// CellSizeM is the AOO cell side compared against coordinate uncertainty.
	// Zero selects 2000.
	CellSizeM float64 `json:"cell_size_m,omitempty"`
	// CoarseDecimals enables COARSE_COORDINATE_PRECISION for records whose
	// input X and Y are both whole multiples of 10^-CoarseDecimals input-CRS
	// units. Nil disables the rule.
	CoarseDecimals *int `json:"coarse_decimals,omitempty"`
	// UncertaintyField names the record property holding coordinate
	// uncertainty in metres. Empty disables UNCERTAINTY_EXCEEDS_CELL_SIZE.
	UncertaintyField string `json:"uncertainty_field,omitempty"`
	// SourceFlagField names the record property holding source quality flags,
	// separated by commas or semicolons. Empty disables SOURCE_QUALITY_FLAG.
	SourceFlagField string `json:"source_flag_field,omitempty"`
	// ContextFields are record properties copied verbatim into every finding's
	// evidence (e.g. locality), so a reviewer sees them next to the finding.
	ContextFields []string `json:"context_fields,omitempty"`
}

// AuditIssue is one finding about one record. It never states a biological
// conclusion and never removes the record.
type AuditIssue struct {
	Code       string         `json:"code"`
	Severity   string         `json:"severity"`
	InputIndex *int           `json:"input_index"`
	SourceRow  *int           `json:"source_row,omitempty"`
	RecordID   string         `json:"record_id,omitempty"`
	Evidence   map[string]any `json:"evidence"`
	Message    string         `json:"message"`
}

// AuditReport summarizes data-quality findings without running a grid search
// or changing which records a calculation uses.
type AuditReport struct {
	SchemaVersion   string      `json:"schema_version"`
	RulesVersion    string      `json:"rules_version"`
	SoftwareVersion string      `json:"software_version"`
	Config          AuditConfig `json:"config"`
	InputCRS        string      `json:"input_crs"`
	// InputSHA256 matches Result.InputSHA256 for the same records; it is null
	// when some record has no numeric coordinates and so cannot be hashed the
	// same way.
	InputSHA256           *string        `json:"input_sha256"`
	SourceSHA256          string         `json:"source_sha256,omitempty"`
	InputRecords          int            `json:"input_records"`
	InvalidRecords        int            `json:"invalid_records"`
	RecordsWithIssues     int            `json:"records_with_issues"`
	ValidUniqueCoords     int            `json:"valid_unique_coordinates"`
	DuplicateRecordIDs    int            `json:"duplicate_record_ids"`
	UncertaintyUnknown    *int           `json:"uncertainty_unknown,omitempty"`
	CountsByCode          map[string]int `json:"counts_by_code"`
	CountsBySeverity      map[string]int `json:"counts_by_severity"`
	CalculationCompatible bool           `json:"calculation_compatible"`
	Issues                []AuditIssue   `json:"issues"`
	Note                  string         `json:"note"`
}

// sourceFlagMeaning documents the source flags this version interprets: GBIF
// OccurrenceIssue names and the rgbif abbreviations of the same issues
// (https://gbif.github.io/gbif-api/apidocs/org/gbif/api/vocabulary/OccurrenceIssue.html).
// Other flags are preserved verbatim with severity info and no interpretation.
var sourceFlagMeaning = map[string]struct{ issue, severity string }{
	"zerocd":    {"ZERO_COORDINATE", "warning"},
	"cdiv":      {"COORDINATE_INVALID", "warning"},
	"cdout":     {"COORDINATE_OUT_OF_RANGE", "warning"},
	"cucdmis":   {"COUNTRY_COORDINATE_MISMATCH", "warning"},
	"preneglat": {"PRESUMED_NEGATED_LATITUDE", "warning"},
	"preneglon": {"PRESUMED_NEGATED_LONGITUDE", "warning"},
	"preswcd":   {"PRESUMED_SWAPPED_COORDINATE", "warning"},
	"cdround":   {"COORDINATE_ROUNDED", "info"},
	"gass84":    {"GEODETIC_DATUM_ASSUMED_WGS84", "info"},
}

func init() {
	for _, v := range []struct{ issue, severity string }{
		{"ZERO_COORDINATE", "warning"}, {"COORDINATE_INVALID", "warning"}, {"COORDINATE_OUT_OF_RANGE", "warning"},
		{"COUNTRY_COORDINATE_MISMATCH", "warning"}, {"PRESUMED_NEGATED_LATITUDE", "warning"}, {"PRESUMED_NEGATED_LONGITUDE", "warning"},
		{"PRESUMED_SWAPPED_COORDINATE", "warning"}, {"COORDINATE_ROUNDED", "info"}, {"GEODETIC_DATUM_ASSUMED_WGS84", "info"},
	} {
		sourceFlagMeaning[strings.ToLower(v.issue)] = v
	}
}

// missingValue reports values that conventionally mean "not recorded".
func missingValue(v any) bool {
	if v == nil {
		return true
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	return s == "" || strings.EqualFold(s, "NA") || strings.EqualFold(s, "null") || strings.EqualFold(s, "NaN")
}

// InputHash returns the SHA-256 that Calculate records for these points.
func InputHash(points []Point) (string, error) {
	b, err := json.Marshal(points)
	if err != nil {
		return "", fmt.Errorf("input metadata must be JSON-compatible: %w", err)
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:]), nil
}

// AuditOccurrences diagnoses records one by one. It reuses Calculate's domain
// checks and CRS transformation but never runs the grid search and never
// changes the record set: suspect records are reported, not removed.
func AuditOccurrences(ctx context.Context, records []AuditRecord, cfg AuditConfig) (AuditReport, error) {
	if cfg.InputCRS == "" {
		cfg.InputCRS = "EPSG:4326"
	}
	if cfg.CellSizeM == 0 {
		cfg.CellSizeM = 2000
	}
	r := AuditReport{SchemaVersion: "1", RulesVersion: AuditRulesVersion, SoftwareVersion: Version, Config: cfg, InputCRS: cfg.InputCRS, InputRecords: len(records), CountsByCode: map[string]int{}, CountsBySeverity: map[string]int{}, Issues: []AuditIssue{}}
	r.Note = "Findings are indications for curation, not conclusions. No record is excluded; use an explicit curation manifest for exclusions."
	if _, err := normalizeConfig(Config{InputCRS: cfg.InputCRS, CellSizeM: cfg.CellSizeM}); err != nil {
		return r, err
	}
	if cfg.CoarseDecimals != nil && (*cfg.CoarseDecimals < 0 || *cfg.CoarseDecimals > 12) {
		return r, fmt.Errorf("coarse_decimals must be between 0 and 12")
	}
	if len(records) == 0 || len(records) > MaxPoints {
		return r, fmt.Errorf("expected 1..%d records", MaxPoints)
	}
	src, err := godal.NewSpatialRef(cfg.InputCRS)
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

	flagged := map[int]bool{}
	add := func(i int, code, severity, msg string, evidence map[string]any) {
		idx := i
		if len(cfg.ContextFields) > 0 {
			ctxv := map[string]any{}
			for _, f := range cfg.ContextFields {
				if v, ok := records[i].Properties[f]; ok {
					ctxv[f] = v
				}
			}
			evidence["context"] = ctxv
		}
		is := AuditIssue{Code: code, Severity: severity, InputIndex: &idx, RecordID: records[i].ID, Evidence: evidence, Message: msg}
		if records[i].SourceRow > 0 {
			row := records[i].SourceRow
			is.SourceRow = &row
		}
		r.Issues = append(r.Issues, is)
		r.CountsByCode[code]++
		r.CountsBySeverity[severity]++
		flagged[i] = true
	}

	// Structural and domain validity, in input units.
	valid := make([]bool, len(records))
	var idx []int
	var x, y []float64
	for i, rec := range records {
		if err = checkContext(ctx, i); err != nil {
			return r, err
		}
		switch {
		case rec.ParseError != "":
			code := IssueInvalidCoordinate
			if strings.HasPrefix(rec.ParseError, "record:") {
				code = IssueInvalidRecord
			}
			add(i, code, "error", "record cannot be used by calc: "+rec.ParseError, map[string]any{"parse_error": rec.ParseError})
		case !finite(rec.X) || !finite(rec.Y):
			add(i, IssueInvalidCoordinate, "error", "non-finite coordinate", map[string]any{"x": fmt.Sprint(rec.X), "y": fmt.Sprint(rec.Y)})
		case src.Geographic() && (rec.X < -180 || rec.X > 180 || rec.Y < -90 || rec.Y > 90):
			add(i, IssueInvalidCoordinate, "error", "geographic coordinate outside longitude [-180,180] or latitude [-90,90]", map[string]any{"lon": rec.X, "lat": rec.Y})
		default:
			valid[i] = true
			idx = append(idx, i)
			x, y = append(x, rec.X), append(y, rec.Y)
		}
	}
	// Transform one record at a time so a failure is attributed, not fatal.
	wx, wy := append([]float64(nil), x...), append([]float64(nil), y...)
	if err = transform(src, wgs, wx, wy); err != nil {
		for k, i := range idx {
			px, py := []float64{x[k]}, []float64{y[k]}
			if e := transform(src, wgs, px, py); e != nil {
				valid[i] = false
				add(i, IssueInvalidCoordinate, "error", "coordinate cannot be transformed to WGS84", map[string]any{"x": x[k], "y": y[k], "error": e.Error()})
				continue
			}
			wx[k], wy[k] = px[0], py[0]
		}
	}
	unique := map[[2]float64]bool{}
	for k, i := range idx {
		if !valid[i] {
			continue
		}
		lon, lat := wx[k], wy[k]
		if lon < -180 || lon > 180 || lat < -90 || lat > 90 {
			valid[i] = false
			add(i, IssueInvalidCoordinate, "error", "transformed WGS84 coordinate out of range", map[string]any{"lon": lon, "lat": lat})
			continue
		}
		if lon == 180 {
			lon = -180
		}
		if math.Abs(lat) == 90 {
			lon = 0
		}
		unique[[2]float64{lon, lat}] = true
		if lon == 0 && lat == 0 {
			add(i, IssueZeroZero, "warning", "longitude and latitude are both exactly 0 in WGS84, a common placeholder for missing coordinates; verify against locality before use", map[string]any{"lon": 0, "lat": 0, "input_x": x[k], "input_y": y[k]})
		}
		if d := cfg.CoarseDecimals; d != nil && isMultiple(x[k], *d) && isMultiple(y[k], *d) {
			add(i, IssueCoarsePrecision, "info", fmt.Sprintf("both input coordinates have at most %d decimal places; this is numerical rounding only and says nothing definite about georeferencing precision", *d), map[string]any{"x": x[k], "y": y[k], "max_decimals": *d, "input_crs": cfg.InputCRS})
		}
	}
	for i := range records {
		if !valid[i] {
			r.InvalidRecords++
		}
	}
	r.ValidUniqueCoords = len(unique)

	if cfg.UncertaintyField != "" {
		unknown := 0
		for i, rec := range records {
			v, ok := rec.Properties[cfg.UncertaintyField]
			if !ok || missingValue(v) {
				unknown++
				continue
			}
			u, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
			if err != nil || !finite(u) || u < 0 {
				unknown++
				continue
			}
			if u > cfg.CellSizeM {
				add(i, IssueUncertaintyExceedCell, "warning", fmt.Sprintf("declared coordinate uncertainty %g m exceeds the %g m cell size", u, cfg.CellSizeM), map[string]any{"field": cfg.UncertaintyField, "uncertainty_m": u, "cell_size_m": cfg.CellSizeM})
			}
		}
		r.UncertaintyUnknown = &unknown
	}
	if cfg.SourceFlagField != "" {
		for i, rec := range records {
			v, ok := rec.Properties[cfg.SourceFlagField]
			if !ok || missingValue(v) {
				continue
			}
			for _, f := range strings.FieldsFunc(fmt.Sprint(v), func(c rune) bool { return c == ',' || c == ';' }) {
				f = strings.TrimSpace(f)
				if f == "" || strings.EqualFold(f, "NA") {
					continue
				}
				ev := map[string]any{"field": cfg.SourceFlagField, "flag": f}
				if m, ok := sourceFlagMeaning[strings.ToLower(f)]; ok {
					ev["interpretation"] = "GBIF OccurrenceIssue " + m.issue
					add(i, IssueSourceQualityFlag, m.severity, "source flag "+f+" ("+m.issue+") reported by the data provider", ev)
				} else {
					add(i, IssueSourceQualityFlag, "info", "source flag "+f+" preserved without interpretation", ev)
				}
			}
		}
	}

	ids := map[string]int{}
	for _, rec := range records {
		if rec.ID != "" {
			ids[rec.ID]++
		}
	}
	for _, n := range ids {
		if n > 1 {
			r.DuplicateRecordIDs += n
		}
	}
	r.RecordsWithIssues = len(flagged)
	r.CalculationCompatible = r.InvalidRecords == 0
	if r.InvalidRecords == 0 {
		pts := make([]Point, len(records))
		for i, rec := range records {
			pts[i] = rec.Point
		}
		if h, err := InputHash(pts); err == nil {
			r.InputSHA256 = &h
		}
	}
	sort.SliceStable(r.Issues, func(a, b int) bool { return *r.Issues[a].InputIndex < *r.Issues[b].InputIndex })
	return r, ctx.Err()
}

// isMultiple reports whether v is a whole multiple of 10^-d, judged on the
// shortest decimal representation of the float64 (what the source wrote).
func isMultiple(v float64, d int) bool {
	s := strconv.FormatFloat(math.Abs(v), 'f', -1, 64)
	dot := strings.IndexByte(s, '.')
	return dot < 0 || len(s)-dot-1 <= d
}
