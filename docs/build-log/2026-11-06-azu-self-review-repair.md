<!-- SPDX-License-Identifier: Apache-2.0 -->
# Constructed callers and replay completion repair

The self-review repair request identifies three defects in the AZU lane:
a Go raw-string path join bypasses foreign-caller completeness, a lane-added
ownership test contradicts the accepted registered driver, and a replay report
can accept its first merge before later merges fail. This repair implements
AZU-V0-002, AZU-V0-004, AZU-V0-009, AZU-V0-8901 and AZU-V0-8903 within the
existing contracts. No golden, wire shape, field name, diagnostic or exit code
changes.

Decision: directory joins recognize Go backtick literals through the existing
conservative matcher. They retain foreign-invocation uncertainty for undeclared
callers; they do not infer new caller edges. The end-to-end
`TestRawStringJoinRetainsUnknown_AZU_V0_8901` builds the shell/Go graph, requires
UNKNOWN for an undeclared raw-string join, and retains the declared caller's
selection. The existing completeness table also verifies that listing such a
caller permits its declaration.

The owner's current repair instruction resolves the incompatible assertion
recorded in `2026-11-06-azu-merge-digest-repair.md`. The lane-added
`TestAffectedAuditDriverDoesNotChangeDefaultPlan_AZU_V0_002` now requires
BOUNDED scope, no unknowns, and `driver:make:Makefile#check` selection for its
Makefile. This reconciles it with the accepted registered-driver behavior;
AFU-V1-024 and core frozen-receipt assertions remain unchanged. The earlier
assertion-conflict blocker is superseded by this scoped repair.

Decision: intermediate replay reports stay unaccepted. Only successful
processing of every requested revision can accept the aggregate. On a planning,
receipt or revision error, an existing per-merge safety row retains
`NOT_PRODUCED` and an explicit incomplete/error detail. The original returned
error remains unchanged. If even that report update fails, the prior report is
already unaccepted. The revision loop is extracted only to exercise real local
Git merges with a fixture planner without replacing the contractual frozen SHA
prefix. `TestReplayIncompleteRemainsUnaccepted_AZU_V0_8903` covers a qualifying
first merge followed by a missing receipt, a planning failure, and successful
completion of both merges. Fixture observations are not historical safety
qualification.

Focused whole-package evidence under the configured heavy semaphore:

- Shell package PASS, 0.486 s; log SHA-256
  `fd4355828ac0f275c34445a2141828a4c9d30c613502bf72f7d46586e9f26d9d`.
- Replay tool package PASS, 0.201 s; log SHA-256
  `cc32247931c93d50243ad372aa98cf6c1c9666c92af13a59e0a2039fd669e44a`.
- CLI whole package FAIL, 308.115 s, 45 top-level test failures. The reconciled
  Makefile assertion and all three frozen receipt test groups PASS. Remaining
  failures include private executable materialization under foreign-owned
  sandbox ancestors and outside-root fixture refusals; no checks are loosened.
  Log SHA-256
  `1b4ec1df9dce3aadfe0e3e1184eaab897c0e71c0a6ef307861f44263e32903c3`.
- Spec requirements and line citations PASS; log SHA-256
  `7e40de974f6a7322be4d991c6310812e105a5cafafe806a300f83d8bb68a33ea`.

Native affected selection against the supplied base selects all three touched
packages. The installed binary's stale-source warning is retained, alongside
selection from the prior repair's source binary. Raw logs remain outside Git
under `/home/russell/lanes/azu-new-evidence/self-review-repair/`; the external
handback records CLI whole-package results and final structural verification.

The pre-change capture is reused. CEM rebind/check follows final edits;
independent review belongs to the orchestrator. No seal, publication or native
ticket write is authorized. Historical full-suite/mutation receipts, aggregate
zero-unknown qualification, Darwin and external interoperability remain
NOT_PRODUCED. The self-development routes used are affected selection, focused
package verification and CEM review; new prechange reads are excluded by the
repair instruction, and no unrelated integration route applies.

Rollback reverts only this repair's source, tests and documentation. It restores
the three reported defects and must not change frozen receipts to accept them.
