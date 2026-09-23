# go-iucn-eoo-aoo

**Version 0.1.0 — an auditable Go library, CLI and local API for IUCN-oriented EOO/AOO calculations with GDAL/PROJ.**

The core transforms coordinates with [`airbusgeo/godal` v0.0.18](https://github.com/airbusgeo/godal/tree/v0.0.18), computes metrics in equal-area projected coordinate systems, and records projection, grid, GDAL/PROJ versions and input provenance in every result. It never measures area in Web Mercator and never approximates kilometres with a fixed degrees conversion.

Two projection strategies are explicit in every result:

- `local-laea` (default): a local Lambert Azimuthal Equal Area projection centred on the assessment distribution;
- `iucn-cea`: World Cylindrical Equal Area (ESRI:54034), matching the projection documented by the IUCN EOO Calculator.

`iucn-cea` is a methodological compatibility mode, not IUCN certification and not an official reproduction of the ArcGIS tool. See [docs/PROJECTIONS.md](docs/PROJECTIONS.md).

The Go module is:

```text
github.com/lsbjordao/go-iucn-eoo-aoo
```

The executable is `eoo-aoo`.

## Installation

### Go library

Add the versioned module to your Go project:

```bash
go get github.com/lsbjordao/go-iucn-eoo-aoo@v0.1.0
```

Import the public package as `eooaoo`. API documentation and runnable examples
are available on [pkg.go.dev](https://pkg.go.dev/github.com/lsbjordao/go-iucn-eoo-aoo).
Building applications that use the library requires Go 1.23+, GDAL 3.8+, PROJ,
`pkg-config`, a C/C++ compiler, and CGO enabled.

### Linux and macOS — prebuilt release

The normal installer downloads a precompiled, SHA-256-verified release. It does **not** install Go and does **not** compile the source code.

```bash
curl -fsSL https://raw.githubusercontent.com/lsbjordao/go-iucn-eoo-aoo/main/scripts/install.sh | sh
```

Or, from a cloned repository:

```bash
sh scripts/install.sh
```

The default destination is `~/.local/bin/eoo-aoo`. Override it with:

```bash
PREFIX=/usr/local sh scripts/install.sh
```

Supported prebuilt Unix targets are:

- Debian 13 amd64;
- Ubuntu 24.04 amd64;
- macOS Apple Silicon (`arm64`);
- macOS Intel (`amd64`).

Linux uses the distribution GDAL/PROJ runtime packages. macOS uses Homebrew for the GDAL runtime. Neither path requires a Go toolchain.

### Debian 13 package

Tagged releases also publish a native `.deb`:

```bash
sudo apt install ./eoo-aoo-debian13-amd64.deb
```

The package installs `/usr/bin/eoo-aoo` and declares its GDAL/PROJ runtime dependencies through Debian packaging.

### Windows 10/11 x64 — prebuilt portable bundle

The Windows release is built natively with MSYS2 UCRT64 and ships with the GDAL/PROJ DLL dependency set and spatial resource data. End users do **not** need Go, GCC or MSYS2.

From PowerShell:

```powershell
iwr https://raw.githubusercontent.com/lsbjordao/go-iucn-eoo-aoo/main/scripts/install.ps1 -OutFile $env:TEMP\install-eoo-aoo.ps1
powershell -ExecutionPolicy Bypass -File $env:TEMP\install-eoo-aoo.ps1
```

The installer downloads `eoo-aoo-windows-amd64.zip`, verifies its SHA-256 checksum, installs it under `%LOCALAPPDATA%\Programs\eoo-aoo`, and adds that directory to the user `PATH`.

The ZIP can also be downloaded from GitHub Releases and `eoo-aoo.exe` run directly from the extracted directory.

### Build from source

Source compilation is a developer path rather than the default installation route:

```bash
git clone https://github.com/lsbjordao/go-iucn-eoo-aoo.git
cd go-iucn-eoo-aoo
sh scripts/install-from-source.sh
```

Building from source requires Go 1.23+, GDAL 3.8+, PROJ, `pkg-config`, and a C/C++ compiler because `godal` uses CGO.
GDAL 3.8 is the floor because earlier versions (such as the 3.6 series in Debian 12) do not split antimeridian-crossing
polygons into a `MultiPolygon` during RFC 7946 GeoJSON export, which produces display geometries that span the globe.

### Container

```bash
docker compose up --build -d
```

The container is useful for reproducible server and pipeline deployments.

## Input formats

CSV, JSON and GeoJSON are first-class input formats. TSV is also accepted.

The canonical tabular occurrence schema is deliberately small:

| Field | Type | Required | Meaning |
|---|---|---:|---|
| `id` | string | no | Record identifier preserved in provenance |
| `taxon` | string | recommended | Taxon name or assessment-unit label; required by `batch` |
| `lon` | number | yes | Longitude in degrees with the default `EPSG:4326` input CRS |
| `lat` | number | yes | Latitude in degrees with the default `EPSG:4326` input CRS |

For projected or other coordinate reference systems, custom coordinate columns can be supplied through `--x-col` and `--y-col`; numeric values are then interpreted according to `--input-crs`.

For interoperability with biodiversity datasets, `decimalLongitude`/`decimalLatitude` and `scientificName` are also recognized automatically in tabular CSV/JSON input. The canonical schema in this repository remains `id,taxon,lon,lat`.

### CSV

```csv
id,taxon,lon,lat
demo-001,Mimosa demonstrativa,-43.2,-22.9
demo-002,Mimosa demonstrativa,-44.3,-21.5
demo-003,Mimosa demonstrativa,-42.9,-20.8
```

```bash
eoo-aoo calc --in occurrences.csv --out result.json
```

### JSON

JSON input is an array of records using the same field names:

```json
[
  {"id":"demo-001","taxon":"Mimosa demonstrativa","lon":-43.2,"lat":-22.9},
  {"id":"demo-002","taxon":"Mimosa demonstrativa","lon":-44.3,"lat":-21.5},
  {"id":"demo-003","taxon":"Mimosa demonstrativa","lon":-42.9,"lat":-20.8}
]
```

```bash
eoo-aoo calc --in occurrences.json --out result.json
```

### GeoJSON

GeoJSON input is an RFC 7946 `FeatureCollection` containing `Point` geometries. The record identifier can be supplied as the Feature `id`, while `taxon` belongs in `properties`:

```json
{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "id": "demo-001",
      "properties": {"taxon": "Mimosa demonstrativa"},
      "geometry": {
        "type": "Point",
        "coordinates": [-43.2, -22.9]
      }
    },
    {
      "type": "Feature",
      "id": "demo-002",
      "properties": {"taxon": "Mimosa demonstrativa"},
      "geometry": {
        "type": "Point",
        "coordinates": [-44.3, -21.5]
      }
    }
  ]
}
```

```bash
eoo-aoo calc --in occurrences.geojson --out result.json
```

GeoJSON is always interpreted as WGS84 (`EPSG:4326`) as required by RFC 7946. Null geometries, LineStrings, Polygons, MultiPoints and other non-Point geometries are rejected. Altitude, when present as a third coordinate, is ignored.

## Quick start

```bash
eoo-aoo calc --in examples/brazil.csv --out result.json \
  --gpkg result.gpkg --geojson-dir maps --html result.html --details
```

Equivalent JSON and GeoJSON examples:

```bash
eoo-aoo calc --in examples/occurrences.json --out result.json --html result.html

eoo-aoo calc --in examples/occurrences.geojson --out result.json --html result.html
```

### Choose the projection strategy

```bash
# Regional geodetic default
eoo-aoo calc --in occurrences.csv \
  --projection local-laea --out local.json --html local.html

# World Cylindrical Equal Area used by the documented IUCN EOO Calculator method
eoo-aoo calc --in occurrences.csv \
  --projection iucn-cea --out iucn-cea.json --html iucn-cea.html
```

### Compare both strategies

```bash
eoo-aoo compare --in occurrences.csv --out projection-comparison.json
```

The comparison runs the same records and grid configuration under both strategies and reports `iucn-cea - local-laea` differences for raw EOO, assessment EOO, AOO and occupied-cell count, including percentage differences when defined.

## Output

`calc` always returns a JSON calculation report, either to `stdout` or to the path supplied with `--out`. Additional GIS and interactive outputs can be requested independently.

An abridged result has this shape; numeric values below are illustrative:

```json
{
  "schema_version": "1",
  "software_version": "0.1.0",
  "gdal_version": "...",
  "proj_version": "...",
  "taxon": "Mimosa demonstrativa",
  "input_sha256": "...",
  "input_records": 5,
  "unique_coordinates": 4,
  "duplicate_records": 1,
  "config": {
    "input_crs": "EPSG:4326",
    "projection_strategy": "local-laea",
    "cell_size_m": 2000,
    "grid_mode": "auto",
    "grid_steps": 10,
    "origin_m": [0, 0]
  },
  "projection": {
    "strategy": "local-laea",
    "reference": "...",
    "input_crs": "EPSG:4326",
    "analysis_proj": "...",
    "center_lon_lat": [-43.4, -21.8],
    "center_method": "spherical mean of sorted unique WGS84 coordinates",
    "max_angular_distance_deg": 1.5,
    "axis_order": "traditional GIS: longitude/easting, latitude/northing"
  },
  "eoo": {
    "raw_area_km2": 1234.56,
    "assessment_area_km2": 1234.56,
    "status": "ok",
    "adjusted_to_aoo": false,
    "hull_wkt": "POLYGON ((...))"
  },
  "aoo": {
    "area_km2": 16,
    "occupied_cells": 4,
    "cell_size_m": 2000,
    "iucn_2km_scale": true,
    "grid_mode": "exact",
    "origin_m": [412.3, 781.4],
    "candidate_origins": 25,
    "min_cells": 4,
    "max_cells": 5,
    "complete_translation_search": true,
    "orientation": "analysis CRS axes; no rotation search",
    "boundary_rule": "..."
  },
  "warnings": [
    "AUTO_CENTER: freeze center and grid origin for temporal comparisons",
    "POINT_BASED_ESTIMATE: records alone may underestimate occupancy; occurrence suitability and other Red List subcriteria require assessment"
  ]
}
```

The actual `projection` object also contains the full input and analysis WKT definitions. They are omitted from the example above only to keep the README readable.

### Main JSON fields

| Field | Meaning |
|---|---|
| `schema_version` | Version of the JSON result contract; `1` for the initial public release |
| `software_version` | Version of `go-iucn-eoo-aoo` that produced the result |
| `gdal_version`, `proj_version` | Native geospatial library versions loaded at runtime |
| `taxon` | Assessment-unit label when present in the input |
| `input_sha256` | SHA-256 of the normalized input records, useful for reproducibility |
| `input_records` | Number of records received |
| `unique_coordinates` | Number of distinct WGS84 coordinate pairs after normalization |
| `duplicate_records` | Records sharing coordinates with another input record |
| `config` | Effective configuration after defaults and user overrides are applied |
| `projection` | Analysis projection strategy, CRS definitions, centre and axis metadata |
| `eoo` | Extent of Occurrence result |
| `aoo` | Area of Occupancy result |
| `warnings` | Methodological or reproducibility warnings that should be reviewed |

### EOO fields

| Field | Meaning |
|---|---|
| `raw_area_km2` | Convex-hull area in the selected equal-area analysis plane; `null` when a non-degenerate hull cannot be constructed |
| `assessment_area_km2` | EOO value used by the result after the explicit rule that EOO must not be smaller than AOO |
| `status` | `ok`, `insufficient_unique_points`, `collinear`, or `numerically_degenerate` |
| `adjusted_to_aoo` | `true` when the assessment EOO was set to AOO because raw EOO was unavailable or smaller |
| `hull_wkt` | Exact convex hull in the analysis plane when a valid hull exists; omitted when no hull can be constructed |

With fewer than three unique coordinates, collinear points, or a numerically degenerate hull, `raw_area_km2` is `null`. The software does not invent a replacement polygon: `assessment_area_km2` uses AOO and a warning is emitted.

### AOO fields

| Field | Meaning |
|---|---|
| `area_km2` | `occupied_cells × cell_size_m²`, converted to km² |
| `occupied_cells` | Number of occupied grid cells for the selected origin |
| `cell_size_m` | Grid side length; default is 2,000 m |
| `iucn_2km_scale` | Whether the 2 × 2 km IUCN reference scale is being used |
| `grid_mode` | Effective search mode: `auto`, `exact`, `sampled`, or `fixed` |
| `origin_m` | Selected grid origin in analysis-CRS metres |
| `candidate_origins` | Number of candidate translations evaluated |
| `min_cells`, `max_cells` | Occupancy range among evaluated origins |
| `complete_translation_search` | Whether the translation search is exhaustive for the fixed grid orientation |
| `orientation` | Records that the square grid follows the analysis CRS axes |
| `boundary_rule` | Rule used when a point lies exactly on a grid-cell boundary |

Grid translation is optimized; grid rotation is not. Two equal-area projections can therefore produce different AOO values because their axes orient the square grid differently.

### `--details`

By default, `calc` omits the potentially large `cells` and `points` provenance arrays. Add `--details` to retain them:

```bash
eoo-aoo calc --in occurrences.csv --out result.json --details
```

Detailed results add:

- `cells`: occupied AOO cell indices, unique-point counts and projected bounds;
- `points`: normalized unique occurrence coordinates, projected coordinates, zero-based input indices and all original records sharing each coordinate.

The EOO hull WKT, when it exists, is part of both the default and detailed JSON result. `--details` specifically controls the large cell and point provenance arrays.

### Spatial and interactive outputs

`calc` can create several outputs in one run:

```bash
eoo-aoo calc --in occurrences.csv --out result.json \
  --gpkg result.gpkg \
  --geojson-dir maps \
  --html result.html
```

| Output | Option | Purpose |
|---|---|---|
| JSON | `--out result.json` | Authoritative metrics, configuration, versions, warnings and optional provenance |
| GeoPackage | `--gpkg result.gpkg` | Exact analysis-plane occurrence points, occupied AOO cells and EOO hull with the analysis CRS |
| GeoJSON directory | `--geojson-dir maps` | WGS84 display layers for occurrences, AOO and EOO |
| HTML | `--html result.html` | Interactive MapLibre report with embedded display GeoJSON and calculation metadata |

The GeoPackage is preferred for authoritative GIS geometry. Exported GeoJSON and the HTML map are display products; do not recalculate authoritative areas from their rendered geometries or pixels.

### `compare` output

```bash
eoo-aoo compare --in occurrences.csv --out projection-comparison.json
```

The comparison report contains:

- `local_laea`: summary result using local LAEA;
- `iucn_cea`: summary result using World Cylindrical Equal Area;
- `delta_iucn_cea_minus_local_laea`: absolute and percentage differences in raw EOO, assessment EOO and AOO, plus occupied-cell difference;
- `note`: interpretation warning about projection dependence.

The sign of every delta is **`iucn-cea - local-laea`**. The command reports sensitivity; it does not automatically choose one strategy as universally superior.

### `batch` output

```bash
eoo-aoo batch --in examples/batch.csv --workers 4 --out batch.json
```

`batch` groups records by `taxon` and returns an array with one item per assessment unit. Each item contains `taxon` plus either a `result` or an `error`. Use `--details` if full per-taxon provenance is required.

## Interfaces

| Interface | Purpose |
|---|---|
| Go library | `Calculate(ctx, points, config)` for workers and pipelines |
| `calc` | One assessment unit; rejects mixed taxa |
| `compare` | Runs `local-laea` and `iucn-cea` on the same input |
| `batch` | Splits input by taxon and processes groups concurrently |
| `serve` | `POST /v1/calculate` and `GET /healthz` |
| GeoPackage | Exact analysis-plane cells, occurrences and EOO hull |
| GeoJSON | WGS84 input FeatureCollections and WGS84 display layers via GDAL RFC 7946 |
| HTML | Interactive MapLibre EOO/AOO/occurrence report |

Additional examples:

```bash
# SIRGAS 2000 geographic coordinates
eoo-aoo calc --in occurrences.csv --input-crs EPSG:4674 --out result.json

# Projected input with custom coordinate columns
eoo-aoo calc --in utm.csv --x-col easting --y-col northing \
  --input-crs EPSG:31983 --out result.json

# Fixed projection centre and grid for a temporal series
eoo-aoo calc --in examples/brazil.csv \
  --config examples/fixed-brazil.json --out series.json

# Go library example
go run -buildvcs=false ./examples/library
```

`examples/fixed-brazil.json` is an example configuration, not a required CRS for Brazilian flora. Repeated temporal analyses should preserve projection strategy, centre when applicable, grid origin, resolution, inclusion rules and native GDAL/PROJ versions.

## Grid-origin search

With a single unique coordinate (including duplicate records at that coordinate), `auto`, `exact`, and `sampled` center the occupied square on the projected point. For the default 2 km cell, its bounds extend 1,000 m in each axis direction from the point and AOO remains 4 km². The selected `aoo.origin_m` is the square's lower-left corner. `fixed` always preserves the configured origin.

| Mode | Behaviour |
|---|---|
| `auto` (default) | Complete translation search when within the work budget; otherwise a 10 × 10 sampled search |
| `exact` | Enumerates coordinate-residue intervals; minimum for the **fixed CRS-axis orientation** |
| `sampled` | Lowest occupied-cell count among `--grid-steps`² origins; does not prove the global translation minimum |
| `fixed` | Explicit origin, useful for temporal comparisons |

The work limit is 25 million point/origin evaluations per calculation. Grid translation is optimized; grid rotation is not. Because square-grid orientation follows the projected CRS axes, two equal-area projections can produce different AOO values. The `compare` command exposes that sensitivity rather than hiding it.

See [docs/METHODOLOGY.md](docs/METHODOLOGY.md) and [docs/PROJECTIONS.md](docs/PROJECTIONS.md).

## API

```bash
eoo-aoo serve --listen 127.0.0.1:8080 --workers 4

curl --fail-with-body http://127.0.0.1:8080/v1/calculate \
  -H 'Content-Type: application/json' --data-binary @examples/request.json
```

The API `points` member accepts the same JSON-array records or an RFC 7946 GeoJSON FeatureCollection. `projection_strategy` can be supplied in the API `config` object. When `EOO_AOO_TOKEN` is defined, calculations require `Authorization: Bearer <token>`. The default server listens only on loopback and is stateless.

The HTTP server is an implementation detail used by the CLI and lives under `internal/server`; it is not part of the public Go import API. The public library API is the root `eooaoo` package.

See [docs/INTEGRATION.md](docs/INTEGRATION.md) for the integration contract.

## Project layout

The root directory is the public Go package. The command and implementation-only packages are separated according to normal Go module conventions:

```text
go-iucn-eoo-aoo/
├── calculate.go
├── comparison.go
├── export.go
├── grid.go
├── html.go
├── hull.go
├── projection.go
├── types.go
├── cmd/
│   └── eoo-aoo/
├── internal/
│   ├── input/
│   └── server/
├── examples/
├── docs/
└── scripts/
```

External Go programs import only the public root package:

```go
import eooaoo "github.com/lsbjordao/go-iucn-eoo-aoo"
```

Packages below `internal/` are intentionally unavailable to external modules.

## Scientific scope

Scale and interpretation rules follow IUCN Red List guidance. This software is **not certified by the IUCN** and does not assign threat categories. Occurrence records still require biological curation for identification, origin, current presence, seasonality, precision and assessment period. Point-only AOO may underestimate actual occupancy.

`local-laea` is this project's regional geodetic implementation choice. `iucn-cea` uses the World Cylindrical Equal Area projection documented by the IUCN EOO Calculator materials. Equal-area projection does not eliminate projection dependence in hull shape or square-grid orientation; antimeridian distributions deserve particular inspection in a global cylindrical projection.

## Release engineering

Tagged releases are built natively on Linux, macOS and Windows. Windows uses the MSYS2 **UCRT64** environment and recursively identifies DLL dependencies before creating the portable ZIP. Every published asset is covered by `SHA256SUMS`, and the end-user installers verify that checksum before installation.

GitHub-hosted macOS release builds target both `arm64` and Intel `amd64`. Windows currently targets x64. Linux prebuilt binaries currently target Debian 13 and Ubuntu 24.04 on amd64.

## Validation

Validation includes `go vet`, race-enabled tests, antimeridian and polar cases, degenerate inputs, GIS exports, concurrency, projection-strategy comparison and independent geodesic checks. See [docs/VALIDATION.md](docs/VALIDATION.md) for the exact environment, evidence and limitations.

## Provenance and license

MIT licensed. `godal` is Apache-2.0; GDAL/PROJ and bundled Windows runtime libraries retain their own licenses. See [THIRD_PARTY.md](THIRD_PARTY.md).
