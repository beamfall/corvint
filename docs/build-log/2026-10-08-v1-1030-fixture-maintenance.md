# V1-1030 command-local Git maintenance safeguards in test fixtures

Ticket V1-1030. Test-only change: no production code, wire contract or spec requirement changed.

## Intent

Fixtures that hide the host's global Git config (`HOME=` or `GIT_CONFIG_GLOBAL`) no longer inherit CI's
global `maintenance.auto=false` and `gc.auto=0`. Git subcommands that end in automatic maintenance
(`am`, `cherry-pick`, `commit`, `fetch`, `merge`, `pull`, `rebase`, `revert`) can then launch a detached
`git maintenance` that outlives the test and races `t.TempDir()` cleanup. Each such fixture now passes
`-c maintenance.auto=false -c gc.auto=0` on its own Git invocations.

## Re-verified list

The ticket's 74 files were re-scanned on `origin/main` with a Go-token scan (string literals only). Result:

- 72 of the 74 still applied and were fixed. Most edits are one line in the package's existing Git helper
  (`exec.Command("git", args...)` becomes `append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...`);
  helper variants taking `-C root`, a `git` or `gitExecutable` variable, `exec.CommandContext`, `gitrun.Run`,
  `worksource.Source.Git`, `procgroup`-style `pass(..., "git", ...)` call sites and direct `commit` calls
  were edited in place.
- `cmd/corvint-analyzer-go/production_spy_darwin_test.go`: not applicable. It runs no Git subcommand; it only
  sets `HOME=` for a sandboxed child. Unchanged.
- `internal/cem/verify/stable_repository_test.go`: its helper only runs `commit-tree`, which does not trigger
  maintenance; the helper was guarded anyway because it is shared.
- `cmd/corvint/work_final_check_test.go`: its fixture commits through `materializationGit` (in
  `work_materialization_test.go`, now guarded) and its shell scripts already carry both flags.
- `conformance/release-artifact-v0/loose_gate_ignored_source_test.go` commits through `runGitTest` in
  `archive_gate_test.go` (now guarded).
- `internal/dashboard/repository/parse_test.go` only asserts a child-environment list and a `"commit"` object
  type; no Git process runs there.
- Newly found by the package-level guard or the re-scan (not in the ticket list): `cmd/corvint/host_adapter_fail_open_test.go`
  (already set `GIT_CONFIG_PARAMETERS`; also given the flags on the real-git call), `internal/extevidence`
  (three call sites), and `internal/dogfoodflow/change_test.go` (shared `testGit`).
- Review follow-up: `internal/worksource/source_test.go` and `internal/liveverify/mutate/mutate_test.go` take their
  isolated Git environment from production helpers, so the scan cannot see them; both test helpers were guarded by hand.
  The guard also recognises shell-form `git ... commit` literals.
- `tools/corvint-pr-tests` fixtures go through the production `git()` helper, so the flags are added at the
  test call sites, not in `main.go`.

## Guard

`tools/fixture-maintenance-check/guard_test.go` walks `cmd`, `conformance`, `internal`, `interop` and `tools`
test files and, per package, fails when the package hides the global Git config, names an auto-maintenance
subcommand literal and carries no `maintenance.auto=false` plus `gc.auto=0` literal (or the
`GIT_CONFIG_PARAMETERS` quoted form). Only string literals count, so comments cannot satisfy it.
`TestUnguardedDetectsAMissingSafeguard` plants negatives (missing either flag, comment-only, split across files).
A floor of 40 isolating packages stops the walk silently shrinking. The package is declared in
`.corvint/test-read-scopes.json` with the five scanned roots.

No new spec requirement was added: batch G's AFP-V0-039 guard (liveverify-scoped, equality matching) is
unmerged, and numbering a second requirement here would collide with it. This guard is a superset; when G
lands, its liveverify files already satisfy its literal rule. A requirement can be recorded in a follow-up.

## Limits

- The guard is package-level and literal-based: a package where one file is guarded can mask an unguarded sibling
  helper, and a `"commit"` literal in a non-Git sense is an over-approximation (none is exempted today).
- `interop/cem01-go` is a separate module; it is scanned as text and verified with its own `go test`.
- A fixture whose isolation (`GIT_CONFIG_GLOBAL`) lives only in a production file is invisible to the scan.
- Fixtures that run Git only through a child process the test does not spell out (for example a built binary) are
  not detectable by this scan.
