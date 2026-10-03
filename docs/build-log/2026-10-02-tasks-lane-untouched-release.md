# CAL-V0-067 explicit untouched release: scoped implementation evidence

The issue 479 owner intent and accepted Gate A proposal are retained by the genuine S17
intent seed `4d4bef35d311c67a4ac200e43f021d7abfb5f749`, based on public
`094700bfbc7b637bd2d6405cd82ab5508ac4aa20`. Source commit
`4396ba2147d10a184c0c4d2b6bb51f1879cee7a1` implements the closed origin/attestation
profiles, strict native eligibility, exact occupancy transition and original-receipt response.
Test-only repair `7ab3ed503fbd6351d8af6ed7aed13bdf69f1f587` completes the publication
fault and archived historical replay discriminators. No production code changed in that repair.

One independent Astra/high source and acceptance reviewer found a P2: the initial terminal
harness stopped only before the receipt and first post, and archived only the current attempt.
The same reviewer passed the narrow repair after inspecting every-publication fault coverage
and archived original replay after successor overwrite. The final report SHA-256 is
`8bce0e8300c053e7137a2ee2486e7461b86e7d91d9a620fcdd29d298835466e2`.
This source judgement is distinct from the dynamic evidence below and from final integration.

At source `7ab3ed50`, with `GOTOOLCHAIN=local`, the actual raw commands passed:

- `go test -v -count=1 -timeout 30m -run '^TestPoolLaneUntouched_ArchiveCrash$' ./internal/tasks/store`
  (package 3.799s): actual emitted order RECEIPT, attempt, pools, request, reservations, HEAD;
  faults before all six publications, receipt-fenced recovery, complete attestation/allocation
  and reservation/occupancy afterimages, and byte-equal replay. After successor overwrite,
  verified archive history preserves the original receipt and attestation/origin; a separate
  test fixture reconstructed at the recorded authority path replays the original response
  without changing its successor or the separately preserved original fixture.
- `go test -v -count=1 -timeout 30m -run '^TestPoolLaneUntouched_NativeFixture$' ./internal/tasks/cli`
  (package 1.823s): a compiled binary, fixture:false NATIVE queue, configured-cleanup/no-health
  positive release, malformed/wrong-role controls, successor/original replay, default quarantine,
  required-cleanup safe-confirm refusal and consistent audit.

No EVIDENCE artifact or STAGE artifact role is emitted by this inline release. Stage-file and
stage-descriptor internals are outside the publication fault hook and remain NOT_INJECTED.
The native fixture's synthetic cutover events exercise parser/admission wiring; they are not
passing CAL019 results or deployment qualification. Archive reconstruction is a private test
fixture at its immutable recorded authority path, not a new restore or portable relocation feature.
Actor authentication, physical non-use/revocation and private Dispatcher history remain
NOT_OBSERVED; private Dispatcher was not scanned. Quiescence is logical FENCED, never physical PROVED.

Earlier pure/replay proofs cover closed optional codecs, immutable direct origin, prepared/retry
exclusion, renewal/usage/tuple refusals, inventory-bound ADMITTED-before-ATTACH Programs, exact-only
occupancy removal, default quarantine, inline/blob original payload and damaged-payload refusal.
The strict-policy paired proof preserves ordinary issue482 compatible handoff with quarantine
while flagged reuse refuses STALE_POLICY. Admission-witness readability through ordinary
supervisor ATTACH is separately tested. An initial compile failure (`wire.CodeUnauthorized`) and
an initial positive-test failure exposed implementation omissions; both were repaired before
passing proofs. The retained earlier `code-compile.log` is a failed iteration, not PASS evidence.

The first enrollment remains key
`117496238c494b52827b243f0696a8bf0bdb1b5f84d871c8f587e99bca119040`, plan digest
`e51280e0db44788cf4df8423a87c7ac582967ebe5b58d045dc0980d787406acd`, against the same seed.
All six selected checks disallow CEM-only reuse. They must bind the final clean CEM commit;
raw source proofs above do not substitute for those receipts. Final CEM/OCM report review,
strict finish/check, pure seal, CI/integration and supported native completion remain separate
required outcomes; no success is inferred before their actual evidence exists.

Original prechange query and impact omitted three and 193 ranked results respectively, with
no reported critical missing paths. The original zero-diff bootstrap remains incomplete:
CEM preparation `git-diff-failed`, citation map absent, OCM preparation/status exit-2 and
aggregate intent-scope-drift, CEM status map-unavailable, local outcome no-source-paths.
No enrollment reset or fabricated map replaced those observations. Current affected advice
also suggests an exhaustive gate; the scoped owner policy governs instead. Its full raw output
and unknowns are retained, and repository-wide gate coverage is NOT_RUN.

Current issue482 status is reconciled from coordinator verification of merge `094700bf`,
native completion receipt 2173 and GitHub closure at 2026-10-03T01:29:06Z. Historical intent
and build-log entries remain unchanged. Its scoped fixture and unobserved authentication/runtime
limits remain in force; it grants no physical reuse authority to CAL067.

Rollback stops new opt-in use while retaining the metadata and original receipt replay with a
compatible reader. Never strip witness/attestation history, silently downgrade to an old reader,
or overwrite a successor. Issue 479 remains OPEN until its final integration and native
submit/gate/complete/readback/audit succeed.
