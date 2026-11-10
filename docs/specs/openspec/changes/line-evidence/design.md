# Design: line-evidence

## Decisions

1. **Writing verb.** `corvint line-evidence refresh [--runner R] [--root DIR]` (name proposed,
   Open 1) runs each instance's tests under its collector and writes the result. It is the only
   writer. `corvint affected`, `corvint test`, `corvint watch` and MCP only read. Source: review
   T-B1 and NEW-testing-3.

2. **Storage.** One file per runner instance at
   `<git-dir>/corvint/line-evidence/<runner>@<root-digest>.linecov`, in the existing
   `linecov-index/4` encoding (GLS-V0-001) with the extension's `Language`, `RunnerFamily`,
   `Command` and `Config` set for the instance. Tests in the index use canonical test IDs
   (CTI-V1-009). The store is private and never committed.

3. **Tree binding and freshness.** The index binds the commit, the runner identity and a blob OID
   per instrumented file (GLS-V0-001). A reader compares those with the planned tree. A changed file
   with no matching blob, a different runner identity or config, or a commit that is not an ancestor
   of the planned base, makes that instance's evidence stale for the files concerned; those files
   widen to `file`, and an instance-wide mismatch widens the instance to `full`. Reason
   `line-evidence-stale`. For `corvint watch`, the planned change is the working tree and index
   against HEAD, with HEAD checked to descend from the evidence commit; otherwise FULL (owned by
   change `watch-seven-runners`, review NEW-testing-2). One-shot `corvint affected`, `corvint test`
   and the MCP plan tool use the same rule (LEV-V1-015; R42 (panel pick) A7).

4. **Collectors.**

   | Runner | Collector | Tier when fresh and qualified |
   | --- | --- | --- |
   | Go | `internal/linecov` (existing) | `line` |
   | Vitest | `internal/jslinecov` runner `vitest` | `line` after FIH proof |
   | Jest | `internal/jslinecov` runner `jest` | `line` after FIH proof |
   | RSpec | `internal/rubylinecov` RSpec hook | `line` after FIH proof |
   | Minitest | `internal/rubylinecov` Minitest hook | `line` after FIH proof |
   | pytest | none in 1.0 (owner decision R9) | `file`, labelled `file` |
   | Playwright | none used for removal (G2) | `flow` or `full`; line evidence may add or rank |

5. **Tiers.** Every selected unit carries `tier` in `line | file | flow | full` and a separate
   boolean `exact` (settled S4, owner decision R16). `exact` is true only when the runner ran exactly
   the selected tests. `flow` does not count as narrowed for the no-missed-test claim. Every
   selection prints one line naming the tier and the reason (G2).

6. **Playwright.** A browser E2E test leaves the selection only through accepted flow links plus
   static reachability at file and symbol level (G2; changes `affected-plan-kinds`,
   `e2e-inventory-discovery`, `flow-link-proposals`). Line evidence can add a test to the selection
   or order selected tests; output shows ordering as `rank`, never as narrowing. The per-test E2E line
   profile `--selection-profile e2e-line-experimental` (name proposed) is opt-in, labelled
   experimental, and its promotion needs a fault-injected zero-miss proof and a measured net
   wall-clock win.

7. **Qualification.** A runner's evidence produces tier `line` only after that runner passes the
   fault-injection harness and is line-level proven (FIH-V1-015; the harness computes its own line
   plans for that proof; owner decision R19: N=100, at most 3 misses, Wilson interval,
   at most 5 percent excluded). Before that, fresh evidence for the runner is used at `file` and the
   plan prints `line-evidence-unqualified`. Go keeps its current qualification check (GLS-V0-034).

8. **Consumers.** One reader package used by `corvint affected`, `corvint test`, `corvint watch` and
   the MCP affected tool. `corvint watch` accepts every language the reader supports, replacing the
   Go-only `watch-index-language` refusal in `cmd/corvint/watch.go` (integ 9ec3a29d0). The explicit
   `--line-index FILE` flag and GLS-V0-010 are unchanged.

9. **GitHub Action.** The Action runs `corvint line-evidence refresh` on the default branch and saves
   the store with the Actions cache keyed by commit and instance. A PR job restores the newest cache
   from an ancestor commit; a cache miss widens with `line-evidence-absent`. Coordination with change
   `github-action` is Open 4.

10. **Argument limits.** When the selected test names for one invocation exceed the runner's argument
    limit, Corvint passes them in an argument file where the runner accepts one; otherwise it widens
    the invocation to file level with reason `argv-limit` (review T-M9).

## Failure modes

| Failure | Behaviour |
| --- | --- |
| No evidence for an instance | Widen to `file` for reached files, reason `line-evidence-absent`. |
| Evidence from a non-ancestor commit or other runner identity | Widen the instance to `full`, reason `line-evidence-stale`. |
| One changed file not in the evidence | That file's reach widens to `file`, reason `line-evidence-stale`. |
| Runner not yet FIH-qualified | Fresh evidence used at `file`, reason `line-evidence-unqualified`. |
| Damaged index section | `index-section-invalid` fallback for that path (GLS-V0-002), widened. |
| pytest change | `file` tier for reached test files; no line narrowing. |
| Playwright with line evidence only | Never removed; added or ranked; reason printed. |
| Collector fails during refresh | That instance's file is not replaced; readers treat it as absent or stale. |
| Too many selected names | Argument file, else widen to file, reason `argv-limit`. |

## Acceptance evidence

- Per runner, a fixture where a one-line change selects exactly the covering tests, with tier `line`
  after the runner's FIH record exists and `file` before it.
- pytest fixture showing tier `file` and the label in the plan.
- Playwright fixture showing a test with line evidence and no accepted link is selected, and that a
  rank appears only as `rank`.
- Stale, absent and unqualified fixtures with their reasons.
- A read-only test: affected, watch and MCP leave the store unchanged.
- The FIH result per runner (owner decision R19) linked from the spec before `line` is enabled.

## Rollback

Revert the change's commits and delete `<git-dir>/corvint/line-evidence/`. Plans fall back to file
and full tiers with `line-evidence-absent`, which only widens. The Go `--line-index` path is
unchanged. LEV-V1-001 to LEV-V1-015 stay listed as retired if the change is withdrawn.

## Open

1. The verb name `corvint line-evidence refresh`.
2. The experimental E2E profile name.
3. Whether `tools/linecov` keeps a separate program after folding, or is removed.
4. The Action cache step, to be agreed with change `github-action`.

## Open decisions

- Decided, R42 (panel pick) A7: one-shot `corvint affected`, `corvint test` and the MCP plan tool
  use the `corvint watch` working-tree rule of decision 3, so an uncommitted edit gets line
  evidence instead of the `worktree-dirty` fallback (LEV-V1-015; selection verdicts, finding 10).
