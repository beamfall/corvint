# Context fixture verifier repair 2

Human intent: repair only the repeated frozen context test failures reported by
the host verifier for lane covcoll against
base `39c03198086681fb24f509fbd00b34696d3268c4`.

The first repair demonstrated that inherited Git identity overrides the fixture's
`user.name=t` and `user.email=t@x`. Its test invocation cleared that environment,
but the host verifier still exports the required repository commit identity.
Requiring callers to clear it did not repair the fixture itself. The failing
packet carries commit `05d3142624a106eafdac3a4f644b6f44eacaf3eb` instead of
the frozen `9ab69ef47d196e1a757a3de3319527eb3686ff8b`; its tree is unchanged.

Decision: pin author and committer names and emails to the existing fixture's
`t <t@x>` identity in `gitFixture`'s subprocess environment, beside its existing
pinned dates. This applies TCP-V0-024's frozen default packet check and
TCP-V0-150's commit identity requirement. The parent environment and repository
commit identity remain Russell Lewis <me@russe11.com>. No process-wide environment
mutation is needed in these parallel tests. No assertion, golden, product source,
wire contract, error code, exit code or stderr text changes. No spec amendment is
needed because the repair restores the existing test fixture contract. Rollback
reverts only the helper environment change and restores the inherited-identity
failure; it does not rewrite the golden.

Used the existing context, query, impact, affected-plan and CEM/OCM routes.
The actual context read returned READY and query/impact succeeded; their bytes
are retained as `repair2-{context,query,impact}.json` under
`/home/russell/lanes/covcoll-evidence/`. These are repair-start observations,
not retroactive BASE receipts. Before tests, `repair2-affected.json` selected
`cmd/corvint` directly for the changed test helper and the lane's collector/plugin
packages for their original changes. It remains UNKNOWN: 144 selected units,
82 unknowns (78 LANGUAGE_FRONTIER, one NO_SELECTABLE_TEST, one UNOWNED_DIRTY_PATH,
two UNSELECTED_BINARY_CONSUMER). Its SHA-256 is
`e69e60c11319ac4ba6e654b6bfccd5aaccd53080aee97ce901f8b728d4ae60b7`.

The unchanged `Freeze|GPKV0002|Golden` selection passed with all four Russell Lewis
identity variables exported: package 2.241s, run 20.9s after 120s load admission.
`repair2-freeze.log` SHA-256:
`69b5c40a7e25c69496fbc1b40f66ceb20a61b2dd96da61b37859355297a37758`.
Fresh whole-package tests also passed for `internal/nativelinecov` (0.148s),
`internal/langplugin` (0.277s) and `internal/langplugin/decl` (no test files).
All tests used `script/heavy.sh`, one package per command, and `-timeout 30m`.
`script/heavy.sh --admit-only make windows-vet` passed, including its Go 1.27.3
pin and both Core and interoperability Windows vet steps (43.0s run).
Raw logs remain outside the repository; final command results and CEM disposition
are retained in `/home/russell/lanes/covcoll.handback.md`.

The required whole `cmd/corvint` run was executed and failed (467.373s package,
470.2s run, 120s admission). `repair2-cmd-whole.log` SHA-256:
`2ac89cabec206d30f500579a43bbc7836cc18f3791ce358b373443c66a84f79b`.
Its additional failures report temporary paths under the mandated TMPDIR as
inside `/home/russell/lanes`, and reject that ancestor's actual mode 0775 in
executable/work-scratch qualification. The suite also exposes inherited identity
in other, untouched init/adopt fixtures and a migration-profile digest failure.
The exact causes of all additional failures are not established; the log retains
every assertion. A separate post-run `git -C /home/russell/lanes/covcoll.tmp
rev-parse --show-toplevel` reports no Git repository, so the in-test parent
discovery is not claimed independently reproduced. The targeted frozen selection
passed before this run with the same exported identity. No broader test/helper,
ancestor permission, TMPDIR or frozen-contract change is made for these failures.
The local outcome is failed, and whole-package qualification remains for the host
verifier. This is not represented as a passing whole-package run.

Independent review is reserved to the orchestrator. Native live qualification and
audit-rate measurement remain unavailable for the original documented external
blockers; this repair changes no ticket disposition or collector promotion claim.
