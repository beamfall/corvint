# V1-0147: DCW test-claim linkage through supported rebinding

Date: 2026-10-10
Base: `686fb1ac9c067046c1b7b3dc6776ac41e5c61ee6`
Scope: regression evidence for existing `DCW-V0-013..015`; no requirement,
acceptance status, production behavior or wire contract changes. Owner acceptance
is not asserted. Rollback is removal of the regression and its evidence citation.

## Implementation and evidence boundary

`TestDogfoodDCWLinksSurviveRebind` runs the built Corvint producer in a disposable
Git repository. It copies the actual DCW spec and the executing regression's source,
prepares/cites a CEM, and feeds three test-claim rows through `DOGFOOD_OCM_LINKS`.
The production OCM reader verifies the emitted bytes, current target, source blob,
all three obligations, supported hunk references and claim references.

The regression observes the initial uncommitted-sidecar refusal and remediation,
a clean bound pass, a later content commit, the stale-report refusal, and the
supported rebind: regenerate/cite the CEM, commit the sidecar, then replay the same
saved link plan on the clean new target. It compares claim links across the rebind
and runs the strict daily check. The fixture archives the real test source for
claim extraction; the fixture's selected Go check is separate from execution of
this outer regression. No hand-built proof map or claim object supplies evidence.

Links are restored by explicit plan replay. This does not establish automatic
retention without a plan, preservation of manual marks, full DCW clause adequacy,
external adopter qualification or owner acceptance. `docs/DOGFOOD.md` already
specifies replay on every later pass and loss of manual marks; no requirement
amendment is needed. The existing spec evidence row cites the regression.

## Verification and review

Passed with Go 1.27.1 and the environment below:

- `go test -count=1 -timeout 30m ./internal/lrfrepo ./internal/dogfoodocm`.
- `go test -count=1 -timeout 30m ./cmd/corvint -run '^(TestDogfoodDCWLinksSurviveRebind|TestDogfoodDailyPathRunsFromBinaryInForeignRepository)$'`.
- `make -s spec-requirements-check requirement-definitions-check traceability-tests-check decision-numbers-check line-citations-check error-code-ownership-check unbounded-readers-check use-case-receipts-check use-case-receipts-test`.
- `sh script/repin-use-case-receipts.sh --check`: all 18 receipts current;
  receipt script tests passed all five cases. No repin needed.

Independent review found that nonempty claim references alone did not assert the
intended requirement-to-selector mapping. The regression now checks each exact
selector before comparing preserved claim IDs. The reviewer inspected the repair
and reported the finding resolved, with no further findings. The final focused
CLI run passed after that repair.
The exhaustive gate and `./...` are NOT_RUN as explicitly directed. No repository
commit, push, ticket mutation, decision file or plugin manifest change is made.

## Self-use and environment limits

Used: installed Core query and tracked-path impact; affected advice at the empty
baseline (no selected tests, so the ticket's owning packages govern focused checks).
The final dirty-tree plan selects cmd/corvint directly plus broad declared/path-reader
units and reports scope UNKNOWN. The requested focused checks and doc gates govern
this test-only change; no exhaustive coverage is inferred from that advice.
The query retained three omitted results and withheld test-path candidates;
impact retained source-size/build-path exclusions. Native host token/credit
telemetry and cost are NOT_OBSERVED; no savings claim is made. One builder and one
native Sol/medium reviewer are used for this bounded regression review.

Core reports `1.0.0-rc.3` build 407; Tasks reports
`0.0.0-tcp01-unverified+build.407`. Official update checks for both components fail
`process-after-start-failed: descendant snapshot unavailable`; available versions
and freshness remain UNKNOWN. No update is installed and no verification is bypassed.
The requested index refresh fails writing the shared Git directory outside this
sandbox. Direct source inspection is the supported fallback.

The original Git launcher emits an xcrun cache warning into fixture revision output,
causing existing lrfrepo tests to fail `invalid-arguments`; direct Xcode Git avoids
the launcher. Apple's mktemp also selects an unwritable system temp directory;
the installed GNU mktemp honors the supplied TMPDIR. Scratch PATH shims select
those existing tools, with GOTOOLCHAIN=local, GOMAXPROCS=3 and the requested TMPDIR.
The worktree's actual index is outside writable roots, so staging is refused.
Index-based doc gates use a temporary index/object directory containing the edited
files; the original index is preserved. `make -s spec-requirements` regenerates
REQUIREMENTS.tsv from that staged spec; its bytes are unchanged.

Final repository CEM binding/check/seal and native ticket completion remain
NOT_PRODUCED because this task explicitly prohibits commits and ticket-store writes.
Tooling-friction filing is likewise deferred under the no-ticket-write instruction.
