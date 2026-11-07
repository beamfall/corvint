<!-- SPDX-License-Identifier: Apache-2.0 -->
# Frozen graph identity after AZU merge-forward

The host verifier at `aa1d702ca584d6c8402023f63c8e8812f93e733f` failed
`TestAFUV1FlowsQueryGoldens`, `TestAFUV1024StrictAndCoverageBytesUnchanged`
and five affected/prove cases in `TestCoreVerbsEmitTheFrozenProfiles`.
Only graph digests and their derived exclusion identities differed. The core
fixture produced `03f13ebd2bc818635f33e99e37789b95a55a5935a4cc20721b6f6d6e473fba17`;
adding its historical `driver` namespace to the canonical language list yielded
the frozen `0882fbe408d2ed17743334e12d4ce395ed12f51f7e59017ccb918e403bfbda38`.

Decision: distinguish the registered driver from an explicitly optional driver.
Mainline at the supplied base already registers the driver and freezes its
namespace even when no driver files exist. Restore the CLI's use of that
provider registry. `RegisteredDriver` retains the same observations, ownership
and edges while preserving the namespace. Explicit `Driver` sets retain the
existing AZU-V0-8905 empty-driver parity assertion unchanged. This repairs the
reported AFU-V1-024 and CCF-V1-002 frozen failures without changing a wire shape,
golden, error, diagnostic or existing assertion. No observed vertex is hidden.

The existing optional parity test and new
`TestRegisteredDriverRetainsEmptyIdentity_AZU_V0_8905` exercise both identities.
The three original CLI failures pass unchanged in the focused run (0.933 s).
Its external log SHA-256 is
`e2b25e6313931a92d689ae69daa131f0d404e78b845367ad5d28a8fc61052066`.

## Retained incompatible assertion

The whole CLI package (305.179 s, failed) exposed a lane-only assertion:
`TestAffectedAuditDriverDoesNotChangeDefaultPlan_AZU_V0_002` requires a changed
Makefile to remain unowned and UNKNOWN in default planning. The restored base
instead observes and selects its driver target. A namespace-only candidate
preserved this assertion but still failed the strict golden: that frozen graph
also includes the fixture's empty make gate target. This candidate was removed.
Neither frozen receipts nor the existing assertion may change under the owner's
repair instructions. V1-4685 / AZU-V0-001/002/8900 integration therefore remains
PARTIAL pending external owner resolution of these incompatible contracts.
The assertion is retained, and the local outcome remains failed. The final
handback records every whole-package result and this exact remaining blocker.

That run also retained sandbox/source-boundary failures: temporary nonrepository
fixtures discover the parent `/home/russell/lanes` repository, and private work
materialization refuses ancestors `/` and `/home` owned by UID 65534. The repair
does not weaken these ownership checks or change their diagnostics.

The first whole affected-package run (221.623 s, failed) failed only the existing
live selection budget test: final-round p95 141.421809 ms and 5.31 calibration
probes exceeded 100 ms and 5.00 probes. That fixture uses only Go and Python
providers, whose inputs this driver repair does not alter. Its external log
SHA-256 is `e4c0c9f5d177805a4ccd550cfde70be1d0cd2fdf8a74fdfecd484d2c39ae503b`.
Retry the whole package without another long lane package run overlapping it;
retain both outcomes. The precise cause of this latency failure is NOT_OBSERVED.
No threshold or assertion is relaxed and no unrelated optimization is added.
The serial whole-package retry passed in 244.096 s; its external log SHA-256
is `deb0689fc6b9704fba87c354f72004763b6b32f2269fa6ead835f9812626328a`.

The source edit invalidated the UC-CHANGE-CONSEQUENCE implementation subject
pin. Its maintained repinner changes only the source pin, receipt revision and
ledger digest (UCV0-004/005). No evidence claim or expectation changes.

The pre-change capture is reused. Native affected selection against
`c5379b4cf039fa8dd2df2aa4ee6e5b7b897cf514` selects every repair-touched package;
the installed-binary staleness is retained separately. Historical full-suite
changed-outcome/mutant receipts, aggregate zero-unknown qualification and external
interoperability remain NOT_PRODUCED. Independent review is orchestrator-owned.
No new commands, authority, publication, ticket write or seal are part of this
repair. Exact verification, artifact digests and CEM limits are retained in the
external Verifier repair 1 handback under `/home/russell/lanes/`.

The final whole CLI run (435.720 s, failed) explicitly reports all three
originally failing tests PASS. It retains 46 top-level failures, including the
incompatible default-driver assertion and the sandbox/host failures above.
Its external JSON-lines log SHA-256 is
`c290b93b7e8cefd8312327ab21f358634712a51a65b0cf9bd0c5809ddaeaf386`.
The languages and conformance whole packages pass, and final spec-requirements,
line-citation and conformance validation checks pass. The local outcome is
failed, not an overall lane qualification.

Rollback reverts this repair and restores the original frozen-digest failures;
it must not update goldens to accept those failures. The wider tickets remain
open pending the specified external contract resolution and safety evidence.
