# Heavycap independent review: closed flag arity table

Date: 2026-11-07. Repair starts at `fb1685b359a75bc8cab9df6b9df410e9201965d1`.
Affected and CEM base: `c3b29cbb54d31c8526d3cce4ad74356b6fb9b943`.
Owner intent: independent review class charter, V1-5007 / ASD-V0-2478.

## Class, decision and rollback

The soundness class is an uncertified flag operand masquerading as a selection.
The starting revision already refuses the three reported debug flag commands and
has a closed syntax check. This repair makes the arity contract explicit in one
`go_test_flag_arity` table shared by syntax validation and operand consumption.
Its only successful results are boolean (no separate operand) and value (exactly
one operand unless inline). Unknown names have no arity and fail closed in both
selection scanners, retaining the wider claim and refusing automatic authority.
Known flags remain subject to the existing exclusion and instrumentation checks.
No diagnostics, exit codes, wire contracts, goldens or existing assertions change.

Amend ASD-V0-2478 in place to require that single closed table, exact arities and
recognized last-wins selection positions. The original debug regression cases
remain: actiongraph, runtime-trace and trace operands cannot impersonate -run.
Add the charter's exact `-debug-futureflag x -run=. ./p` refusal, its inline form,
and a final narrow selection following the unknown flag. Extend existing real
execution assertions with inline tags and boolean values. Valid repeated narrow
selections still execute only TestOther; whole runs still reach TestExisting's
injected source regression. Unknown future flags need no new rejection case in
the implementation. Roll back this table consolidation, added regressions and
requirement amendment together; the earlier debug fixes remain in the parent.

## Verification and limits

The retained affected plan selects tools/test-commands-check; scope UNKNOWN,
365 selected, 526 excluded, 69 LANGUAGE_FRONTIER unknowns. Its installed binary
reports cb403735903b and 30 changed Go source paths, so this is its own selection,
not a fresh-binary equivalence claim. Pre-change capture is reused as instructed.
Only the focused test, spec and citation routes apply to this scanner repair.

The full tools/test-commands-check package passes through heavy.sh in 55.397 s
(wrapper 55.9 s, queue 0.0 s). `make -s spec-requirements-check line-citations-check`
passes: internal/specindex whole 0.969 s (26 s slot wait), required internal/lrfrepo
selection 0.070 s. Citation repin finds no moved content. Raw evidence is outside
the repository under `/home/russell/lanes/heavycap-evidence/`. Bash syntax, pinned Go 1.27.3 and
whitespace checks pass. No existing assertion is weakened or removed. Independent
review is assigned to the orchestrator; no model-backed reviewer is invoked.
The external pre-commit integration for V1-5007 is still absent, so that ticket
remains PARTIAL for the previously recorded external blocker. Other ticket
implementations are unchanged. Broad gates, publication, Tasks writes and sealing
are excluded by the charter. Final CEM binding/checking follows all edits.

## Evidence digests

| Artifact | SHA-256 |
| --- | --- |
| class-affected.json | 4db397cf93e1b0c11b5c12d1a36911cd9966a3c9771999b76e8c1e378d015839 |
| class-affected.stderr | 0ee4ade61bddb0b8c027b78fa8737c82aace500a40af0cd6dfa11e8abed5f83e |
| class-package.log | be4668b6c17a670c3eba34393561c5a34d6b3bb51c7bfa4d2b06578bfe1084ed |
| class-spec-lines.log | e545ae041fa57c605576e8c9fbfe87cb9528a380d5f4314f8a9acff571e50f02 |
| class-repin.log | 85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133 |
