package eooaoo

import (
	"fmt"
	"github.com/airbusgeo/godal"
	"math"
)

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// GDALVersion identifies the loaded native GDAL library.
func GDALVersion() string {
	v := godal.Version()
	return fmt.Sprintf("%d.%d.%d", v.Major(), v.Minor(), v.Revision())
}

// transform allocates independent GDAL objects per operation; no shared handles.
func transform(src, dst *godal.SpatialRef, x, y []float64) error {
	if len(x) == 0 {
		return nil
	}
	t, err := godal.NewTransform(src, dst)
	if err != nil {
		return err
	}
	defer t.Close()
	ok := make([]bool, len(x))
	if err = t.TransformEx(x, y, nil, ok); err != nil {
		return err
	}
	for i := range x {
		if !ok[i] || !finite(x[i]) || !finite(y[i]) {
			return fmt.Errorf("coordinate transformation failed at point %d", i)
		}
	}
	return nil
}

func sphericalCenter(p []UsedPoint) ([2]float64, error) {
	var x, y, z float64
	for _, v := range p {
		lon, lat := v.Longitude*math.Pi/180, v.Latitude*math.Pi/180
		x += math.Cos(lat) * math.Cos(lon)
		y += math.Cos(lat) * math.Sin(lon)
		z += math.Sin(lat)
	}
	if math.Sqrt(x*x+y*y+z*z)/float64(len(p)) < 1e-8 {
		return [2]float64{}, fmt.Errorf("ambiguous projection center: near-antipodal/global distribution is outside the local LAEA method")
	}
	c := [2]float64{math.Atan2(y, x) * 180 / math.Pi, math.Atan2(z, math.Hypot(x, y)) * 180 / math.Pi}
	if math.Abs(c[1]) > 89.999999999 {
		c[0] = 0
	}
	return c, nil
}

func angularDistance(p UsedPoint, c [2]float64) float64 {
	p1, p2 := p.Latitude*math.Pi/180, c[1]*math.Pi/180
	d := math.Sin(p1)*math.Sin(p2) + math.Cos(p1)*math.Cos(p2)*math.Cos((p.Longitude-c[0])*math.Pi/180)
	return math.Acos(math.Max(-1, math.Min(1, d))) * 180 / math.Pi
}

func localLAEARef(c [2]float64) (*godal.SpatialRef, string, error) {
	def := fmt.Sprintf("+proj=laea +lat_0=%.15g +lon_0=%.15g +datum=WGS84 +units=m +no_defs +type=crs", c[1], c[0])
	sr, err := godal.NewSpatialRef(def)
	return sr, def, err
}

func iucnCEARef() (*godal.SpatialRef, string, error) {
	// ESRI:54034 World Cylindrical Equal Area, matching the projection named by
	// the IUCN Red List EOO Calculator documentation and mapping standards.
	def := "+proj=cea +lat_ts=0 +lon_0=0 +x_0=0 +y_0=0 +datum=WGS84 +units=m +no_defs +type=crs"
	sr, err := godal.NewSpatialRef(def)
	return sr, def, err
}

func analysisRef(strategy string, c [2]float64) (*godal.SpatialRef, string, string, error) {
	switch strategy {
	case ProjectionLocalLAEA:
		sr, def, err := localLAEARef(c)
		return sr, def, "Local Lambert Azimuthal Equal Area centered on the assessment points", err
	case ProjectionIUCNCEA:
		sr, def, err := iucnCEARef()
		return sr, def, "IUCN-compatible World Cylindrical Equal Area (ESRI:54034)", err
	default:
		return nil, "", "", fmt.Errorf("unknown projection strategy %q", strategy)
	}
}
