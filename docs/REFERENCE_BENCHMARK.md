# Reference implementation benchmark — 2026-09-23

This document records an initial reproducible comparison of `go-iucn-eoo-aoo` against independent EOO/AOO implementations. It is evidence about implementation behaviour, not an accuracy ranking and not an IUCN certification.

The benchmark harness is described in [`benchmark/README.md`](../benchmark/README.md) and can be reproduced with:

```bash
make benchmark
```

The automated comparison used:

- `go-iucn-eoo-aoo` v0.1.0, both `local-laea` and `iucn-cea`;
- `vicentecalfo/eoo-aoo-calculator` 1.2.2, pinned to commit `d47f41deb6041b622aa9aa1587bff4702b953b73`;
- `gdauby/ConR` 2.1, pinned to commit `50b9924bcd6bcec2bf2f2d775fa47bb29b721ba6`;
- ConR EOO in both its default spheroid mode and `mode = "planar", proj_type = "cea"`;
- ConR AOO with 2 km cells, `nbe.rep.rast.AOO = 0`, and `proj_type = "cea"`.

GeoCAT and the IUCN EOO Calculator were not included in this first measured snapshot; the harness generates upload-ready inputs and result templates for those manual/reference checks.

## Observed results

### Regional Brazil example

| Tool / mode | Raw EOO km² | AOO km² |
|---|---:|---:|
| `go-iucn-eoo-aoo / iucn-cea` | 15631.477162575591 | 12 |
| `ConR / planar-cea` | 15631.5 | 12 |
| `go-iucn-eoo-aoo / local-laea` | 15678.73789460172 | 12 |
| `ConR / default-spheroid` | 15722 | 12 |
| `vicentecalfo/eoo-aoo-calculator` | 15707.505680367984 | 12 |

For the like-for-like projected CEA comparison, ConR and this implementation differ by approximately **0.000146%**. This is strong evidence that the projected convex-hull/area path agrees across independent software stacks when the same broad method is used.

### Antimeridian case

Synthetic occurrences: approximately 179.9°E to 179.9°W at 10–10.1°N.

| Tool / mode | Raw EOO km² | AOO km² |
|---|---:|---:|
| `go-iucn-eoo-aoo / local-laea` | 242.50282819926466 | 16 |
| `ConR / default-spheroid` | 243.5 | 16 |
| `go-iucn-eoo-aoo / iucn-cea` | 436262.5896382405 | 16 |
| `ConR / planar-cea` | 436263 | 16 |
| `vicentecalfo/eoo-aoo-calculator` | 439023.7223915208 | 0 |

This is the most important diagnostic result in the first benchmark. Two independent implementations form the same methodological groups:

- regional/geodesic-like treatment: about **243 km²**;
- Greenwich-centred planar CEA treatment: about **436,263 km²**.

The Go CEA result and ConR planar CEA result differ by only about **0.000094%**, while the Go local-LAEA and ConR spheroid values are also close. The approximately three-orders-of-magnitude difference is therefore not evidence of random numerical instability in the Go implementation. It demonstrates the known planar-hull problem for an antimeridian-crossing distribution under a Greenwich-centred cylindrical representation.

`iucn-cea` deliberately retains that behaviour as a compatibility-oriented method and emits `ANTIMERIDIAN_CEA`. Assessors should inspect `local-laea` or another scientifically appropriate regional/geodesic treatment rather than silently interpreting the world-spanning CEA hull as universally suitable.

The Turf-based reference returned AOO = 0 for this valid four-point case. That value is not used as evidence that another implementation is correct; it is recorded as a cross-tool behavioural difference requiring methodological inspection.

### Collinear coordinates

| Tool / mode | Raw EOO km² | AOO km² |
|---|---:|---:|
| `go-iucn-eoo-aoo / local-laea` | null | 12 |
| `go-iucn-eoo-aoo / iucn-cea` | null | 12 |
| `ConR / default-spheroid` | 0.211 | 12 |
| `ConR / planar-cea` | 0.000006 | 12 |
| `vicentecalfo/eoo-aoo-calculator` | 0 | 12 |

