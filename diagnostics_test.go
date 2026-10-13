package eooaoo

import (
	"context"
	"math"
	"math/rand"
	"strings"
	"testing"
)

// zeroZeroPoints mirrors testdata/diagnostics/zero-zero.csv without the
// invalid rows: a South American cloud plus one record at exactly (0,0).
func zeroZeroPoints() []Point {
	return []Point{
		{X: -48.5, Y: -23.5, ID: "r1"},
		{X: -54.9, Y: -34.9, ID: "r2"},
		{X: -41.4, Y: -12.6, ID: "r3"},
		{X: -60.36, Y: -24.91, ID: "r4"},
		{X: -50.1, Y: -27.2, ID: "r5"},
		{X: 0, Y: 0, ID: "r6", Properties: map[string]any{"locality": "Corrientes, Argentina", "issue": "cucdmis,gass84,zerocd"}},
		{X: -48.5, Y: -23.5, ID: "r1"},
	}
}

func auditRecords(p []Point) []AuditRecord {
	r := make([]AuditRecord, len(p))
	for i := range p {
		r[i] = AuditRecord{Point: p[i], SourceRow: i + 2}
	}
	return r
}

func issuesFor(r AuditReport, code string) []AuditIssue {
	var out []AuditIssue
	for _, is := range r.Issues {
		if is.Code == code {
			out = append(out, is)
		}
	}
	return out
}

func TestAuditZeroZeroAndProvenance(t *testing.T) {
	p := zeroZeroPoints()
	r, err := AuditOccurrences(context.Background(), auditRecords(p), AuditConfig{SourceFlagField: "issue", ContextFields: []string{"locality"}})
	if err != nil {
		t.Fatal(err)
	}
	z := issuesFor(r, IssueZeroZero)
	if len(z) != 1 || *z[0].InputIndex != 5 || *z[0].SourceRow != 7 || z[0].RecordID != "r6" || z[0].Severity != "warning" {
		t.Fatalf("zero-zero not attributed: %+v", z)
	}
	if ctx, _ := z[0].Evidence["context"].(map[string]any); ctx["locality"] != "Corrientes, Argentina" {
		t.Fatalf("locality not preserved: %+v", z[0].Evidence)
	}
	flags := issuesFor(r, IssueSourceQualityFlag)
	var names []string
	for _, f := range flags {
		names = append(names, f.Evidence["flag"].(string))
	}
	if strings.Join(names, ",") != "cucdmis,gass84,zerocd" {
		t.Fatalf("source flags not preserved verbatim: %v", names)
	}
	if r.InvalidRecords != 0 || !r.CalculationCompatible || r.InputSHA256 == nil || r.DuplicateRecordIDs != 2 || r.ValidUniqueCoords != 6 {
		t.Fatalf("%+v", r)
	}
	// Audit must hash exactly as Calculate does, and calc must keep the record.
	c := mustCalc(t, p, Config{ProjectionStrategy: ProjectionIUCNCEA})
	if c.InputSHA256 != *r.InputSHA256 || c.InputRecords != len(p) || c.UniqueCoordinates != 6 {
		t.Fatalf("calc changed the record set: %+v", c)
	}
	found := false
	for _, w := range c.Warnings {
		found = found || strings.HasPrefix(w, "ZERO_ZERO_COORDINATE: 1 record")
	}
	if !found {
		t.Fatalf("calc did not warn about (0,0): %v", c.Warnings)
	}
}

func TestAuditDoesNotFlagSingleZeroAxis(t *testing.T) {
	p := []Point{{X: 0, Y: -10.25, ID: "greenwich"}, {X: -45.75, Y: 0, ID: "equator"}, {X: 0, Y: 51.48, ID: "observatory"}}
	r, err := AuditOccurrences(context.Background(), auditRecords(p), AuditConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Issues) != 0 {
		t.Fatalf("legitimate zero-axis coordinates flagged: %+v", r.Issues)
	}
	c := mustCalc(t, p, Config{})
	for _, w := range c.Warnings {
		if strings.HasPrefix(w, "ZERO_ZERO") {
			t.Fatal(w)
		}
	}
}

