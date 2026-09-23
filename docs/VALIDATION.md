# Validation performed on 2026-09-22 and 2026-09-23

## Environment used

- Linux amd64; Go **1.27.1**; `airbusgeo/godal` binding **v0.0.18**.
- GDAL **3.8.4**, PROJ **9.4.0**, using real native libraries.
- Separate cross-check: pyproj **3.8.0**, PROJ **9.8.1**, and Shapely/GEOS.
- `CGO_ENABLED=1`, `PROJ_NETWORK=OFF`.

The code was compiled, exercised through the CLI, and served through HTTP. It is not only a project skeleton.

## Checks performed

| Check | Result |
|---|---|
| `go vet ./...` | Passed |
| `go test -race -count=1 -cover ./...` | Passed; no Go data race detected |
| CLI `calc`, `batch`, `version` | Executed |
| Real HTTP API: health, authenticated POST, SIGTERM shutdown | 200, 200, exit 0 |
| GeoPackage opened with `ogrinfo` | Layers and CRS recognized |
| Antimeridian GeoJSON | Polygons split without longitude-spanning segments >180° |
| One-point case: JSON, GPKG and GeoJSON | AOO 4 km²; empty hull; raw EOO null |
| Seven reference-package JSON inputs | All processed; metrics stored in `reference-input-validation.json` |

Measured coverage was **85.1% in the core**, 85.2% in input parsing, 83.9% in the HTTP handler, and 13.8% in the CLI package. Full CLI commands were also exercised separately outside coverage instrumentation. Coverage percentages are not scientific certification.

Tests cover known-area cases at 0°, 23°S, 70°N and 85°S; fewer than three points; collinearity; EOO/AOO flooring; duplicate records and ID preservation; input-order invariance; antimeridian and polar regions; rejection of near-antipodal/global local-LAEA input; invalid coordinates; negative/positive cell boundaries; narrow-interval translation minima; exact-search comparison against an independent enumeration; work limits; cancellation; taxon separation; concurrent calls; and GIS exports.

Version 0.1.0 includes explicit tests for `local-laea`, `iucn-cea`, the projection comparison report, the interactive HTML generator, and preservation of the `ANTIMERIDIAN_CEA` warning for antimeridian-crossing input under the global CEA strategy.

## Independent area cross-check

`scripts/validate_geodesy.py` constructs **100 × 100 km** squares in the LAEA analysis plane, transforms their vertices to WGS84, and sends them through the CLI. It compares the result against 10,000 km², GEOS/Shapely planar area, and ellipsoidal geodesic area along edges densified every 100 m.

Across the four cases, relative difference from the geodesic measurement was below **3.1 × 10⁻¹¹**. Complete values are in `geodesy-validation.json`. This validates those synthetic geometries and the transformation/area-computation path; it does not represent uncertainty in real occurrence data and does not prove that a given hull is biologically appropriate for every species. The comparison shares PROJ's mathematical implementation but uses a different area algorithm and a different native-library version.

```bash
make build
python -m pip install pyproj shapely
python scripts/validate_geodesy.py
```

## Reference implementation benchmark

On 2026-09-23 the reproducible benchmark harness was run against two independent implementations:

- `gdauby/ConR` 2.1 at pinned commit `50b9924bcd6bcec2bf2f2d775fa47bb29b721ba6`;
- `vicentecalfo/eoo-aoo-calculator` 1.2.2 at pinned commit `d47f41deb6041b622aa9aa1587bff4702b953b73`.

The same normalized occurrence datasets were supplied to each implementation. The most important observed comparisons were:

| Case | This project | Independent reference | Observation |
|---|---:|---:|---|
| Brazil raw EOO, CEA | 15631.477162575591 km² | ConR planar CEA 15631.5 km² | ~0.000146% difference |
| Antimeridian raw EOO, CEA | 436262.5896382405 km² | ConR planar CEA 436263 km² | ~0.000094% difference |
| Antimeridian regional/geodesic-like | local-LAEA 242.50282819926466 km² | ConR spheroid 243.5 km² | close values under a different methodological family |
| Tiny-triangle AOO | 4 km² | ConR 4 km²; Turf-based reference 12 km² | exposes grid-construction sensitivity |
| Collinear raw EOO | `null` | ConR small jitter-derived value; Turf-based reference 0 | semantic/methodological difference, not interchangeable outputs |

The antimeridian result is especially diagnostic. The Go implementation and ConR independently reproduce essentially the same **large** value under planar CEA, while `local-laea` and ConR's spheroid mode both produce values around 243 km². This supports the interpretation that the world-spanning CEA hull is a projection/method effect rather than random numerical failure in one implementation. The software therefore retains `iucn-cea` for compatibility-oriented comparison but emits a dedicated warning when the input crosses the antimeridian.

The benchmark also confirmed agreement on the 4 km² AOO for single-point and duplicate-coordinate cases across this project and ConR. The Turf-based reference returned AOO = 0 for the antimeridian case and 12 km² for the tiny triangle; these are preserved as observed behavioural differences rather than used as an accuracy ranking.

Complete interpretation and exact values are documented in [REFERENCE_BENCHMARK.md](REFERENCE_BENCHMARK.md). Reproduce the automated part with:

```bash
make benchmark
```

GeoCAT and the IUCN EOO Calculator remain pending manual/reference measurements on the same corpus.

## Observed microbenchmark

A synthetic set of 1,000 points using automatic centring and a sampled 10 × 10 grid search took approximately **5.85 ms/op**, allocating about 6.36 MB/op. Only three repetitions were run in the validation environment (AMD EPYC 9V74). This is not a performance study, does not represent a typical workstation, and cannot be used as a direct comparison with Node/Turf. Reproduce with:

```bash
go test -run '^$' -bench BenchmarkCalculate1000 -benchtime=3x -benchmem .
```

## Release-platform validation

The repository defines native release builds for:

- Debian 13 amd64;
- Ubuntu 24.04 amd64;
- macOS 15 arm64;
- macOS 15 Intel amd64;
- Windows Server 2025 / Windows x64 using MSYS2 UCRT64.

The Windows release job installs the UCRT64 GCC, GDAL, PROJ, pkg-config and `ntldd` toolchain, runs the Go tests with CGO enabled, recursively resolves DLL dependencies, bundles GDAL/PROJ resource directories, and then runs the packaged executable with the MSYS2 runtime removed from `PATH`. This is intended to detect accidentally missing DLLs before a release is published.

Release assets are hashed into `SHA256SUMS`, and both end-user installers verify that checksum before installation.

At the time this document was updated, GitHub Actions jobs for this repository were failing before any workflow step started (`runner_id: 0` / no steps). That is an account/runner execution issue rather than a recorded test failure, so a tagged release must not be described as validated on a platform until its native release job has actually completed successfully.

## Validation limits

There has been no independent IUCN homologation, no completed GeoCAT/IUCN EOO Calculator comparison on the benchmark corpus, no formal security audit, and no validation of every possible datum transformation. The ConR and `vicentecalfo/eoo-aoo-calculator` comparison is an initial external implementation benchmark, not universal scientific validation. The software remains a functional, auditable pre-1.0 implementation with explicit methodological choices and known limits that should continue to undergo scientific review before institutional use.
