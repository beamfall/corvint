# Decision 0168 — Scope-lease overlap case-folds paths

Date: 2026-09-12. Status: accepted. Authority: repository owner instruction, 2026-09-12,
delegated coordinator call. Base: `2c4e1952dc13227392409f545c2b508c7dc3c72e`.

`internal/scopelease` (`overlaps`) compared lease scopes with exact string equality and exact
glob-free-prefix containment, so `Internal/x` and `internal/x` did not overlap. On a
case-insensitive volume (macOS APFS, the default on this host) both spellings name one file, so
two agents could each hold a lease on the same file. `docs/specs/scope-lease-v0.md` was silent on
case sensitivity; SCL-V0-002 only says an undecidable comparison is an overlap.

The owner call: fail closed on every host. `overlaps` lowercases both scopes before the equality
and glob-free-prefix tests, without probing the volume's actual case sensitivity. A false overlap
on a case-sensitive host only refuses a lease (one renamed scope); a missed overlap on a
case-insensitive host lets two writers collide silently, which is the failure the lease exists to
prevent. This amends SCL-V0-002 and adds no requirement ID.

`covers` (used by `Check`, SCL-V0-009) stays case-exact. With overlap folded, the refusal path
can no longer admit two live leases whose scopes differ only in case, so a case-exact `covers`
cannot hide a double coverage that `acquire` would have produced; a case-variant touched path is
reported `uncovered`, the conservative direction for that helper.

Not addressed: Unicode normalization. APFS is also normalization-insensitive, so NFC and NFD
spellings of one non-ASCII name remain distinct scopes; lowercasing is `strings.ToLower`, not
full case folding.

Rollback: remove the `strings.ToLower` line in `overlaps` and the case-fold cases in
`TestOverlapDecisionIsConservative`, and restore SCL-V0-002's wording. No stored lease document
changes shape; leases written under either rule stay readable.

## Addendum, 2026-09-12: Unicode normalization

Measured on this host's volume (APFS, under the scratchpad): a file created with an NFC-spelled
non-ASCII name (`café`, precomposed) stats and opens successfully under the NFD spelling
(`café`, `e` plus combining acute) and vice versa, and the directory holds one entry. So APFS
is normalization-insensitive as well as case-insensitive, and the byte-unequal NFC/NFD spellings of
one lease scope named one file, unhandled by decision 0168's `strings.ToLower` fold.

`golang.org/x/text` (or any normalization table) is not a module dependency (`go.mod` requires
nothing beyond the standard library), and AGENTS.md invariant 7 keeps the local product a single
Go binary with no added dependency for this. Implementing canonical decomposition was therefore not
available; the owner call is the other SCL-V0-002 branch: fail closed. `overlaps` now also treats
any non-ASCII byte in either compared literal prefix as undecidable, hence an overlap, after the
literal-equality and glob-free-prefix checks find no relation. A false overlap between two
unrelated non-ASCII scopes only refuses a lease; a missed overlap between an NFC and an NFD
spelling of the same path lets two writers collide silently, which is the failure the lease exists
to prevent.

`covers` (SCL-V0-009) is unchanged: it stays exact-byte, so a normalization-variant touched path is
reported `uncovered` rather than silently matched, the same conservative direction decision 0168
already chose for that helper.

Rollback: remove the non-ASCII byte scan appended to `overlaps` (and `containsNonASCII`) and the
non-ASCII cases in `TestOverlapDecisionIsConservative`, and restore SCL-V0-002's addendum sentence.
No stored lease document changes shape.
