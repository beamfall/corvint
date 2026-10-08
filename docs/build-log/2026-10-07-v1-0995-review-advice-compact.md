# 2026-10-07: compact `review` affected section and advisory command (V1-0995)

## Intent

Ticket V1-0995 follows V1-0943, which made `affected-plan/1` the compact `corvint affected` default.
Two repeats were left. `corvint review` still embedded the full `affected-plan/0` receipt, and the
advisory Go command in `advice.checks` repeated the package list that `provider.go.packages`
already carries. The change adds proposed AFP-V0-038 to `docs/specs/affected-plan-v0.md` and
proposed RGV-V0-015 to `docs/specs/repository-guidance-v0.md`.

## Decisions

- **The advisory command references the provider list.** In the `/1` default the advisory check
  becomes `command="GOTOOLCHAIN=local go test -count=1"` with `arguments="provider.go.packages"`.
  Appending those entries, POSIX single-quoted and joined by one space, gives the `/0` command
  back byte for byte. A command that is not exactly that prefix followed by the quoted packages is
  kept whole and has no `arguments`, so nothing can be dropped silently. Mandatory checks and
  `--full` do not change.
- **Review embeds `/1` and a resolvable reference.** `review.affected` is now the `/1` projection
  of the receipt that review planned. It is byte-equal to the default `corvint affected --base SHA`
  document, so RGV-V0-013's byte equality with the standalone command still holds. The new
  `review.affectedFull` gives `argv` `["corvint","affected","--base",SHA,"--full"]` and `digest`
  `affected-plan:sha256:<hex>`, the hash of the canonical `/0` receipt (that command's stdout
  without its trailing newline). An agent that needs test files or individual exclusions runs the
  argv and checks the digest. A mismatch means the worktree plan differs from the plan that review
  composed. The test runs the argv and checks the digest on both the immutable-snapshot path and
  the over-cap fallback path.
- **Not done.** `advice.unknown` still repeats every `plan.unknown` entry as a string. On
  this repository that is all 37 entries, 2,141 B. This is a candidate for a follow-up and is unchanged here.

## Compatibility classification

- `affected --full` (`affected-plan/0`) is unchanged: the `affected-full-*` core-freeze goldens
  were not regenerated and still pass.
- `affected` default (`affected-plan/1`, proposed by V1-0943 and unreleased): the
  `affected-committed-range.json` golden was regenerated with `CORVINT_UPDATE_GOLDEN=1`. By the
  letter of CCF-V1-006 the change is additive. It adds the optional member
  `advice.checks[].arguments`. It changes the value of `command`, which is not a CCF-V1-005 frozen
  member, a CCF-V1-002 identifier or a registered enumeration. It does change what that command
  means: a reader that pastes `command` and ignores `arguments` runs `go test` in its working
  directory rather than in the selected packages. So it should be treated as part of the
  breaking `affected-plan/1` bump that is still waiting for its CCF-V1-006 decision record, not as
  a separate compatible change. No N-1 reader is affected, because 0.8.1 has no `/1`.
- `review` (`repository-guidance/0`) is experimental and outside the CCF-V1-002 freeze. Its
  `review.affected` member is retyped from a `/0` to a `/1` document (`plan.excluded` changes from
  an array to an object), and `review.affectedFull` is added. Under CCF-V1-006 terms this would be
  breaking. The embedded document carries its own `profile`, so a reader can branch on it. The
  guidance profile identifier is unchanged, because the profile is experimental.

## Measurements

The commands were run on a clean checkout of `ff0ebbab` (the V1-0943 head), with
`--base 0b5096ca0896351a584bca271812d9e1dd34f06c` (90 selected units). The before binary was
built from `ff0ebbab` and the after binary from `e5242385`. Sizes are bytes on stdout.

| Document | Before | After |
|---|---:|---:|
| `affected` default (`/1`) | 39,938 | 35,155 |
| of which `advice` | 8,327 | 3,544 |
| `affected --full` (`/0`) | 152,019 | 152,019 |
| `review.affected` (plus `affectedFull` after) | 152,018 | 35,154 + 204 = 35,358 |

The default affected document is 12% smaller, and the section that review embeds is 77% smaller.
Running `review` end to end on this repository was NOT_OBSERVED. The before binary refused with
`git-timeout` at RGV's 60-second global deadline (host load average about 63), and the after run
refused with `local ref drift` while concurrent sessions moved local refs. The review row is
therefore derived. Before, the embedded member was the canonical `/0` receipt (the `--full`
stdout without its newline), and RGV-V0-013 tests that it is byte-equal. After, it is the `/1`
document (the default stdout without its newline), which RGV-V0-015 tests, plus the fixed-shape
`affectedFull` member (204 B with a 40-hex base).

## Verification

The focused run was `go test -count=1 -parallel 2 -run
'AFPV003|RepositoryGuidance|TestAffected|TestUseCaseHostileChangeConsequence|TestCoreVerbsEmitTheFrozenProfiles$'
./cmd/corvint`, and it exited 0. `-parallel 2` is needed because on this loaded host the
parallel affected fixtures otherwise hit Git deadlines (`git-cancelled`). A second run,
`-run 'Help|UseCase|Guidance'`, also exited 0. The local doc gates (spec-requirements,
requirement-definitions, traceability-tests, decision-numbers, line-citations,
error-code-ownership, unbounded-readers, use-case-receipts and diagnostic-coverage) exited 0
after three line citations shifted by the help and constant lines were repinned and the
UC-CHANGE-CONSEQUENCE receipt was repinned.

## Rollback

Drop `compactAffectedAdvice` and `affectedCompactCheck`, and copy the `/0` advice into the `/1`
projection. Restore `guidanceReview.Affected` to `affectedReceipt`, remove `affectedFull` and
`setAffected`, and regenerate `affected-committed-range.json`.
