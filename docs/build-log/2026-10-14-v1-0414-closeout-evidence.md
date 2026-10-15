# 2026-10-14 V1-0414 closeout: hidden Unicode and self-modified governing authority

Ticket V1-0414 (security: hidden or bidirectional Unicode in governing documents and governing
files changed in the same diff) was delivered by PR #467 (merge `93fda492e`) but never closed. This
entry audits each acceptance criterion at BASE `495eda08cf3dc4d8122b169a4cc16d9c4e599b19` with the
original evidence, and closes the one residual gap found: `cem cite` still wrote a citation of a
governing file that the same change modifies. Scope: 1.0 scope per the V1-0020 dependency.

## Acceptance audit

1. **Hidden-Unicode governing row is warned and downgraded, with fixtures: met.**
   `internal/contextindex/authority_screen.go` screens every reserved row (TCP-V0-055) for the
   zero-width, bidi-control and tag (U+E0000..U+E007F) sets, attaches a `hidden-unicode` warning
   with classes, count and first site, and downgrades the row to `downgraded-authority`
   (TCP-V0-056). Coverage per class: `TestHiddenUnicodeClassesAreExactlyTheSpecSets` walks every
   code point of all three sets, including the whole tag block, and the nearest outsiders;
   `TestScreenHiddenUnicodeReportsCountClassesAndFirstSite` reports all three classes from one
   text; `TestTaskContextDowngradesAGoverningFileHidingUnicode` downgrades a governing `AGENTS.md`
   holding zero-width and bidi code points end to end; `TestTaskContextDowngradesASpecMentionedRowHidingUnicode`
   downgrades a reserved row holding tag characters. The downgrade is observable in `corvint context`
   output through the core-freeze golden `cmd/corvint/testdata/core-freeze/context-downgraded-governing-row.json`
   (case `context downgraded governing row` of `TestCoreVerbsEmitTheFrozenProfiles`: bidi controls,
   `governance_refused` names `AGENTS.md` with `hidden-unicode`). No change.
2. **Self-modified governing file: packet and `cem status` met; "not cited" residual, fixed here.**
   The packet downgrades a governing row the working tree modifies with `self-modified-authority`
   (TCP-V0-057, `TestTaskContextReportsAGoverningFileTheWorkingTreeModifies`), and `cem status`
   reports `selfModifiedAuthority` for a committed change (CEM-CB-026,
   `TestStatusReportsSelfModifiedAuthority`). But CEM-CB-026 said refusing such citations was not
   accepted, and `cem cite` (`internal/cem/workflow/commands.go`, `Cite`, the only producer path
   that builds a hunk basis through `attachEvidence`) wrote a citation of a changed `AGENTS.md`
   whenever its span was stable: the existing status test itself cited `AGENTS.md` for both hunks
   of a change that edits it. Fix: `CEM-V1-114` (new, `cem-stable-v1.md`) makes `cem cite` refuse
   such an evidence path with the new producer-only code `cite-self-modified-authority`, before the
   span check and without writing the map. Verifier, status and wire are unchanged.
3. **Threat recorded with the two external references: met.** `docs/specs/task-context-packet-v0.md`
   section "Authority screen for reserved rows" cites pillar.security (Rules File Backdoor) and
   backslash.security (AGENTS.md injection in Codex CLI). No change; the new `CEM-V1-114` section
   points back to it.

## Decisions

- Refuse at write time, not at verification: a verifier refusal would change the verdict of maps
  produced by other routes and the frozen digest. Status keeps reporting a hand-written citation.
- Same governing rule as CEM-CB-026 and TCP-V0-008 (`GoverningInstructionPath`), same changed-path
  test (a hunk path equal to the evidence path), so the three surfaces agree.
- `CEM-V1-114` is accepted under the owner's standing auto-accept of lane requirements (2026-10-11);
  CEM-CB-026's last sentence now points to it instead of calling refusal unaccepted.

## Failure modes and limits

- A change that edits a governing file and cites its unchanged lines now fails at `cem cite`; cite
  the spec or decision the edit implements, or keep the hunk unknown with a reason.
- Not screened: a renamed governing file's old path, governing names `documentKind` does not
  classify, a hand-written map (reported by status only), and non-governing specs the change edits.
- Follow-up outside this lane's scope: `internal/dogfoodflow` has no `fix:` hint for
  `cem-cite:cite-self-modified-authority` (the runner falls back to no hint), and
  `internal/observations` does not admit the code in the self-observation ledger.

## Acceptance evidence

All commands ran with `GOTOOLCHAIN=local` (go1.27.2) on a shared host (load average 40 to 50).

- AC1: `go test -count=1 -run 'HiddenUnicode|Downgrad' ./internal/contextindex` passed, and
  `go test -count=1 -run TestCoreVerbsEmitTheFrozenProfiles ./cmd/corvint` passed (including
  `context_downgraded_governing_row`).
- AC2:
  - `go test -count=1 -run 'SelfModified|Cite' ./internal/cem/workflow` passed.
  - The same filter under `-race` passed.
  - `TestCiteRefusesSelfModifiedGoverningAuthority` covers four cases:
    - Every hunk citing the changed `AGENTS.md` is refused with `cite-self-modified-authority`.
    - The map bytes are unchanged after a refusal.
    - An ordinary file is still admitted.
    - An unchanged governing `CLAUDE.md` is still admitted.
  - Mutation check: with the `refuseSelfModifiedAuthority` call removed, the test fails
    (`got <nil>, want cite-self-modified-authority`).
  - `TestStatusReportsSelfModifiedAuthority` still reports a hand-written citation.
  - `TestTaskContextReportsAGoverningFileTheWorkingTreeModifies` passed.
- AC3: read-back of `docs/specs/task-context-packet-v0.md` ("Authority screen for reserved rows").
- `go vet ./internal/cem/cemcode ./internal/cem/workflow` was clean. `gofmt -l cmd internal tools
  .github/cishards .github/testconfine` printed nothing. The 12 CI doc gates passed.
- A focused run of the `internal/cem/...` and dogfood packages under load had timing-only failures
  in untouched tests. Each passed when rerun once alone:
  - gitrun: `TestOperationBudgetCountsPhysicalNestedSpawns`, `TestSessionServesRecordsAndReapsOnEveryExit`, `TestRunStreamContainsDescendants`
  - verify: `TestStableS0EPublicCases`
  - workflow: `TestDiscriminateRecordsWitnessAndReportDowngrades`
  - dogfoodflow: `TestAggregateOwnedProcessRetiresDescendants`
- Codex read-only review, one round: CONCERNS, with two findings. Both are fixed:
  - This section was a placeholder.
  - No test showed that an unchanged governing file stays citable. The `CLAUDE.md` case was added.

## Rollback

Revert the V1-0414 change commit: `cem cite` writes such citations again, `cem status` keeps
reporting them, and `CEM-V1-114` and its traceability row go away.
