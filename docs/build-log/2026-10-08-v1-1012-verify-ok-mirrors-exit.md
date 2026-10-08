# V1-1012: dogfood verify `ok` mirrors the exit status (2026-10-08)

## Intent

Batch C stage 3 saw `corvint dogfood verify` exit 1 for a selected check with two real failures
while its `corvint-local-completion/0` envelope reported top-level `ok:true`. A caller reading only
`ok` would treat the failed check as passing. `docs/specs/local-completion-policy-v0.md` did not
define `ok`, so the fix makes it reflect the result rather than documenting it as transport success.

## Reproduction (before)

Fresh repository, plan with one check `["sh","-c","echo two real failures >&2; exit 1"]`,
`corvint dogfood begin` then `corvint dogfood verify --check fails` at `origin/main` `388fb832`:

    exit 1
    {"ok": true, "tool": "dogfood-verify", "mutates": true, ...
     "policy": {"checks": [{"id": "fails", "qualified": false, "exit": 1, ...}], ...}}

`dogfood finish` had the same shape: exit 1 with `ok:true` while the policy was unsatisfied
(for example before report review), on both the ordinary and the transport-adapted recovery route.

## Change

- `LCP-V0-017` (proposed): the envelope's `ok` equals whether the command exits 0. `verify` emits
  `ok:false` when the selected check is unqualified; `finish` emits `ok:false` while unsatisfied.
  `status` is unchanged (exit 0, `ok:true`). The `policy` member and exit codes are unchanged.
- `cmd/corvint/local_completion.go`: the exit decision moved into `localCompletionExit`, computed
  before the envelope is emitted; the recovery route sets `ok` to `result.Satisfied`.
- Consumers checked: no script, Makefile target, adapter or `internal/` caller reads `ok` from
  `dogfood verify`/`finish`; `internal/opencodequalification` and the skills use the exit status.
  The core-freeze contract pins only `dogfood status`, which is unchanged.

## Verification

After the change, the same reproduction exits 1 with `"ok": false` and the unchanged observation
(`qualified:false`, `exit:1`, `unmet` naming `selected-check-unverified:fails`).

- `TestLocalCompletionVerifyOKMirrorsCheckResult` fails at the base (`ok:true` for the failing
  check) and passes with the change.
- `TestDogfoodFinishRunsFromBinaryInForeignRepository` now also asserts pre-review `ok:false`
  and satisfied `ok:true`.

## NOT_RUN

`make gate`, the exhaustive `go test ./...`, and the post-commit dogfood CEM bind/check/seal loop
were not run in this lane; the owner's scoped-work preference is focused tests plus review.
