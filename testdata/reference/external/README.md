# External reference results

Store independently generated comparison records here when they are intended to become permanent scientific fixtures.

For routine/repeatable comparison runs, use the harness in [`benchmark/`](../../../benchmark/README.md). It automatically runs this project, `vicentecalfo/eoo-aoo-calculator`, and ConR, prepares manual GeoCAT/IUCN inputs, and produces consolidated comparison tables. Once an external measurement has been reviewed and is worth preserving as a stable fixture, copy it here with full provenance.

The first reviewed benchmark snapshot is:

- [`benchmark-2026-09-23.csv`](benchmark-2026-09-23.csv), containing the measured Go, ConR and `vicentecalfo/eoo-aoo-calculator` values discussed in [`docs/REFERENCE_BENCHMARK.md`](../../../docs/REFERENCE_BENCHMARK.md).

This CSV is a preservation snapshot rather than an executable test oracle. Cross-tool values can legitimately differ because methods differ; automated regression tests should continue to assert scientifically justified invariants rather than force equality to another package.

Each committed result should identify:

- source tool and exact version or service date;
- input file or corpus case name;
- input CRS and any preprocessing;
- projection and grid settings when configurable;
- raw EOO, assessment EOO, AOO and occupied-cell count when available;
- date generated;
- a short note describing any methodological mismatch that prevents direct numerical comparison.

Suggested filename pattern:

```text
<case>__<tool>__<version-or-date>.json
```

Suggested record shape:

```json
{
  "case": "antimeridian-local-laea",
  "tool": "GeoCAT",
  "tool_version": "record exact version or service date",
  "generated_at": "YYYY-MM-DD",
  "settings": {},
  "metrics": {
    "raw_eoo_km2": null,
    "assessment_eoo_km2": null,
    "aoo_km2": null,
    "occupied_cells": null
  },
  "notes": "Explain comparability and any differences in projection/grid rules."
}
```

Do not commit placeholder values as measured results. The example above documents structure only.
