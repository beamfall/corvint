# Hosted-runner shell self-test isolation (V1-4641, V1-4642)

Base: `d436dff76de1b8e75c427069ae7f80003ae99bc2`.
Owner scope: fix the two hosted-runner failures and the same assumptions found in sibling shell
fixtures; no product behavior, Makefile recipe, CI or task-store changes.

V1-4641 retains DCG-V0-801: repin default, explicit and environment base plans clear BASE and the
three recursive-make variables MAKEFLAGS, MAKELEVEL and MFLAGS. The sibling dogfood-cache test
also clears those three variables because its ASD-V0-019 default/environment assertions otherwise
inherit the parent's command-line BASE. Other make dry runs were inspected: lane-scripts' windows-vet
cache probe already clears recursive-make state; its admission probes check HEAVY_ADMIT/HEAVY_CLASS,
not BASE; batch-precheck's probes set their tested variables explicitly; spec-requirements-conflict
queries only a scratch makefile's phony targets. No other BASE-default dry-run fix was needed.

V1-4642 retains CRB-V0-018: runtime-environment exports a fixed test identity to child fixtures.
The sibling bind-range test's no-commit merge also sets command-local identity: Git requires it
even before a merge commit is created. The three landing-watch scratch-origin commits in
lane-scripts now set their own identity (ASD-V0-833). The scratch-test audit retained identity
locations for 45 scripts; other commit fixtures use command-local identity, repository config or
exported author/committer values. The archive-injection fixture reuses existing commits.

Evidence: before, `make -s repin-test BASE=0123abc` exited 2 with
`repin_test: FAIL default base: script/repin.sh --base "0123abc"`; after, it and
`make repin-test` exit 0 with `repin_test: ok`. With empty HOME and GIT_CONFIG_NOSYSTEM=1,
runtime-environment exited 128 with `fatal: empty ident name`; after, it exits 0 with
`runtime environment helpers: ok`. The cache test under inherited MAKEFLAGS BASE exits 1 before
(the missing-BASE refusal instead exits 0) and passes afterward with
`DCW-V0-038 dogfood cache selection: PASS`. Exact landing-watch commit commands extracted from
BASE fail 128 under empty HOME; their patched counterparts create three commits with fixed identity.
Focused tools/test-commands-check tests and `make spec-requirements` pass; the latter regenerates
no files. `make traceability-tests-check line-citations-check` passes. No new requirement IDs.

Corvint routes used: change-start query/context/impact receipts, affected selection, CEM/OCM binding
and local outcome. Pre-change receipts are in the worktree Git directory's corvint subdirectory.
Affected advice reports scope UNKNOWN with LANGUAGE_FRONTIER for shell foreign invocations and the
repin read scope; those unknowns are retained, not interpreted as complete coverage. Retrieval,
learning, providers, interoperability and deployed-host qualification are not applicable to these
fixture fixes; frozen ranking evaluations and the exhaustive gate are NOT_RUN for this scoped lane.
Independent review is deferred to the orchestrator by owner instruction. Tokens, comparative cost
and savings are NOT_OBSERVED. No model-backed reviewer is invoked.

Friction: heavy admission reads an unwritable host lock path
`/home/russell/bench/rt/.locks`; the cache regression passes with HEAVY_LOCK_DIR set to
`/tmp/corvint-heavy-1000` and HEAVY_FALLBACK_LOG in lane scratch. Spec generation warns about five
existing specs at 256 obligations; no requirement is added. The wrapper's optional common Git
ledger at `/home/russell/projects/corvint/.git/corvint/dogfood-change-runs-all.tsv` is outside the
lane's writable Git metadata; no common-ledger evidence is claimed. Required binding/check results
and any final environment refusal are retained in the lane handback. Rollback reverts this change's
fixture and evidence edits; no production state migration is needed.