func TestAuditInvalidAndOptionalRules(t *testing.T) {
	two := 2
	recs := []AuditRecord{
		{Point: Point{X: -48.512, Y: -23.517, ID: "ok", Properties: map[string]any{"unc": "30"}}, SourceRow: 2},
		{Point: Point{X: math.NaN(), Y: -20, ID: "missing"}, SourceRow: 3, ParseError: "lon/X: missing or non-numeric coordinate"},
		{Point: Point{X: -47, Y: -91, ID: "range", Properties: map[string]any{"unc": "NA"}}, SourceRow: 4},
		{Point: Point{X: -41.25, Y: -12.5, ID: "coarse", Properties: map[string]any{"unc": 5000.0}}, SourceRow: 5},
		{Point: Point{ID: "row"}, SourceRow: 6, ParseError: "record: expected 4 fields, found 3"},
	}
	r, err := AuditOccurrences(context.Background(), recs, AuditConfig{CoarseDecimals: &two, UncertaintyField: "unc"})
	if err != nil {
		t.Fatal(err)
	}
	if r.InvalidRecords != 3 || r.CalculationCompatible || r.InputSHA256 != nil {
		t.Fatalf("%+v", r)
	}
	if r.CountsByCode[IssueInvalidCoordinate] != 2 || r.CountsByCode[IssueInvalidRecord] != 1 {
		t.Fatalf("format and coordinate errors not separated: %v", r.CountsByCode)
	}
	if c := issuesFor(r, IssueCoarsePrecision); len(c) != 1 || c[0].RecordID != "coarse" {
		t.Fatalf("coarse precision: %+v", c)
	}
	if u := issuesFor(r, IssueUncertaintyExceedCell); len(u) != 1 || u[0].RecordID != "coarse" {
		t.Fatalf("uncertainty: %+v", u)
	}
	// Missing records (NA) and records without the field are unknown, not flagged.
	if r.UncertaintyUnknown == nil || *r.UncertaintyUnknown != 3 {
		t.Fatalf("unknown uncertainty count: %v", r.UncertaintyUnknown)
	}
	// Without explicit configuration the optional rules stay off.
	r, err = AuditOccurrences(context.Background(), recs, AuditConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if r.CountsByCode[IssueCoarsePrecision]+r.CountsByCode[IssueUncertaintyExceedCell] != 0 || r.UncertaintyUnknown != nil {
		t.Fatalf("optional rules ran without configuration: %v", r.CountsByCode)
	}
}

func TestAuditCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := AuditOccurrences(ctx, auditRecords(zeroZeroPoints()), AuditConfig{}); err == nil {
		t.Fatal("cancelled audit returned a report")
	}
}

func zeroZeroManifest(t *testing.T, p []Point) CurationManifest {
	h, err := InputHash(p)
	if err != nil {
		t.Fatal(err)
	}
	id, lon, lat := "r6", 0.0, 0.0
	return CurationManifest{ManifestVersion: "1", InputSHA256: h, Decisions: []CurationDecision{{InputIndex: 5, RecordID: &id, Lon: &lon, Lat: &lat, Action: "exclude", Justification: "(0,0) with locality in Argentina"}}}
}

