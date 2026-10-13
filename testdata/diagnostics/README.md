# Diagnostic fixtures

Synthetic, minimal inputs for the audit, curation and influence diagnostics.
They are not real occurrences.

`zero-zero.csv` has GBIF-style columns. Its record `r6` sits at exactly (0,0) with
a locality in Argentina and the source flag `zerocd`, mirroring the pattern found
in the *Acanthochelys spixii* benchmark set. Records `r7` and `r8` lie on
Greenwich and on the equator and must not be flagged as (0,0). Rows 10 and 11
(`r9`, `r10`) are invalid on purpose. `r1` is repeated to exercise duplicate IDs
and duplicate coordinates.
