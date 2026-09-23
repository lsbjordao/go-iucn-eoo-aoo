package eooaoo

import (
	"context"
	"math"
	"testing"
)

func FuzzCalculateWGS84(f *testing.F) {
	f.Add(-43.2, -22.9, -44.3, -21.5, -42.9, -20.8)
	f.Add(179.9, 10.0, -179.9, 10.0, 179.8, 10.1)
	f.Add(0.0, 80.0, 10.0, 80.1, -10.0, 79.9)

	f.Fuzz(func(t *testing.T, lon1, lat1, lon2, lat2, lon3, lat3 float64) {
		values := []float64{lon1, lat1, lon2, lat2, lon3, lat3}
		for _, v := range values {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return
			}
		}
		for _, lon := range []float64{lon1, lon2, lon3} {
			if lon < -180 || lon > 180 {
				return
			}
		}
		for _, lat := range []float64{lat1, lat2, lat3} {
			if lat < -90 || lat > 90 {
				return
			}
		}

		points := []Point{{X: lon1, Y: lat1}, {X: lon2, Y: lat2}, {X: lon3, Y: lat3}}
		r, err := Calculate(context.Background(), points, Config{ProjectionStrategy: ProjectionIUCNCEA})
		if err != nil {
			return
		}
		if r.AOO.AreaKM2 <= 0 || r.AOO.OccupiedCells < 1 || r.UniqueCoordinates < 1 || r.UniqueCoordinates > 3 {
			t.Fatalf("invalid successful result: %+v", r)
		}
		if r.EOO.RawAreaKM2 != nil && (!finite(*r.EOO.RawAreaKM2) || *r.EOO.RawAreaKM2 < 0) {
			t.Fatalf("invalid EOO: %+v", r.EOO)
		}
	})
}