func TestCurationExcludesOnlyWithMatchingManifest(t *testing.T) {
	p := zeroZeroPoints()
	m := zeroZeroManifest(t, p)
	out, log, err := ApplyCuration(p, m)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != len(p)-1 || len(p) != 7 || log.OutputRecords != 6 || len(log.Excluded) != 1 || log.Excluded[0].RecordID != "r6" || log.Excluded[0].Record.Properties["locality"] != "Corrientes, Argentina" {
		t.Fatalf("%+v", log)
	}
	if h, _ := InputHash(out); h != log.OutputSHA256 || log.InputSHA256 != m.InputSHA256 || log.ManifestSHA256 == "" {
		t.Fatal("hash chain broken")
	}
	before := mustCalc(t, p, Config{ProjectionStrategy: ProjectionIUCNCEA})
	after := mustCalc(t, out, Config{ProjectionStrategy: ProjectionIUCNCEA})
	if *after.EOO.RawAreaKM2 >= *before.EOO.RawAreaKM2/2 || after.InputSHA256 != log.OutputSHA256 {
		t.Fatalf("curated EOO %v vs original %v", *after.EOO.RawAreaKM2, *before.EOO.RawAreaKM2)
	}

	bad := func(name string, edit func(*CurationManifest), pts []Point) {
		t.Helper()
		mm := zeroZeroManifest(t, p)
		mm.Decisions = append([]CurationDecision(nil), mm.Decisions...)
		edit(&mm)
		if _, _, err := ApplyCuration(pts, mm); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
	other := append([]Point(nil), p...)
	other[0].ID = "changed"
	bad("different input", func(*CurationManifest) {}, other)
	bad("wrong id", func(m *CurationManifest) { s := "r5"; m.Decisions[0].RecordID = &s }, p)
	bad("wrong coordinates", func(m *CurationManifest) { v := 1.0; m.Decisions[0].Lon = &v }, p)
	bad("index only", func(m *CurationManifest) {
		m.Decisions[0] = CurationDecision{InputIndex: 5, Action: "exclude", Justification: "x"}
	}, p)
	bad("duplicate selector", func(m *CurationManifest) { m.Decisions = append(m.Decisions, m.Decisions[0]) }, p)
	bad("no justification", func(m *CurationManifest) { m.Decisions[0].Justification = " " }, p)
	bad("unknown action", func(m *CurationManifest) { m.Decisions[0].Action = "move" }, p)
	bad("out of range", func(m *CurationManifest) { m.Decisions[0].InputIndex = 99 }, p)
	bad("version", func(m *CurationManifest) { m.ManifestVersion = "2" }, p)
}

func TestEOOInfluenceFrozenCRS(t *testing.T) {
	p := zeroZeroPoints()
	r, err := AnalyzeEOOInfluence(context.Background(), p, Config{}, InfluenceConfig{RecomputeProjection: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "complete" || !r.RankingComplete || r.EvaluatedGroups != r.HullVertexGroups || len(r.Groups) == 0 {
		t.Fatalf("%+v", r)
	}
	top := r.Groups[0]
	if top.Longitude != 0 || top.Latitude != 0 || top.RecordIDs[0] != "r6" || top.RelativeReductionPct == nil || *top.RelativeReductionPct < 50 {
		t.Fatalf("(0,0) is not the most influential vertex: %+v", top)
	}
	// Frozen-CRS removal is reported separately from recentring local-laea.
	if top.RecomputedRawEOOKM2 == nil || *top.RecomputedRawEOOKM2 == *top.RawEOOWithoutKM2 {
		t.Fatalf("recomputed projection not distinguished: %+v", top)
	}
	for i := 1; i < len(r.Groups); i++ {
		if *r.Groups[i].RelativeReductionPct > *r.Groups[i-1].RelativeReductionPct {
			t.Fatal("groups not ordered by relative reduction")
		}
	}
	// Influence never excludes: the calculation still sees every record.
	if c := mustCalc(t, p, Config{}); c.InputRecords != len(p) {
		t.Fatal("record set changed")
	}
}

func TestEOOInfluenceDuplicatesAndDegenerate(t *testing.T) {
	p := []Point{{X: 10, Y: 10, ID: "a"}, {X: 10.5, Y: 10, ID: "b"}, {X: 10, Y: 10.5, ID: "c"}, {X: 10, Y: 10.5, ID: "c2"}}
	r, err := AnalyzeEOOInfluence(context.Background(), p, Config{}, InfluenceConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if r.HullVertexGroups != 3 || len(r.Groups) != 3 {
		t.Fatalf("%+v", r)
	}
	for _, g := range r.Groups {
		// Removing any vertex of a triangle leaves two points: no hull, no delta.
		if g.RawEOOWithoutKM2 != nil || g.ReductionKM2 != nil || g.RelativeReductionPct != nil || g.StatusWithout != "insufficient_unique_points" {
			t.Fatalf("invented delta for degenerate hull: %+v", g)
		}
		if g.Latitude == 10.5 && (len(g.InputIndices) != 2 || strings.Join(g.RecordIDs, ",") != "c,c2") {
			t.Fatalf("duplicate group not kept as one unit: %+v", g)
		}
	}
	r, err = AnalyzeEOOInfluence(context.Background(), p[:2], Config{}, InfluenceConfig{})
	if err != nil || r.Status != "unavailable" || len(r.Groups) != 0 {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestEOOInfluencePartialBudget(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	var p []Point
	for i := 0; i < 40; i++ {
		a := rng.Float64() * 2 * math.Pi
		p = append(p, Point{X: -50 + 3*math.Cos(a), Y: -20 + 3*math.Sin(a)})
	}
	r, err := AnalyzeEOOInfluence(context.Background(), p, Config{}, InfluenceConfig{MaxCandidates: 5, TopK: 2})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "partial" || r.RankingComplete || r.EvaluatedGroups != 5 || r.HullVertexGroups <= 5 || len(r.Groups) != 2 {
		t.Fatalf("partial budget not explicit: %+v", r)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := AnalyzeEOOInfluence(ctx, p, Config{}, InfluenceConfig{}); err == nil {
		t.Fatal("cancelled influence analysis returned a report")
	}
}

func TestProtocolRoundTripAndFreeze(t *testing.T) {
	p := zeroZeroPoints()[:5]
	for _, strategy := range []string{ProjectionLocalLAEA, ProjectionIUCNCEA} {
		orig := mustCalc(t, p, Config{ProjectionStrategy: strategy})
		proto := *orig.Protocol
		if orig.ProtocolSHA256 != proto.SHA256() || proto.GridMode != "fixed" {
			t.Fatal(proto)
		}
		rep, err := CalculateWithProtocol(context.Background(), p, proto, Config{})
		if err != nil {
			t.Fatal(err)
		}
		if *rep.EOO.RawAreaKM2 != *orig.EOO.RawAreaKM2 || rep.AOO.OccupiedCells != orig.AOO.OccupiedCells || rep.AOO.Origin != orig.AOO.Origin || rep.ProtocolSHA256 != orig.ProtocolSHA256 {
			t.Fatalf("%s: reproduction differs: %+v vs %+v", strategy, rep.AOO, orig.AOO)
		}
		// Added records must not move the centre or the grid origin.
		more := append(append([]Point(nil), p...), Point{X: -70, Y: -40})
		grown, err := CalculateWithProtocol(context.Background(), more, proto, Config{})
		if err != nil {
			t.Fatal(err)
		}
		if grown.Projection.AnalysisPROJ != orig.Projection.AnalysisPROJ || grown.AOO.Origin != orig.AOO.Origin || grown.ProtocolSHA256 != orig.ProtocolSHA256 {
			t.Fatalf("%s: frozen protocol drifted", strategy)
		}
		if auto := mustCalc(t, more, Config{ProjectionStrategy: strategy}); strategy == ProjectionLocalLAEA && auto.Projection.AnalysisPROJ == orig.Projection.AnalysisPROJ {
			t.Fatal("test does not exercise centre drift")
		}
		// Incompatible settings fail before calculation.
		for name, c := range map[string]Config{
			"cell":       {CellSizeM: 1000},
			"projection": {ProjectionStrategy: map[string]string{ProjectionLocalLAEA: ProjectionIUCNCEA, ProjectionIUCNCEA: ProjectionLocalLAEA}[strategy]},
			"grid":       {GridMode: "exact"},
			"origin":     {Origin: [2]float64{1, 2}},
		} {
			if _, err := CalculateWithProtocol(context.Background(), p, proto, c); err == nil {
				t.Fatalf("%s: incompatible %s accepted", strategy, name)
			}
		}
		bad := proto
		bad.Orientation = "rotated"
		if _, err := ApplyProtocol(Config{}, bad); err == nil {
			t.Fatal("incompatible orientation accepted")
		}
		bad = proto
		bad.AnalysisPROJ += " +x_0=1"
		if _, err := ApplyProtocol(Config{}, bad); err == nil {
			t.Fatal("tampered analysis_proj accepted")
		}
	}
}

func TestProtocolEnvironmentChangeIsReported(t *testing.T) {
	p := zeroZeroPoints()[:5]
	proto := *mustCalc(t, p, Config{}).Protocol
	proto.Environment.PROJVersion = "0.0.1"
	r, err := CalculateWithProtocol(context.Background(), p, proto, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(r.Warnings, "\n"), "PROTOCOL_ENVIRONMENT_CHANGED") {
		t.Fatal(r.Warnings)
	}
}

func TestGeometryHashIgnoresOrderAndMetadata(t *testing.T) {
	p := zeroZeroPoints()
	a := mustCalc(t, p, Config{})
	q := append([]Point(nil), p...)
	q[0], q[3] = q[3], q[0]
	q[1].ID = "renamed"
	b := mustCalc(t, q, Config{})
	if a.InputSHA256 == b.InputSHA256 || a.GeometrySHA256 != b.GeometrySHA256 {
		t.Fatal("input hash must follow records; geometry hash must not")
	}
	// -0 and 180/-180 normalize to the same geometry.
	c := mustCalc(t, []Point{{X: 180, Y: 10}, {X: 179, Y: 11}, {X: -179, Y: 9}}, Config{})
	d := mustCalc(t, []Point{{X: -180, Y: 10}, {X: 179, Y: 11}, {X: -179, Y: 9}}, Config{})
	e := mustCalc(t, []Point{{X: math.Copysign(0, -1), Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 2}}, Config{})
	f := mustCalc(t, []Point{{X: 0, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 2}}, Config{})
	if c.GeometrySHA256 != d.GeometrySHA256 || e.GeometrySHA256 != f.GeometrySHA256 {
		t.Fatal("normalization not reflected in geometry hash")
	}
	if c.Extent.LonSpanDeg > 2.0000001 {
		t.Fatalf("antimeridian longitude span %v", c.Extent.LonSpanDeg)
	}
}

func TestGeodesicDiagnosticAndExtentWarning(t *testing.T) {
	small := []Point{{X: -43.2, Y: -22.9}, {X: -44.3, Y: -21.5}, {X: -42.9, Y: -20.8}, {X: -45, Y: -22}}
	r := mustCalc(t, small, Config{})
	if r.EOO.GeodesicAreaKM2 == nil || math.Abs(*r.EOO.GeodesicAreaKM2 / *r.EOO.RawAreaKM2 - 1) > 0.01 || r.EOO.Method == "" {
		t.Fatalf("geodesic diagnostic of a small hull should be within 1%% of the planar area: %+v", r.EOO)
	}
	for _, w := range r.Warnings {
		if strings.HasPrefix(w, "PROJECTION_SENSITIVE_EXTENT") {
			t.Fatal(w)
		}
	}
	wide := mustCalc(t, zeroZeroPoints(), Config{})
	if !strings.Contains(strings.Join(wide.Warnings, "\n"), "PROJECTION_SENSITIVE_EXTENT") {
		t.Fatal(wide.Warnings)
	}
	if quiet := mustCalc(t, zeroZeroPoints(), Config{ExtentWarningDeg: 90}); strings.Contains(strings.Join(quiet.Warnings, "\n"), "PROJECTION_SENSITIVE_EXTENT") {
		t.Fatal("threshold not configurable")
	}
	if _, err := Calculate(context.Background(), small, Config{ExtentWarningDeg: -1}); err == nil {
		t.Fatal("negative threshold accepted")
	}
}

func TestCompareFlagsDifferentSearchCoverage(t *testing.T) {
	p := make([]Point, 60)
	rng := rand.New(rand.NewSource(4))
	for i := range p {
		p[i] = Point{X: -50 + rng.Float64()*8, Y: -20 + rng.Float64()*8}
	}
	same, err := CompareProjections(context.Background(), p, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if len(same.Warnings) != 0 {
		t.Fatalf("same coverage flagged: %+v", same.Warnings)
	}
	exact := AOOResult{EffectiveGridMode: "exact", SearchGuarantee: GuaranteeTranslationMinimum}
	sampled := AOOResult{EffectiveGridMode: "sampled", SearchGuarantee: GuaranteeTestedOriginsMinimum, FallbackReason: FallbackExactWorkLimit}
	codes := map[string]string{}
	for _, w := range comparisonWarnings(Result{AOO: sampled}, Result{AOO: exact}) {
		codes[w.Code] = w.Message
	}
	if codes["GRID_SEARCH_COVERAGE_DIFFERS"] == "" || !strings.Contains(codes["SAMPLED_GRID"], FallbackExactWorkLimit) {
		t.Fatalf("coverage difference hidden: %v", codes)
	}
}
