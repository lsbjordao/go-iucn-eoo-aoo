# JSON schemas

Version 0.2.0 publishes machine-readable JSON Schema contracts for its canonical JSON interfaces:

- `input.schema.json` — canonical occurrence arrays and RFC 7946 Point FeatureCollections;
- `result.schema.json` — `calc` and HTTP calculation results (`schema_version: "1"`);
- `comparison.schema.json` — `compare` results;
- `batch.schema.json` — `batch` results;
- `protocol.schema.json` — frozen analysis protocols (`calc --protocol-out`, `result.protocol`);
- `audit.schema.json` — `audit` reports;
- `curation-manifest.schema.json` — curation manifests read by `curate`;
- `curation-log.schema.json` — `curate` logs;
- `influence.schema.json` — `influence` reports.

Fields added after the initial release (for example `geometry_sha256`, `protocol`, `eoo.geodesic_area_km2` and the `aoo` search-guarantee fields) are optional in the schema, so results from earlier 0.1.x builds still validate. Their meaning is documented in `docs/METHODOLOGY.md`. `schema_test.go` validates every output type against these files.

The canonical tabular CSV schema uses the same record fields as JSON: `id,taxon,lon,lat`.

The parser also accepts documented interoperability aliases such as Darwin Core `decimalLongitude`, `decimalLatitude` and `scientificName`; these aliases are intentionally not part of the canonical input schema.

Schema version and software version are separate concepts. The initial software release is `v0.1.0`, while its public result contract is schema version `1`. Future software releases may retain schema version `1` while the JSON contract remains backward compatible.
