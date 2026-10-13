package eooaoo

import (
	"math"
	"math/big"
	"math/rand"
	"sort"
	"testing"

	"github.com/airbusgeo/godal"
)

func naiveCross(o, a, b xy) float64 { return (a.x-o.x)*(b.y-o.y) - (a.y-o.y)*(b.x-o.x) }

// Kettner et al. "Classroom examples of robustness problems": points within a
// few ulps of the line through (12,12) and (24,24) make the naive determinant
// return wrong signs. The fast path must never disagree with the exact sign.
func TestOrientMatchesExactNearCollinear(t *testing.T) {
	q, r := xy{12, 12}, xy{24, 24}
	naiveWrong := 0
	for i := 0; i < 64; i++ {
		for j := 0; j < 64; j++ {
			p := xy{0.5 + float64(i)*0x1p-53, 0.5 + float64(j)*0x1p-53}
			want := orientExact(p, q, r)
			if got := orient(p, q, r); got != want {
				t.Fatalf("orient(%v)=%d exact=%d", p, got, want)
			}
			s := naiveCross(p, q, r)
			if (s > 0 && want <= 0) || (s < 0 && want >= 0) || (s == 0 && want != 0) {
				naiveWrong++
			}
		}
	}
	if naiveWrong == 0 {
		t.Fatal("test inputs do not exercise the rounding-error regime")
	}
}

func TestOrientRandomAgreesWithExact(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	for i := 0; i < 20000; i++ {
		scale := math.Pow(10, float64(rng.Intn(14)-4))
		off := (rng.Float64() - 0.5) * math.Pow(10, float64(rng.Intn(8)))
		o := xy{off + rng.NormFloat64()*scale, off + rng.NormFloat64()*scale}
		a := xy{off + rng.NormFloat64()*scale, off + rng.NormFloat64()*scale}
		// b on or near the line o-a.
		tt := rng.Float64()*3 - 1
		b := xy{o.x + tt*(a.x-o.x), o.y + tt*(a.y-o.y)}
		if rng.Intn(2) == 0 {
			b.x = math.Nextafter(b.x, math.Inf(1))
		}
		if got, want := orient(o, a, b), orientExact(o, a, b); got != want {
			t.Fatalf("orient(%v,%v,%v)=%d exact=%d", o, a, b, got, want)
		}
	}
}

func hullKey(h []xy) []xy {
	k := append([]xy(nil), h...)
	sort.Slice(k, func(i, j int) bool {
		if k[i].x != k[j].x {
			return k[i].x < k[j].x
		}
		return k[i].y < k[j].y
	})
	return k
}

func checkHull(t *testing.T, p, h []xy) {
	t.Helper()
	for i := range h {
		if orientExact(h[i], h[(i+1)%len(h)], h[(i+2)%len(h)]) <= 0 {
			t.Fatalf("hull is not strictly convex counter-clockwise at vertex %d: %v", i, h)
		}
	}
	for _, v := range p {
		for i := range h {
			if orientExact(h[i], h[(i+1)%len(h)], v) < 0 {
				t.Fatalf("point %v lies outside hull edge %d: %v", v, i, h)
			}
		}
	}
}

