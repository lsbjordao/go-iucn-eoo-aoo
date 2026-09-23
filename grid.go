package eooaoo

import (
	"context"
	"fmt"
	"math"
	"sort"
)

type xy struct{ x, y float64 }
type cellKey struct{ x, y int64 }

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

func occupied(ctx context.Context, p []xy, w float64, o [2]float64) (map[cellKey]int, error) {
	m := make(map[cellKey]int, len(p))
	for i, v := range p {
		if err := checkContext(ctx, i); err != nil {
			return nil, err
		}
		// No arbitrary epsilon: points at a boundary belong to the right/top cell.
		k := cellKey{int64(math.Floor((v.x - o[0]) / w)), int64(math.Floor((v.y - o[1]) / w))}
		m[k]++
	}
	return m, nil
}

func calculateGrid(ctx context.Context, p []xy, c Config) (AOOResult, []Cell, error) {
	w := c.CellSizeM
	mode := c.GridMode
	xs, ys := []float64{0}, []float64{0}
	if mode == "auto" || mode == "exact" {
		xs = exactOffsets(p, w, c.Origin[0], 0)
		ys = exactOffsets(p, w, c.Origin[1], 1)
		work := int64(len(xs)) * int64(len(ys)) * int64(len(p))
		if work > gridWorkLimit {
			if mode == "exact" {
				return AOOResult{}, nil, fmt.Errorf("exact grid search needs %d point evaluations (limit %d); choose sampled", work, gridWorkLimit)
			}
			mode = "sampled"
		} else {
			mode = "exact"
		}
	}
	if mode == "sampled" {
		if int64(c.GridSteps)*int64(c.GridSteps)*int64(len(p)) > gridWorkLimit {
			return AOOResult{}, nil, fmt.Errorf("sampled grid search exceeds work limit; reduce grid_steps")
		}
		xs = make([]float64, c.GridSteps)
		ys = make([]float64, c.GridSteps)
		for i := range xs {
			xs[i] = float64(i) * w / float64(c.GridSteps)
			ys[i] = xs[i]
		}
	}
	// With one unique point every translation ties. Prefer a square centered
	// on the point, unless the caller explicitly requested a fixed grid.
	if len(p) == 1 && mode != "fixed" {
		c.Origin = [2]float64{p[0].x - w/2, p[0].y - w/2}
		xs, ys = []float64{0}, []float64{0}
	}
	best, maxn := len(p)+1, 0
	var origin [2]float64
	var bestCells map[cellKey]int
	for _, dx := range xs {
		for _, dy := range ys {
			o := [2]float64{c.Origin[0] + dx, c.Origin[1] + dy}
			m, err := occupied(ctx, p, w, o)
			if err != nil {
				return AOOResult{}, nil, err
			}
			if len(m) > maxn {
				maxn = len(m)
			}
			if len(m) < best {
				best = len(m)
				bestCells = m
				origin = o
			}
		}
	}
	cells := make([]Cell, 0, best)
	for k, n := range bestCells {
		x, y := origin[0]+float64(k.x)*w, origin[1]+float64(k.y)*w
		cells = append(cells, Cell{Column: k.x, Row: k.y, UniquePoints: n, Bounds: [4]float64{x, y, x + w, y + w}})
	}
	sort.Slice(cells, func(i, j int) bool {
		if cells[i].Row != cells[j].Row {
			return cells[i].Row < cells[j].Row
		}
		return cells[i].Column < cells[j].Column
	})
	r := AOOResult{AreaKM2: float64(best) * w * w / 1e6, OccupiedCells: best, CellSizeM: w, ReferenceScale: w == 2000, GridMode: mode, Origin: origin, Candidates: len(xs) * len(ys), MinCells: best, MaxCells: maxn, CompleteTranslationSearch: mode == "exact", Orientation: "fixed analysis CRS axes; no rotation", BoundaryRule: "[left,right) x [bottom,top); no snapping"}
	return r, cells, nil
}
