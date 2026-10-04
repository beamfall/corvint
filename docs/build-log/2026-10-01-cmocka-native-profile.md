# Dedicated CMocka native profile and explicit argument-free TEST phases

Source integration of the reviewed scratch proposal
`cmocka-profile-proposal-r1` (patch sha256
`1e9061155cead78d55d4cb520810605e5e8e5afc7bcd3d4f95d307973d2d4605`, authored
against `a5326ba649cab3d0aed209d272960a19d8bc7ca3`) onto
`07ea04b37708ea1e55ec13c62919cbbc4e81d0ba`. All twelve proposal paths had their
recorded original bytes on this base. The eight source/test files and the
provenance fixture carry the proposed bytes; the test-runner spec, native README
and this entry were edited further for the root decision, review observations
and the runner counts. `REQUIREMENTS.tsv` was regenerated. CEM binding and native
Tasks completion remain separate closeout steps.

The owner scope requires dedicated C-framework evidence beyond a CTest
application driver. CMocka and Unity form a proposed bounded first inventory;
this slice addresses only CMocka 2.0.2. Independent Gate A required per-test
teardown evidence, nonempty expected inventory, and preservation of shared
Normalize's UNKNOWN states for incomplete reports.

Acquired source showed that XML-only mode can hide a group teardown error while
returning 0. The profile therefore requires original STANDARD and XML witnesses.
Actual static native captures reproduced this behavior. Unknown or contradictory
output abstains instead of inferring ordinary assertion cause.

## Shared executor contract change

The first frozen nine-case CLI attempt was refused before any native phase:
the shared executor rejected an empty argv. Those nine receipts remain unchanged
under the hashes recorded in the r1 manifest. The repair is a shared executor
contract change that applies to every runner profile, not a CMocka-only rule:
empty argv is admitted only for an explicit phase with Kind `TEST` and Tool
exactly `primary` (`TRE-V0-015`). Implicit invocation and empty argv on `BUILD`,
`DISCOVER`, `DECODE`, auxiliary-tool and implicit-primary-name phases keep
refusing, each with a retained negative case in
`TestExecuteExplicitPrimaryTestWithoutArguments`. Only `cmocka-xml` emits empty
argv; no existing profile's fixed plan changed, and `TestHistoricalPlanByteIdentity`
still passes.

The root decision record `cmocka-zero-argv-root-decision/ROOT-DECISION.json`
(sha256 `e4de2129c7498d62fb8b292dc2c20b532d2d00a5c88d23382c1ae23a9204e161`)
accepted this widening with five conditions; it binds the addendum
`6ffab3ecf60feb458a63daa50a7075b4a940e7f4e2af747a009c64f789c7710a` and the
independent review `1fed70e34aa95c1581270009ba4ae999c706672f46d4fcea4a81f3a9ce6fcaaf`
(PASS_BOUNDED, no confirmed defects). This entry answers review observation O1
by citing that record and O5 by correcting the README and log spacing. The
Ginkgo profile also edits `native.go`; CMocka lands first on this branch, so the
Ginkgo integration must rebase onto it, re-pin and rerun its focused tests.

Rollback has two independent parts: removing the additive `cmocka-xml`
profile, and restoring the executor's argv lower bound (`len(p.Argv) == 0`
refuses every phase). Restoring only the lower bound makes CMocka plans refuse
with `argv bound` and affects no other profile. Historical reports and receipts
are not rewritten.

## Provenance and verification

The official archive and immutable commit source matched all 150 release files.
Exact licenses and signatures are retained; publisher signature authenticity was
not observed and was explicitly accepted only for this bounded trusted-local
experiment. No global dependency installation, network build, default promotion,
all-C/JNI qualification or full SDK closure is claimed.

On the integration base, the focused runner and CLI packages and vet passed on
Go 1.27.1, and the opt-in `TestCMockaNativeReceiptReadback` re-parsed the nine
retained r1 receipts with matching direct, registry and receipt states. That is a
readback of retained native bytes, not fresh execution. The root decision's
repeat qualification on the integration base remains NOT_RUN: the nine retained
test executables were not re-executed in this slice (independent review notes they
are still present and hash-pinned, so a rerun is feasible; it was not performed). `corvint affected` retained 23
unknowns; the repository-wide gate was not run.
