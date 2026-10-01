# Experimental task criterion experiments for a possible CEM 0.4

Date: 2026-09-30
Owner implementation authorization: "ok, work on it" after the CEM 0.4/Tasks/tests exploration.
Tasks: V1-0573 (research parent), V1-0575 (bounded implementation).
Public baseline: `9afd8313edad658d6c40b1abe454fe7417eb6cad`.

## Decision and authority

Build an optional `corvint-cem-experiments` companion rather than change a frozen CEM
wire. The proposed [contract](../specs/cem-criterion-experiments-v0.md) binds every
native task acceptance criterion to an immutable oracle, canonical CEM hunks and
registered incorrect alternatives. Historical verification recomputes recorded
inputs/classifications; a separate live gate checks applicability to the current
submitted task attempt. Semantic relevance remains reviewer-attested and execution
caller-reported. No default Tasks policy, Core release content or existing wire
changes. Owner technical acceptance and protocol promotion remain open.

The hypothesis is useful continuity of experimental evidence through planning,
testing, handoff and acceptance changes. Tests that distinguish wrong patches
already exist; their presence does not establish product novelty. The proposed
external three-arm outcome protocol compares equal strong testing with and without
packet persistence, retaining incorrect accepts/rejects, effort and all screened
denominators. Cohort/threshold acceptance, power analysis and external outcomes are
`NOT_RUN`. The research parent's acceptance criteria remain unchanged and open.

## Context and bounded implementation

The original Corvint query and path-impact/affected receipts are retained privately
under `/private/tmp/cem04-build`. The query included project governance and withheld
test candidates; it did not establish complete source context. Mixed-tree/non-Go
impact and affected-plan uncertainty were retained. The source build uses a clean
worktree from public `origin/main`; the primary's unrelated dirty changes and newer
uncommitted Tasks review code are not dependencies.

One Astra/medium builder and one independent Astra/medium reviewer were selected for
immutable identity, native authority and process lifecycle reasoning. Gate A passed
with no HIGH findings; coherent audited snapshots, retained gate digests and
historical/live separation were adopted. The initial dogfood-change ran before the
implementation and refused because there was no diff and intent/outcome inputs were
absent. Keyed check enrollment followed the initial doc and builder start; it was
not retrospectively recorded as preceding edits.

The initial runner is restricted to literal committed Go modules with standard
library imports, named top-level tests, one pinned oracle overlay, bounded source,
time and output, a pinned executable and explicit exact-plan approval. It uses fresh
private workspaces and a closed offline Go environment. Trusted test code is not
an OS sandbox; complete toolchain attestation, detached hostile processes and
broader providers remain outside qualification. Raw source/log artifacts are local;
the existing CEM wire stays source-content-free.

## Independent findings and actual integration friction

The first live nonfixture CLI attempts refused planning. They exposed native
`writeBarrier:"NONE"`, the mandatory trailing LF for nested canonical records, and
distinct policy identities: queue status carries the WQO content identity, while an
external attempt binds the raw canonical policy file digest. The companion now
captures the validated raw policy at the audited intent tree and checks both
identities separately. Failed observations were preserved rather than replaced.

Independent review found an assertion substring false-positive: `UNEXPECTED` or a
marker-bearing filename with a wrong message counted as the expected assertion.
It also found error diagnostics followed by passing events were accepted, and that
Git executable modes differed from staged permissions. A first repair still admitted
an embedded source-location substring; the second bounded repair fixes the first
source-location delimiter. The final classifier requires an exact marker at the start
of the parsed diagnostic message and refuses contradictory passing observations.
Staging preserves Git regular-file permissions and explicitly chmods against umask.
Independent implementation re-review passed with all three findings resolved. Reproduction and review reports remain private
in the evidence directory. Primitive null values, native snapshot movement and
immediate output-overflow cancellation receive focused regression coverage.

The native Tasks claim initially collided with another live change's shared spec
indexes and CEM archive scope. Only new source/spec paths were claimed and edited.
Shared paths were acquired through `widen` after that holder released its attempt;
the refusal receipt and subsequent successful acquisition are retained. Lifetime
effects coverage stays INCOMPLETE while the explicit requested path scope is checked;
it was not relabelled to bypass admission.

## Evidence and promotion boundary

Baseline CEM wire and Tasks ticket unit tests passed. Actual matching-source native
lease qualification (`TestCALV0019_*`) passed in 50.495 seconds. The disposable
nonfixture lifecycle, focused source/CLI tests, docs traceability, independent final
review and post-commit dogfood evidence are recorded below when observed. No full
`make gate` is claimed under the owner's focused-scope policy.

Final source observations: the repaired focused suite passed in 37.543 seconds, CLI
unit tests in 0.254 seconds, and focused vet passed. Frozen CEM verifier corpus and
process-group tests passed (3.783/0.211 seconds). The final independent implementation
review is PASS. Gate B records implementation criteria PASS and delivery PARTIAL
until terminal binding/publication and production landing/native completion.

The compiled disposable lifecycle succeeded with eight requested scenarios: three
candidate passes, repair base expected failure, preservation base pass, and three
registered control expected failures. Actual native init/qualification/cutover,
claim/submit/COMMAND gate/local integration/completion/readback/audit were retained.
A dirty submitted candidate was refused while the attempt was live; historical
verification survived later completion and native acceptance refinement, while live
reuse refused. That acceptance-change example also has a terminal attempt, so isolated
binding field checks remain component tests, not an external causal qualification.
The final classifier binary repeats the disposable lifecycle; exact observations and
artifact digests stay in the private evidence manifest. No product superiority is
inferred from the clamp application.

Terminal spec/index/requirement/traceability checks, frozen selected check observations,
CEM/base-verifier consistency, report acknowledgement, native focused-docs gate and
rename-only sealing are retained in the same private evidence set. They must actually
pass before publication; this log describes their required evidence rather than
asserting unobserved success. The final handoff names any remaining closeout step.

## Rollback and remaining work

Remove the optional companion and its experimental route to roll back behavior;
preserve historical packets and receipts. No production queue migration or release
was performed. Ready PR publication is authorized; merge/release is not. V1-0575
remains open until production integration and native completion succeed. V1-0573
additionally retains external comparative outcome evaluation and promotion work.
