#!/usr/bin/env python3
"""Optional independent geometric checks: pip install pyproj shapely.

Uses pyproj's ellipsoidal geodesic area algorithm on densified LAEA boundaries,
and GEOS (Shapely) for the exported hull, independently of the Go area code.
PROJ is shared as a mathematical implementation, not an independent authority.
"""
import argparse
import json
from pathlib import Path
import subprocess

import pyproj
from pyproj import CRS, Geod, Transformer
from shapely import wkt

parser = argparse.ArgumentParser()
parser.add_argument("--binary", default="bin/eoo-aoo")
args = parser.parse_args()
binary = str(Path(args.binary).resolve())
checks = []
for latitude in (0, -23, 70, -85):
    center = [30, latitude]
    crs = CRS.from_proj4(f"+proj=laea +lat_0={latitude} +lon_0=30 +datum=WGS84 +units=m")
    inverse = Transformer.from_crs(crs, "EPSG:4326", always_xy=True)
    corners = [(-50000,-50000),(50000,-50000),(50000,50000),(-50000,50000)]
    points = [dict(zip(("lon","lat"), inverse.transform(x,y))) for x,y in corners]
    proc = subprocess.run([binary,"calc","--in","-","--center",f"30,{latitude}"],
                          input=json.dumps(points), text=True, capture_output=True, check=True)
    result = json.loads(proc.stdout)
    raw = result["eoo"]["raw_area_km2"]
    shapely_area = wkt.loads(result["eoo"]["hull_wkt"]).area / 1e6
    boundary = []
    for i,(x1,y1) in enumerate(corners):
        x2,y2 = corners[(i+1)%4]
        for j in range(1000):
            boundary.append(inverse.transform(x1+(x2-x1)*j/1000, y1+(y2-y1)*j/1000))
    lons,lats = zip(*boundary)
    geodesic = abs(Geod(ellps="WGS84").polygon_area_perimeter(lons,lats)[0])/1e6
    assert abs(raw-10000)<1e-4, (latitude,raw)
    assert abs(raw-shapely_area)<1e-8, (raw,shapely_area)
    assert abs(raw-geodesic)/raw<2e-6, (latitude,raw,geodesic)
    checks.append({"center_latitude":latitude,"expected_km2":10000,
                   "go_gdal_km2":raw,"shapely_km2":shapely_area,
                   "geodesic_densified_km2":geodesic,
                   "relative_geodesic_difference":abs(raw-geodesic)/raw})
print(json.dumps({"pyproj_version":pyproj.__version__,"pyproj_proj_version":pyproj.proj_version_str,
                  "checks":checks,"status":"passed"},indent=2))
