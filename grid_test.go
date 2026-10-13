package eooaoo

import (
	"context"
	"math/rand"
	"strconv"
	"testing"
)

func formatFloat(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }

func TestSinglePointCellCentered(t *testing.T) {
	for _, mode := range []string{"auto", "exact", "sampled"} {
		for _, width := range []float64{1000, 2000} {
			p := xy{-4321.25, 8765.5}
			a, cells, err := calculateGrid(context.Background(), []xy{p}, Config{CellSizeM: width, GridMode: mode, GridSteps: 10, Origin: [2]float64{17, -31}})
			if err != nil {
				t.Fatal(err)
			}
			want := [4]float64{p.x - width/2, p.y - width/2, p.x + width/2, p.y + width/2}
			if len(cells) != 1 || cells[0].Bounds != want || a.AreaKM2 != width*width/1e6 {
				t.Fatalf("mode %s width %v: expected centered bounds %v, got %+v %+v", mode, width, want, a, cells)
			}
		}
	}
}

func TestSinglePointFixedOriginPreserved(t *testing.T) {
	origin := [2]float64{100, 200}
	a, cells, err := calculateGrid(context.Background(), []xy{{150, 250}}, Config{CellSizeM: 2000, GridMode: "fixed", Origin: origin})
	if err != nil {
		t.Fatal(err)
	}
	if a.Origin != origin || len(cells) != 1 || cells[0].Bounds != [4]float64{100, 200, 2100, 2200} {
		t.Fatal(a, cells)
	}
}

func TestGridBoundaryNegativeAndPositive(t *testing.T) {
	p := []xy{{-2000, -2000}, {-0.1, -0.1}, {0, 0}, {1999.999, 1999.999}, {2000, 2000}}
	a, c, e := calculateGrid(context.Background(), p, Config{CellSizeM: 2000, GridMode: "fixed"})
	if e != nil {
		t.Fatal(e)
	}
	if a.OccupiedCells != 3 || a.AreaKM2 != 12 || c[0].UniquePoints != 2 || c[1].UniquePoints != 2 || c[2].UniquePoints != 1 {
		t.Fatal(a, c)
	}
}

func TestExactSearchFindsNarrowInterval(t *testing.T) {
	p := []xy{{10, 1000}, {2009, 1000}}
	a, _, e := calculateGrid(context.Background(), p, Config{CellSizeM: 2000, GridMode: "exact", GridSteps: 10})
	if e != nil {
		t.Fatal(e)
	}
	b, _, e := calculateGrid(context.Background(), p, Config{CellSizeM: 2000, GridMode: "sampled", GridSteps: 10})
	if e != nil {
		t.Fatal(e)
	}
	if a.OccupiedCells != 1 || !a.CompleteTranslationSearch || b.OccupiedCells != 2 || b.CompleteTranslationSearch {
		t.Fatal(a, b)
	}
}

func TestExactAgainstIndependentDenseOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(123))
	for trial := 0; trial < 30; trial++ {
		p := make([]xy, 6)
		for i := range p {
			p[i] = xy{float64(rng.Intn(50) - 25), float64(rng.Intn(50) - 25)}
		}
		c := Config{CellSizeM: 10, GridMode: "exact", Origin: [2]float64{-31, 17}}
		a, _, e := calculateGrid(context.Background(), p, c)
		if e != nil {
			t.Fatal(e)
		}
		best := len(p) + 1
		// Integer coordinates: all occupancy intervals have integer endpoints.
		// Independent all-pairs floor enumeration on half-integer offsets.
		for x := 0; x < 10; x++ {
			for y := 0; y < 10; y++ {
				m := map[[2]int]bool{}
				for _, q := range p {
					ix := intFloor((q.x + 31 - float64(x) - 0.5) / 10)
					iy := intFloor((q.y - 17 - float64(y) - 0.5) / 10)
					m[[2]int{ix, iy}] = true
				}
				if len(m) < best {
					best = len(m)
				}
			}
		}
		if a.OccupiedCells != best {
			t.Fatalf("trial %d: exact=%d oracle=%d", trial, a.OccupiedCells, best)
		}
	}
}
func intFloor(f float64) int {
	v := int(f)
	if float64(v) > f {
		v--
	}
	return v
}

