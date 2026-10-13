package eooaoo

import (
	"fmt"
	"github.com/airbusgeo/godal"
	"math"
	"math/big"
	"sort"
	"strings"
)

// orientErrBound is Shewchuk's ccwerrboundA for IEEE-754 binary64.
var orientErrBound = (3 + 16*0x1p-53) * 0x1p-53

// orient returns the exact sign of the turn o->a->b for the given float64
// values: +1 counter-clockwise, -1 clockwise, 0 collinear. The fast float
// determinant is trusted only when it exceeds its rounding-error bound;
// otherwise the determinant is evaluated exactly with rational arithmetic.
// No epsilon is applied, so valid thin geometry is never discarded.
func orient(o, a, b xy) int {
	l := (a.x - o.x) * (b.y - o.y)
	r := (a.y - o.y) * (b.x - o.x)
	det := l - r
	if math.Abs(det) > orientErrBound*(math.Abs(l)+math.Abs(r)) {
		if det > 0 {
			return 1
		}
		return -1
	}
	return orientExact(o, a, b)
}

func orientExact(o, a, b xy) int {
	q := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	ax, ay := new(big.Rat).Sub(q(a.x), q(o.x)), new(big.Rat).Sub(q(a.y), q(o.y))
	bx, by := new(big.Rat).Sub(q(b.x), q(o.x)), new(big.Rat).Sub(q(b.y), q(o.y))
	return new(big.Rat).Sub(new(big.Rat).Mul(ax, by), new(big.Rat).Mul(ay, bx)).Sign()
}

// Andrew's monotone chain in the analysis plane. GDAL measures its area.
func convexHull(points []xy) []xy {
	p := append([]xy(nil), points...)
	sort.Slice(p, func(i, j int) bool {
		if p[i].x != p[j].x {
			return p[i].x < p[j].x
		}
		return p[i].y < p[j].y
	})
	if len(p) < 3 {
		return nil
	}
	l, u := make([]xy, 0, len(p)), make([]xy, 0, len(p))
	for _, v := range p {
		for len(l) >= 2 && orient(l[len(l)-2], l[len(l)-1], v) <= 0 {
			l = l[:len(l)-1]
		}
		l = append(l, v)
	}
	for i := len(p) - 1; i >= 0; i-- {
		v := p[i]
		for len(u) >= 2 && orient(u[len(u)-2], u[len(u)-1], v) <= 0 {
			u = u[:len(u)-1]
		}
		u = append(u, v)
	}
	h := append(l[:len(l)-1], u[:len(u)-1]...)
	if len(h) < 3 {
		return nil
	}
	return h
}

func polygonWKT(p []xy) string {
	var s strings.Builder
	s.WriteString("POLYGON ((")
	for i := 0; i <= len(p); i++ {
		if i > 0 {
			s.WriteByte(',')
		}
		v := p[i%len(p)]
		fmt.Fprintf(&s, "%.17g %.17g", v.x, v.y)
	}
	s.WriteString("))")
	return s.String()
}

// calculateEOO also returns the counter-clockwise hull vertices when the
// status is "ok", for diagnostics that must reuse exactly the same polygon.
func calculateEOO(p []xy, sr *godal.SpatialRef, aoo float64) (EOOResult, []xy, error) {
	r := EOOResult{AssessmentAreaKM2: aoo, AdjustedToAOO: true, Method: HullMethod + "; " + AreaMethod}
	if len(p) < 3 {
		r.Status = "insufficient_unique_points"
		return r, nil, nil
	}
	h := convexHull(p)
	if len(h) < 3 {
		r.Status = "collinear"
		return r, nil, nil
	}
	r.HullWKT = polygonWKT(h)
	g, err := godal.NewGeometryFromWKT(r.HullWKT, sr)
	if err != nil {
		return r, nil, err
	}
	defer g.Close()
	area := g.Area() / 1e6
	if !finite(area) || area < 0 {
		return r, nil, fmt.Errorf("invalid GDAL hull area")
	}
	// <1e-6 m² is numerically degenerate; this is an implementation tolerance.
	if area < 1e-12 {
		r.Status = "numerically_degenerate"
		r.HullWKT = ""
		return r, nil, nil
	}
	r.RawAreaKM2 = &area
	r.Status = "ok"
	if area >= aoo {
		r.AssessmentAreaKM2 = area
		r.AdjustedToAOO = false
	}
	return r, h, nil
}
