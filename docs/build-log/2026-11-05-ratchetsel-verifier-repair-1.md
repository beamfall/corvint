# V1-4729: verifier repair 1

Date: 2026-11-05
Base: `9db7a94169821d9b28701eaa88866aee2fec1083`.
Repair input: host verifier at `ed7a993867a9f3a082ae676c75c21a7d41d9a753`.

## Decision and remaining limit

The host reproduced two frozen Core failures: committed-range affected receipts gained
`plan.unknown[0..2].hint` members. The charter forbids changing these goldens and requires
reverting the incompatible behavior. Repositories without the ownership ratchet therefore
retain their original unowned-path receipts. Declaration guidance is now conditional on
the ratchet test file belonging to a unit in the observed graph. The two pre-existing CLI
expectations are restored byte for byte from the base; no golden or freeze assertion changed.
The new regression checks complete serialized plan equality without the ratchet, including
retention of the unowned path. Existing positive guidance assertions remain unchanged.

V1-4729 and AZU-V0-9901 remain PARTIAL: universal committed-path guidance conflicts with
the frozen-output contract and needs owner resolution. Guidance with the ratchet, fixture
closure, AZU-V0-9900 selection and AZU-V0-9902 shared audit classification remain implemented.
The requirement text is retained; the spec's acceptance evidence records this delivery limit.

The other host failure was a stale use-case subject digest for `cmd/corvint/affected.go`.
The existing repin script updates that digest, the receipt revision and the ledger pin only
(UCV0-004/005). No subject behavior, receipt shape or acceptance assertion changes.

## Verification and evidence

Raw outputs remain in `/home/russell/lanes/ratchetsel-evidence/repair1-*`.
Selection used `corvint affected --base 9db7a94169821d9b28701eaa88866aee2fec1083`;
both modified Go packages are selected. The installed binary reports `self-binary-stale`,
so its receipt establishes its selection only, not current-tree plan parity. A subsequent
binary built through `heavy.sh` from the repaired tree also selects both packages, without
the stale-binary warning (`repair1-affected-current.json`); only those two packages run whole.

The focused CLI run passes the complete frozen-profile test, all ownership tests and the
two restored pre-existing expectations (1.973 seconds). The standalone
`make -s use-case-receipts-check` passes with all 18 receipts current.
`script/check-line-citations.sh --repin` reports no moved citation;
`make -s spec-requirements-check line-citations-check` passes.

The complete CLI package runs without a filter and fails in 338.734 seconds, with 45 top-level
failures. Comparing their names with the previous retained whole-package run finds no new
failure; `TestCoreVerbsEmitTheFrozenProfiles` is the sole previous failure no longer present.
Remaining failures include ancestor Git discovery under `/home/russell/lanes`, profiling paths
inside that ancestor repository, private scratch/executable qualification, and the retained
legacy migration-record digest mismatch. These are not repaired outside this round's scope;
the host must rerun the package. The prior round reproduced three representative sandbox
failures at the unchanged base; that evidence is retained without another base rerun.
The complete affected package passes in 116.051 seconds, without a filter, including all
existing tests and `TestOwnershipGuidanceRequiresRatchet_AZU_V0_9901`. The final local outcome
is recorded as failed because the CLI whole-package run did not pass; structural CEM closure
does not override that result or complete the PARTIAL ticket.

| Evidence | SHA-256 |
| --- | --- |
| `repair1-cli-focused.log` | `038aa9d2fd605e4041f0e4405e6e312d32e8674abbc08c70d299598aa847dc4c` |
| `repair1-cli-whole.log` | `410ead33412f7afbdd5db120a47ab54bc2976c214cb6e1fbb815e4e58b3e5754` |
| `repair1-affected-whole.log` | `dfaaf299d4595a81d1c1a9076d527c9ca811c4bd442693fb364cd4f8b65e8fb0` |
| `repair1-affected-current.json` | `dbd421c100cc9f97d53355bc62e9389f0f2d299b2af82a398ae59bc59ffbfbdf` |
| `repair1-use-case-receipts-check.log` | `0a63f24b3db20d5982df9bc85ea43d66a60ca3724d1c5b051a2e59dedd8ad435` |
| `repair1-spec-citations-check.log` | `cf9190b7bbefb7f17896c2262923e8d7ecb49c5973abe59913b9a758110b16b2` |

All Go tests use `script/heavy.sh`, one package per command, with the prescribed private
TMPDIR and without author/committer identity exports or lock overrides. Whole-repository
selection/gates, model-backed review, native task writes, publication and sealing are NOT_RUN
under the lane charter. The orchestrator owns independent review and the deferred host gates.
