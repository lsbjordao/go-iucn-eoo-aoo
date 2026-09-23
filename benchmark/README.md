# Reference benchmark harness

This directory provides a reproducible comparison harness for EOO/AOO implementations. It is intentionally separate from the package unit tests: unit tests protect internal invariants, while this harness records how independent tools behave on the same occurrence datasets.

## What is automated

`make benchmark` runs the reproducible comparators that can be executed locally:

1. `go-iucn-eoo-aoo` with `local-laea`;
2. `go-iucn-eoo-aoo` with `iucn-cea`;
3. [`vicentecalfo/eoo-aoo-calculator`](https://github.com/vicentecalfo/eoo-aoo-calculator), pinned to commit `d47f41deb6041b622aa9aa1587bff4702b953b73` (package version 1.2.2);
4. [`gdauby/ConR`](https://github.com/gdauby/ConR), pinned to commit `50b9924bcd6bcec2bf2f2d775fa47bb29b721ba6` (package version 2.1), using both its default spheroid EOO and its planar CEA EOO mode.

GeoCAT and the IUCN EOO Calculator are treated as manual/reference measurements unless a stable public automation interface is available. `make benchmark-prepare` creates upload-ready inputs and result templates for them.

## Quick start

```bash
make benchmark
```

The first run may install/build external reference dependencies. It requires Python 3, Git, Node.js/npm, R/Rscript and the native libraries required by ConR. The Go calculator itself still requires the normal GDAL/PROJ build environment.

On Debian/Ubuntu, an optional convenience target installs the common system prerequisites:

```bash
make benchmark-deps-debian
```

This target uses `sudo apt-get`; inspect it before running if you manage system packages another way.

Check tools and script syntax without running calculations:

```bash
make benchmark-doctor
make benchmark-check
```

## Targets

```text
make benchmark-deps-debian    Install common Debian/Ubuntu system prerequisites
make benchmark-doctor         Report required tool versions and pinned revisions
make benchmark-check          Parse-check Python, Node and R benchmark scripts
make benchmark-prepare        Generate normalized benchmark datasets and manual templates
make benchmark-self           Run this project under local-laea and iucn-cea
make benchmark-vicente        Run the pinned vicentecalfo/eoo-aoo-calculator
make benchmark-conr           Run the pinned ConR reference
make benchmark-collect        Consolidate all available results and compute deltas
make benchmark-report         Alias for benchmark-collect
make benchmark                Run all automated references and collect results
make benchmark-clean          Remove generated auto results/cache, preserving manual measurements
make benchmark-clean-all      Remove all benchmark work/results/cache, including manual measurements
```

`make benchmark-vicente` clones the exact pinned commit into `benchmark/.cache/`, runs `npm ci`, builds it, and calls its library API. Canonical `id,taxon,lon,lat` records are converted in-memory to its expected `longitude,latitude` coordinate objects; the source datasets are not modified. The built reference is cached between runs.

`make benchmark-conr` installs the pinned ConR commit into an isolated R library under `benchmark/.cache/`. ConR AOO is run with 2 km cells, `nbe.rep.rast.AOO = 0`, and `proj_type = "cea"`. EOO is recorded in both `mode = "spheroid"` and `mode = "planar", proj_type = "cea"` so methodological differences are visible rather than hidden.

## Datasets

The harness reuses the versioned synthetic cases under `testdata/reference/cases.json` and adds `examples/brazil.csv`. `benchmark/scripts/prepare.py` materializes equivalent CSV and JSON files under the ignored `benchmark/work/` directory.

The synthetic cases include degenerate and edge conditions such as one point, duplicate coordinates, EOO-to-AOO flooring, collinearity, and the antimeridian. They are regression fixtures, not biological validation datasets.

## Results

Generated results live under the ignored `benchmark/results/` directory:

```text
benchmark/results/
├── go.csv
├── vicentecalfo.csv
├── conr.csv
├── raw/
│   ├── go/
│   └── vicentecalfo/
├── manual/
│   ├── geocat.csv
│   └── iucn-eoo-calculator.csv
├── comparison.csv
└── comparison.md
```

All automated result tables use a common long-form schema with the dataset, tool, tool version/revision, method mode, raw EOO, assessment EOO when applicable, AOO, occupied-cell count, error text and settings.

`comparison.csv` adds absolute and percentage differences relative to this project's `iucn-cea` result for the same dataset. That baseline is a convenience for inspection, not a claim that it is universally correct.

## Adding GeoCAT and IUCN measurements

After `make benchmark-prepare`, upload the external-tool-friendly CSV files in:

```text
benchmark/work/manual-input/
```

These use `id,taxon,longitude,latitude` while the internal benchmark datasets retain the package's canonical `id,taxon,lon,lat` schema.

Then fill the corresponding generated templates:

```text
benchmark/results/manual/geocat.csv
benchmark/results/manual/iucn-eoo-calculator.csv
```

At minimum record the measured `eoo_raw_km2` and/or `aoo_km2`, the tool version/date, and any projection/grid settings. `make benchmark-prepare` preserves existing manual rows while adding newly introduced datasets. Do not infer or copy values from this implementation into those files.

Run:

```bash
make benchmark-collect
```

again to incorporate the manual measurements.

## Scientific interpretation

A discrepancy is not automatically a bug. EOO can differ because tools construct or measure hulls differently, and AOO is particularly sensitive to grid translation, grid orientation, projection, duplicate handling and boundary rules. The purpose of this harness is to make those differences inspectable and reproducible.

The first reviewed run, dated 2026-09-23, is interpreted in [`docs/REFERENCE_BENCHMARK.md`](../docs/REFERENCE_BENCHMARK.md). A compact machine-readable preservation snapshot is committed as [`testdata/reference/external/benchmark-2026-09-23.csv`](../testdata/reference/external/benchmark-2026-09-23.csv).

Those preserved values are **not** executable golden answers. The regression suite should continue to test justified scientific invariants and explicit method behaviour rather than force equality to another package whose projection, grid or degeneracy rules may differ.
