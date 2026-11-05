# AZU small self-review repair

Date: 2026-11-05
Scope: V1-4688 / AZU-V0-005 and V1-4690 / AZU-V0-007; V1-4689 / AZU-V0-006 unchanged.
Base: `8e2c13aa926fcaf1731fae71b27c41c41c8f09af`.
Repair starts at `9b3a0248987ab8ce89074ce08efe421f1718ac9c`.

The supplied review identified three integration defects: hidden Go packages made the runner's
wildcard discovery disagree with the indexed universe, runnable consumer argv disappeared before
session execution, and mixed source/test changes replaced a consumer's build witness with a test
witness. This repair addresses only those defects. No JSON fields, frozen error/stderr contracts,
committed goldens, core-freeze outputs or existing assertions change.

Decision: discovery adds explicitly addressed hidden packages to the existing `go list ./...`
probe and keeps exact universe comparison. Missing packages, extra packages and a failed listing
remain incomplete and launch no Go invocation. `TestAZUV0005RunnerExecutesHiddenPackages` exercises
the runner through discovery, execution and terminal outcome folding, including dot and underscore
packages and the three refusal cases. The Go executable in this regression is a recording fixture.

Decision: carry each consumer's argv in the existing in-memory declared scope, keyed by its
`declared-consumer:FILE` selection ID, and use the existing declared-command adapter. The existing
declaration digest binds that argv; the plan digest also binds the selected run. This preserves
the manifest and receipt schemas. Read the independent full-scope declaration before augmenting it,
so planning failure still has its original full fallback and no later read discards consumer argv.
The local profile executes from the repository root; CI records each command as still required
without scheduling an unsupported declared invocation. `TestAZUV0007CLIExecutesDeclaredConsumers`
runs two real bash commands through the public CLI planning/execution path, checks literal
arguments and root working directory, and checks CI's required commands. Go is a recording fixture;
this test is not browser or external interoperability qualification.

Decision: retain build witnesses separately before own-test and reader selection changes. Consumer
selection extends that snapshot, while ordinary tests keep their final selection witnesses.
`TestAZUV0007RunnableConsumer` now asserts the original library dirty path and the entire
library-to-command-to-consumer chain for both dependency-only and mixed dependency/test changes.
Existing test-only and unrelated exclusions remain asserted.

The first new CLI regression failed because the old post-plan declaration read replaced the
carried commands. Moving the declaration read repaired that defect; no assertion was weakened.
Focused witness, hidden execution and CLI consumer tests subsequently passed. Raw logs are outside
the repository at `/home/russell/lanes/azu-small-evidence/self-review/`.

Self-development routes: reused the original pre-change capture; used installed and current-tree
`affected` selection before whole-package verification; used CEM rebinding and native reviewer
reports for structural evidence. The current plan selects all four touched Go packages and retains
three language frontiers. Mutation, browser qualification, external integrations and additional
context capture are deferred as outside these three repairs. Independent review belongs to the
orchestrator. Token cost and performance savings remain NOT_OBSERVED.

Rollback: revert this repair's source/spec/test commit and rebind evidence. That restores the
reviewed defects and is not a successful qualification. No new execution authority is introduced:
commands still come only from the project-owned consumer declaration.

## Verification and retained limits

All Go tests ran through `script/heavy.sh`, one package per command, with
`TMPDIR=/home/russell/lanes/azu-small.tmp` (mode 700), Go 1.27.3 and the configured shared
semaphore. No lock-directory override was used. Every touched package ran its existing tests
whole, without a `-run` filter. Comments clarifying the discovery universe were updated after
starting the whole runs; no executable Go statement changed after those runs started.

| Check | Result |
|---|---|
| Whole `internal/sessiontest` | PASS, 4.079s |
| Whole `internal/liveverify/affected/golang` | PASS, 9.404s |
| Whole `internal/liveverify/affected` | PASS, 136.117s |
| Whole `cmd/corvint` | FAIL, 278.276s; exactly the same 49 top-level failure names as the original lane's final run, no new failure names |
| Focused CLI consumer, legacy consumer check, compact/by-path, flow golden and frozen core profile/envelope groups | PASS, 2.899s |
| Current-tree `corvint test --base BASE --dry-run` | Exit 0, NARROWED / PLANNED, no incomplete entries; both web-flow consumer argv retained |
| Read-scope, unbounded-reader, requirement-definition, traceability and line-citation checks | PASS; one pre-existing planned traceability item retained |
| Spec requirements regeneration check, formatting and whitespace checks | PASS; generated metadata unchanged |

The whole CLI failure is not dismissed as entirely environmental. It retains scratch qualification
failures under the mandated parent `/home/russell/lanes` (mode 775), plus the prior testless-package
expectation conflicts, including `TestAffectedUntestedGoPackageIsUnknownScope`,
`TestAffectedUntestedPackageGetsCompileCheck_AFP_V0_468` and
`TestUseCaseHostileChangeConsequence`. Those failures already appear in the original final log;
this repair does not change AZU-V0-006 or its frozen assertions. Qualification remains incomplete
and the local outcome is `failed`. The orchestrator must resolve or disposition these retained
failures; the new passing integration tests do not substitute for a passing whole CLI package.

`script/check-line-citations.sh --repin` moved one unchanged citation in
`docs/specs/falsifiable-packet-v0.md`; the second invocation and terminal invocation converged.
`make line-citations-check` passed. The original line content and hash were inspected.

The independent model-backed reviewer is NOT_RUN by charter; the orchestrator owns that review.
Full `make gate`, direct selected-test execution, browser execution, Darwin qualification, task
writes, publication, Windows vet before push and sealing are NOT_RUN under the lane restrictions.
The full selected web-flow driver starts background processes, so only its planned argv and two
small real declared commands are qualified here. Native guarding-test execution remains
NOT_PRODUCED because it would bypass the required heavy wrapper. Reused pre-change chronology,
OCM linkage limits and historical unknowns remain as recorded in the lane handback.

Raw evidence digests (SHA-256; all files remain in the external self-review evidence directory):

| File | Bytes | SHA-256 |
|---|---:|---|
| `test-sessiontest-whole.log` | 593 | `01c9c303e1d427a5892cfe5d8fb70b072af52eaa0394212cce3a0474d1591520` |
| `test-golang-whole.log` | 610 | `8895c206c51e97aa0fa242ef83d54702993704a88e0e7b7e6192e8f8664560f3` |
| `test-affected-whole.log` | 799 | `db7bf764739fa6b6f6197d7b23d44fc43a440a5380268d758db1ba4de4b06bab` |
| `test-cli-whole.log` | 68525 | `855730bdb3f37f10b2d927e5b16bc660258bae06ca0848c7fda31743a447a613` |
| `test-cli-focused-final.log` | 765 | `2812c38acd0025e8886d788e9e373ea89613550fba8a935895c98479e86d5805` |
| `scoped-gates.log` | 435 | `7f56814c4adb727fc13d10e85d86b3737d2a154bc5d18b3b28bb0e018c1292db` |
| `test-plan-current.json` | 46185 | `cd194e2242f17ce901efc03a3e03d7770d92d2103c9aa958a4633692fc75c786` |
| `cli-failure-comparison.json` | 6067 | `8f8f6b6f0ec5d399725968203a5e8a88c9f1a5d0a91345dcab6617b733adb704` |
