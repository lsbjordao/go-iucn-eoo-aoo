# Methodology and limitations — 0.2.0

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

When the unique points span more than `extent_warning_deg` (default 20°) of longitude or latitude, the result carries `PROJECTION_SENSITIVE_EXTENT`. The longitude span is the shortest arc covering all points, so it is correct across the antimeridian. The 20° default is provisional, chosen so that the three benchmark taxa separate as expected (a 60° set warns, a 13° × 16° set does not); it should be calibrated on more taxa, including wide-ranging ones.

## Data audit and explicit curation

`calc` remains strict: it rejects invalid coordinates and never drops, imputes or reorders records. Data quality is diagnosed separately by `AuditOccurrences` (`eoo-aoo audit`), which reads files permissively so that rows `calc` would reject are reported with their CSV line number. Findings are per record and never remove anything:

| Code | Severity | Rule |
|---|---|---|
| `INVALID_RECORD` | error | Malformed record (wrong field count, non-object, non-Point feature) |
| `INVALID_COORDINATE` | error | Missing, non-numeric, non-finite, out-of-domain or untransformable coordinate |
| `ZERO_ZERO_COORDINATE` | warning | Longitude and latitude both exactly 0 in WGS84. A zero on one axis alone (equator, Greenwich) is never flagged |
| `COARSE_COORDINATE_PRECISION` | info | Only with an explicit decimal threshold: both input values have at most that many decimals. Numerical rounding only; says nothing definite about georeferencing precision |
| `UNCERTAINTY_EXCEEDS_CELL_SIZE` | warning | Only with an explicitly named uncertainty field (metres): declared uncertainty greater than the cell size. Missing values are counted as unknown, never inferred |
| `SOURCE_QUALITY_FLAG` | warning/info | Only with an explicitly named flag field: each flag is kept verbatim. Only documented GBIF `OccurrenceIssue` names and their rgbif abbreviations are interpreted |

The audit's `input_sha256` equals the `input_sha256` of `calc` for the same records. Country-polygon checks are deliberately absent: they need their own polygons, version and policy.

`calc` also emits `ZERO_ZERO_COORDINATE` as a warning when (0,0) records are present, and calculates with them. Exclusion requires a versioned curation manifest bound to the exact input hash. Each decision selects a record by zero-based index plus the expected record ID and/or coordinates and carries a justification; a different input, a mismatched selector, a repeated index or a missing justification is an error before anything is written. `curate` writes the derived set in the input's format (CSV keeps its column order) and a log with the manifest hash, input and output hashes, counts and the complete excluded records. The derived file reads back to the logged hash.

## EOO

An Andrew monotone-chain implementation in Go constructs the convex hull of projected points. Its orientation test is exact for the float64 inputs: a floating-point determinant is trusted only when it exceeds Shewchuk's rounding-error bound, and otherwise the sign is computed with rational arithmetic. There is no epsilon, so thin but valid geometry is kept and exactly collinear sets are recognised. GDAL measures planar area in square metres, which is converted to km²; `eoo.method` names both steps. The hull is not clipped by continents, habitat or political boundaries. Straight hull edges belong to the **analysis plane** and are not generally geodesic segments between vertices.

The raw area is preserved. The assessment field applies the explicit AOO floor rule; no replacement polygon is invented. With fewer than three distinct points, collinearity in the analysis plane, or area below 1e-6 m², raw EOO is `null`. Applying the AOO floor in these degenerate cases is an explicit API convention that an assessor can review. Collinearity in longitude/latitude does not necessarily imply collinearity in the selected projection.

