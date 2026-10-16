# CI shadow historical compatibility: classification and Git identity

Ticket `ci-shadow-historical-compat` (P1 spike; requirement refs AFP-V0-019, AFP-V0-022). The
retained hosted row (run 36897095402, frozen pair base
`d3c8d0f1fceecbc66cf29c375cc16b122e8aa2fe`, target `29e185103e6927e6c63e2fbf1922c1661300754f`)
recorded `internal/frontiernextrepo` and `internal/tracerecordrepo` as omitted failing packages,
so the driver refused promotion. This entry follows
[2026-10-01-ci-shadow-graft-advice](2026-10-01-ci-shadow-graft-advice.md) (PR #462), which
changed the driver.

## Audit before building

| Criterion | Status at `origin/main` 449c9d1a2 | Evidence |
|---|---|---|
| 1. Reproduce the pair and both packages; keep row receipts; separate compatibility from change-caused omissions | Partly met. PR #462 replayed only the target, without `-race`. The ticket's retained evidence files (`/private/tmp/corvint-ci-qualification-*`) are no longer on this host, so their hashes cannot be rechecked here | `fda3374ee`, `ff72c4cc3`; the 2026-10-01 build log |
| 2. Resolve or bound the Git/graft and protected-execution failures under an owner-accepted execution profile | Resolved in code by PR #462 (`testEnv`). The profile is recorded only as unnumbered implementation notes, and AFP-V0-014's accepted text does not mention Git | `testEnv` and `toolIdentity` in `tools/corvint-pr-tests/main.go` |
| 3. Decide and verify whether Git joins the frozen identity; never replay different inputs under the old identity | Git path, digest and version were added by PR #462. Drift checks covered `execute` only; no test exercised `shadow` resume against a corpus frozen under another identity | `TestSelectedFailureAndFallback`, `TestExecuteRequiresGitIdentity`, `TestContainerRunResult` |
| 4. Keep rows, no-miss gates and full coverage; no replacement rows, overlays, hardcoded names or advertised selection | Met. Nothing relabels rows; CI pins remain empty | `shadow.go` `validateRow`, `admit`; `.github/workflows/ci.yml` unchanged |

Related tickets were read but not changed: V1-0616 (overall promotion), V1-0246 and V1-0081
(reader coverage). All three remain open.

## Reproduction (criterion 1)

`git diff --stat base target` touches five paths: a CEM map, `docs/BUILD-LOG.md`,
`conformance/use-cases-v0/ledger.json`, a hostile-tests fixture and one use-case test. Neither
package imports `conformance/use-cases-v0`. Their only out-of-package literal reads
(`../wp3codec/codec.go`, `../../tools/local-authority/driver.go`) are unchanged in the pair. So
the selector's omission matches the static dependency evidence.

The two packages were replayed locally at both commits with the frozen argv
(`go test -json -p 1 -count=1 -race -timeout 50m`). The run used the driver's closed environment
allowlist with fresh HOME/TMP and an empty module cache. Three variants differ only in their Git
entries:

- `closed`: the driver's `closedEnv` with `GIT_GRAFT_FILE=/dev/null` and `GIT_CONFIG_COUNT=0`.
  This is the environment the retained row handed to tests.
- `test`: the current `testEnv`, which adds `advice.graftFileDeprecated=false`.
- `nograft`: `closed` without `GIT_GRAFT_FILE`. This is a control only and is not a proposed
  profile.

| Commit | Variant | frontiernextrepo | tracerecordrepo | Failing tests | Load (1/5/15 min) |
|---|---|---|---|---|---|
| target | closed | FAIL | FAIL | 27 | 39.97 46.67 55.21 |
| base | closed | FAIL | FAIL | 27, the same set | 52.98 57.76 58.20 |
| target | test | pass | pass | 0 | 44.97 51.79 55.11 |
| base | test | pass | pass | 0 | 45.42 52.67 56.01 |
| target | nograft | pass | pass | 0 | 62.73 64.58 60.08 |

The `closed` failures reproduce the ticket's lines exactly:

- `grafts_reviewer_test.go:48` and `adapter_test.go:240/300` parse
  `hint: Support for <GIT_DIR>/info/grafts is deprecated` as a Git result.
- `repository_test.go:149/213/270/313` and `request_memo_test.go:75` report
  `invalid-protected-execution`.

Only the Git environment varies, and switching it turns both packages from fail to pass at both
commits. That makes the protected-execution failures the same graft-advice cause, surfacing
through the local-authority wire's refusal (`internal/localauthority.ErrInvalid`).

**Classification.** Both omitted failures are historical/profile compatibility failures. They
are not change-caused omissions: they fail identically at the base and are independent of the
pair's diff. This does not waive the row. AFP-V0-014 still makes an omitted failing package fail
promotion, and the retained row stays invalid for promotion. It says nothing about selector
sufficiency on any other pair.

## Git identity decision (criterion 3)

Git joins the frozen runtime identity as path, SHA-256 and `--version` (PR #462). Git
configuration needs no separate field. Every config input the driver allows (`NOSYSTEM`, a
`/dev/null` global file, a private HOME, the `GIT_CONFIG_COUNT` entries, graft and replace
refusal) is carried by `identity.Env`. That field is frozen, compared before execution and
required equal in rows. Repository-local config belongs to the tested checkout or to the fixture
repositories tests create.

New evidence: `frozenWithoutGit`, run from `TestSelectedFailureAndFallback`, writes a valid
200-pair corpus under two identities: one with an empty Git field (the shape a corpus frozen
before PR #462 decodes to) and one with a different Git. It asserts that `shadow` resume refuses
with an identity mismatch before creating a checkout, and that `validateRow` refuses the observed
row relabelled with either identity while its files and the current identity stay unchanged.
Two mutations, each run alone:

- With the `!equal(id, c.Identity)` check in `shadow` replaced by a no-op, the test fails at
  `main_test.go:207` ("no Git field: corpus resumed under the current runtime: git exited 128
  ..."), because the resume proceeds to clone and check out the frozen pair.
- With only the `!equal(r.Identity, id)` comparison removed from `validateRow`, the test fails at
  `main_test.go:207` ("no Git field: row reused under the current identity"; 636.9 s, load 89.90
  73.00 64.37). The relabelled row keeps the original files, so this comparison is the only guard
  that rejects it.

`qualify` needs no new guard, because it refuses any corpus whose frozen driver digest differs
from the running qualifier, and drivers that predate the Git field have different digests.

Review: one Codex round found that an earlier draft of this paragraph called the row assertion
defence in depth and said the row-only mutation would not bite. The second mutation above
disproves that; the paragraph was corrected. Codex also noted that the helper writes an empty
`"Git": ""` rather than omitting the field; both decode to the same value, and the wording now
says so. No other findings.

## Execution profile (criterion 2): owner fork

AFP-V0-053 is drafted as **proposed**. It is not self-accepted, because the lane rules reserve a new
execution profile for the owner. It states the Git identity, the config-through-environment
binding, the single advice entry, the no-relabel rule and the controlled-replay classification
rule. It was first drafted as AFP-V0-052 and renumbered before publication, because open PR #992
(V1-1514) claims AFP-V0-052. Options for the owner:

- (a) Accept AFP-V0-053 as written.
- (b) Fold it into AFP-V0-014 as an amendment of accepted text. Amendments need an owner decision.
- (c) Leave it proposed, so the shipped `testEnv` stays an implementation note.

With (c), criterion 2 remains only partly met.

## Verification

- `go test -count=1 -run TestSelectedFailureAndFallback ./tools/corvint-pr-tests/` passes:
  357.8 s at load about 46 to 57.
- Each mutation above fails it.
- The doc gates, `go vet`, `gofmt` and the cross-OS vet are listed in the PR.

## Limits

- The replay ran locally on Darwin with Apple Git 2.54.0 (`/usr/bin/git` is an `xcrun` shim) and
  Go 1.27.2. The hosted row used Linux, Git 2.55.0 and Go 1.27.1. The local build cache was shared
  between variants; `-count=1` reran every test.
- The retained hosted row receipts are not on this host. Their hashes were verified when the ticket
  was filed and were not re-verified here.
- Hosted re-qualification, a fresh freeze and the 200-row campaign are `NOT_RUN`. V1-0616 owns
  them. The `node` PATH gap from the 2026-10-01 entry remains open.

Rollback: revert this change. That removes the proposed requirement and the test helper, and
driver behaviour is unchanged.
