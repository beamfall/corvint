<!-- SPDX-License-Identifier: Apache-2.0 -->
# mixbugs fix round 1

Scope: the five review findings against `c9bc03e85c2868affc1562d360deda18b97f8f36`,
with lane base `d436dff76de1b8e75c427069ae7f80003ae99bc2`. No new ticket scope.

## Decisions and requirement bindings

- V1-3710 / TCP-V0-8200: take the requested conservative rollback. Default additive
  diagnostics changed the reason parsed by `rowAction`; changing the numeric score also
  violated the default recipe golden. Preserve default scores/reasons and both base goldens,
  retain selected lookup source-before-test ordering, and defer default component reporting.
  V1-3710 is PARTIAL for that explicit limitation. The existing opt-in additive experiment
  stays experimental under CRK-V0-005/008; this repair does not promote it or change its wire.
- V1-3758 / CEM-V1-8201, CEM-V1-963: restore the original lineage register byte for byte.
  Archive locators survive source transfer; private tags cannot be prerequisites. Check
  reachability when the archive commit and base are present. Missing archive/base/ref history,
  historical `none` locators and unavailable default refs report `NOT_RUN` with a reason.
  Keep mismatched/stale/unknown labels, invalid locators and unrelated available tips as
  failures. No absent object or disclosed history proves CEM validity.
- V1-0580 / LCP-V0-8209: regenerate the affected Core golden with
  `CORVINT_UPDATE_GOLDEN=1`; its only delta is
  `policy.enrollmentEvidence: "NOT_OBSERVED"`. The inactive-status test shares that file.
- V1-1323 / SRG-V0-8202: an exact separator counts only inside an opened conflict block
  in the same file. Preserve opener/closer diagnostics; accept a setext heading outside
  the block, including after its closer. Fixture exception rules stay in force.
- V1-2651 / AMAP-V0-8206: preserve hash-router route fragments across redirect chains,
  with declared base paths, and use that same prefix for redirect freshness. Test `#` and
  `#!`, ordinary and explicitly fragment-prefixed destinations, and chained redirects.

## Evidence

Scratch receipts, exact commands/return codes and logs:
`/home/russell/lanes/mixbugs.tmp/fix-round-1/`. Final handback:
`/home/russell/lanes/mixbugs.handback.md`.

The context broad pattern `TestContext|TestTaskContext|TestDefinition|TestAnalyzerSchemaOutput`
passes, including `TestContextRRFMembership_TCPV02222`,
`TestDefinitionRowKeepsTheStrongestMatchPerPath_TCPV0162` and the default recipe checks.
The Core freeze and local-completion families pass with fixture identities isolated from
seal identity. The first run failed three unrelated init/adopt goldens because exported
seal identity overrides those fixtures' Git config; their goldens were not changed.

The marker family, local-completion LCP/export/import family, and Tasks CLI drain/reap
families pass. The appmap broad family passed; the final targeted family also checks hash
freshness alongside existing mount/ranking controls. An overlay using the pre-fix
`router_redirect.go` makes `TestAMAPV08206HashRedirect` fail `no-matching-screen`;
this expected negative is retained as `hash-negative.log` (rc=1).

The lineage fixture test clones without tags from a repository carrying a private lineage
pin, and passes with an explicit unavailable-archive `NOT_RUN`. Available unrelated archive
and local tips still fail. The real corpus passes 355 maps/17 labelled while disclosing all
17 unavailable historical checks; archive verification is not claimed. No lineage ref is
created, deleted or published in this worktree by this repair.

Byte comparisons confirm the context recipe golden, Core context-task golden and lineage
register equal the lane base. A structural JSON comparison proves the enrollment golden
changes only the intended field. The affected plan retains UNKNOWN scope: the initial
receipt selected 217 units, excluded 434 and retained 83 unknowns (78 LANGUAGE_FRONTIER,
3 UNOWNED_DIRTY_PATH, 2 UNSELECTED_BINARY_CONSUMER); this is focused evidence, not a full gate.

## Limits and closeout

The original external limitations stay open: V1-0486 lacks actual editor clients/original
receipts; V1-4637/V1-4638 await the next hosted flake report. Native ticket completion,
publication and seal are prohibited in this lane. Independent review belongs to the
orchestrator. No model-backed reviewer is run here.

No full gate, full store suite, fresh frozen retrieval evaluation, or cross-platform live
qualification is claimed. This repair restores default frozen fields; it does not rebind
old benchmark measurements to this new source. The current bound CEM, reviewer report and
dogfood-check result are retained at closeout; they prove structural binding, not semantic
adequacy, external validation or completed native tickets. Stop at dogfood-check; do not seal.
