# Lane ctx-1021c: a clean worktree's affected plan is bounded; a defined requirement ID is a specific term (V1-2126, V1-2235)

Date: 2026-10-21. Lane base: `0106d06cd` (`origin/main`). Requirements: `AFP-V0-004` and
`AFP-V0-009` (`docs/specs/affected-plan-v0.md`) and `TCP-V0-016`
(`docs/specs/task-context-packet-v0.md`) are amended, with no new IDs. The amendments are accepted
under the owner's standing lane auto-accept (2026-10-11).

## Problem

- V1-2126: on a clean worktree, `corvint affected --base <HEAD>` gave `scope=UNKNOWN`. It listed
  23 `LANGUAGE_FRONTIER` unknowns and a `NO_ADVISORY_GO_COMMAND` gap, even though nothing had
  changed. Under the V1-0289 participation rule every plugin took part when "no path is dirty", so
  every plugin-wide frontier reason was named. The advice then reported the empty Go selection as
  a gap.
- V1-2235: a task that names only a requirement ID (for example `DMX-V0-001`) already got the
  defining spec as a `spec-mentioned` row (TCP-V0-009). Its verdict was still
  `no-specific-terms`, because `specificTerms` dropped the ID: it is not name-shaped, and its
  postings did not reach the spec.

## Decisions

- An empty dirty set has no edge a frontier could hide, so no plugin takes part. The plan is
  `BOUNDED` with empty `unknown`, `dirty` and `selected`. An `EMPTY_SELECTION` over an empty dirty
  set adds no Go advice line, which follows the V1-2076 no-Go-units precedent. The mandatory checks
  declared in `AGENTS.md` and the `Makefile` still stand (AFP-V0-009 authority). An unowned dirty
  path still makes every plugin take part.
- A requirement ID that a tracked spec defines becomes a specific term. Its sources are the
  defining specs and its tracked count is their number. An ID that no spec defines adds no term,
  so it can neither invent support nor withhold rows. The `context` default wire golden changes
  only in `coverage.answerability`: task `DMX-V0-001` is now `supported` by `demux-v0.md`. This is
  a value change within the existing enum, and `context` is not byte-frozen (CCF-V1).
- The analyzer schema is bumped once with `script/analyzer-schema.sh bump`, because
  `taskcontext.go` is an analyzer input (IDX-SNAP-V0-017/060).

## Evidence

- `TestAffectedEmptyDiffIsABoundedEmptyPlan_V1_2126` (cmd/corvint) and the updated nil-dirty
  case of `TestFrontierBearsOnlyOnThePlansItTakesPartIn_V1_0289` both fail without the fix and
  pass with it.
- `TestTaskContextCountsADefinedRequirementIDAsSpecificTerm_V1_2235` (internal/contextindex)
  fails without the fix and passes with it.
- Live check on the lane clone. Before the fix: `UNKNOWN`, 23 frontiers plus
  `NO_ADVISORY_GO_COMMAND`. After the fix: `BOUNDED`, no unknown.

## Not delivered in this lane

- V1-2127 (`context` reports a tree as `revision`): reproduced, but skipped because it needs an
  owner decision. The packet `revision` is a tree OID by contract.
  `experimental-source-views-v0.md` says so, and source_handoff, pi_tools, the trace codec,
  attest, the MCP bridge, slotlearn and host_adapter_compaction all read it as a tree. Changing it
  would break the wire format across those consumers. One option is an additive `commit` member.
- V1-1757 (ancestry checks trust grafts and shallow history): confirmed as a suspect. Ancestry
  reads with no graft or shallow guard remain in internal/dogfoodflow (check.go, batch.go,
  migration.go, finishlane.go, flow.go), internal/extevidence/freshness.go,
  internal/postmergeconnector/git.go, cmd/corvint/runs_baseline.go,
  internal/breakagemap/compile.go and internal/dashboard/repository/trace.go. The audit is too
  large for this lane, so the ticket stays open.
- V1-1478 (JS/TS registry reader): shape (1) is confirmed by reading. `readTemplate` lexes a
  template with a `${...}` substitution as one `webString` token, so `registryCallSites` never
  sees an accessor call inside it. Shapes (2) and (3) were not examined. The fix may change
  accepted NGI-V0-008 behaviour, which needs an owner decision.
- V1-0313 (non-UTF-8 history path): skipped. It needs a GPK amendment that keeps oracle parity,
  or an owner decision.

## NOT_RUN

- `make gate` was not run, per lane policy. Only focused packages were run.
- `TestCEMEndToEndAndGoldenEnvelope` fails locally only on the `goVersion` member: the host has
  go1.27.3 and the golden pins go1.27.2. The failure comes from the environment and also happens
  without this change.

## Rollback

Revert the lane commits. `participants` then returns every plugin for an empty dirty set again,
the advice gap comes back, `specificTerms` drops requirement IDs again, and the golden and the
analyzer pins return to their earlier values.
