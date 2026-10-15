# Shared tamper matrix (CEM-V1-226..229)

Status: like the rest of `protocol/cem-1.0/stable/`, implemented and run by both
consumers; the technical contract is proposed and not promoted.

One clean `cem/1.0` map and twenty-two tampered copies of it (twenty-three cases in
`EXPECTED.json`, the clean map included), each with the one
verdict every stable consumer must return. The native verifier
(`internal/cem/verify`) and the portable consumer (`interop/cem01-go`) both
load this directory and assert every case; the tests are named
`TestCEMV1226*` in both.

- `repository.recipe.json`: two flat commits, built exactly like
  `../neutral-criterion/repository.recipe.json`. `SPEC.md` defines the
  requirements `GRT-001` (cited by the `greet.go` hunk) and `GRT-002` (cited by
  the `greet_test.go` hunk); each hunk's added lines name its requirement ID
  and each cited line names the hunk's file, so the clean map is `anchored`
  on both hunks.
- `stable.json`: the clean map (case `clean`). Its producer-only arrays are
  empty, so it declares no artifacts.
- `cases/<name>.json`: the tampered maps, canonical JSON plus one LF. A case
  whose tamper changes an evidence record recomputes its ID only when the
  description says so.
- `EXPECTED.json`: profile `cem-stable-tamper-matrix/1`. For each case:
  `name`, `tamper` (the tamper class), `description`, `map` (relative to this
  directory), `exit`, `outcome`, `stage`, `code`, `issueCodes` and
  `citation`. A REJECT pins the exact stage and closed code (`code` for the
  wire and references stages, `issueCodes` for verification) and a null
  `citation`. An ACCEPT pins the whole `cem-citation-support/0` block, so its
  per-hunk tiers, anchors and `specificity` are fixed.

Run each case with either consumer (absolute paths, `--artifacts` set to this
directory):

```sh
corvint cem verify-stable --repository R --map CASE/MAP \
  --expected-base BASE --target TARGET --artifacts CASE
cem01-go verify-stable-canonical --repository R --map CASE/MAP \
  --expected-base BASE --target TARGET --artifacts CASE
```

`evidence-ids-swapped` is an ACCEPT, not a REJECT. Each swapped evidence
record is still a real, digest-bound span of the base `SPEC.md`, and a
support tier never changes the verdict (CEM-V1-080). The swap shows up only as
the degraded tier: both hunks drop from `anchored` to `cited`, and
`specificity` drops from 2 to -2 (CEM-V1-229).
