# Validation record

This record separates four kinds of evidence that must not be confused: **software tests** (the code does what it specifies), **implementation agreement** (independent tools give the same number for the same method), **method sensitivity** (how much a methodological choice moves the number) and **biological adequacy** (whether the records and method suit an assessment). Only the first three are assessed here; biological adequacy is outside the scope of this software.

## 2026-10-13: data audit, search guarantees, influence, protocols and traceable benchmark

Environment: Linux amd64, AMD Ryzen 9 9900X; Go 1.24.0 toolchain (module `go 1.23.0`); GDAL 3.8.4; PROJ 9.4.0; R sf 1.1.3 with GEOS 3.12.1, lwgeom 0.2.17, s2 1.1.13; Python 3.12.3.

### Software tests

| Check | Result |
|---|---|
| `make check` (gofmt, `go vet ./...`, `go test -race ./...`) | Passed |
| Coverage | core 85.4%, input 85.2%, HTTP 84.7%, CLI 59.1% |
| Event-sweep grid search vs the previous nested-loop enumeration | Identical minimum, maximum, chosen origin, base-origin count and cell list on 65 adversarial and random sets × 3 base origins × exact/sampled/fixed |
| Exact search vs an independent rational-arithmetic oracle (`math/big`, no implementation helpers) | Identical on 40 sets including negative coordinates, exact cell multiples, coincident residues, residues one ulp apart and coordinates near 1e7 m |
| Exact ≤ every sampled origin, same CRS/orientation/base origin | Held on 40 random sets × 4 sample sizes |
| Orientation predicate vs exact rational sign | Agreed on the Kettner et al. near-collinear grid (where the naive determinant is wrong) and on 20,000 random near-degenerate triples |
| Hull invariants | Strictly convex, contains every point, unchanged by permutation and duplication, on near-collinear clouds at four offsets and three scales |
| GDAL planar area vs exact shoelace of the same vertices | Within 4·n·ε·max\|coord\|·span + 1e-12 relative on 200 random hulls |
| Audit, curation, influence, protocol, geometry hash, geodesic diagnostic, extent warning | Dedicated unit and CLI tests, including cancellation, duplicate IDs, missing metadata, incompatible manifests and incompatible protocols |
| Published JSON Schemas | Every output type (full and summary result, fallback result, protocol, comparison, audit, manifest, curation log, influence) validates against its schema; the checker rejects undeclared fields |
| Reference corpus | 10 cases, adding two points, an equator-crossing square, a near-pole ring, exact domain limits and a 70° extent, each with an independently derived expectation |
| Fuzzing of the permissive reader against the strict reader | 11.5 million executions after fixing one panic it found (unterminated CSV quote) |

Grid-search performance (`go test -bench BenchmarkGridSearch -count 3`, seed 42, auto mode, 2 km cells):

| Points | Uniform over 1,000 km | Clustered (20 clusters, 5 km) | Effective mode |
|---:|---:|---:|---|
| 100 | 1.28 ms | 1.26 ms | exact |
| 300 | 10.5 ms | 10.0 ms | exact |
| 1,000 | 111 ms | 102 ms | exact |
| 10,000 | 30.2 ms | 25.4 ms | sampled (budget fallback) |

The previous enumeration took 43 ms (100 points) and 1.27 s (300 points) for the same exact searches, with 2 GB allocated at 300 points against 0.6 MB now. The largest exact search inside the unchanged 25-million budget, 2,850 uniform points, took 0.96 s. Times are from one workstation and are not a cross-language comparison.

### Implementation agreement (benchmark run `20261013-improvements-2`)

The sibling harness `go-iucn-eoo-aoo-tests` sent the same normalized points to every tool. CSV and JSON copies were checked by geometry hash. Revisions are real, not `pinned` labels: ConR `50b9924b…` (from the installed DESCRIPTION), Vicente `d47f41de…` (from the build cache). For `red` 1.6.3 from CRAN no VCS revision exists, so the run records `unknown`.

| Comparison | Result |
|---|---|
| Go planar CEA EOO vs ConR planar CEA | Differences 0.16, 0.33 and 0.46 km², within ConR's whole-km² rounding |
| Go planar EOO vs R sf/GEOS hull of the same projected points, same CRS | Equal to printed precision (< 1e-6 km²) for all 8 runs; same vertex count |
| Go AOO vs independent Python enumeration in exact integer arithmetic | Translation minimum, maximum and base-origin count identical for all 8 exact runs |
| Go geodesic-edge diagnostic vs lwgeom (GeographicLib) | Identical; both use Karney's algorithm, so this checks vertex transformation and handling, not the algorithm |
| Rerun of the whole harness | No numeric, method or revision change against run `20261013-improvements` |

### Method sensitivity