func withWorkLimit(t *testing.T, limit int64) {
	t.Helper()
	old := gridWorkLimit
	gridWorkLimit = limit
	t.Cleanup(func() { gridWorkLimit = old })
}

func TestAutoFallbackAndWorkLimit(t *testing.T) {
	p := make([]xy, 400)
	for i := range p {
		p[i] = xy{float64(i) * 3.1, float64(i) * 4.3}
	}
	withWorkLimit(t, 50000)
	a, _, e := calculateGrid(context.Background(), p, Config{CellSizeM: 2000, GridMode: "auto", GridSteps: 10})
	if e != nil {
		t.Fatal(e)
	}
	if a.GridMode != "sampled" || a.EffectiveGridMode != "sampled" || a.RequestedGridMode != "auto" || a.Candidates != 100 {
		t.Fatal(a)
	}
	if a.FallbackReason != FallbackExactWorkLimit || a.ExactWorkEstimate <= gridWorkLimit || a.SearchGuarantee != GuaranteeTestedOriginsMinimum || a.SampledSteps == nil || *a.SampledSteps != 10 {
		t.Fatalf("fallback not disclosed: %+v", a)
	}
	if _, _, e = calculateGrid(context.Background(), p, Config{CellSizeM: 2000, GridMode: "exact"}); e == nil {
		t.Fatal("exact budget not enforced")
	}
}

func TestSearchGuaranteeMetadata(t *testing.T) {
	p := []xy{{10, 1000}, {2009, 1000}, {5000, -300}}
	for _, tc := range []struct{ mode, effective, guarantee string }{
		{"auto", "exact", GuaranteeTranslationMinimum},
		{"exact", "exact", GuaranteeTranslationMinimum},
		{"sampled", "sampled", GuaranteeTestedOriginsMinimum},
		{"fixed", "fixed", GuaranteeFixedOrigin},
	} {
		a, _, e := calculateGrid(context.Background(), p, Config{CellSizeM: 2000, GridMode: tc.mode, GridSteps: 4})
		if e != nil {
			t.Fatal(e)
		}
		if a.RequestedGridMode != tc.mode || a.EffectiveGridMode != tc.effective || a.SearchGuarantee != tc.guarantee || a.FallbackReason != "" {
			t.Fatalf("%s: %+v", tc.mode, a)
		}
		if (a.SampledSteps != nil) != (tc.effective == "sampled") {
			t.Fatalf("%s: grid_steps_effective must only describe sampled searches: %+v", tc.mode, a)
		}
		if a.TestedMinCells != a.OccupiedCells || a.TestedMaxCells < a.TestedMinCells || a.BaseOriginCells < a.TestedMinCells || a.BaseOriginCells > a.TestedMaxCells {
			t.Fatalf("%s: inconsistent tested range: %+v", tc.mode, a)
		}
		if a.BaseOriginAreaKM2 != float64(a.BaseOriginCells)*4 || a.EstimatedPointEvaluations <= 0 {
			t.Fatalf("%s: %+v", tc.mode, a)
		}
	}
}

// Exact search covers every translation at fixed orientation, so no sampled
// origin over the same CRS, orientation and base origin can do better.
func TestExactNeverWorseThanSampled(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for trial := 0; trial < 40; trial++ {
		p := make([]xy, 2+rng.Intn(40))
		for i := range p {
			p[i] = xy{rng.Float64()*20000 - 10000, rng.Float64()*20000 - 10000}
		}
		c := Config{CellSizeM: 2000, Origin: [2]float64{rng.Float64() * 5000, -rng.Float64() * 5000}}
		c.GridMode = "exact"
		ex, _, err := calculateGrid(context.Background(), p, c)
		if err != nil {
			t.Fatal(err)
		}
		for _, steps := range []int{1, 3, 10, 37} {
			c.GridMode, c.GridSteps = "sampled", steps
			s, _, err := calculateGrid(context.Background(), p, c)
			if err != nil {
				t.Fatal(err)
			}
			if ex.OccupiedCells > s.OccupiedCells || ex.TestedMaxCells < s.TestedMaxCells {
				t.Fatalf("trial %d steps %d: exact [%d,%d] sampled [%d,%d]", trial, steps, ex.OccupiedCells, ex.TestedMaxCells, s.OccupiedCells, s.TestedMaxCells)
			}
		}
	}
}
