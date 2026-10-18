# V1-1698, V1-1699 shared-constant setup keys and the changes range bound

Date: 2026-10-18. Base: `d35b0ee33e0efda8468db03a22956d0b629bf4d8`.
Scope: new experimental `TSA-V0-006`/`TSA-V0-007` in `docs/specs/test-source-analysis-v0.md` and
`UCL1142-V0-008` (with the matching `UCL1142-V0-004` deadline wording) in
`docs/specs/upgrade-checklist-v0.md`, agent-drafted and accepted under the owner's standing
auto-accept of lane requirements (2026-10-11). No decisions, Core profile changes or ticket-store
writes. The analyzer schema moves to `corvint-analyzer/122` because the contextindex sources changed
(IDX-SNAP-V0-017).

## V1-1698 (GitHub #1082): setup-select failed open on a shared constant

`corvint tests setup-select` returned `selected` with an empty list for a spec that reads
`SHARED.scenario_a`, because a property of an exported constant was neither an import of a setup
file nor a registry key. The fix treats a literal property of an exported object constant as a key:
the spec's reached files and every setup file are scanned for reads of the same constant (followed
through re-exports, aliases and namespaces, at most 8 hops), and a setup file reading a reached key
is selected and traversed like a registry seeder. Anything the selector cannot reduce to a literal
key (a computed `SHARED[k]`, a whole-object use, a namespace used as a value) returns ALL with the new
closed reason `shared-constant-key-unresolved` whenever an unselected setup file reads the same
constant. A spec reaching no shared constant is unchanged. Selection over-selects when many setups
read the same key; that is the safe direction for a test provider.

## V1-1699 (GitHub #1083): wide ranges refused as changes-git-unavailable

Measured on the Corvint checkout with the installed binary: a 666-commit range (59 first-parent)
completed in 25.7 s; a 920-commit range (89 first-parent) refused at exactly 30 s. `git rev-list
--count` over the same range took under a second. The cause was the command's own 30-second deadline
surfacing through the Git reader as `changes-git-unavailable`. `Compile` now counts the range first
and refuses over 512 commits with `changes-range-too-large` (`limit`, `commits`), and its own
deadline refuses with `changes-timeout` (`limitMs`, `elapsedMs`). A caller's cancellation and Git
failures keep `changes-git-unavailable`. Paging and `--max-commits` were not built.

## Review fix (PR #1096 Codex review, 2026-10-18)

One Codex round reported seven P1 fail-open cases in `setup_shared.go`, one P2 false ALL and one
P2 timeout misclassification. Each is fixed, each has a regression case that failed on the
reviewed head `eca56102`, and `TSA-V0-006`, `TSA-V0-007` and `UCL1142-V0-008` were amended in place
under the same standing auto-accept. The rule applied throughout: reaching an analysis bound or an
unsupported form is never the same as "not a constant", so it returns ALL, never a silent
`selected`.

- `export * as NS from` re-exports, and nested `NS.INNER.NAME` members, now resolve to a module
  namespace instead of being discarded.
- Resolution is three-valued (constant, not a constant, unknown). A chain past 8 hops, past 4096
  export-table lookups, or through an unreadable or unresolved module is unknown; an unknown read
  in a reached file, or held by an unselected setup file, returns ALL with
  `shared-constant-key-unresolved`.
- The `export *` cycle marker and bound-shaped results are no longer cached; only path-independent
  results are.
- Literal `require("m")`/`import("m")` resolve when used as `.NAME`; any other use (destructuring,
  assignment) fails closed. A setup file using a namespace as a value fails closed too.
- A file's own exported object constants are read in that file.
- A template whose `${}` substitutions nest deeper than 8 fails closed.
- Shadowing: declarations (`const|let|var|function|class C`, `C =>`, simple `function`, arrow and
  `catch` parameters) are no longer reads, so the reported false ALL is gone. There is no scope
  analysis: reads through the shadowing binding still count as reads of the constant (safe
  over-selection), and a destructured or method parameter still reads as a whole-object use
  (conservative ALL). Both are documented in the spec's boundaries.
- `Compile` now classifies `changes-timeout` by the context's cancellation cause
  (`context.WithTimeoutCause`), which the first cancellation fixes, so a caller deadline expiring
  while Git is still reaped after the command's own deadline no longer yields
  `changes-git-unavailable`.

Order dependence that remains: a resolution cached by an earlier, shorter path is reused by a
longer chain that would itself exceed the hop bound. The cached result is a path-independent fact
and traversal order is deterministic, so it only ever adds precision.
