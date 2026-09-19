# Decision 0322 — The CEM sidecar selects only readers whose literal resolves to it

Date: 2026-09-19. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (owner instruction, 2026-09-19).

Amends `AFP-V0-012` (c) (decision 0131). Every dogfooded change commits `.corvint/change.cem.json`,
so rule (c) ran on it for every branch. Its component runs `.corvint` and `change.cem.json` are the
names packages use to build fixture repositories under `t.TempDir()`, so 30 packages at `2f2aad1`
were selected as readers of a file none of their tests opens. The sidecar is derived from the rest
of the diff and self-excluded from the CEM patch (`internal/frontier.ExcludedPath`).

The call: for that one path, a rule (c) reader is kept only when its token, joined to the holder's
directory (a token starting with `/`, which includes a root module import path and a literal
concatenated after a root variable, is taken from the root), can form the sidecar or an ancestor
directory of it while preserving rule (c)'s outer partial-component matches. When concatenation
splits a root-climbing or compatible root-anchored token from a naming fragment, their common
package is conservatively kept because the literal-only index cannot prove the expressions do not
compose. A test that opens the repository's own sidecar must reach the root by a literal
that climbs (`../../.corvint/change.cem.json`, kept), by `..` components alone, `runtime.Caller`,
`os.Getwd`, or `git rev-parse --show-toplevel` (`unresolved`, selected on every dirty path under
rule (d)), or through a dependency that does so (`unresolved` by dependency). What remains undetected
is the residual already recorded in decision 0131: a root from an environment variable, a flag, or
another subprocess. Rules (a), (b), and (d) and every fallback are unchanged, and no other path is
narrowed.

Rejected: dropping the sidecar from the dirty set, which would also skip rule (d) and the enclosing
package; and applying resolution to every data path, which would narrow `docs/specs` and similar
inputs whose readers take the root from a caller, a change with no derived-evidence argument behind
it.

Measured on a branch from `3f30a02` changing `internal/touchsurprise/compute.go` and the sidecar:
121 packages before, 116 after, equal to the 116 selected for the Go change alone. Sidecar-only
selection falls from 112 to 104, of which 103 are the rule (d) `unresolved` floor and one is
`internal/observations`, kept because its gitignore literal `/.corvint/` is root-anchored. On
`feat/build-number` against `05e17d0` the selection falls from 150 to 148.

Consequence: no wire, receipt, or requirement-ID change. `TestSelectPackagesNarrowsChangeEvidenceReaders`
pins the constant to `frontier.ExcludedPath` and replays fixture, climbing, root-anchored,
partial-concatenation, and ancestor-directory tokens, plus a sibling `.corvint` path that keeps the
component-run rule.

Rollback: revert the commit; the sidecar then selects every component-run reader again.
