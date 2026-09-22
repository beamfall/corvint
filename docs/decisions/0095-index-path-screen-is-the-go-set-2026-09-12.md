# Decision 0095 — the index path screen is the current Go set, `.claude` included

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls
and record them (orchestration thread, 2026-09-12).

`DR-0034` (`conformance/divergence-register.md`) was an open `spec-gap`: no accepted spec said
which path components, prefixes, or patterns index admission must skip, so Go's `.claude` member
of `forbiddenParts` (`internal/contextindex/index.go`) was unowned behavior. The retired Python
oracle cannot be reconstructed to discriminate (`GOC-V0-002`), so the gap closes only by spec text.

The spec adopts the screen Go applies today, unchanged, as `IDX-SNAP-V0-018`
(`docs/specs/index-snapshot-v0.md`):

- Components `.git`, `vendor`, `node_modules`, `app-dist`, `dist`, `build`, `coverage`, `.next`,
  `.cache`, `target`, `.claude`, whole-component and case-sensitive, reason `vendor/build excluded`;
  then prefixes `internal/store/migrate/` and `internal/conformance/testdata/`, reason
  `protected path`; then the `generatedPath` pattern, reason `generated path`. First match wins.
- `.claude` stays. An agent worktree copy under it duplicates the repository's own files as stale
  first-party source, which inflates the symbol population; the exclusion is recorded in
  `Exclusions`, so nothing is hidden (invariants 1 and 2).
- No member was found to contradict an invariant. The cost is that first-party source under a
  member-named directory (a `build/` Go package) is excluded with a disclosed reason; that is an
  omission, not invented certainty, and is recorded as the clause's failure mode.
- No production code changes, so the analyzer schema is not bumped. Future edits to the set amend
  the clause and follow `IDX-SNAP-V0-017`'s schema rule.
- The learned-trace screen in `internal/trace` (and its dashboard copy) keeps the ten-member set
  without `.claude`. It is a separate contract, not decided here, and is filed as a follow-up.

`DR-0034` is re-adjudicated `python-defect` / known-divergent under `IDX-SNAP-V0-018` and closed;
no discriminating parity row can be authored because the oracle is retired. The beta-rung record
drops its disclosure for `query` and `eval`; no admission label changes.

Rollback: revert this decision's commit. That removes `IDX-SNAP-V0-018` and its test, reopens
`DR-0034` with its disclosures, and restores the fixes backlog entry; no runtime bytes change.
