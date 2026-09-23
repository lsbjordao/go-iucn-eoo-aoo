# JSON schemas

Version 0.1.0 publishes machine-readable JSON Schema contracts for its canonical JSON interfaces:

- `input.schema.json` — canonical occurrence arrays and RFC 7946 Point FeatureCollections;
- `result.schema.json` — `calc` and HTTP calculation results (`schema_version: "1"`);
- `comparison.schema.json` — `compare` results;
- `batch.schema.json` — `batch` results.

The canonical tabular CSV schema uses the same record fields as JSON: `id,taxon,lon,lat`.

The parser also accepts documented interoperability aliases such as Darwin Core `decimalLongitude`, `decimalLatitude` and `scientificName`; these aliases are intentionally not part of the canonical input schema.

Schema version and software version are separate concepts. The initial software release is `v0.1.0`, while its public result contract is schema version `1`. Future software releases may retain schema version `1` while the JSON contract remains backward compatible.
