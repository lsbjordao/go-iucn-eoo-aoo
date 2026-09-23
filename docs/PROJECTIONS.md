# Projection strategies

`go-iucn-eoo-aoo` makes the analysis projection an explicit, recorded choice. The IUCN Red List criteria define EOO and AOO, the 2 × 2 km reference scale for AOO, and the use of minimum convex polygons as a common EOO implementation, but they do not mandate one projected CRS for every assessment.

The IUCN Mapping Standards recommend **Cylindrical Equal Area (world)** for area/geometry calculations while noting that other projections can be used where they provide better results for particular locations. The IUCN EOO Calculator profile documents that its minimum convex polygon is projected to a cylindrical equal-area coordinate system for the area calculation and then projected back to WGS 84.

Sources:

- IUCN Red List Guidelines: https://nrl.iucnredlist.org/resources/redlistguidelines
- IUCN Mapping Standards: https://nrl.iucnredlist.org/resources/mappingstandards
- IUCN EOO Calculator tool profile: https://nc.iucnredlist.org/redlist/content/attachment_files/Tool_profile_-_EOO_Calculator.pdf
- IUCN EOO Calculator instructions: https://nc.iucnredlist.org/redlist/content/attachment_files/EOO_Calculator_Tool_Instructions_v1_5.pdf

## `local-laea` (default)

A Lambert Azimuthal Equal Area projection is created for each assessment unit with its origin at the spherical mean of the unique WGS84 occurrence coordinates. An explicit center can be frozen for reproducible temporal comparisons.

```text
+proj=laea +lat_0=<center_lat> +lon_0=<center_lon> +datum=WGS84 +units=m +no_defs +type=crs
```

This strategy is intended for regional distributions. The implementation rejects points more than 80 angular degrees from the chosen center. Because an equal-area projection preserves area but not shape, the convex hull and the orientation of a square AOO grid remain projection-dependent.

## `iucn-cea`

This compatibility-oriented strategy uses the **World Cylindrical Equal Area** definition commonly identified as ESRI:54034:

```text
+proj=cea +lat_ts=0 +lon_0=0 +x_0=0 +y_0=0 +datum=WGS84 +units=m +no_defs +type=crs
```

The projection name and parameters correspond to `World_Cylindrical_Equal_Area`, the projection documented by the IUCN EOO Calculator materials. The mode is called `iucn-cea` rather than `iucn` because this project is not an IUCN-certified reproduction of the ArcGIS toolbox and may differ in software implementation details.

For distributions crossing the antimeridian, a Greenwich-centered cylindrical projection can produce a world-spanning planar convex hull. The program emits `ANTIMERIDIAN_CEA` in that case; `local-laea` or another scientifically appropriate regional/geodesic treatment should be inspected as a comparison rather than silently assuming the world-spanning hull is universally suitable.

The initial reference benchmark on 2026-09-23 reproduced this effect independently. For a synthetic distribution spanning 179.9°E to 179.9°W:

- `go-iucn-eoo-aoo / local-laea`: **242.502828 km²**;
- `ConR / spheroid`: **243.5 km²**;
- `go-iucn-eoo-aoo / iucn-cea`: **436262.589638 km²**;
- `ConR / planar CEA`: **436263 km²**.

The Go and ConR CEA values differ by only about **0.000094%**, while the regional/geodesic-like pair is also close. The extreme difference therefore reflects the planar representation/hull construction across the antimeridian, not a random numerical discrepancy unique to this implementation. See [REFERENCE_BENCHMARK.md](REFERENCE_BENCHMARK.md).

## Why AOO can change with projection

The IUCN reference scale is 2 × 2 km. This implementation searches translations of the square grid and, in `exact` mode, identifies the minimum occupied-cell count for the **fixed orientation of the analysis CRS axes**. It does not rotate the grid.

Changing projection can rotate or deform the planar representation of a distribution. Therefore two equal-area projections may preserve polygon area closely while still producing different convex hull shapes or different occupied 2 km cells.

That dependence is reported rather than hidden. The initial reference benchmark also exposed grid-construction sensitivity: a tiny synthetic triangle produced AOO = 4 km² in this project and ConR, but 12 km² in the Turf-based `vicentecalfo/eoo-aoo-calculator`. Such a discrepancy must be interpreted through grid placement/construction rules rather than treated as an automatic accuracy score.

## Comparing both strategies

Use:

```bash
eoo-aoo compare --in occurrences.csv --out projection-comparison.json
```

The command runs the same normalized records and grid configuration twice and reports:

- the `local-laea` result;
- the `iucn-cea` result;
- `iucn-cea - local-laea` differences for raw EOO, assessment EOO, AOO and occupied cells;
- relative percentage differences where a non-zero baseline exists;
- the same input SHA-256 for both calculations.

The comparison requires the automatic `local-laea` center so that it represents the package defaults. For temporal analyses where a local projection center and grid origin have already been frozen, compare those series under a deliberately fixed protocol instead of changing projection between dates.

## Interactive HTML map

`calc` can produce an interactive MapLibre report:

```bash
eoo-aoo calc --in occurrences.csv \
  --projection local-laea \
  --out result.json \
  --html result.html
```

The HTML embeds WGS84 display GeoJSON for:

- occurrence points;
- occupied AOO cells;
- the EOO convex hull.

It also records EOO/AOO metrics, projection strategy, GDAL/PROJ versions and the input hash. MapLibre GL JS and the demonstration basemap are loaded over the network when the HTML is opened; the assessment geometries themselves are embedded in the document.
