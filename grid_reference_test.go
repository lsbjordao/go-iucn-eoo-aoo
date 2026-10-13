package eooaoo

import (
	"context"
	"math"
	"math/big"
	"math/rand"
	"sort"
	"testing"
)

// enumerateGrid is the original nested-loop search, kept as the reference the
// event sweep must reproduce: same candidates, same arithmetic, same ties.
func enumerateGrid(p []xy, w float64, base [2]float64, xs, ys []float64) (best, worst int, origin [2]float64) {
	best = len(p) + 1
	for _, dx := range xs {
		for _, dy := range ys {
			o := [2]float64{base[0] + dx, base[1] + dy}
			m := map[[2]int64]bool{}
			for _, v := range p {
				m[[2]int64{int64(math.Floor((v.x - o[0]) / w)), int64(math.Floor((v.y - o[1]) / w))}] = true
			}
			if len(m) > worst {
				worst = len(m)
			}
			if len(m) < best {
				best, origin = len(m), o
			}
		}
	}
	return best, worst, origin
}

func adversarialSets(rng *rand.Rand) [][]xy {
	w := 2000.0
	sets := [][]xy{
		// Exact multiples of the cell size, negative and positive.
		{{-4000, -2000}, {0, 0}, {2000, 4000}, {-2000, 2000}},
		// Coincident residues: points exactly one or more periods apart.
		{{10, 10}, {2010, 10}, {4010, 2010}, {-1990, -3990}},
		// Residues one ulp apart around a boundary.
		{{2000, 1}, {math.Nextafter(2000, 0), 1}, {math.Nextafter(2000, 1e9), 1}},
		// Large coordinates with tiny separations.
		{{1e7, 1e7}, {1e7 + 1e-6, 1e7}, {1e7 + w, 1e7 - 1e-6}, {-1e7, 1e7 + 0.5}},
		// Narrow window: exactly one origin interval fits both points.
		{{10, 1000}, {2009, 1000}},
	}
	for k := 0; k < 60; k++ {
		p := make([]xy, 1+rng.Intn(25))
		for i := range p {
			switch rng.Intn(3) {
			case 0: // integer metres: many coincident events
				p[i] = xy{float64(rng.Intn(12000) - 6000), float64(rng.Intn(12000) - 6000)}
			case 1: // exact half-cells
				p[i] = xy{float64(rng.Intn(16)-8) * w / 2, float64(rng.Intn(16)-8) * w / 2}
			default:
				p[i] = xy{rng.NormFloat64() * 8000, rng.NormFloat64() * 8000}
			}
		}
		sets = append(sets, p)
	}
	return sets
}

func TestSweepMatchesReferenceEnumeration(t *testing.T) {
	rng := rand.New(rand.NewSource(20261013))
	w := 2000.0
	for n, p := range adversarialSets(rng) {
		for _, base := range [][2]float64{{0, 0}, {-731.25, 999.5}, {1e6 + 0.1, -1e6}} {
			for _, mode := range []string{"exact", "sampled", "fixed"} {
				c := Config{CellSizeM: w, GridMode: mode, GridSteps: 7, Origin: base}
				got, cells, err := calculateGrid(context.Background(), p, c)
				if err != nil {
					t.Fatal(err)
				}
				if len(p) == 1 && mode != "fixed" {
					continue // centered single-point convention, tested separately
				}
				xs, ys := []float64{0}, []float64{0}
				switch mode {
				case "exact":
					xs, ys = exactOffsets(p, w, base[0], 0), exactOffsets(p, w, base[1], 1)
				case "sampled":
					xs = sampledOffsets(w, 7)
					ys = xs
				}
				best, worst, origin := enumerateGrid(p, w, base, xs, ys)
				if got.OccupiedCells != best || got.TestedMaxCells != worst || got.Origin != origin || len(cells) != best {
					t.Fatalf("set %d base %v %s: sweep=(%d,%d,%v) reference=(%d,%d,%v)", n, base, mode, got.OccupiedCells, got.TestedMaxCells, got.Origin, best, worst, origin)
				}
				b0, _, _ := enumerateGrid(p, w, base, []float64{0}, []float64{0})
				if got.BaseOriginCells != b0 {
					t.Fatalf("set %d %s: base-origin cells %d, reference %d", n, mode, got.BaseOriginCells, b0)
				}
			}
		}
	}
}

