# 2026-10-14 Batch 4: stable consumer parity (V1-1291, V1-1306, V1-1289, V1-1295, V1-1297)

Goal: the native consumer (`internal/cem/verify`, `internal/cem/wire`, `internal/cem/cli/stable.go`)
and the portable consumer (`interop/cem01-go`) return the same outcome, stage and code for the same
`cem/1.0` input. Base `ee27168739ec1b65dd5a653d720cf811338c37ae`. The contract is the new section
"Stable consumer argument and reference parity" in `docs/specs/cem-stable-v1.md`.

## Accepted requirements

`CEM-V1-115`, `CEM-V1-116` (V1-1291), `CEM-V1-117` (V1-1306), `CEM-V1-176` and `CEM-V1-177`
(V1-1289) are accepted under the owner's standing auto-accept of lane requirements (2026-10-11),
with 1.0 scope per V1-0020 dependency. No other requirement ID is used.

## V1-1291: argument refusals are UNSUPPORTED

Measured at base with both binaries on the same argv (neutral-criterion map, absolute artifact
root unless stated):

| Argv | Native at base | Portable at base |
| --- | --- | --- |
| relative `--repository` | `REJECT` `arguments` / `invalid-arguments` | `UNSUPPORTED` same stage and code |
| no `--artifacts` | `REJECT` `authority-arguments` / `artifacts-required` | `UNSUPPORTED` same |
| relative `--artifacts` | `REJECT` `authority-arguments` / `invalid-arguments` | `UNSUPPORTED` same |

Decision: `UNSUPPORTED`, the portable answer. A bad invocation asserts nothing about the change,
`accept` `null` is the closed form for "not judged", and `OPERATION.json` already fixes stage and
exit 2 without an outcome, so no packet byte changes. The native verifier, the Core stable read,
the receipt bundle export source check and the stable universe admission now refuse with
`UNSUPPORTED`; two existing tests that pinned `REJECT` were updated (`internal/receiptbundle`,
`internal/lrfrepo`). The decision sentence sits beside `CEM-V1-084`, whose trusted-keys refusal is
one of these.

## V1-1306: relative path fix

A relative `--root`, `--repository`, `--map`, `--artifacts` or `--trusted-keys` is still refused,
never resolved (`CEM-V1-017`). The fix is printed on stderr by `corvint cem verify-stable`,
`cem01-go verify-stable` and `cem01-go verify-stable-canonical`, and put in an envelope `fix`
member by the Core stable read, so the closed result and `verdictSha256` are unchanged
(`CEM-V1-187`). The absolute form of each case passes both argument stages with empty stderr.
`cmd/corvint` helpers that required empty stderr now admit exactly that one line. Review finding:
the native command first took the fix from the parsed values, so `--repository /abs --repository
rel` lost the line the portable consumer printed; both now scan the supplied argv.

## V1-1289: reference precedence

Six one-mutation copies of the neutral-criterion map, run at base:

| Case | Native at base | Portable at base | Both now |
| --- | --- | --- | --- |
| hunk-basis-unresolved | `wire` / `invalid-field` | same | same |
| hunk-duplicated | `duplicate-reference` | `verification` / issue `patch-hunk-count`, exit 1 | `duplicate-reference` |
| link-ids-duplicated-unknown | `unresolved-reference` | `duplicate-reference` | `duplicate-reference` |
| criterion-link-missing | `reference-coherence` | `unresolved-reference` | `reference-coherence` |
| runner-link-missing | `reference-coherence` | `unresolved-reference` | `reference-coherence` |
| artifact-duplicated | `duplicate-reference` | `artifact-role-closure` | `duplicate-reference` |

Each rule took the answer that matches the existing coarse order in `ALGORITHMS.md` (uniqueness
before resolution, membership before role closure). The native wire validator gained a per-link
uniqueness pass before resolution. The portable consumer now refuses a repeated hunk ID (it kept
one record and failed later on the hunk count), moved its unused criterion/runner check after
basis membership with code `reference-coherence`, and checks a repeated artifact path or digest
before the role. The hunk case came from the review round. The rule is now an `ALGORITHMS.md`
paragraph and the six cases are the new `protocol/cem-1.0/stable/reference-precedence/` suite,
added through the manifest extension rule. Existing vectors, the sealed S0E cases and the
committed map results are byte-identical:
`TestStableS0EPublicCases` and `TestStableCanonicalFullResults` pass unchanged.

## V1-1295: OPEN_RESIDUAL, partial drift stays in the result

The preferred option (no partial drift in an `UNSUPPORTED` resource-limit result) was implemented
in both consumers and then reverted, because sealed vectors pin the opposite.
`repository-envelope-packet/FULL-RESULT-CASES.json` case `ledger-337-evidence` expects
`UNSUPPORTED` `repository` / `unsupported-resource-limit` with 335 drift items, and
`partial-drift-one-item-cancel` expects `UNSUPPORTED` `drift` / `verification-timeout` with one.
Both consumers run those cases, and the lane may not change existing vector bytes.

The divergence that remains is on the f0fda542 map: native 69 drift items (verdict
`54067cc24cc10ff2...`) against portable 103 (`cfd56726a48ac30d...`), both `UNSUPPORTED` `repository`
/ `unsupported-resource-limit`, at base and unchanged here. Inference, not measured: the two
consumers spend a different number of Git operations before drift, so the budget ends at a
different item. Closing it needs either a new sealed contract revision that drops partial drift,
or a shared per-operation accounting rule both consumers implement. The `nativePortableDiffer`
sentence in the spec now points here.

## V1-1297: orchestrator lane decision, OPEN_RESIDUAL

Decision (orchestrator-delegated lane decision, 2026-10-14): a missing pinned object keeps
`git-read-failed`; no distinct `repository-object-unavailable` answer in this lane. Reasons:

- Sealed cells pin the current code. `REPOSITORY-ENVELOPE.cells.json` cell `missing-base` (exact
  loose base commit deleted) and `shallow-missing` (shallow clone, base missing) both expect
  `UNSUPPORTED` `repository` / `git-read-failed`.
- The consumers already agree. An empty repository with the neutral-criterion map gives
  `UNSUPPORTED` `repository` / `git-read-failed` from `corvint cem verify-stable` and both portable
  commands, which is what the `CEM-V1-103` row records; that row is left unchanged.
- It is not cheap. The native `Repository.Resolve` (`internal/cem/gitauth/object.go`) maps any
  non-zero `rev-parse` exit to `git-read-failed`; separating "absent" needs an extra
  `git cat-file -e` in both consumers, which also changes the Git operation count the resource
  budget charges.

## NOT_PRODUCED

- No independent interoperability or outcome evidence; parity is between the two in-repository
  consumers.
- No full `go test ./...` and no `make gate` (lane rule). A first broad run on a host with load
  average 50 to 77 timed out in `internal/cem/gitauth`, `gitrun`, `verify`, `wire`, `workflow`,
  `console`, `doccorpus`, `extevidence` and `localcompletion`; it was stopped, and the touched
  packages were rerun narrowly and pass (`verify.txt` in the lane's dogfood record).

## Rollback

Revert the V1-1291 change commit. The native verifier returns `REJECT` for argument refusals
again, both consumers lose the reference order and the fix line, and `reference-precedence/` and
its manifest entries are deleted. No committed map, seal or digest depends on these answers.
