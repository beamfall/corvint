# PR #99 driver and Pi review repairs

Base: `93935678b568a0bb090db20f8ef441e6b30fa079` (sealed batch head).
Owner request: verify third-party findings A–G, repair supported findings, retain regressions,
and seal one additional constituent without publishing or writing native Tasks state.

All seven findings are supported by the original source. The driver fixes implement the existing
AZU-V0-001/003 soundness boundary, clarified by experimental AZU-V0-9850 through AZU-V0-9853.
Unsupported make word selection/concatenation now retains a computed invocation; exact `sort`
also removes duplicate words before a literal suffix attaches. `env` chdir options retain their
site as computed. Composite flow steps use the workflow fallback, and an unresolved local
`uses` lookup retains its caller, including actions outside `.github/actions`. These are bounded
conservative selections, not claims of arbitrary make evaluation or local-action discovery.

Pi's documented MCP scope test uses the existing host-prerequisite helper (PWV-V0-8300), retaining
its Pi version, successful registration and unchanged global-config assertions when installed.
The capability row now describes the ownership audit's clean-worktree requirement, JSON and
exit codes. Pi workflow documentation describes the existing resolved-HEAD identity checks under
PWV-V0-8104. The package version is synchronized to 0.3.15 under the existing version gate.
Rollback is a revert of this repair constituent; no wire format changes are introduced.

Regression witnesses are `TestDriverMakeFunctionsComputed_AZU_V0_9850`,
`TestDriverMakeSortWords_AZU_V0_9850`, `TestDriverEnvChdirComputed_AZU_V0_9851`,
`TestDriverCompositeFlowSteps_AZU_V0_9852`, and `TestDriverUnresolvedLocalUses_AZU_V0_9853`.
Their focused run passes. The Node prerequisite, scope, capability and workflow checks pass
37 tests and skip the one actual-Pi test because Pi is absent. A restricted PATH also produces
the named Pi skip. Live installed-Pi qualification remains NOT_OBSERVED. Windows vet passes.

The first affected-package run failed the new fixtures' incorrect assumptions about testless Go
packages and non-runnable action units; those fixtures now check tested packages, action frontiers
and runnable caller selections. That run also failed the calibrated latency test under host load
(p95 205.77 ms, 5.17 calibration probes against a 5.00 bound); its failure is retained rather than
treated as a passing performance observation.
A subsequent complete-package run passes the functional checks but again fails latency
(p95 150.25 ms, 5.26 probes). A focused run of the unchanged latency assertion passes with
`GOMAXPROCS=4`; this bounded scheduler result is distinct from the failed unconstrained runs.
The complete affected package then passes with `GOMAXPROCS=4` (243.14 s); all assertions,
including the calibrated latency gate, remain enabled.
The same five regression functions fail against an isolated Git archive of H with only the new
test file copied in (expected exit 1), and pass against the repaired tree. Focused CLI Pi and
ownership-audit tests pass; they do not relabel the complete CLI-package attempt as successful.

The complete CLI-package attempt fails environment-sensitive fixtures: scratch paths below the
mandated TMPDIR inherit `/home/russell/lanes` as an enclosing Git repository, and work qualification
rejects that shared directory's group-writable permissions. No repository-parent permissions or
fixture security checks are changed. A new independent review could not start: both normal and
ephemeral read-only Codex invocations fail app-server initialization on a read-only filesystem.
The supplied third-party review findings were verified here; independent review of the repairs
remains NOT_RUN, not a claimed pass.

Dogfood use includes the requested clean-H start, affected planning, CEM/OCM binding and report
inspection, and the required final checks recorded in the handback. The initial start refused
stale ignored OCM files, which were moved to its prescribed private archive before retrying.
The existing prechange receipt binding names the prior batch's mainline base, so H-specific
prechange receipts remain NOT_OBSERVED; the start wrapper reused that binding. The installed
affected binary also reports itself stale; its plan stays UNKNOWN with the retained frontier.
The default cross-branch requirement collision scan reports an inherited PWV-V0-8302 wording
difference on older local branches; the four new AZU IDs are collision-free against H.

Excluded by scope: task-store mutations and publication (owner prohibition), exhaustive `make gate`
(scoped repair), hosted Pi and Darwin qualification (prerequisites absent), and new performance or
productivity claims. Final command outcomes and the sealed batch identity are retained in
`/home/russell/lanes/b1105a2g.handback.md` and this lane's private TMPDIR command logs.