// ratTranslationMinimum is an independent oracle: it computes residues,
// candidate offsets and cell indices with exact rational arithmetic on the
// float64 inputs and does not call any implementation helper.
func ratTranslationMinimum(p []xy, w float64, base [2]float64) int {
	W := new(big.Rat).SetFloat64(w)
	floorDiv := func(a *big.Rat) *big.Int {
		q := new(big.Rat).Quo(a, W)
		z := new(big.Int).Quo(q.Num(), q.Denom()) // truncates toward zero
		if q.Sign() < 0 && !q.IsInt() {
			z.Sub(z, big.NewInt(1))
		}
		return z
	}
	axis := func(get func(xy) float64, b float64) ([]*big.Rat, []*big.Rat) {
		B := new(big.Rat).SetFloat64(b)
		vals := make([]*big.Rat, len(p))
		var res []*big.Rat
		for i, v := range p {
			vals[i] = new(big.Rat).Sub(new(big.Rat).SetFloat64(get(v)), B)
			k := floorDiv(vals[i])
			r := new(big.Rat).Sub(vals[i], new(big.Rat).Mul(new(big.Rat).SetInt(k), W))
			res = append(res, r)
		}
		sort.Slice(res, func(i, j int) bool { return res[i].Cmp(res[j]) < 0 })
		var u []*big.Rat
		for _, r := range res {
			if len(u) == 0 || r.Cmp(u[len(u)-1]) != 0 {
				u = append(u, r)
			}
		}
		cand := []*big.Rat{new(big.Rat)}
		for i := range u {
			next := new(big.Rat).Add(u[(i+1)%len(u)], new(big.Rat))
			if i == len(u)-1 {
				next.Add(next, W)
			}
			mid := new(big.Rat).Quo(new(big.Rat).Add(u[i], next), big.NewRat(2, 1))
			if mid.Cmp(W) >= 0 {
				mid.Sub(mid, W)
			}
			cand = append(cand, mid)
		}
		return vals, cand
	}
	vx, cx := axis(func(v xy) float64 { return v.x }, base[0])
	vy, cy := axis(func(v xy) float64 { return v.y }, base[1])
	best := len(p) + 1
	for _, tx := range cx {
		for _, ty := range cy {
			m := map[string]bool{}
			for i := range p {
				ix := floorDiv(new(big.Rat).Sub(vx[i], tx))
				iy := floorDiv(new(big.Rat).Sub(vy[i], ty))
				m[ix.String()+","+iy.String()] = true
			}
			if len(m) < best {
				best = len(m)
			}
		}
	}
	return best
}

func TestExactAgainstRationalOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	for n, p := range adversarialSets(rng)[:40] {
		for _, base := range [][2]float64{{0, 0}, {-731.25, 999.5}} {
			got, _, err := calculateGrid(context.Background(), p, Config{CellSizeM: 2000, GridMode: "exact", Origin: base})
			if err != nil {
				t.Fatal(err)
			}
			if want := ratTranslationMinimum(p, 2000, base); got.OccupiedCells != want {
				t.Fatalf("set %d base %v: exact=%d rational oracle=%d points=%v", n, base, got.OccupiedCells, want, p)
			}
		}
	}
}

func TestGridSearchCancellation(t *testing.T) {
	p := make([]xy, 3000)
	rng := rand.New(rand.NewSource(1))
	for i := range p {
		p[i] = xy{rng.Float64() * 1e6, rng.Float64() * 1e6}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := calculateGrid(ctx, p, Config{CellSizeM: 2000, GridMode: "exact"}); err == nil {
		t.Fatal("cancelled grid search returned a result")
	}
}
