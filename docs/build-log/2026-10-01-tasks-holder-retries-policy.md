# Tasks holder signals, retry observations and CREATE/policy help

Owner-requested issues [426](https://github.com/beamfall/corvint/issues/426),
[427](https://github.com/beamfall/corvint/issues/427),
[428](https://github.com/beamfall/corvint/issues/428) and
[430](https://github.com/beamfall/corvint/issues/430) amend the accepted external-agent
contract with CAL-V0-048..051. Native tickets are V1-0625..0628. Base is public
cf4c8a39e171d72374e8baadeb744d4105116eda. No production queue policy or live attempt metadata
was migrated. Source generation106 used an exact REQUESTED scope, including every spec,
registry, test and CEM path; unknown feature-wide effects stayed INCOMPLETE.

## Decisions and limits

An explicit heartbeat journals a generation-local timestamp and a fixed 600-second observation
TTL. It never extends the work lease, retires processes, releases a reservation or preserves a
retry merely because a signal is stale. Request replay retains the original timestamp across
expiry/readmission. Canonical criterion capture uses the record projection and excludes derived
holder display metadata. Legacy absence and backwards clocks remain explicit observations.

Retry reads share the admission exhaustion predicate and current acceptance revision. Prospective
charged readmission partitions debt by terminal cause; clean handoffs copy it. Legacy debt is
UNKNOWN, never assigned an invented reason. Reason totals must equal retryCount. A policy
change adjusts the bound without forgiving debt, and remaining zero is distinct from refusing
initial admission or an eligible clean handoff.

Policy show uses the consistent native read callback. Its SHA is the existing policy content
identity over canonical body bytes, distinct from a simple file checksum. Update help specifies
canonical JSON and current-version versus next-file-version semantics. CREATE already supports
chosen IDs through optional payload localToken; this change fixes misleading target flags and
proves the reported board IDs without adding another identity or alias mechanism.

PR429's reviewed criterion-binding help source/tests are composed here without its unrelated
build-log/CEM. They retain capture versus offline verification and no-stdin help guarantees.
This is a source dependency, not independent evidence for the new holder or retry behavior.

## Evidence and qualification

Corvint Core rc.1 build163 and official Tasks developer build202 were current against observed
release tags. Release checks retain publisher identity NOT_VERIFIED and limited release
qualification. The clean-base Corvint query selected the owning lease spec; omitted results,
unparsed/excluded sources and the initial empty range-impact receipt remain explicit. Actual
prechange query/impact receipts are retained with their original bytes; empty range impact is
OUT_OF_SCOPE, not positive path coverage. The initial dogfood attempt retained NOT_PRODUCED
reasons (no diff, missing intent scope/outcome); final immutable binding is separate.
Affected advice's unconditional full-gate recommendation is known open question V1-0563; the
project's explicit scoped-issue policy governs. Repository-wide make gate is NOT_RUN.
Unsupported language/provider, held-out model-learning, corpus/index performance, browser,
console and release-promotion routes do not fit this Tasks slice and were not invoked.

Go1.27.1 is confirmed. Transaction and snapshot clean-base units passed. New executable
regressions failed before the corresponding fixes. Focused tests passed for TTL boundaries,
legacy closed codecs, reason totals, current acceptance, zero budgets, clean handoff debt,
actual CLI policy reads, board identity and heartbeat replay/fencing. The compiled candidate
SHA-256 is 3540b6e3571b518ef0d73eeff8e000a12f004e86ca66d8ae8d269d5cf8a1a560.
Actual-binary qualification exercised CAL-V0-045/047..051 and canonical criterion capture on
disposable native stores. Broader CLI/store/snapshot/transaction/mutation/spec-index suites ran;
two integration failures were retained: decorated attempt reads broke criterion capture, and
an existing tamper fixture became internally invalid under reason-total validation. Both were
repaired and their targeted reruns passed. Other package results passed. Relevant go vet and
focused registry/definition/traceability/decision/citation checks passed. No hosted outcome,
production liveness, external interoperability or performance claim follows from these checks.

Gate A's independent Sol/medium review found no HIGH. Its MED findings required request replay
without timestamp refresh, a mutually exclusive reason partition with exact totals, and explicit
terminal/expiry/backwards-clock states; implementation and witnesses address those findings.
The builder retained its existing runtime for stateful invariants; live model controls and billed
per-task tokens are NOT_OBSERVED. Independent source/acceptance review found no actionable findings. Each ticket's behavior
criteria scored PASS; immutable-evidence criteria remain PARTIAL at review time. The final
full CLI package rerun passed after the capture repair. Final CEM/OCM closure is a separate
post-commit boundary, and publication/integration/native completion are not source-review claims.

## Rollback and completion

Preserve journals, requests and optional record members. Stop admissions before replacing a
writer and use a reader/writer compatible with added metadata; do not silently downgrade an old
closed decoder over new records. Candidate binaries remain disposable, never installed as an
official release. Native completion requires integrated candidate trees plus the declared gate;
PR publication or a CEM seal does not complete the four tickets. Keep them open until supported
integration/completion succeeds, and release the exact source reservation after verified PR
publication instead of holding global evidence paths through hosted CI.
