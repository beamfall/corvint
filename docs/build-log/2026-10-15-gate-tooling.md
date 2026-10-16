# Gate tooling: undeclared test read check, hosted N-1 replay, two owner decisions

Date: 2026-10-15. Base: `883caa3d218df05375063c462fc59f3d47808f97` (origin/main).
Tickets: V1-1351 (AFP-V0-051, `docs/specs/affected-plan-v0.md`), V1-1382 (CCF-V1-007 evidence,
`docs/specs/core-compatibility-freeze-v1.md`, runbook step 8), V1-0682 and V1-0359 (owner decisions,
no code change). Changed `.github/` files: `.github/workflows/ci.yml` and
`.github/workflows/release-gates.yml`.

## V1-1351: undeclared repository literals in declared packages (AFP-V0-051)

V1-1340 was found only when the confined shard denied `internal/cem/verify` its read of
`protocol/cem-1.0/stable/tamper-matrix/`. The error looked like a content failure.
`tools/test-read-scopes-check` now reports that case before CI does. It names the package, the test
file and the path.

Decisions:

- The literals come from the AFP-V0-021 scanner. `golang.FilePathTokens` exposes the
  `observeDirectory` lexicon for one file; there is no second parser.
- A literal resolves against the package directory. It resolves against the repository root too
  only if the package's tests can locate the root. This is the same narrowing AFP-V0-021 applies to
  a root-relative reader. A first draft that always resolved from the root reported single words and
  recipe tokens as paths.
- A match must be a Git-tracked path of two or more components, compared case-exactly. On macOS a
  literal `makefile` otherwise matched the tracked `Makefile`.
- A directory above the package or above an entry is in scope. A test climbs through it, or joins
  more components onto it. Without this rule, every parent-only climb reported `internal/cem`.
- Eleven literals on the current tree are not reads. They are fixture text, expected messages, spec
  paths quoted in assertions, and one live-Docker test the confined shard skips. Each one is recorded
  with its reason in `.corvint/test-read-scope-literals.json`. A stale record only reports, like the
  AFP-V0-025 ratchet, so two merges that drop and keep a literal do not fail each other.
- Wiring:
  - The check is a `make gate` step.
  - It is in the CI `doc-gates` step list, which is also the lane doc-gate list.
  - `doc-gates` runs in parallel with `go-product-shard`. It reports within minutes, well before the
    confined Go tests step. It is not a `needs:` prerequisite of the shard, because adding a
    dependency to the required shard jobs would change their critical path. Read "before the confined
    shard runs" as "before it reports".

Evidence:

- `TestUndeclaredRepositoryLiteralFailsNamingPackageAndPath_V1_1351` covers 12 cases. At BASE the
  check and the test do not exist (0 tests). With the change, all 12 pass.
- The V1-1340 reproduction on the real tree: with the `protocol/cem-1.0/stable/tamper-matrix/` entry
  of `internal/cem/verify` removed from `.corvint/test-read-scopes.json`, the check exits 1 with
  `package internal/cem/verify: internal/cem/verify/stable_tamper_matrix_test.go names
  protocol/cem-1.0/stable/tamper-matrix`. With the entry restored it exits 0 (`ok`, about 3 s).
- Mutations, each restored afterwards:

  | Mutation | Result |
  |---|---|
  | Always resolve from the root | 10 failure lines |
  | Skip the tracked-path test | 6 failures |
  | Drop "ancestor of an entry" coverage | the declared-file-below case fails |
  | Keep only the first tracked candidate of a literal | the root-shadowed case fails |
  | Count `.corvint/change.cem.json` as tracked | 8 cases report it |

- The shared change-evidence map `.corvint/change.cem.json` is not a tracked path for the check. It
  is tracked only between `dogfood bind` and `dogfood seal`, which moves it (DCW-V0-027), and nine
  declared packages name it for the fixture repositories their tests write. The first run after this
  lane's own bind commit reported it 17 times. AFP-V0-051 (2) states the exclusion.

## V1-1382: the N-1 replay in a hosted workflow

`release-gates.yml` gains two matrix gates on ubuntu-24.04, `core-n1-replay-v1.0.0-rc.1` and
`core-n1-replay-v0.8.1`. Each runs `make core-n1-replay CORE_N1_TAG=<tag>` at the dispatched `sha`,
and nightly on `main`. Logs and sha256 files are uploaded as
`core-n1-replay-<tag>-<release>-ubuntu-24.04`, like every other gate.

- The documented N-1 baselines are `v1.0.0-rc.1` (decision 0434 item 3) and `v0.8.1` (the
  CCF-V1-007 text, and what the Makefile default resolves to today). The first draft ran only
  `v0.8.1`; the review found that this skips the decision 0434 baseline, so both run.