func TestHullAdversarialSets(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	var sets [][]xy
	// Near-collinear clouds at several scales and offsets.
	for _, off := range []float64{0, 1e6, -7.3e6, 1.5e7} {
		for _, scale := range []float64{1e-3, 1, 1e5} {
			var p []xy
			for i := 0; i < 40; i++ {
				s := rng.Float64() * scale
				v := xy{off + s, off + s}
				for k := rng.Intn(4); k > 0; k-- {
					v.y = math.Nextafter(v.y, math.Inf(rng.Intn(2)*2-1))
				}
				p = append(p, v)
			}
			p = append(p, xy{off, off + scale}, xy{off + scale, off}) // keep it non-degenerate
			sets = append(sets, p)
		}
	}
	// Small differences on large coordinates.
	sets = append(sets, []xy{{1e7, 1e7}, {1e7 + 1e-6, 1e7}, {1e7, 1e7 + 1e-6}, {1e7 + 5e-7, 1e7 + 5e-7}, {1e7 + 1e-6, 1e7 + 1e-6}})
	for n, p := range sets {
		h := convexHull(p)
		if len(h) < 3 {
			t.Fatalf("set %d unexpectedly degenerate", n)
		}
		checkHull(t, p, h)
		// Permutation and duplication must not change the vertex set.
		q := append(append([]xy(nil), p...), p[:len(p)/2]...)
		rng.Shuffle(len(q), func(i, j int) { q[i], q[j] = q[j], q[i] })
		h2 := convexHull(q)
		a, b := hullKey(h), hullKey(h2)
		if len(a) != len(b) {
			t.Fatalf("set %d: permutation changed hull size %d -> %d", n, len(a), len(b))
		}
		for i := range a {
			if a[i] != b[i] {
				t.Fatalf("set %d: permutation changed hull vertices", n)
			}
		}
	}
}

func TestHullExactlyCollinearIsDegenerate(t *testing.T) {
	p := []xy{{0, 0}, {1, 1}, {2, 2}, {1e6, 1e6}, {0.5, 0.5}}
	if h := convexHull(p); h != nil {
		t.Fatalf("collinear input produced hull %v", h)
	}
	// One ulp off the line is genuine (if thin) geometry and must be kept.
	p = append(p, xy{2, math.Nextafter(2, 3)})
	if h := convexHull(p); len(h) != 3 {
		t.Fatalf("thin triangle discarded: %v", h)
	}
}

// ratShoelace is an exact area oracle for a polygon given as float64 vertices.
func ratShoelace(h []xy) float64 {
	s := new(big.Rat)
	for i := range h {
		a, b := h[i], h[(i+1)%len(h)]
		s.Add(s, new(big.Rat).Sub(
			new(big.Rat).Mul(new(big.Rat).SetFloat64(a.x), new(big.Rat).SetFloat64(b.y)),
			new(big.Rat).Mul(new(big.Rat).SetFloat64(b.x), new(big.Rat).SetFloat64(a.y))))
	}
	f, _ := new(big.Rat).Quo(s, big.NewRat(2, 1)).Float64()
	return math.Abs(f)
}

// GDAL's planar area must agree with the exact shoelace value of the same
// float64 vertices. Each shoelace term rounds with error about ε·|x|·span,
// so the tolerance is 4·n·ε·max|coordinate|·span plus 1e-12 relative.
func TestHullAreaAgainstExactShoelace(t *testing.T) {
	sr, _, _, err := analysisRef(ProjectionIUCNCEA, [2]float64{})
	if err != nil {
		t.Fatal(err)
	}
	defer sr.Close()
	rng := rand.New(rand.NewSource(3))
	for trial := 0; trial < 200; trial++ {
		off := xy{(rng.Float64() - 0.5) * 2e7, (rng.Float64() - 0.5) * 1.4e7}
		scale := math.Pow(10, float64(1+rng.Intn(6)))
		p := make([]xy, 3+rng.Intn(30))
		for i := range p {
			p[i] = xy{off.x + rng.Float64()*scale, off.y + rng.Float64()*scale}
		}
		h := convexHull(p)
		if len(h) < 3 {
			continue
		}
		g, err := godal.NewGeometryFromWKT(polygonWKT(h), sr)
		if err != nil {
			t.Fatal(err)
		}
		got := g.Area()
		g.Close()
		want := ratShoelace(h)
		maxAbs := math.Max(math.Abs(off.x), math.Abs(off.y)) + scale
		tol := 1e-12*want + 4*float64(len(h))*0x1p-52*maxAbs*scale
		if math.Abs(got-want) > tol {
			t.Fatalf("trial %d: GDAL area %.17g exact %.17g", trial, got, want)
		}
	}
}
