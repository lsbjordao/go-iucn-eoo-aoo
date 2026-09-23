package eooaoo

import (
	"fmt"
	"github.com/airbusgeo/godal"
	"sort"
	"strings"
)

func cross(o, a, b xy) float64 { return (a.x-o.x)*(b.y-o.y) - (a.y-o.y)*(b.x-o.x) }

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
		for len(l) >= 2 && cross(l[len(l)-2], l[len(l)-1], v) <= 0 {
			l = l[:len(l)-1]
		}
		l = append(l, v)
	}
	for i := len(p) - 1; i >= 0; i-- {
		v := p[i]
		for len(u) >= 2 && cross(u[len(u)-2], u[len(u)-1], v) <= 0 {
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

func calculateEOO(p []xy, sr *godal.SpatialRef, aoo float64) (EOOResult, error) {
	r := EOOResult{AssessmentAreaKM2: aoo, AdjustedToAOO: true}
	if len(p) < 3 {
		r.Status = "insufficient_unique_points"
		return r, nil
	}
	h := convexHull(p)
	if len(h) < 3 {
		r.Status = "collinear"
		return r, nil
	}
	r.HullWKT = polygonWKT(h)
	g, err := godal.NewGeometryFromWKT(r.HullWKT, sr)
	if err != nil {
		return r, err
	}
	defer g.Close()
	area := g.Area() / 1e6
	if !finite(area) || area < 0 {
		return r, fmt.Errorf("invalid GDAL hull area")
	}
	// <1e-6 m² is numerically degenerate; this is an implementation tolerance.
	if area < 1e-12 {
		r.Status = "numerically_degenerate"
		r.HullWKT = ""
		return r, nil
	}
	r.RawAreaKM2 = &area
	r.Status = "ok"
	if area >= aoo {
		r.AssessmentAreaKM2 = area
		r.AdjustedToAOO = false
	}
	return r, nil
}