- The tags are named explicitly. The runbook already asks for the Core tag to be named after a
  history rewrite, and a moved ancestry default must not silently change the baseline.
- The checkout already fetches full history and tags.
- Runbook step 8 now names the job and its artifact.
- The CCF-V1-007 (d) evidence row cites the job. Only the evidence row changed; the requirement text
  did not.

Local proof (host load averages 20.66 to 42.13 during the runs):

- `make core-n1-replay CORE_N1_TAG=v0.8.1` passed (`ok`, 55.4 s).
- `make core-n1-replay CORE_N1_TAG=v1.0.0-rc.1` (`5d6cbe8`) passed (`ok`, 21 s wall; loads 27.66 to
  34.67).
- A scratch commit `f0537d7` was built on `v0.8.1^{commit}` and was never referenced or pushed. It
  changes only the `claim` value that `dogfood status` emits. `make core-n1-replay
  CORE_N1_TAG=f0537d72daad10e935ea37ea41cf755e074dabd1` failed (make exit 2):
  `TestCoreVerbsEmitTheFrozenProfiles/dogfood_status_retained_outcome/N-1` reported
  `claim = "deliberately-broken-n1" ..., want "caller-owned-selected-workflow-only"`.
- A hosted run that fails on a broken N-1 mode is NOT_RUN. A `workflow_dispatch` with `--ref` of
  this branch could run the new gates before merge, but it runs the whole release-gate matrix, which
  takes hours of hosted time, and the lane scoped hosted runs to after merge. The first nightly on
  `main` is the first hosted pass. A recorded hosted failure needs a deliberately broken baseline tag
  and remains open on V1-1382.

## V1-0682: `make gate` labelled mandatory (owner decision, no change)

Observed with corvint 1.0.0-rc.3 at BASE: `corvint affected` advice lists `make gate` as `mandatory`,
with source `Makefile` and the reason "the repository Makefile declares a gate target". The
AGENTS.md Verify commands are mandatory too, even though AGENTS.md scopes them to the terminal
boundary.

This is the accepted AFP-V0-009 behaviour (decisions 0052, 0424): a Makefile `gate` target yields a
mandatory `make gate`, because requiring too much is safe. Changing the label amends an accepted
requirement, which the standing auto-accept does not cover.

Proposed amendment for the owner: when a project instruction scopes the full gate conditionally,
label it `conditional` and name the condition. Otherwise keep it mandatory and name the
uncertainty.

## V1-0359: linux/arm64 leg in hosted vet (owner decision, no CI change)

Accepted AFP-V0-042 (decision 0496) states as a non-goal that `GOOS=linux GOARCH=arm64` stays with
`make cross-vet` and the nightly gates. Adding the leg to the go-static step amends that
requirement, so this change does not add it.

The ticket's failure case is already caught by the required step:

- A scratch `_test.go` calling an unconstrained `syscall.Mkfifo` makes the exact CI command
  `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go vet ./...` exit 1, with
  `undefined: syscall.Mkfifo`. That was the only error over the module.
- The same file passes `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go vet`, because linux/arm64 has
  `Mkfifo`. An arm64 leg would not add this catch.
- Loads were 32.05 to 51.58 during the runs.
- The scratch file was removed.

The remaining AC, the arm64 leg itself, waits for the owner to decide whether to amend AFP-V0-042.

## Review

One Codex round (`codex exec -s read-only`) over `BASE..HEAD`. Its findings, all fixed in the
review-fix commit:

1. P1: the replay ran only `v0.8.1`, while decision 0434 item 3 names `v1.0.0-rc.1` as the N-1
   baseline. Both baselines now run as separate gates. The runbook and the CCF-V1-007 (d) evidence
   row name both.
2. P2: `named` returned the in-package match first, so a literal that also named an undeclared path
   from the root was masked. It now returns every tracked candidate. The root-shadowed case and its
   mutation cover this.
3. P3: the build log said `workflow_dispatch` cannot run an unmerged workflow. That was wrong for an
   existing workflow dispatched with `--ref`. The NOT_RUN reason is corrected above.

## Residual

`TestSelectionOnTheLiveDirtyWorktree` (`internal/liveverify/affected`) failed during this lane with a
14-path dirty worktree: selection ratio 10 to 14 against a bound of 6.67. With the same 10 modified
paths dirtied at BASE, the ratio was 7.33 (fail) and 6.39 (pass), so the failure predates this
change and depends on the size of the dirty set. It passes on a clean tree.
