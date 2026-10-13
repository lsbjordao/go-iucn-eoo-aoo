package eooaoo

import (
	"context"
	"fmt"
	"math"
	"sort"
)

type xy struct{ x, y float64 }
type cellKey struct{ x, y int64 }

// Grid search modes and guarantees reported in AOOResult.
const (
	GuaranteeTranslationMinimum   = "translation_minimum"
	GuaranteeTestedOriginsMinimum = "tested_origins_minimum"
	GuaranteeFixedOrigin          = "fixed_origin"

	// FallbackExactWorkLimit means auto mode estimated more work for the
	// complete translation search than gridWorkLimit allows.
	FallbackExactWorkLimit = "EXACT_WORK_LIMIT"
)

func phase(v, w float64) float64 {
	v = math.Mod(v, w)
	if v < 0 {
		v += w
	}
	if v == w {
		return 0
	}
	return v
}

// Every open interval between coordinate residues is a constant-occupancy
// region. Its midpoint avoids boundary round-off. Event values have the same
// occupancy as an adjacent interval under the documented half-open rule.
func exactOffsets(p []xy, w, base float64, axis int) []float64 {
	r := make([]float64, len(p))
	for i, v := range p {
		a := v.x
		if axis == 1 {
			a = v.y
		}
		r[i] = phase(a-base, w)
	}
	sort.Float64s(r)
	u := r[:0]
	for _, v := range r {
		if len(u) == 0 || v != u[len(u)-1] {
			u = append(u, v)
		}
	}
	o := []float64{0}
	for i, a := range u {
		b := u[(i+1)%len(u)]
		if i == len(u)-1 {
			b += w
		}
		if b > a {
			o = append(o, phase(a+(b-a)/2, w))
		}
	}
	sort.Float64s(o)
	return o
}

func sampledOffsets(w float64, steps int) []float64 {
	o := make([]float64, steps)
	for i := range o {
		o[i] = float64(i) * w / float64(steps)
	}
	return o
}

// cellIndex is the single assignment rule used by every search strategy.
// No arbitrary epsilon: points at a boundary belong to the right/top cell.
func cellIndex(v, origin, w float64) int64 { return int64(math.Floor((v - origin) / w)) }

func occupied(ctx context.Context, p []xy, w float64, o [2]float64) (map[cellKey]int, error) {
	m := make(map[cellKey]int, len(p))
	for i, v := range p {
		if err := checkContext(ctx, i); err != nil {
			return nil, err
		}
		m[cellKey{cellIndex(v.x, o[0], w), cellIndex(v.y, o[1], w)}]++
	}
	return m, nil
}

// transition records that point moves to cell index value at candidate at.
type transition struct {
	at    int
	point int
	value int64
}

// axisTransitions evaluates cellIndex(v, base+offsets[j], w) for ascending
// offsets. Floating-point subtraction, division and floor are monotone, so
// each point's index is non-increasing in j; binary search finds every change
// using exactly the arithmetic of a direct evaluation at each origin.
func axisTransitions(ctx context.Context, vals []float64, base, w float64, offsets []float64) ([]int64, []transition, error) {
	m := len(offsets)
	init := make([]int64, len(vals))
	var ev []transition
	for i, v := range vals {
		if err := checkContext(ctx, i); err != nil {
			return nil, nil, err
		}
		at := func(j int) int64 { return cellIndex(v, base+offsets[j], w) }
		cur, last, lo := at(0), at(m-1), 0
		init[i] = cur
		for cur != last {
			j := lo + 1 + sort.Search(m-1-lo, func(k int) bool { return at(lo+1+k) < cur })
			cur, lo = at(j), j
			ev = append(ev, transition{j, i, cur})
		}
	}
	sort.SliceStable(ev, func(a, b int) bool { return ev[a].at < ev[b].at })
	return init, ev, nil
}

type sweepPlan struct {
	xs, ys       []float64
	initX, initY []int64
	evX, evY     []transition
}

func newSweepPlan(ctx context.Context, p []xy, w float64, base [2]float64, xs, ys []float64) (sweepPlan, error) {
	vx, vy := make([]float64, len(p)), make([]float64, len(p))
	for i, v := range p {
		vx[i], vy[i] = v.x, v.y
	}
	s := sweepPlan{xs: xs, ys: ys}
	var err error
	if s.initX, s.evX, err = axisTransitions(ctx, vx, base[0], w, xs); err != nil {
		return s, err
	}
	s.initY, s.evY, err = axisTransitions(ctx, vy, base[1], w, ys)
	return s, err
}

// work counts cell-count updates: every X candidate rebuilds n counts and
// each Y event moves one point (one decrement and one increment).
func (s sweepPlan) work(n int) int64 {
	return int64(len(s.xs)) * (int64(n) + 2*int64(len(s.evY)))
}

type sweepResult struct {
	best, worst, base int
	bx, by            int
}

