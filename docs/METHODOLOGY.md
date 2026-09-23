# Methodology and limitations — 0.1.0

## Primary references

1. IUCN Standards and Petitions Committee (2024). *Guidelines for Using the IUCN Red List Categories and Criteria*, version 16, §§4.9, 4.10.2, 4.10.3 and 4.10.8. [Document](https://cmsdocs.s3.amazonaws.com/RedListGuidelines.pdf).
2. IUCN Red List. *Mapping Standards and Data Quality*. [Resources](https://nrl.iucnredlist.org/resources/mappingstandards).
3. IUCN Red List Team. *EOO Calculator tool profile* and *EOO Calculator instructions*. [Spatial tools and data](https://nrl.iucnredlist.org/resources/spatialtoolsanddata).
4. PROJ: [Lambert Azimuthal Equal Area](https://proj.org/en/stable/operations/projections/laea.html) and [Lambert Cylindrical Equal Area](https://proj.org/en/stable/operations/projections/cea.html).
5. GDAL: [spatial references and coordinate transformation](https://gdal.org/en/stable/tutorials/osr_api_tut.html), [GeoJSON driver](https://gdal.org/en/stable/drivers/vector/geojson.html).
6. Airbus: [godal v0.0.18](https://github.com/airbusgeo/godal/tree/v0.0.18).

References were checked on 2026-09-22. IUCN Guidelines version 16 is the explicitly adopted methodological baseline; future guideline updates should be reviewed before changing the implementation.

## Coordinate systems

Input has a declared CRS and traditional GIS axis order: longitude/easting, latitude/northing. WGS84 is the intermediate geographic CRS. Geographic input coordinates are treated as degrees. Geographic CRSs using other angular units are outside the contract of this version.

After transformation, exactly equal WGS84 coordinates are aggregated. `180°` and `-180°` are treated as equivalent, and longitude at an exact pole is normalized. Nearby coordinates are not rounded or fuzzily deduplicated. Every coordinate group retains its original records and input indices. Duplicate herbarium specimens and identity of collecting events are separate curation problems that belong upstream.

The analysis projection is an explicit `projection_strategy` choice:

- `local-laea` (default): ellipsoidal WGS84 Lambert Azimuthal Equal Area, automatically centred on the spherical vector mean of sorted unique coordinates, or on an explicit fixed centre;
- `iucn-cea`: World Cylindrical Equal Area, equivalent to ESRI:54034 (`+proj=cea +lat_ts=0 +lon_0=0 ... +datum=WGS84`). This follows the projection documented for area calculation in the IUCN EOO Calculator, but it is not a certified reproduction of the ArcGIS tool.

Both are equal-area projections: they preserve area, not shape. Convex-hull shape and the orientation of a square AOO grid therefore remain projection-dependent. The result records the strategy and the exact PROJ/WKT definition used. See [PROJECTIONS.md](PROJECTIONS.md).

For `local-laea`, the 80° angular-distance rejection and warning above 30° are implementation safeguards, **not IUCN-prescribed limits**. For `iucn-cea`, antimeridian-crossing distributions can produce a very large planar hull because the central meridian is Greenwich; the program emits a warning for this case.

Datum changes depend on the operations and grids available to PROJ. This version does not certify or estimate transformation accuracy.

## EOO

An Andrew monotone-chain implementation in Go constructs the convex hull of projected points. GDAL measures planar area in square metres, which is converted to km². The hull is not clipped by continents, habitat or political boundaries. Straight hull edges belong to the **analysis plane** and are not generally geodesic segments between vertices.

The raw area is preserved. The assessment field applies the explicit AOO floor rule; no replacement polygon is invented. With fewer than three distinct points, collinearity in the analysis plane, or area below 1e-6 m², raw EOO is `null`. Applying the AOO floor in these degenerate cases is an explicit API convention that an assessor can review. Collinearity in longitude/latitude does not necessarily imply collinearity in the selected projection.

The `compare` command runs the same normalized records under `local-laea` and `iucn-cea` and reports `iucn-cea - local-laea` for raw EOO, assessment EOO, AOO and occupied cells. Its purpose is to make projection sensitivity measurable, not to automatically select the smallest result.

## AOO and grid translation

Each point is assigned to `floor((x-origin_x)/w), floor((y-origin_y)/w)`. Cell intervals are closed on the left/bottom and open on the right/top, so a point on a boundary belongs to exactly one cell. The algorithm does not add buffers and does not materialize a full grid over the bounding rectangle. Each cell has area `w²` in the equal-area plane; physical side lengths on the ground may differ because equal-area projections do not preserve shape.

At fixed grid orientation, occupancy changes only when the origin crosses a coordinate residue modulo `w`. `exact` evaluates a representative of each interval in X and Y plus the base origin; the final circular interval is also included. Exact boundary equality follows the half-open rule and belongs to an adjacent interval. This is a complete translation enumeration for the floating-point coordinates, without rotation.

`sampled` distributes offsets uniformly over one cell period. The minimum and maximum occupied-cell counts among tested origins are recorded. `auto` chooses between complete and sampled search according to an estimated work budget. The effective mode and selected origin are stored in the result; sampled mode does not claim a proven translation minimum. Non-standard cell resolution triggers a warning.

Translation search does not remove **orientation** dependence: the CRS axes determine square-grid orientation. Consequently, two equal-area projections can produce different AOOs even when polygon area is preserved to very high precision.

## Comparability and export

For a temporal series, freeze projection strategy, centre when applicable, grid origin, orientation, resolution, record-inclusion rules and GDAL/PROJ versions. Re-optimizing grid origin or recalculating a local projection centre independently at each date can introduce differences unrelated to biological change. A fixed-grid time series and a separate sensitivity analysis can be useful complementary outputs.

The GeoPackage contains exact geometries in the analysis plane: unique occurrences, occupied AOO cells and EOO hull, with the analysis CRS attached to each layer. Detailed JSON is the complete record-provenance source; the GPKG retains provenance indices and counts. When no valid hull exists, the EOO layer exists but is empty.

GeoJSON is a display representation. GDAL segmentizes projected edges every 250 m, transforms them to WGS84 and writes RFC 7946 output, including antimeridian cutting. Nine decimal places are serialization precision, not an accuracy claim. Cells very near a pole and polygons enclosing a pole require inspection in the consuming GIS; prefer the GeoPackage for authoritative geometry. Do not recalculate authoritative metrics from map pixels or display GeoJSON.

The MapLibre HTML embeds these WGS84 display GeoJSON layers in a single document. The interactive map is not the authoritative source of area values; authoritative metrics remain the JSON report and the analysis-plane geometries. MapLibre GL JS and the demonstration basemap are loaded from the network when the HTML is opened.

## Outside the scope of this version

The software does not estimate occupancy probability, sampling correction, georeferencing uncertainty, suitable habitat, number of locations, fragmentation, decline, or IUCN categories. It does not infer taxonomic-name validity. Biological filtering must occur before calculation. It also does not implement a geodesic/global hull, grid-rotation optimization, polygon/raster input, persistence, or incremental EOO/AOO updates.

Go provides the application integration, concurrency and distribution layer; GDAL/PROJ provides established geospatial transformation and geometry capabilities. A Node implementation could also be correct. The improvement claimed here is an explicit, auditable and testable spatial contract, not automatic language superiority or an already-demonstrated comparative performance advantage.
