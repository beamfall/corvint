# T7 shared runner selection consumption and confirmed misses

Ticket: V1-5109. Scope: RSP-V0-038, RSP-V0-039, RSP-V0-046, RSP-V0-057.
Build-only lane: test execution and independent review belong to integration.

## Decision and implementation

Keep the existing selector's wire output and shadow-only claim unchanged. The
shared `internal/runnerpolicy` consumer supplies separate internal run sets to
CI and local hosts. The declaration-only `affected` path uses that consumer;
it admits no rollout, qualification, sampling configuration or miss history.
Before first admission, existing governing static plans remain in force. After
protected rollout admission, unqualified runners become FULL, even if their
old static plans narrow. Admission is latched: losing qualification cannot
restore static narrowing. Local resolution replays CI and widens to include it;
an unverifiable comparison or missing CI runner becomes FULL.

Sampling configuration is a single decimal probability in [0,1] at
`.corvint/runner-selection-sample-rate`, supplied from the protected base by the
core host. This separate text input adds no JSON fields or command switches.
There is no committed rate: owner choice is UNSET and measured CI cost is
NOT_OBSERVED. An absent, malformed, capped or unadmitted rate authorizes no
sampling or exclusion. The host's explicit protected admission is required;
reading candidate bytes does not establish that admission. Default-branch
sampling uses a deterministic hash of the exact base and head.

The detector consumes a retained actual native run set. Native supersets remove
exclusions; native subsets refuse. It captures FULL base/head outcomes and
repeats each excluded base-pass/head-fail candidate once at the same revisions.
A nonreproducing head failure is a flake. A deterministic failure blocks every
class implicated by the retained change for that runner (conservative when
several classes changed). Missing or malformed capture evidence records unknown
and forces FULL. A later admitted T6 qualification with a newer protected
sequence clears only the qualified blocked classes. Read commands never capture,
write history, or start sampling. The caller owns protected persistence and must
supply complete admitted history; absent history is FULL after rollout.

The RSP-V0-039 template is `TEMPLATE-rsp-baseline-update.md` in this directory.
The internal baseline check refuses expectations narrower than the soundness fix
and entries that omit newly selected tests. No existing golden or baseline was
changed by this lane. Rollback keeps shadow/FULL and stops sampling; it must not
restore an exclusion known to miss.

## Evidence and limits

Positive and negative fixtures are in `internal/runnerpolicy/policy_test.go`,
with names beginning `TestRSPV0038`, `TestRSPV0039`, `TestRSPV0046` and
`TestRSPV0057`. They cover parity widening, shadow/admission, unknown inputs,
owner-rate/unset handling, pinned confirmation, flake/unknown classification,
actual native supersets, and newer versus invalid requalification.

Implementation tests: NOT_RUN by the final build-only instruction. The explicitly
permitted `make -s spec-requirements-check line-citations-check` unexpectedly
invoked the whole `internal/specindex` package and the focused `internal/lrfrepo`
spec enumeration tests; both passed before an interruption could be applied.
No T7 implementation tests were executed. Live rollout, default-branch
sampling cost, retained protected miss storage, and runner qualification are
NOT_OBSERVED; no production runner is enabled. The implementation is the shared
consumer and detector, not an assertion that an operator has installed admitted
runtime inputs. Independent review: NOT_PRODUCED here; orchestrator-owned.
Dogfood start/bind/check/seal and use-case receipts: NOT_PRODUCED under the final
instruction prohibiting CEM/receipt edits and assigning repinning to integration.
Native context/query/impact and affected selection: NOT_RUN under the build-only
allowlist; integration selects and executes whole touched packages. Task writes
and publication are prohibited. No native ticket completion is claimed.

Static check results and exact commit binding are retained in the lane handback.
