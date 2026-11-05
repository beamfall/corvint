# Ranking verifier repair 1: fixture identity and trusted CI classification

Owner request: repair only the two host-verifier failures at
`7f5530ab72ed9810270c9447142b6543335b9e4e` against immutable base
`39c03198086681fb24f509fbd00b34696d3268c4`.

The context golden failures were fixture identity contamination, not ranking
output drift. Git's exported author/committer variables override `-c user.*`.
Pin both identities only in `evidenceSummaryRepository`, alongside dates, using the
original fixture identity. The exact existing golden assertion now also runs
under an explicitly different operator identity (serially, as `t.Setenv` requires).
The golden is unchanged: SHA-256
`5882cd3d50f885e098136d5bab84ae77278896d5ddf523341acd9613368dc866`
at BASE and at delivery. This exercises TCP-V0-024 and TCP-V0-150 without
changing production code, packet members, ranking, errors or frozen outputs.

Design decision for the batch precheck's pre-existing V1-4525 failure:
CIN-V0-2304 deliberately uses CI pull-request inputs and a verifier built from
trusted main. A local mirror cannot supply that trust. Under ASD-V0-036,
classify only its complete normalized literal run block as CI-only NOT_RUN,
with an explicit row in the step list, print-only list and normal runner.
Match all commands, never a step name, substring or broad unknown-command
exception. Keep all shared make/go-run gates and the CI workflow unchanged.
Changed/added/missing commands and unsafe step directives still fail closed;
the complete existing fixture suite includes four new negative controls.
Folded YAML blocks do not qualify for the exception. The owning ASD-V0-036
clause and traceability row describe this bounded exception; spec requirements
were regenerated. TCP-V0-024's acceptance evidence documents fixture isolation.
No new requirement IDs or governing specs are introduced.

Rollback: revert this repair's source/spec changes together. That restores the
reported operator-identity golden instability and rejects the trusted CI block
locally again; it does not change trusted batch enforcement in CI.

Dogfood context/query/impact were used before edits and retained under
`/home/russell/lanes/ranking-evidence/repair1-*`. Context was READY and returned
both owning specs; impact returned ok. Existing original-lane prechange receipts
remain bound, not represented as fresh immutable-base observations. Affected
selection after repair was UNKNOWN, 149 selected units and 82 unknowns. Whole
packages were selected from that plan; no exhaustive coverage is inferred.
Optional LSP, live provider, mutation and frozen ranking requalification are not
applicable: no production ranking/provider code changed in this repair.
Independent review belongs to the orchestrator, not this lane.

The first shell run was invalidated by editing its source while its runner was
executing: it reported an EOF parse error after the parallel stub steps. The
failed log is retained as `repair1-batch.log`; only the subsequent complete run
with stable source is eligible verification. No assertion or golden was changed
to suppress a failure. Raw evidence stays outside the repository.

Repair checks: the final `Freeze|GPKV0002|Golden` CLI selector passes (1.321s),
the complete stable-source batch-precheck suite passes, and whole
`internal/contextindex` (132.843s) and `internal/jstestprovider` (9.770s) pass.
Spec generation, line citations, Bash 3.2 compatibility (256 scripts), the marker
and retention suites pass. Whole `cmd/corvint` execution was required and run;
local qualification fails under the mandated scratch environment, not recorded
as a package pass. The observed first whole run took 386.737s with 47 top-level
failures. Its private executable-ancestor checks reject mode-775
`/home/russell/lanes`; profiles reject scratch inside that parent Git worktree;
outside-Git fixtures instead discover its repository. Unchanged
`newActivationFixture` also inherits operator author/committer variables, moving
its frozen core inventory hashes, and the migration-ratchet source-root fixture
fails earlier at candidate digest validation. These source/test files are
unchanged from BASE; baseline parity was not rerun, so not every failure is
claimed to share one proven environmental cause. No assertion, golden or product
refusal is loosened. Host-side private scratch qualification outside Git remains
an external blocker. The final whole-package rerun and exact commands/results
are retained in `repair1-cmd-whole-final.log` and the required handback.

The explicit dogfood outcome is failed to preserve this verification limit;
a structurally accepted CEM and dogfood-check do not certify a passing package.
No fresh ranking trials, exhaustive repository gate, protected CI change, push,
PR, native Tasks write, seal or model-backed review is part of this repair.
CEM/OCM report inspection and the post-commit check are recorded in the handback.

Evidence log SHA-256 digests (the files stay outside the repository):

- Complete stable-source shell suite:
  `c4e5fc52282e3354e95ac77db6ca84d16eff365ea6e0bf6bb18111c7ab1a94f5`.
- Final frozen CLI selector:
  `f40e15659b7dba537c03a8a620a8907d5685ad17705b1f62293be53c54c71aa8`.
- Whole contextindex package:
  `0f53dbc6a146bf097288292031df688296971f1130280dca59688df081643b81`.
- Whole jstestprovider package:
  `f309841ee27698999b2065714ea7dec6c90649647a4a8a638647fa6d956e6fe9`.

Final-source whole CLI rerun: failed, 391.763s package time, 47 top-level
failures. Final failure-log SHA-256:
`553ff9b47795bc6a5ddad28ec25950d98a91bb61a2f8037cbe63854d4d271104`.
