# Batch Integration Checks V0

Owner: Russell Lewis
Date: 2026-11-04
Intent status: accepted
Delivery status: experimental
Authoritative inputs: companion of `corvint-self-development-v0.md`; tickets V1-4153 and V1-4110 under the owner's standing lane auto-accept;
`AGENTS.md`; `docs/SPEC-DRIVEN-DEVELOPMENT.md`; `docs/specs/corvint-self-development-v0.md`.

## Agent digest
- Claim: Batch integration scripts attribute a failure the diff cannot have caused and state the scope limit of the requirement-ID collision screen.
- Status: accepted/experimental
- Exists: the unattributed WARN branch of `script/batch-precheck.sh` and the scope note of `script/requirement-id-collisions.sh`.
- Blocked on: nothing; experimental under the standing lane auto-accept.
- Read next: Requirements; Acceptance and evidence.

## User job and current boundary

The operator integrating lane branches into one batch needs `make batch-precheck` to separate a
failure the batch introduced from one its diff cannot reach, and needs the requirement-ID
collision screen to say what it does not see. This is a companion to
`corvint-self-development-v0.md`, whose `## Requirements` section is at the 256-obligation OCM
bound. It uses the existing ASD-V0 prefix, as `windows-test-portability-v0.md` and
`lane-shell-portability-v0.md` do, so the two IDs below keep the numbers their code, tests and
build logs already cite; their text moved here unchanged during batch integration on 2026-11-04.

## Requirements

- `ASD-V0-110`: (experimental under the standing lane auto-accept, 2026-11-01; V1-4153; refines `ASD-V0-032`) With a base, a `make batch-precheck` step that declares its inputs in `UNATTRIBUTED_INPUTS` (`make host-adapter-node-test`: `integrations/`, `Makefile`, `cmd/corvint/pi_adapter.go`, `docs/specs/pi-workflow-v0.md` and `conformance/`), fails at HEAD after its `ASD-V0-046` rerun, does not fail the same way at the base, whose inputs the diff since the base (tracked changes plus untracked, unignored files, read NUL-delimited so a git-quoted name compares as its path) does not touch, and whose coverage scan passes MUST print `batch-precheck: WARN NAME (Ns) unattributed: ...` naming the inputs and the 1-minute host load, and is not counted as an introduced failure; like a base-present WARN it writes no receipt. The coverage scan reads every parent-relative path literal (`..`, `../x`) in the test files the step's recipe names and in the tracked non-test JavaScript under their directories; a recipe naming no tracked test file, a literal resolving to the repository root, outside the repository or outside the declared inputs, or one naming a module outside the scanned directories is uncertain coverage. A step without declared inputs, uncertain coverage, an unresolvable base or a git error keeps the introduced FAIL. Design: lanes whose diff touched only docs saw `host-adapter-node-test` reported introduced under load. Non-goals: declaring inputs for other steps; deciding flake versus defect; resolving computed paths. Failure mode: an input the scan cannot see (a path a test builds without a parent-relative literal, or reads relative to the working directory) can hide a real regression as unattributed; the WARN still blocks the receipt. Evidence: `make batch-precheck-test` section 6. Rollback: delete `UNATTRIBUTED_INPUTS`, `unattributed` and its branch.
- `ASD-V0-080`: (experimental under the owner's standing auto-accept (2026-11-01); V1-4110; extends `ASD-V0-950` and `ASD-V0-952`) `script/requirement-id-collisions.sh` compares `docs/specs/REQUIREMENTS.tsv` definitions only, so an ID that a build log, charter or ticket names before any spec defines it is not seen. Each verdict, in both modes, MUST be followed on stderr by one `requirement-id-collisions: NOTE scope:` line that states this limit and names the `git grep -nE 'PATTERN' REF -- docs/build-log` that finds such prose mentions on REF. Non-goal: scanning prose, whose IDs may be quotes, examples or other prefixes. Evidence: `script/requirement-id-collisions_test.sh` (a build log naming ASD-V0-055 leaves the block free, the block and collision verdicts carry the note, and the named grep finds the mention). Rollback: drop `scope_note` and its calls.

## Non-goals and trust boundary

Each requirement states its own non-goals. Neither script decides whether a failure is a flake
or a defect, and neither reads prose, tickets or charters as requirement definitions.

## Acceptance and evidence

| Requirement | Implementation and actual evidence |
|---|---|
| ASD-V0-110 | `script/batch-precheck.sh` `UNATTRIBUTED_INPUTS`, `unattributed` and the unattributed WARN branch; `make batch-precheck-test` section 6: a `host-adapter-node-test` failing at HEAD and passing at the base with a diff touching only `docs/build-log/` is an unattributed WARN naming the five inputs and the host load, never introduced and with no receipt; the same failure is an introduced FAIL with an `integrations/` change, a `cmd/corvint/pi_adapter.go` change, an untracked or a committed git-quoted name under `integrations/`, or a recipe test that reads an undeclared path | implemented; shell test PASS |
| ASD-V0-080 | `script/requirement-id-collisions.sh` `scope_note`; `script/requirement-id-collisions_test.sh`: a build log naming ASD-V0-055 leaves the block free, the block and collision verdicts carry the note, and the named grep finds the mention | implemented; shell test PASS |

## Failure and rollback

Rollback for each requirement is stated in its text: delete `UNATTRIBUTED_INPUTS`, `unattributed`
and its branch (ASD-V0-110); drop `scope_note` and its calls (ASD-V0-080). Rolling back either
restores the earlier behaviour without touching any other batch-precheck step.
