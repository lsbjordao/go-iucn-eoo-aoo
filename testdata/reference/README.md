# Scientific reference corpus

This directory contains versioned inputs and expectations used to guard scientific invariants across releases.

`cases.json` contains synthetic cases whose expected behaviour can be justified independently of the current implementation, such as the 4 km² one-cell AOO at the default 2 km grid scale, duplicate-coordinate provenance, degenerate EOO cases, the EOO-to-AOO floor, and an antimeridian sanity bound.

The automated test in `reference_test.go` executes every case. These cases are regression guards, not a substitute for external scientific validation.

## External comparison results

Results from GeoCAT, the IUCN EOO Calculator, ConR or other independent tools must only be committed after they have actually been generated from the same versioned input and their tool version, date, settings and projection assumptions have been recorded. Do not copy or infer reference values from this implementation itself.

The `external/` directory documents the format and provenance requirements for those future comparison records.
