# AZU verifier repair 2: preserve frozen planner identity

The host verifier at `762c6d2349971768869dfecf27f28a007d5ae15a` failed
`TestAFUV1024StrictAndCoverageBytesUnchanged`. Its strict receipt differed only
in the graph digest and the exclusion universe identities. The default driver
registration added actual Makefile targets to the graph. The owner explicitly
requires unchanged goldens, assertions and receipt contracts, and directs rollback
of the conflicting change with a PARTIAL disposition.

## Decision and scope

AFU-V1-024 requires byte-identical strict/coverage receipts. Graph identity must
still cover every admitted vertex and edge (AFP-V0-005); masking driver units in
the digest would violate that property. Roll back driver registration at the
default CLI planner's provider boundary, restoring the language manifest's
pre-driver provider list. This also keeps the CLI's test/prove graph consumers
on the same provider list. Driver implementation and explicit library callers
remain experimental.

The opt-in ownership audit continues to build with the experimental driver
claimant (AZU-V0-002/8900), without using a cache. Its empty result establishes
ownership under that experimental registry only. Default plans retain the prior
unowned driver uncertainty and fallback; the audit is not qualification of those
plans. `TestAffectedAuditDriverDoesNotChangeDefaultPlan_AZU_V0_002` checks both
registries through the CLI: the audit claims a committed Makefile, while default
planning reports its unowned uncertainty. The existing AFU-V1-024 byte comparisons
remain unchanged and check both strict and coverage receipts.

V1-4685 ownership implementation remains present, but default driver integration
and dependent V1-4687 caller and V1-4692 aggregate qualification remain PARTIAL.
The external blocker is the owner's frozen receipt boundary: admitting driver
vertices into default planning requires explicit authorization for changed graph
identities. Historical changed-outcome/mutation receipts also remain absent;
AZU-V0 stays NOT_QUALIFIED. No new requirement, receipt field, diagnostic, golden
or existing assertion is changed. Use-case subject and ledger digests are repinned
through their maintained script (UCV0-004/005).

Rollback of this repair restores the prior CLI registration and audit builder,
reintroducing the observed frozen-receipt failure. Pre-change self-use capture is
reused as directed. The affected route selected the repair's packages; other
self-development capabilities are outside this bounded repair. Independent review
belongs to the orchestrator. Publication, Tasks writes and sealing are prohibited.

## Verification evidence

The pre-test native `affected --base
8e2c13aa926fcaf1731fae71b27c41c41c8f09af --full` plan selected the CLI,
use-case conformance and spec-capacity packages. It retained scope UNKNOWN,
83 unknowns and 310 selected units. The original pre-change capture was not
repeated. All raw evidence stays under
`/home/russell/lanes/azu-new-evidence/repair-2/`.

The go1.27.3 toolchain check passed. The whole-package use-case conformance
command, `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m
./conformance/use-cases-v0`, passed in 0.731 s after 308.5 s of semaphore
queueing. Its log SHA-256 is
`f45e3af0bb71b227cb001483d36fbc8f5339d52e4fe2111666a504c92745818f`.
`make spec-requirements` passed, including its existing heavy-wrapped focused
OCM-capacity test; no generated spec index changed. Its log SHA-256 is
`7e5d1c3dad6ccae7c74c1913208bedc628d2a882d0ed3eaae7db065d6b88aa2f`.
The staged line-citation repinner found no citation whose content moved unchanged,
and `make line-citations-check` passed. No heavy semaphore setting was overridden.

The mandatory whole CLI package ran through the same wrapper with
`go test -count=1 -timeout 30m ./cmd/corvint`. It exited 1 in 405.813 s
(380.9 s queue, 433.4 s wrapper run). Its log SHA-256 is
`c5b6f616e662979d7b1330533d1297705627f5469817f9c6c526d623fc3efa99`.
Neither the frozen strict/coverage comparison nor the new audit regression
failed. Existing outside-repository, profiling, host-adapter and work-source
tests instead encounter the mandated temporary root's outer Git worktree
`/home/russell/lanes`, its mode 0775 without sticky, and consequent
`SOURCE_UNQUALIFIED` or unsafe-parent refusals. These reproduce the previous
lane's recorded environment limits. The shared parent, refusal guards, expected
stderr and existing assertions are preserved. Full-package PASS remains
NOT_OBSERVED here and requires the host-side verifier's qualified environment.

Historical AZU replay metrics in earlier entries remain bound to their earlier
binaries. They are not requalified for this registration rollback. No new
historical full-suite/mutation receipt, independent review, Darwin evidence,
repository-wide gate or Windows pre-push gate is produced by this repair.
