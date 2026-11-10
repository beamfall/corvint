# Tasks: remove the source that 1.0 no longer ships

Change name: `source-removal`. Intent status: proposed. Each task names its requirement IDs
(`specs/source-removal/spec.md`) and its proof. Tasks run in the order listed; a batch task starts
only after the tasks before it are done, except where its gate in `design.md` says otherwise. Every
batch task also carries RMV-V1-001 (order and the 2026-12-01 date), RMV-V1-003 (per-batch proof,
including `make windows-vet` once before that batch's push, owner decision R36 (2026-11-09)),
RMV-V1-004 (importers re-checked on the batch base), RMV-V1-005 (tests rewritten first), RMV-V1-030
(rollback by Git) and RMV-V1-031 (orphans deleted and listed); its record is one build-log entry
(owner decision R34 (2026-11-09)).

## Tasks

1. Owner accepts or amends this change and the companion decision
   `docs/decisions/PROPOSED-supersede-0594-keeps-line.md`, through the acceptance record
   `docs/decisions/PROPOSED-accept-product-plan-1.0.md`. The RMV-V1-014 mechanism (owner decision
   R32 (2026-11-09)) and open items 4 and 5 (owner decision R33 (2026-11-09)) are already decided.
   Requirements: all. Proof: the accepted record names this change's commit SHA.
2. Write `docs/specs/source-removal-v1.md` from the accepted requirements and regenerate
   `docs/specs/REQUIREMENTS.tsv` with `make spec-requirements`. Requirements: RMV-V1-001 to
   RMV-V1-031. Proof: the RMV rows appear in `REQUIREMENTS.tsv`; the line-citation check passes.
3. B0, `.github`-only repair of dangling references, with the ruleset check. Requirements:
   RMV-V1-002, RMV-V1-006. Proof: RMV-V1-003 items 3 and 4 for the removed workflow names; the
   ruleset required-check list before and after.
4. B1, dead leaves and dangling scripts, including `internal/docviews` and the orphans and
   `remove` rows of `path-verdicts.md`. Requirements: RMV-V1-007, RMV-V1-031. Proof: RMV-V1-003;
   the batch's orphan table in its build-log entry.
5. B2, test-quality cut and keeps. Requirements: RMV-V1-008, RMV-V1-009. Proof: RMV-V1-003; the
   kept rule sets still pass their own tests.
6. B3, runner protocol cut, with the RSP-V0-025 MODIFIED and RSP-V0-026 to RSP-V0-032 and
   RSP-V0-047 REMOVED deltas applied as status lines in `runner-selection-plugins-v0.md`.
   Requirements: RMV-V1-010, RMV-V1-011. Proof: RMV-V1-003; zero-hit `git grep` for
   `runner-plugins.json` outside retired specs and history.
7. B4, VS Code extension and G22 extras. `extensions/vscode` is removed only after its frozen,
   unpublished copy is pushed to `beamfall/corvint-vscode` (owner decision R33 (2026-11-09)).
   Requirements: RMV-V1-012, RMV-V1-013. Proof: RMV-V1-003; the pushed commit in
   `beamfall/corvint-vscode`.
8. RMV-V1-014 fail-closed behaviour lands through RNC-V1-004 (change `runner-contract`) and
   NCM-V1-001 (change `non-code-map`): run all tests with a named reason; `UNOWNED_DIRTY_PATH` off
   the allowlist (owner decision R32 (2026-11-09)). Requirements: RMV-V1-014. Proof: the fixture
   test, passing before B5 starts.
9. B5, languages and analyzers (milestone). Requirements: RMV-V1-014 to RMV-V1-018. Proof:
   RMV-V1-003; `corvint affected` on a `.rs` change names the reason and `corvint test` runs all
   tests; milestone checks.
10. B6a, shared package move; then B6b, CEM, OCM and dogfood removal; close V1-0014 and V1-1344 to
    V1-1348. Requirements: RMV-V1-019 to RMV-V1-022. Proof: RMV-V1-003; `go vet` in the root
    module only; `LICENSING.md` pins check passing; ticket read-backs.
11. B7, the G25 "still removed" set, after the revival proposals (milestone). Requirements:
    RMV-V1-023. Proof: RMV-V1-003; the five committed revival proposals; milestone checks.
12. B8a, lints, now; B8b, lane tooling, after the layering merge (about 2026-12-08).
    Requirements: RMV-V1-024, RMV-V1-025. Proof: RMV-V1-003 for each sub-batch.
13. B9, spec status lines (inside each batch that retires a spec, plus one sweep); B10, Tasks specs
    after `beamfall/corvint-tasks` PR #4. Requirements: RMV-V1-026, RMV-V1-027. Proof: each
    retired spec has a status line; `REQUIREMENTS.tsv` regenerated with retired IDs kept as
    history rows.
14. B11, action cut-over (milestone), by 2026-12-15. Requirements: RMV-V1-028. Proof: 30 matched
    shadow PRs; a `.github`-only PR first, then the script removal PR; milestone checks.
15. File the tickets: Jest path through `corvint test` (end-to-end check, G23); `liveverify/mutate`
    (G26). Requirements: RMV-V1-017. Proof: `corvint-tasks ticket show` for each.
16. Throughout: RMV-V1-029 (`perfbudget` is kept). Proof: the package is present and its tests
    pass at each milestone.

## Acceptance evidence

- For each batch: its RMV-V1-003 proof on the exact merged head, the RMV-V1-004 importer list, the
  RMV-V1-031 orphan table, and the batch's merge commit and parent, in the batch's build-log entry.
- B0: the ruleset required-check list before and after.
- RMV-V1-014: the fixture test, passing before B5 starts.
- B5: `corvint affected` names the reason on a `.rs` change; `corvint test` runs all tests.
- B6: `go vet` in the root module only; `LICENSING.md` pins check passing.
- Milestones: `corvint affected --base` output and a green CI run.
- RMV-V1-021: `corvint-tasks ticket show` for each closed ticket and a passing `receipt audit`.