- **Hull edges, not hull construction, explain the EOO differences between tools.** The ellipsoidal geodesic-edge area of the Go CEA hull vertices equals `red`'s EOO (2,936,214, 7,734,450 and 966,393 km²). The same vertices on the s2 sphere equal ConR `default-spheroid` (2,947,403, 7,750,770 and 970,066 km²). This confirms what the improvement plan had listed only as a hypothesis.
- **Analysis plane.** For *A. spixii*, local LAEA gives 19.0% more EOO than CEA; for *A. mentalis* −1.1%. The geodesic diagnostic differs from the planar value by −0.77% (CEA) and +0.35% (LAEA) for *A. mentalis*.
- **Grid translation.** Exact minimum, base origin and maximum span 235–251 cells for *A. spixii* in CEA; a 10 × 10 sample found 238. Grids from other tools are not equivalent constructions and are listed only for inspection.
- **Records.** Excluding the single (0,0) record of *A. spixii* through a manifest gives 2,403,674.1904 km² in CEA (−60.3%). Under the frozen LAEA protocol of the original run it gives 2,403,128.7489 km², and 2,403,094.4256 km² with a recomputed centre. This is a sensitivity experiment, not a curation decision.
- **Exact domain.** With the cheaper sweep, *A. spixii* under local LAEA is now searched exactly: 944 km², previously 956 km² from a sample. Its EOO is unchanged.

### Still pending

GeoCAT and the IUCN EOO Calculator have not been measured on this corpus. Shapely could not be installed in this environment (no pip/venv), so R sf/GEOS served as the GEOS oracle.

## Validation performed on 2026-09-22 and 2026-09-23

### Environment used

- Linux amd64; Go **1.27.1**; `airbusgeo/godal` binding **v0.0.18**.
- GDAL **3.8.4**, PROJ **9.4.0**, using real native libraries.
- Separate cross-check: pyproj **3.8.0**, PROJ **9.8.1**, and Shapely/GEOS.
- `CGO_ENABLED=1`, `PROJ_NETWORK=OFF`.

The code was compiled, exercised through the CLI, and served through HTTP. It is not only a project skeleton.

### Checks performed

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

### Independent area cross-check

`scripts/validate_geodesy.py` constructs **100 × 100 km** squares in the LAEA analysis plane, transforms their vertices to WGS84, and sends them through the CLI. It compares the result against 10,000 km², GEOS/Shapely planar area, and ellipsoidal geodesic area along edges densified every 100 m.

Across the four cases, relative difference from the geodesic measurement was below **3.1 × 10⁻¹¹**. Complete values are in `geodesy-validation.json`. This validates those synthetic geometries and the transformation/area-computation path; it does not represent uncertainty in real occurrence data and does not prove that a given hull is biologically appropriate for every species. The comparison shares PROJ's mathematical implementation but uses a different area algorithm and a different native-library version.

```bash
make build
python -m pip install pyproj shapely
python scripts/validate_geodesy.py
```

### Reference implementation benchmark

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

### Observed microbenchmark

A synthetic set of 1,000 points using automatic centring and a sampled 10 × 10 grid search took approximately **5.85 ms/op**, allocating about 6.36 MB/op. Only three repetitions were run in the validation environment (AMD EPYC 9V74). This is not a performance study, does not represent a typical workstation, and cannot be used as a direct comparison with Node/Turf. Reproduce with:

```bash
go test -run '^$' -bench BenchmarkCalculate1000 -benchtime=3x -benchmem .
```

### Release-platform validation

The repository defines native release builds for:

- Debian 13 amd64;
- Ubuntu 24.04 amd64;
- macOS 15 arm64;
- macOS 15 Intel amd64;
- Windows Server 2025 / Windows x64 using MSYS2 UCRT64.

The Windows release job installs the UCRT64 GCC, GDAL, PROJ, pkg-config and `ntldd` toolchain, runs the Go tests with CGO enabled, recursively resolves DLL dependencies, bundles GDAL/PROJ resource directories, and then runs the packaged executable with the MSYS2 runtime removed from `PATH`. This is intended to detect accidentally missing DLLs before a release is published.

Release assets are hashed into `SHA256SUMS`, and both end-user installers verify that checksum before installation.

At the time this document was updated, GitHub Actions jobs for this repository were failing before any workflow step started (`runner_id: 0` / no steps). That is an account/runner execution issue rather than a recorded test failure, so a tagged release must not be described as validated on a platform until its native release job has actually completed successfully.

### Validation limits

There has been no independent IUCN homologation, no completed GeoCAT/IUCN EOO Calculator comparison on the benchmark corpus, no formal security audit, and no validation of every possible datum transformation. The ConR and `vicentecalfo/eoo-aoo-calculator` comparison is an initial external implementation benchmark, not universal scientific validation. The software remains a functional, auditable pre-1.0 implementation with explicit methodological choices and known limits that should continue to undergo scientific review before institutional use.