This implementation deliberately reports raw EOO as `null` with status `collinear` rather than perturbing the data or presenting a zero-area polygon as a normal EOO. ConR documents a jitter-based treatment for this special case, which explains why it can return a small non-zero value. These outputs should not be treated as directly equivalent semantics.

The assessment EOO in this project then uses the explicit AOO floor convention while retaining the fact that no non-degenerate raw hull existed.

### One unique coordinate and duplicate coordinates

For both a single occurrence and duplicated records at one coordinate:

- Go: raw EOO `null`, AOO 4 km²;
- ConR: EOO unavailable, AOO 4 km²;
- `vicentecalfo/eoo-aoo-calculator`: EOO 0, AOO 4 km².

The agreement on AOO and the difference between `null` and `0` are primarily output-semantics differences. This project uses `null` plus an explicit status to distinguish “EOO cannot be constructed” from a valid measured zero.

### Tiny triangle / EOO floor case

| Tool / mode | Raw EOO km² | AOO km² |
|---|---:|---:|
| `go-iucn-eoo-aoo / iucn-cea` | 0.00006154536039644392 | 4 |
| `go-iucn-eoo-aoo / local-laea` | 0.00006154536039647489 | 4 |
| `ConR / default-spheroid` | 0.000062 | 4 |
| `ConR / planar-cea` | 0.000062 | 4 |
| `vicentecalfo/eoo-aoo-calculator` | 0.00006196014515233713 | 12 |

All implementations produce very similar raw EOO. Go and ConR also agree on the minimum 4 km² AOO, whereas the Turf-based reference produces 12 km². This is consistent with AOO being sensitive to grid construction and translation. It supports keeping grid origin/search behaviour explicit in result provenance rather than treating a single arbitrary grid placement as an invariant quantity.

## What the benchmark supports

The first comparison provides evidence for the following claims:

1. **Like-for-like CEA EOO agrees extremely closely with ConR.** The Brazil and antimeridian planar-CEA cases differ by roughly 0.0001% or less.
2. **Projection/method choice can dominate numerical implementation differences.** The antimeridian example changes by roughly three orders of magnitude between regional/geodesic-like and Greenwich-centred planar treatments, while independent tools agree within each treatment.
3. **Degenerate EOO semantics must be explicit.** `null`, zero, and jitter-generated positive areas are not interchangeable meanings.
4. **AOO is sensitive to grid construction.** The tiny-triangle case agrees between Go and ConR (4 km²) but not the Turf-based reference (12 km²).
5. **Cross-tool disagreement is diagnostic, not automatically evidence of a bug.** A discrepancy should be traced to projection, hull construction, grid origin/orientation, boundary rules, duplicate handling, or degenerate-case policy before any implementation is declared correct or incorrect.

## What the benchmark does not establish

This comparison does **not** establish that one tool is universally more accurate than another. In particular:

- no IUCN homologation has been performed;
- GeoCAT and the IUCN EOO Calculator still need to be measured on the same corpus;
- the synthetic corpus is intentionally small and focused on edge/invariant cases;
- real occurrence datasets introduce georeferencing, taxonomic, temporal, origin/presence, sampling and biological-suitability uncertainties that these geometric comparisons do not address;
- the benchmark does not choose a universally preferred projection for every distribution.

The intended use of the comparison is reproducibility, regression detection and methodological diagnosis.

## Release interpretation

These results materially strengthen the evidence for the v0.1.0 scientific core. They support release as an auditable pre-1.0 implementation while retaining the documented limitations and the need for further external comparison. Before making stronger claims of external validation, add GeoCAT and IUCN EOO Calculator measurements to the same corpus and preserve their exact version/service date and settings.
