# Integration into local stacks

## Go library

Import the module directly:

```go
import geo "github.com/lsbjordao/go-iucn-eoo-aoo"
```

The example in `examples/library/main.go` compiles against this package.

Each `Calculate` call creates and closes its own native objects; independent calls can run in goroutines. Do not share mutable property maps while a calculation is running. The Go context interrupts Go-side stages and loops; a native call that is already in progress is not preempted by context cancellation. `-race` checks concurrent access in Go code but does not prove the absence of internal failures in C/C++ libraries.

## HTTP API

`POST /v1/calculate`, `Content-Type: application/json`:

```json
{
  "points": [
    {"id":"occ-1","taxon":"Mimosa demonstrativa","lon":-43.2,"lat":-22.9},
    {"id":"occ-2","taxon":"Mimosa demonstrativa","lon":-44.3,"lat":-21.5},
    {"id":"occ-3","taxon":"Mimosa demonstrativa","lon":-42.9,"lat":-20.8}
  ],
  "config": {
    "input_crs":"EPSG:4326",
    "projection_strategy":"local-laea",
    "cell_size_m":2000,
    "grid_mode":"auto"
  }
}
```

The canonical tabular record fields are `id`, `taxon`, `lon` and `lat`, matching the CSV and JSON CLI inputs documented in the README. `points` also accepts a GeoJSON FeatureCollection. The default response is a summary. `?details=1` includes occupied cells and all source-record provenance and can substantially increase response size. The API does not write files on the server; GIS and HTML exports are produced through the CLI or library.

| Status | Meaning |
|---|---|
| 200 | Calculation or health check completed |
| 400 | Invalid JSON/columns or unknown configuration field |
| 401 | Missing/incorrect bearer token when authentication is configured |
| 413 | Request body exceeds 16 MiB |
| 415 | Unsupported content type |
| 422 | Coordinates/configuration rejected, spatial domain exceeded, or calculation interrupted |
| 503 | All calculation slots are occupied |

`GET /healthz` reports the software and GDAL versions without requiring a token. HTTPS and access control between machines belong to deployment infrastructure. The default Compose configuration publishes only on the local host.

## Jobs, lakehouses and updates

A useful unit of work is `(taxon_id, period, inclusion_rules, data_version, spatial_config)`. A worker can fetch already-curated occurrences, call the Go library, and persist JSON/GPKG output together with the snapshot identifiers used for the run.

For Airflow or a Go job broker, call the CLI with a JSON/file input or embed the library directly in the worker. Batch mode records per-taxon errors so failed groups can be retried independently. The CLI exits non-zero when one or more groups fail, while preserving the batch report for successfully processed groups.

Cells and hulls are not incrementally updated in this version. When records for one assessment unit change, recalculate that unit. Removing records can alter both the hull and the optimal grid origin, so incrementing counters is not a substitute for a complete recalculation. Persistence, taxon indexing and incremental orchestration remain responsibilities of the surrounding stack.

## Release installation in managed environments

For managed Linux hosts, prefer the tagged release artifacts rather than compiling on every worker. Debian 13 can use the `.deb`; Ubuntu 24.04 can use the prebuilt tarball through `scripts/install.sh`. Both still use the host's GDAL/PROJ runtime libraries.

Windows x64 releases are portable bundles containing `eoo-aoo.exe`, the recursively resolved MSYS2 UCRT64 runtime DLL set, and bundled `share/gdal` and `share/proj` resources. `scripts/install.ps1` installs the bundle per-user and updates the user PATH; no Go compiler or MSYS2 installation is required on the target machine.

macOS releases are built separately for Apple Silicon and Intel and use the Homebrew GDAL runtime on the target host. This avoids source compilation while keeping native GDAL updates under the package manager.

Every release artifact is covered by `SHA256SUMS`; the provided end-user installers verify SHA-256 before installation.

## Future extensions with separate contracts

Possible future work includes direct GPKG/PostGIS input through GDAL, minimum-accuracy requirements for datum operations, coordinate-uncertainty intervals, validated comparison with additional reference tools, asynchronous APIs for very large batches, ARM64 Windows packaging, and further study of geodesic hulls and global distributions. These are not advertised as current features.
