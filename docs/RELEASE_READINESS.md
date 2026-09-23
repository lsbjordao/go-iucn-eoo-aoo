# v0.1.0 release readiness

This document separates implemented release safeguards from evidence that must be produced by an actual tagged build.

## Implemented safeguards

- Public package documentation and executable `ExampleCalculate` for pkg.go.dev.
- Canonical CSV/JSON/GeoJSON input contract.
- JSON Schema contracts for input, result, comparison and batch output.
- Versioned synthetic scientific regression corpus under `testdata/reference/`.
- Parser and calculation fuzz targets.
- CI matrix covering the declared minimum Go version (1.23.x) and current Go 1.27.x on Linux.
- Native Windows CI with GDAL/PROJ through MSYS2 UCRT64.
- Tagged release workflow for Debian 13, Ubuntu 24.04, macOS arm64/amd64 and Windows x64.
- Portable Windows bundle test with MSYS2 removed from `PATH`.
- SHA-256 checksums for all release assets.
- End-user installers that verify release checksums.

## Evidence required before publishing v0.1.0

Do not describe a target as release-validated until its native workflow has completed successfully.

1. Run the normal CI successfully on Linux Go 1.23.x, Linux Go 1.27.x, Windows and container jobs.
2. Create a release-candidate tag such as `v0.1.0-rc.1`.
3. Confirm all native release jobs complete successfully: Debian 13, Ubuntu 24.04, macOS arm64, macOS amd64 and Windows x64.
4. Download every generated artifact and verify it against `SHA256SUMS`.
5. Install the Debian package on a clean Debian 13 environment and run `eoo-aoo version` plus at least one CSV, JSON and GeoJSON calculation.
6. Extract the Windows ZIP on a clean Windows 10/11 environment and run `eoo-aoo.exe version` plus at least one calculation without Go or MSYS2 installed.
7. Exercise both macOS archives on their native architectures with Homebrew GDAL installed.
8. Review the generated GitHub Release contents and third-party license files.
9. Only after those checks, create the final immutable `v0.1.0` tag from the exact tested commit.
10. After publication, request/confirm module discovery through `proxy.golang.org` and `pkg.go.dev`.

## Scientific follow-up

The synthetic corpus guards known invariants, but it is not external homologation. Independently generated GeoCAT/IUCN/ConR comparisons should be added under `testdata/reference/external/` with exact tool versions, dates and settings. Those measurements must not be inferred from this implementation.

The software should continue to state that it computes spatial metrics and does not assign IUCN threat categories or certify biological suitability of occurrence records.
