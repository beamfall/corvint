# Named-pool native qualification

Frozen source `6955463d56655c20f496eaced24ccd66c56cac46`, Go1.27.1 on darwin/arm64.
Native Tasks binary SHA-256 `159391f5f3b812e610d7b32ba2093a25d0ee61d2c0bbe2036467baf3c47d193c`.
The source remained unchanged through six disposable native queue experiments; all passed.

Two independent CLI processes acquired different members. Exhaustion refused an additional implementer,
a review-reserved member remained usable by that stage, and a missing required pool refused admission.
Queue occupancy preserved file bytes and mtimes. Release quarantined the allocation; exact operator
confirmation enabled reuse. A failing health probe quarantined the first member and reported its
reason/digest; the passing second member was admitted. Configured cleanup was required before explicit
safe confirmation. Every fixture passed receipt audit.

Success, timeout and SIGTERM each observed a parent, child and grandchild and left no live descendants.
The normal-exit case left a residual group for the runner to clean. SIGKILL left PREPARING durable:
recovery refused the matching live runner, replay did not rerun its probe, and orphan recovery moved it
to quarantine. Harness cleanup verified only still-matching owned process identities. Frozen SIGTERM
returned QUIESCENCE_UNPROVED and an INTERRUPTED observation, not malformed input.

| Requirement | Acceptance evidence |
|---|---|
| CAL-V0-028 | Existing omitted-field fixtures; `TestPoolAllocationQuarantine` occupied removal/unrelated policy cases; `TestPoolLargeProjectionCompletionStage` measured2658-byte/11-slot descriptor and three blob afterimages |
| CAL-V0-029 | Native concurrent/distinct/reserved/required-pool cases; `TestPoolAllocationTupleCorrespondence`; `TestPoolReplayReturnsOriginalAllocation` after successor generation |
| CAL-V0-030 | Native release/quarantine/cleanup prerequisite/confirm/reuse; stale-confirm regression; terminal transitions share the reviewed pool post reducer |
| CAL-V0-031 | Native health failure skip/pass, bound observation and replay; `TestPoolHealthSkipsFailedMember` |
| CAL-V0-032 | Actual killed CLI, live-owner recovery refusal, replay and orphan quarantine; journal audits |
| CAL-V0-033 | Actual success/timeout/SIGTERM descendant cleanup; `TestPoolProcessDescendants`; `TestPoolNoHealthConfigReference` regular/missing/wrong/symlink cases |
| CAL-V0-034 | Native byte/mtime purity; `TestPoolPreviewConsumesMembersWithoutProbes` capacity and malformed arguments; pool archive roundtrip and receipt audit |

Independent review closed seven allocation, replay, health and preview findings plus final argument
parity. Initial sandbox process-observation failures and invalid-label test assumptions are retained.
One existing citation moved555→560 with unchanged anchor; documentation checks then passed.

Private evidence is retained under this worktree's Git directory in `corvint/adoption-342/`: source
bundle, fixture archive, original command results, scripts, binary digest, reviews and failed evidence.
The enrolled Tasks/CLI/spec-index tests, Tasks vet and native Corvint completion/check/seal follow this
record against a frozen commit. Their actual results are retained there; no full repository gate is
claimed. GitHub completion/publication remain pending integration and owner authorization.

These are local fixture and trusted process-group observations. Receipts retain their explicit
actorAuthentication/runtimeQualification/liveness NOT_OBSERVED axes. External deployment isolation,
detached hostile descendants and authenticated operator identity are not proved. Safe reuse remains an
operator attestation; cleanup exit zero alone never releases occupancy.
