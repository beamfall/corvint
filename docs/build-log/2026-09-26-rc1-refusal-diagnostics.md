## 2026-09-26 V1-0354 V1-0352 DCW-V0-014 LRF-V0-001: refusals name their remediation and profile

`corvint dogfood change` refused a verification line such as `go test ./x -run '^TestA$'` with
`local-outcome: unsupported-verify-syntax` and no `fix:` line, so the operator had to read the
source to learn the admitted syntax. `DCW-V0-014` requires a fix line for malformed or missing
input. The `fixHints` table in `internal/dogfoodflow/change.go` now carries a hint for
`unsupported-verify-syntax` (the admitted character set and `-run TestName` in place of an anchored
pattern) and for `verify-file-unavailable`, which also printed no hint.
`TestDogfoodChangeNamesVerifySyntaxRemediation` binds a sidecar whose verify file holds an anchored
pattern and asserts the refusal row and its fix line; it fails without the table entry.

`corvint lrf` checked only for `cem/0.2` after parsing the map, so a canonical `cem/0.3` map fell
into the `cem/0.1` path: with no patch it failed `patch-unavailable`, and with a patch it failed
`unsupported-spec: exact-patch verification accepts cem/0.1 only`, neither of which names the map's
profile. `internal/lrfrepo/adapter.go` now refuses any profile other than `cem/0.1` and `cem/0.2`
with `unsupported-spec` naming it, before any patch path runs. `LRF-V0-001` states the refusal, and
`TestLRFRefusesCEM03BeforeAnyPatchPath` covers the no-patch, missing-patch and real-patch cases; the
real-patch case fails without the check. Exit code and the `unsupported-spec` code are unchanged
from the previous patch-path refusal, so no wire code is added.