`eoo.geodesic_area_km2` is a diagnostic: the same hull vertices, transformed back to WGS84 and joined by ellipsoidal geodesics (Karney's algorithm in PROJ). It is a different polygon from the planar hull, is `null` when that polygon is not positively oriented (for example a world-spanning CEA hull), and never feeds the assessment value. In the benchmark it reproduces the `red` package's EOO, which isolates hull-edge construction as the main source of EOO differences between tools.

`AnalyzeEOOInfluence` (`eoo-aoo influence`) measures how raw EOO depends on each hull vertex. It freezes the complete analysis CRS of the original run, including an automatic LAEA centre, removes one vertex coordinate group at a time (all records at that coordinate) and rebuilds the hull from every remaining point, so interior points can become vertices. It reports the original and reduced raw EOO, absolute and relative reduction, record IDs and input indices. A remaining hull that is degenerate gives `null`, not an invented delta. The AOO floor is not applied. With `recompute_projection` it also reports the raw EOO after choosing a new LAEA centre, which separates geometric influence from the effect of moving the projection. A budget limits the evaluated vertices; a partial run says so and does not claim a global ranking. The analysis never labels or excludes records.

The `compare` command runs the same normalized records under `local-laea` and `iucn-cea` and reports `iucn-cea - local-laea` for raw EOO, assessment EOO, AOO and occupied cells. Its purpose is to make projection sensitivity measurable, not to automatically select the smallest result.

## AOO and grid translation

Each point is assigned to `floor((x-origin_x)/w), floor((y-origin_y)/w)`. Cell intervals are closed on the left/bottom and open on the right/top, so a point on a boundary belongs to exactly one cell. The algorithm does not add buffers and does not materialize a full grid over the bounding rectangle. Each cell has area `w²` in the equal-area plane; physical side lengths on the ground may differ because equal-area projections do not preserve shape.

At fixed grid orientation, occupancy changes only when the origin crosses a coordinate residue modulo `w`. `exact` evaluates a representative of each interval in X and Y plus the base origin; the final circular interval is also included. Exact boundary equality follows the half-open rule and belongs to an adjacent interval. This is a complete translation enumeration for the floating-point coordinates, without rotation.

`sampled` distributes offsets uniformly over one cell period. `auto` chooses between complete and sampled search according to an estimated work budget. Every result states `requested_grid_mode`, `effective_grid_mode` and `search_guarantee`:

- `translation_minimum` (exact): the count is the minimum over every translation at the fixed orientation, for the float64 coordinates received;
- `tested_origins_minimum` (sampled): the minimum among the tested origins only; a lower count may exist;
- `fixed_origin`: no search.

`tested_min_cells`/`tested_max_cells` span the evaluated origins; for an exact search they are the translation minimum and maximum. Their range is sensitivity to grid translation, not biological uncertainty. `base_origin_cells` is the count at the configured origin without any search. A fallback from `auto` is reported in `fallback_reason` (`EXACT_WORK_LIMIT`), in `exact_work_estimate`, in a `GRID_SEARCH_FALLBACK` warning, on the CLI's stderr and in the HTML report. `grid_steps_effective` appears only for sampled searches; `grid_steps` has no effect on an exact result. `compare` adds `GRID_SEARCH_COVERAGE_DIFFERS` when the two projections were searched with different guarantees. Non-standard cell resolution triggers a warning.

All modes share one engine. Each point's cell index along an axis is a monotone function of the candidate offset, because floating-point subtraction, division and floor are monotone; binary search therefore finds every index change with exactly the arithmetic of a direct evaluation. For each X candidate, the sweep updates occupied-cell counts as the Y offset crosses each point's change, instead of rebuilding a map per origin. Ties keep the first minimum in X-then-Y order, as the original nested loop did. The previous nested-loop enumeration is kept in the tests as the reference, and the final count is re-checked by direct evaluation at the chosen origin.

The work limit is 25 million cell-count updates, `|X candidates| × (n + 2 × Y changes)`. Measured on an AMD Ryzen 9 9900X, one update costs about 37 ns, against about 47 ns per point evaluation in the previous enumeration, so the limit keeps a worst case near 1 s (2,850 well-spread points: 0.96 s) against 1.27 s before. Memory is O(n) plus the change lists. With this cost model about 2,800 well-spread points still fit an exact search, where the previous limit admitted about 290. As a result, `auto` now returns an exact minimum for data sets that previously fell back to sampled search, for example *Acanthochelys spixii* under local LAEA (944 km² exact, previously 956 km² from a 10 × 10 sample).

Translation search does not remove **orientation** dependence: the CRS axes determine square-grid orientation. Consequently, two equal-area projections can produce different AOOs even when polygon area is preserved to very high precision.

## Comparability and export

For a temporal series, freeze projection strategy, centre when applicable, grid origin, orientation, resolution, record-inclusion rules and GDAL/PROJ versions. Every result carries this as `protocol` (`AnalysisProtocol`, protocol version 1): the input CRS, strategy, full analysis PROJ definition, centre, cell size, effective grid origin, orientation, boundary rule, hull and area methods, inclusion-rules version and the native environment. `calc --protocol` (or `CalculateWithProtocol`) reproduces it with a fixed grid. Settings that disagree with the protocol, an unknown method or a PROJ definition that does not match the centre fail before calculation. A different software, GDAL or PROJ version is reported in `PROTOCOL_ENVIRONMENT_CHANGED`; bit-identical output across native versions is not promised.

Three hashes separate three questions. `input_sha256` covers the ordered records and their metadata. `geometry_sha256` covers the sorted unique normalized WGS84 coordinates (with -0 written as 0, longitude 180 as -180 and pole longitudes as 0), so it is unchanged by record order, duplicates or metadata. `protocol_sha256` covers the protocol without its environment. The geometry hash does not replace record provenance. Re-optimizing grid origin or recalculating a local projection centre independently at each date can introduce differences unrelated to biological change. A fixed-grid time series and a separate sensitivity analysis can be useful complementary outputs.

The GeoPackage contains exact geometries in the analysis plane: unique occurrences, occupied AOO cells and EOO hull, with the analysis CRS attached to each layer. Detailed JSON is the complete record-provenance source; the GPKG retains provenance indices and counts. When no valid hull exists, the EOO layer exists but is empty.

GeoJSON is a display representation. GDAL segmentizes projected edges every 250 m, transforms them to WGS84 and writes RFC 7946 output, including antimeridian cutting. Nine decimal places are serialization precision, not an accuracy claim. Cells very near a pole and polygons enclosing a pole require inspection in the consuming GIS; prefer the GeoPackage for authoritative geometry. Do not recalculate authoritative metrics from map pixels or display GeoJSON.

The MapLibre HTML embeds these WGS84 display GeoJSON layers in a single document. The interactive map is not the authoritative source of area values; authoritative metrics remain the JSON report and the analysis-plane geometries. MapLibre GL JS and the demonstration basemap are loaded from the network when the HTML is opened.

## Outside the scope of this version

The software does not estimate occupancy probability, sampling correction, georeferencing uncertainty, suitable habitat, number of locations, fragmentation, decline, or IUCN categories. It does not infer taxonomic-name validity. Biological filtering must occur before calculation. It also does not implement a geodesic/global hull, grid-rotation optimization, polygon/raster input, persistence, or incremental EOO/AOO updates. It never chooses the smallest EOO among projections and never removes outliers, (0,0) records or old records without an explicit manifest.

Go provides the application integration, concurrency and distribution layer; GDAL/PROJ provides established geospatial transformation and geometry capabilities. A Node implementation could also be correct. The improvement claimed here is an explicit, auditable and testable spatial contract, not automatic language superiority or an already-demonstrated comparative performance advantage.
