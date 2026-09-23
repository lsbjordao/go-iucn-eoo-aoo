package eooaoo

import (
	"context"
	"math"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

func mustCalc(t *testing.T, p []Point, c Config) Result {
	t.Helper()
	r, e := Calculate(context.Background(), p, c)
	if e != nil {
		t.Fatal(e)
	}
	return r
}

func TestSingleAndDuplicateProvenance(t *testing.T) {
	p := []Point{{X: -47.584853, Y: -14.096598, ID: "a"}, {X: -47.584853, Y: -14.096598, ID: "b"}}
	r := mustCalc(t, p, Config{})
	if r.AOO.AreaKM2 != 4 || r.AOO.OccupiedCells != 1 || r.UniqueCoordinates != 1 || r.DuplicateRecords != 1 {
		t.Fatalf("bad counts: %+v", r)
	}
	if r.EOO.RawAreaKM2 != nil || r.EOO.AssessmentAreaKM2 != 4 || r.EOO.HullWKT != "" {
		t.Fatal(r.EOO)
	}
	if len(r.Points[0].Records) != 2 || r.Points[0].Records[1].ID != "b" {
		t.Fatal("lost duplicate provenance")
	}
	if r.Projection.Strategy != ProjectionLocalLAEA {
		t.Fatal(r.Projection.Strategy)
	}
}

func TestSingleCoordinateCenteredAcrossProjections(t *testing.T) {
	for _, projection := range []string{ProjectionLocalLAEA, ProjectionIUCNCEA} {
		for _, count := range []int{1, 2} {
			points := []Point{{X: -43.2, Y: -22.9}, {X: -43.2, Y: -22.9}}
			r := mustCalc(t, points[:count], Config{ProjectionStrategy: projection})
			p := r.Points[0]
			if len(r.Cells) != 1 {
				t.Fatal(r.Cells)
			}
			b := r.Cells[0].Bounds
			if math.Abs((b[0]+b[2])/2-p.X) > 1e-8 || math.Abs((b[1]+b[3])/2-p.Y) > 1e-8 || r.AOO.AreaKM2 != 4 {
				t.Fatalf("projection %s records %d: point %+v is not centered in %+v", projection, count, p, r.Cells)
			}
			if r.AOO.Origin != [2]float64{b[0], b[1]} || r.AOO.Candidates != 1 || r.AOO.MinCells != 1 || r.AOO.MaxCells != 1 {
				t.Fatal(r.AOO)
			}
		}
	}
}

func TestKnownEqualAreaSquares(t *testing.T) {
	for _, lat := range []float64{0, -23, 70, -85} {
		t.Run(strings.ReplaceAll(strings.TrimSpace(formatFloat(lat)), "-", "south"), func(t *testing.T) {
			center := [2]float64{30, lat}
			sr, def, e := localLAEARef(center)
			if e != nil {
				t.Fatal(e)
			}
			sr.Close()
			p := []Point{{X: 100, Y: 100}, {X: 10100, Y: 100}, {X: 10100, Y: 10100}, {X: 100, Y: 10100}}
			r := mustCalc(t, p, Config{InputCRS: def, Center: &center})
			if r.EOO.RawAreaKM2 == nil || math.Abs(*r.EOO.RawAreaKM2-100) > 1e-5 {
				t.Fatalf("expected 100 km² at latitude %v: %+v", lat, r.EOO)
			}
			if r.AOO.AreaKM2 != 16 {
				t.Fatal(r.AOO)
			}
		})
	}
}

func TestIUCNCEAProjection(t *testing.T) {
	p := []Point{{X: -43.2, Y: -22.9}, {X: -44.3, Y: -21.5}, {X: -42.9, Y: -20.8}}
	r := mustCalc(t, p, Config{ProjectionStrategy: ProjectionIUCNCEA})
	if r.Projection.Strategy != ProjectionIUCNCEA || !strings.Contains(r.Projection.AnalysisPROJ, "+proj=cea") {
		t.Fatal(r.Projection)
	}
	if r.EOO.RawAreaKM2 == nil || r.AOO.AreaKM2 <= 0 {
		t.Fatal(r)
	}
}

func TestAntimeridianAndPolar(t *testing.T) {
	for _, p := range [][]Point{
		{{X: 179.9, Y: 10}, {X: -179.9, Y: 10}, {X: -179.9, Y: 10.1}, {X: 179.9, Y: 10.1}},
		{{X: 0, Y: 89}, {X: 90, Y: 89}, {X: 180, Y: 89}, {X: -90, Y: 89}},
	} {
		r := mustCalc(t, p, Config{})
		if r.EOO.RawAreaKM2 == nil || *r.EOO.RawAreaKM2 > 100000 {
			t.Fatalf("inflated geographic hull: %+v", r.EOO)
		}
	}
	r := mustCalc(t, []Point{{X: 180, Y: 0}, {X: -180, Y: 0}}, Config{})
	if r.UniqueCoordinates != 1 {
		t.Fatal("antimeridian equivalence lost")
	}
}

func TestInvalidAndGlobalInput(t *testing.T) {
	for _, p := range [][]Point{nil, {{X: 181, Y: 0}}, {{X: 0, Y: 91}}, {{X: math.NaN(), Y: 0}}, {{X: math.Inf(1), Y: 0}}, {{X: 0, Y: 0}, {X: 180, Y: 0}}, {{X: 0, Y: 0, Taxon: "a"}, {X: 1, Y: 1, Taxon: "b"}}} {
		if _, e := Calculate(context.Background(), p, Config{}); e == nil {
			t.Fatalf("accepted invalid input %+v", p)
		}
	}
	center := [2]float64{0, 0}
	for _, c := range []Config{{CellSizeM: -1}, {GridMode: "best"}, {GridSteps: -1}, {InputCRS: "https://example.org/file"}, {Origin: [2]float64{math.Inf(1), 0}}, {ProjectionStrategy: "unknown"}, {ProjectionStrategy: ProjectionIUCNCEA, Center: &center}} {
		if _, e := Calculate(context.Background(), []Point{{X: 0, Y: 0}}, c); e == nil {
			t.Fatal("accepted invalid config")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Calculate(ctx, []Point{{X: 0, Y: 0}}, Config{}); e == nil {
		t.Fatal("ignored cancellation")
	}
}

func TestCollinearAndEOOFloor(t *testing.T) {
	c := [2]float64{0, 0}
	r := mustCalc(t, []Point{{X: -0.1, Y: 0}, {X: 0, Y: 0}, {X: 0.1, Y: 0}}, Config{Center: &c})
	if r.EOO.RawAreaKM2 != nil || r.EOO.Status != "collinear" || r.EOO.AssessmentAreaKM2 != r.AOO.AreaKM2 {
		t.Fatal(r.EOO)
	}
	r = mustCalc(t, []Point{{X: 0, Y: 0}, {X: 0.0001, Y: 0}, {X: 0, Y: 0.0001}}, Config{})
	if r.EOO.RawAreaKM2 == nil || *r.EOO.RawAreaKM2 >= 4 || r.EOO.AssessmentAreaKM2 != 4 {
		t.Fatal(r.EOO)
	}
}

func TestOrderAndDuplicatesDoNotMoveProjection(t *testing.T) {
	p := []Point{{X: -43.2, Y: -22.9}, {X: -44.3, Y: -21.5}, {X: -42.9, Y: -20.8}}
	a := mustCalc(t, p, Config{})
	p = append(p[1:], p[0], p[0])
	b := mustCalc(t, p, Config{})
	if a.Projection != b.Projection || a.AOO != b.AOO || !reflect.DeepEqual(a.Cells, b.Cells) {
		t.Fatal("record order or duplicates changed geometry")
	}
}

func TestTwoPointsAndCustomScale(t *testing.T) {
	r := mustCalc(t, []Point{{X: -43, Y: -22}, {X: -44, Y: -23}}, Config{CellSizeM: 1000, GridMode: "fixed"})
	if r.AOO.AreaKM2 != 2 || r.AOO.ReferenceScale || r.EOO.RawAreaKM2 != nil {
		t.Fatal(r)
	}
}

func BenchmarkCalculate1000(b *testing.B) {
	rng := rand.New(rand.NewSource(42))
	p := make([]Point, 1000)
	for i := range p {
		p[i] = Point{X: -50 + rng.Float64()*10, Y: -25 + rng.Float64()*10}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, e := Calculate(context.Background(), p, Config{}); e != nil {
			b.Fatal(e)
		}
	}
}