// run evaluates every (X, Y) candidate pair in lexicographic order, updating
// occupied-cell counts incrementally. Ties keep the first minimum, exactly as
// a nested loop over xs then ys would.
func (s sweepPlan) run(ctx context.Context) (sweepResult, error) {
	n := len(s.initX)
	cols := append([]int64(nil), s.initX...)
	rows := make([]int64, n)
	counts := make(map[cellKey]int32, n)
	r := sweepResult{best: n + 1}
	xe, steps := 0, 0
	for xi := range s.xs {
		for ; xe < len(s.evX) && s.evX[xe].at == xi; xe++ {
			cols[s.evX[xe].point] = s.evX[xe].value
		}
		copy(rows, s.initY)
		clear(counts)
		occ := 0
		for i := 0; i < n; i++ {
			k := cellKey{cols[i], rows[i]}
			if counts[k] == 0 {
				occ++
			}
			counts[k]++
		}
		ye := 0
		for yi := range s.ys {
			for ; ye < len(s.evY) && s.evY[ye].at == yi; ye++ {
				i := s.evY[ye].point
				old := cellKey{cols[i], rows[i]}
				if counts[old]--; counts[old] == 0 {
					delete(counts, old)
					occ--
				}
				rows[i] = s.evY[ye].value
				k := cellKey{cols[i], rows[i]}
				if counts[k] == 0 {
					occ++
				}
				counts[k]++
			}
			if xi == 0 && yi == 0 {
				r.base = occ
			}
			if occ > r.worst {
				r.worst = occ
			}
			if occ < r.best {
				r.best, r.bx, r.by = occ, xi, yi
			}
			if steps++; steps%1024 == 0 {
				if err := ctx.Err(); err != nil {
					return r, err
				}
			}
		}
	}
	return r, ctx.Err()
}

func calculateGrid(ctx context.Context, p []xy, c Config) (AOOResult, []Cell, error) {
	w := c.CellSizeM
	r := AOOResult{CellSizeM: w, ReferenceScale: w == 2000, RequestedGridMode: c.GridMode, Orientation: GridOrientation, BoundaryRule: GridBoundaryRule}
	mode := c.GridMode
	var plan sweepPlan
	var err error
	if mode == "auto" || mode == "exact" {
		plan, err = newSweepPlan(ctx, p, w, c.Origin, exactOffsets(p, w, c.Origin[0], 0), exactOffsets(p, w, c.Origin[1], 1))
		if err != nil {
			return r, nil, err
		}
		work := plan.work(len(p))
		if work > gridWorkLimit {
			if mode == "exact" {
				return r, nil, fmt.Errorf("exact grid search needs an estimated %d cell updates (limit %d); choose sampled", work, gridWorkLimit)
			}
			mode = "sampled"
			r.FallbackReason = FallbackExactWorkLimit
			r.ExactWorkEstimate = work
		} else {
			mode = "exact"
		}
	}
	if mode == "sampled" {
		o := sampledOffsets(w, c.GridSteps)
		plan, err = newSweepPlan(ctx, p, w, c.Origin, o, o)
		if err != nil {
			return r, nil, err
		}
		if plan.work(len(p)) > gridWorkLimit {
			return r, nil, fmt.Errorf("sampled grid search exceeds work limit; reduce grid_steps")
		}
		steps := c.GridSteps
		r.SampledSteps = &steps
	}
	if mode == "fixed" {
		plan, err = newSweepPlan(ctx, p, w, c.Origin, []float64{0}, []float64{0})
		if err != nil {
			return r, nil, err
		}
	}
	// With one unique point every translation ties. Prefer a square centered
	// on the point, unless the caller explicitly requested a fixed grid.
	if len(p) == 1 && mode != "fixed" {
		c.Origin = [2]float64{p[0].x - w/2, p[0].y - w/2}
		if plan, err = newSweepPlan(ctx, p, w, c.Origin, []float64{0}, []float64{0}); err != nil {
			return r, nil, err
		}
		r.BaseOriginCells = 1
	}
	r.EstimatedPointEvaluations = plan.work(len(p))
	s, err := plan.run(ctx)
	if err != nil {
		return r, nil, err
	}
	if len(p) != 1 || mode == "fixed" {
		r.BaseOriginCells = s.base
	}
	origin := [2]float64{c.Origin[0] + plan.xs[s.bx], c.Origin[1] + plan.ys[s.by]}
	m, err := occupied(ctx, p, w, origin)
	if err != nil {
		return r, nil, err
	}
	if len(m) != s.best {
		return r, nil, fmt.Errorf("internal grid search error: incremental count %d differs from direct count %d", s.best, len(m))
	}
	cells := make([]Cell, 0, len(m))
	for k, n := range m {
		x, y := origin[0]+float64(k.x)*w, origin[1]+float64(k.y)*w
		cells = append(cells, Cell{Column: k.x, Row: k.y, UniquePoints: n, Bounds: [4]float64{x, y, x + w, y + w}})
	}
	sort.Slice(cells, func(i, j int) bool {
		if cells[i].Row != cells[j].Row {
			return cells[i].Row < cells[j].Row
		}
		return cells[i].Column < cells[j].Column
	})
	r.AreaKM2 = float64(s.best) * w * w / 1e6
	r.OccupiedCells = s.best
	r.GridMode, r.EffectiveGridMode = mode, mode
	r.Origin = origin
	r.Candidates = len(plan.xs) * len(plan.ys)
	r.MinCells, r.MaxCells = s.best, s.worst
	r.TestedMinCells, r.TestedMaxCells = s.best, s.worst
	r.BaseOriginAreaKM2 = float64(r.BaseOriginCells) * w * w / 1e6
	r.CompleteTranslationSearch = mode == "exact"
	switch mode {
	case "exact":
		r.SearchGuarantee = GuaranteeTranslationMinimum
	case "sampled":
		r.SearchGuarantee = GuaranteeTestedOriginsMinimum
	default:
		r.SearchGuarantee = GuaranteeFixedOrigin
	}
	return r, cells, nil
}
