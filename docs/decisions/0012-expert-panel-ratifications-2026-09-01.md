# Decision 0012 — expert-panel ratifications (2026-09-01)

Date: 2026-09-01. Status: accepted. Authority: repository owner, explicit in-session rulings
reproduced below.

## Scope

This record disposes of the ten open owner questions in `docs/agent-memory/questions.md`, completes
decision 0007 D12, and records four directions whose implementation remains in separate in-flight
work. **This record is the authority.** The spec and governance applications authorized here land
with it; Python removal, the Go-baseline performance respecification, the replay-protocol design,
and the four R7 implementations remain separate changes.

Decision numbers already collide across Git lineages. Current main's 0009–0011 differ from the
same numbers on `codex/remove-unreleased-python`; that lineage also has different 0012–0013 and
0014–0017, including its own decision 0012. Date-qualified decision filenames are therefore the
repository convention from this record forward. A uniqueness check is tracked separately.

## R0 — Python is not the oracle and is removed separately

> The Python implementation under `src/` is NOT the oracle, was never proven correct, and is to be
> removed. Python parity is no longer a design constraint; removal is sequenced separately (branch
> `codex/remove-unreleased-python` exists and is not part of this task).

`GPK-V0-033` already makes the frozen spec, not Python, the authority and says agreement is not proof
(`docs/specs/go-production-kernel-migration-v0.md`, `GPK-V0-033`). The immutable removal decision at
`f82e4116d919429112a56e915014f5d8eddd2f94:docs/decisions/0017-unreleased-python-removal-2026-08-31.md`
records that Python never shipped and that historical evidence remains evidence, not executable
authority. Current GPK fallback, mandatory-cross-check, oracle, and staged-retirement language is
stale under this ruling and is corrected by the separate removal/respecification lineage, not here.

## R1 — no `beamfall-impact-path` grant

> `beamfall-impact-path`: NO grant is ratified and `taskSpec.AcceptedDivergence` is NOT widened to a
> set; the GPK-V0-017(d) criteria for that task are recorded permanently NOT_RUN pending a
> Go-baseline respecification of GPK-V0-016/017(c)(d) (separate proposal, not this task).

The closed grant set contains only DR-0004 for `impact-path`, and `AcceptedDivergence` remains one
string (`conformance/perf-v0/manifest.go`). The measured Beamfall rows are already `NOT_RUN` on
unequal stdout (`conformance/perf-v0/results/corvint-beamfall-2026-08-29/report.json`). This ruling
withdraws the contrary grant recorded only on the removal lineage and schedules no measurement.

## R2 — Packet 5 measured result stands

> Packet 5: no rerun, no re-preregistration; the formal `insufficient_evidence`/`NOT_RUN` result is
> the measured truth; artifacts may be imported only as immutable evidence.

The immutable formal receipt at
`6977a2c:conformance/perf-v0/results/packet-5-current-pin-requalification-2026-08-31-formal/report.json`
records `outcome: insufficient_evidence` and `slicePerformanceStatus: NOT_RUN`; `91c1359` records the
run provenance. Importing those exact artifacts cannot relabel or rerun them.

## R3 — Frontier/TCQ remains edge-local

> Frontier/TCQ: plan item 18 is REJECTED; accepted GPK-V0-041 scoping stands and TCQ-V0-047
> edge-local `unsupported-python-grammar` abstention governs the frontier/TCQ surface.

`GPK-V0-041` limits the categorical refusal to OCM claim verification and explicitly delegates the
Frontier/TCQ surface to `TCQ-V0-047`; that clause keeps Go processing available and abstains only the
affected Python edge. DR-0010 is split so its landed `lrf --ocm` repair no longer hides the open
Frontier/TCQ half.

## R4 — Change Frontier is candidate-only

> Change Frontier ships candidate-only: the three outcome gates retained from superseded Verified
> Absence Frontier (0.70/0.50/0.25 at docs/specs/change-frontier-v0.md:459-461) are WITHDRAWN from
> live status; replaced by a deferred preregistered third-party-PR replay protocol whose deadline is
> set at first release cut (protocol construction is a separate task).

The former gates were duplicated in Change Frontier's evaluation section and inherited from
`docs/specs/verified-absence-frontier-v0.md`. They confer no live promotion or kill status after
this ruling. Change Frontier remains a candidate-only preview; the future protocol must freeze its
third-party PR selection and outcome criteria before replay and is `NOT_RUN` until constructed.

## R5 — decision 0007 D12 is complete policy

> CF-V0-004: decision 0007 D12 is COMPLETE policy — finish applying it (see work item 3).

Decision 0007 D12 selected unreachable-but-canonical `EMPTY` over widening OCM. `CF-V0-004`
already records that rule; `CF-V0-018` is corrected here so its old “no linked obligation” sentence
no longer contradicts `CF-V0-011`.

## R6 — an empty OCM requirement enumeration is invalid

> OCM-V0-001 narrowing ACCEPTED: an intent scope enumerating zero requirements is normatively invalid
> (`missing-requirements`), making CF-V0-004 unreachability clause-backed. Both runtimes already
> behave this way; text-only.

The existing Go and Python guards return `missing-requirements`
(`internal/lrfrepo/ocm.go`; `src/context_corvint_ocm.py`), and
`conformance/frontier-v0/fixtures_test.go` executes the refusal. The accepted amendment changes no
runtime behavior.

## R7 — directions delegated to separate in-flight tasks

> Also ratified as design directions implemented by other in-flight tasks (record, do not implement
> here): CEM policy exception = committed digest-bound sidecar with env-var bypass remaining
> hard-fail; SOL-V0-008 single bounded dogfood append surface with Append atomicity precondition;
> harness receipt flag day to `corvint-harness-event/1` whole-response seal + `requestId`; LRF custodial
> gate retired in favor of a label-free falsification protocol (amendment drafted separately).

These resolve the questions grounded respectively in `DOGFOOD-004` and
`script/dogfood-check.sh`, the read-only `SOL-V0-005` boundary, the request-only receipt basis in
`internal/gokernel/harness.go`, and the superseded custodial gate in
`docs/specs/lexical-relevance-floor-v0.md`. They authorize no implementation in this change and do
not turn any in-flight branch into landed evidence.

## R8 — remove Python verification from agent instructions

> AGENTS.md: the Python verification lines are removed (per R0).

Only the two `PYTHONPATH=src` commands are removed. The Go and interoperability verification
commands remain unchanged.

## What this decision does not do

- It does not remove `src/`, amend the Go performance baseline, grant a performance divergence,
  rerun Packet 5, construct the Change Frontier replay protocol, or implement an R7 direction.
- It does not merge, publish, promote, tag, sign, or make a release cut.
- It does not turn historical Python or Packet 5 artifacts into current product authority.

## Consequences

The ten open owner questions are removed. Change Frontier remains candidate-only; `EMPTY` is
clause-backed unreachable in V0; Packet 5 and `beamfall-impact-path` retain `NOT_RUN`; the
Frontier/TCQ grammar defect remains visible as its own OPEN divergence; and Python verification is
no longer part of the repository agent gate.
