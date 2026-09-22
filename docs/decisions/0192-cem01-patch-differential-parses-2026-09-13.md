# Decision 0192 — the two Go CEM 0.1 patch readers agree on seven differential parses

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls
and record them (coordinator lane `ccinteropsettle`, 2026-09-13).

A 2026-09-13 bug hunt parsed the same patch bytes with the Corvint verifier (`internal/cem/patch`,
`internal/cem/sim`) and the Corvint-authored portability probe (`interop/cem01-go`) and found seven
disagreements. `interop/cem-0.1/ALGORITHMS.md` decides four of them; the owner decides the other
three here. The kit's wording is clarified as erratum 2 (`interop/cem-0.1/IMPLEMENTATIONS.md`).

Decided by the existing text, with the wrong reader repaired:

- A hunk header's "optional uninterpreted suffix" is any bytes after ` @@`, so `@@ -1 +1 @@x` is a
  valid header. The verifier had required a leading space and now ignores the suffix.
- Step 4 admits "one or more contiguous hunks" for every group. A create or delete may carry
  several hunks, each with the empty old or new range. The verifier had required exactly one.
- The parser rejects "inconsistent new positions". The probe had checked only monotonic new order
  and caught an exact mismatch later, in simulation. It now checks at parse time.
- Step 2 limits `similarity index` and `dissimilarity index` only to "at most one" line. It does
  not tie them to a rename, and Git's `-B` emits `dissimilarity index` on a modification. The probe
  had rejected them without rename metadata and now accepts them in any state-machine group.

Owner calls, where the text leaves a gap:

1. A percent is canonical decimal: `0`, or a value from 1 to 100 with no leading zero. The
   verifier had accepted `050%`. This matches Git's output and the probe's existing check.
2. Every hunk body record and every `\ No newline at end of file` marker record MUST end in LF.
   The marker is the only mechanism that removes an LF. The verifier had accepted a final body
   line or marker without LF at end of input.
3. "Creates and deletes MUST use the state machine so their tree mode is bound" also binds a
   delete to its base. The `deleted file mode` MUST equal the tree mode of the old path in the
   base; a mismatch is `diff-metadata-mismatch`. The verifier had not compared them. The probe
   already did.

The kit does not order a parse rejection (exit 1) against a repository failure (exit 2). The
verifier resolves the base before parsing; the probe parses first. With a working repository, the
placement of a check changes only which rejection is reported first. No conformance vector
depends on it, and this decision does not order them.

Every frozen `cem/0.1` fixture, manifest entry, and expected result is unchanged, and so is
`runner.py`. The same rules apply to `cem/0.2` through `CEM-CB-003`. No independent implementation
has started, so the kit's freeze does not apply. Evidence is in the `CEM-PILOT-011` and
`CEM-GO-001..003` traceability rows.
