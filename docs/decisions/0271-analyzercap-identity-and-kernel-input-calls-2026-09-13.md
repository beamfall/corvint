# Decision 0271 — analyzercap identities, harness trim set, and kernel verify input bound

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

Four unconfirmed bug-hunt hypotheses about `internal/analyzercap` and the Go kernel were filed in
`docs/agent-memory/ideas.md`. The calls:

1. **Repeated component identity is an invalid profile** (amends `ACC-V0-007`). A profile could
   hold two components with one `(scopeID, role, ecosystemCoordinate, stableInstanceID)` but
   different values, and a requirement matched if either did, a claim broader than the evidence.
   `NewProfile` now refuses it with the existing `PROFILE_UNAVAILABLE`; no new code is needed.
   Canonical component keys lead with the identity fields, so after the strict-sort check a repeat
   can only be adjacent, and the check allocates nothing (the `ACC-V0-020` ratchets are unchanged).
2. **A manifest repeating a clause ID is invalid** (amends `ACC-V0-008`). Clause order keyed on ID
   then body, so two bodies under one ID sorted apart while only the ID is echoed in resolution and
   receipts. `validateManifest` refuses it with the existing manifest code
   `CAPABILITY_PROJECTION_UNAVAILABLE`, the code already used for an empty, oversized, or unsorted
   clause list.
3. **The `user-prompt` trim is not a parity defect.** Python `str.strip()` does trim U+001C..U+001F
   and Go `strings.TrimSpace` does not (both confirmed by scratch probes; the remaining listed
   separators U+0085, U+00A0, U+1680, U+2000..U+200A, U+2028, U+2029, U+202F, U+205F, and U+3000
   are trimmed by both, U+FEFF by neither), but the Python oracle is retired with no live or
   regenerated parity requirement (decision 0088, `GOC-V0-001`, `GOC-V0-002`), and `AHI-016`
   names Go `strings.TrimSpace` (Unicode `White_Space`) as the single trim set every host adapter
   ports, so changing Go to Python's set would break that cross-host contract instead of restoring
   one. No code or spec change.
4. **`kernel verify` stdin is bounded** (amends `CKN-V0-008`). The verb read stdin with
   `io.ReadAll` and no bound. It now reads through `io.LimitReader` at
   `gokernel.MaxInputBytes` (131,072 bytes) plus one, the bound every sibling `corvint` stdin
   verb uses (`readInput`), and refuses a longer input with its existing `invalid-kernel-input`
   code, exit 2, before verification. `CKN-V0-002`'s 600-byte kernel bound does not apply: the
   input is arbitrary summary text that contains the kernel.

Rollback: revert the commits; no persisted state, wire bytes, analyzer output, or requirement IDs
change.
