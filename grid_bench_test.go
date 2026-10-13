package eooaoo

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
)

// benchPoints draws n analysis-plane points (metres) from a fixed seed.
// "uniform" spreads them over 1,000 km; "clustered" puts them in 20 clusters
// with 5 km spread, the shape of many real occurrence sets.
func benchPoints(n int, dist string, seed int64) []xy {
	rng := rand.New(rand.NewSource(seed))
	p := make([]xy, n)
	var centers []xy
	for i := 0; i < 20; i++ {
		centers = append(centers, xy{rng.Float64() * 1e6, rng.Float64() * 1e6})
	}
	for i := range p {
		if dist == "uniform" {
			p[i] = xy{rng.Float64() * 1e6, rng.Float64() * 1e6}
		} else {
			c := centers[rng.Intn(len(centers))]
			p[i] = xy{c.x + rng.NormFloat64()*5000, c.y + rng.NormFloat64()*5000}
		}
	}
	return p
}

// BenchmarkGridSearch measures the event sweep in auto mode (the default) and
// reports the effective mode and estimated work. Run with:
//
//	go test -run '^$' -bench BenchmarkGridSearch -benchmem -count 3 .
func BenchmarkGridSearch(b *testing.B) {
	for _, dist := range []string{"uniform", "clustered"} {
		for _, n := range []int{100, 300, 1000, 10000} {
			p := benchPoints(n, dist, 42)
			b.Run(fmt.Sprintf("%s/n=%d", dist, n), func(b *testing.B) {
				var a AOOResult
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					var err error
					if a, _, err = calculateGrid(context.Background(), p, Config{CellSizeM: 2000, GridMode: "auto", GridSteps: 10}); err != nil {
						b.Fatal(err)
					}
				}
				b.ReportMetric(float64(a.EstimatedPointEvaluations), "updates")
				b.ReportMetric(float64(a.Candidates), "origins")
				if a.EffectiveGridMode != "exact" {
					b.ReportMetric(1, "fallback")
				}
			})
		}
	}
}

// BenchmarkReferenceEnumeration times the original nested-loop exact search on
// the same inputs, for the sizes where it is affordable.
func BenchmarkReferenceEnumeration(b *testing.B) {
	for _, dist := range []string{"uniform", "clustered"} {
		for _, n := range []int{100, 300} {
			p := benchPoints(n, dist, 42)
			xs, ys := exactOffsets(p, 2000, 0, 0), exactOffsets(p, 2000, 0, 1)
			b.Run(fmt.Sprintf("%s/n=%d", dist, n), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					enumerateGrid(p, 2000, [2]float64{}, xs, ys)
				}
				b.ReportMetric(float64(len(xs)*len(ys)*len(p)), "evaluations")
			})
		}
	}
}
