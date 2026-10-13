package eooaoo

/*
#cgo pkg-config: proj
#include <stdlib.h>
#include <geodesic.h>

static double eoo_wgs84_polygon_area(const double *lats, const double *lons, int n) {
	struct geod_geodesic g;
	double area = 0, perimeter = 0;
	geod_init(&g, 6378137.0, 1.0 / 298.257223563);
	geod_polygonarea(&g, (double *)lats, (double *)lons, n, &area, &perimeter);
	return area;
}
*/
import "C"

import (
	"math"
	"unsafe"
)

// wgs84GeodesicArea returns the area in m² of the polygon whose vertices are
// given in WGS84 degrees and whose edges are ellipsoidal geodesics (Karney's
// algorithm as shipped by PROJ). Vertices must be counter-clockwise; ok is
// false for a non-positive or non-finite signed area.
func wgs84GeodesicArea(lon, lat []float64) (area float64, ok bool) {
	n := len(lon)
	if n < 3 || len(lat) != n {
		return 0, false
	}
	buf := C.malloc(C.size_t(2 * n * int(unsafe.Sizeof(C.double(0)))))
	defer C.free(buf)
	all := unsafe.Slice((*C.double)(buf), 2*n)
	for i := 0; i < n; i++ {
		all[i] = C.double(lat[i])
		all[n+i] = C.double(lon[i])
	}
	a := float64(C.eoo_wgs84_polygon_area(&all[0], &all[n], C.int(n)))
	if !finite(a) || a <= 0 || math.IsNaN(a) {
		return 0, false
	}
	return a, true
}
