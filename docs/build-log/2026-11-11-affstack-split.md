# Affstack split: name-listing gate held, link-target fixes ship (V1-5193, V1-5194, V1-5368)

The affstatus and V1-5368 stack (branch lane/affstack-fix, range base `origin/main`
`4c3c203336a6cc4a9772af39b1a771f34236138e`, head `c5e40bd79`) reached the two-round review-fix
convergence cap with the name-listing gate still admitting content reads. The stack was split.

**Decision authority.** The orchestrator made the split under the lane convergence cap. It is an
orchestrator decision, not an owner acceptance, and it accepts no requirement.

## Held

The shell name-listing narrowing gate is held and does not land from this branch:

- V1-5193 and V1-5194;
- AFP-V0-9960 to AFP-V0-9964 (no spec text for them is carried; AFP-V0-9965 was reworded so it
  depends on none of them);
- `internal/liveverify/affected/shell/namelisting.go` and its `lex.go` plumbing;
- the change-status plan input (`WithChangeStatus`, the `affected-change-status` OpenSpec
  change, and `cmd/corvint/affected_change_status_test.go`);
- the affstatus build-log section on name listings, and the substitution parts of the round-2
  and rescope entries.

Branch lane/affstack-fix at `c5e40bd79` is kept for reference; it is not merged.

## Why: three independent reviews, the same class

Each review found a lexer-versus-shell disagreement that lets the gate admit a unit that reads
content, so a content change below a listed path could be missed. The latest round's three P1
repros, as reported by the review (their exact scripts are not retained in this entry, and they
were not re-run during the split):

| Repro | Shell | Result |
|---|---|---|
| Empty here-document delimiter `<<''` | bash | The here-document ends at the first empty line; the lexer ends it elsewhere, so a content-reading command it hides as body text runs and is admitted. |
| `\r#` fake comment | bash | `\r` is not a blank to bash, so `#` after it does not start a comment and the rest of the line runs; the lexer reads a comment and admits the unit. |
| `[[` as a plain command | dash | dash has no `[[` keyword; it runs `[[` as a command name, so the test's words and what follows run as commands; the lexer reads a conditional and admits the unit. |

Round 2 had already found substitution ends that bash 3.2, bash 5.1 and dash close at a
different `)` than the lexer (`docs/build-log/2026-11-11-affstack-fix-round-2.md` on the held
branch). Patching each form after review does not converge.

## Redesign needed for V1-5193 and V1-5194

The gate needs one of:

- **Refuse on raw text before lexing.** Refuse a unit whose source holds any `\r`, any `<<`
  that is not `<<<`, any NUL, or any `[[` unless the shebang names bash, zsh or ksh. Only then
  lex and apply the grammar.
- **An allowlist grammar.** Admit only a small set of exact command shapes recognised on the
  raw text, and refuse everything else, so a lexer disagreement can only refuse.

Either design needs its own spec, failing tests per reported repro, and a fresh review.

## Ships

Branch lane/affstack-symlink delivers V1-5368 / AFP-V0-9965 (proposed, experimental): the
symlink-target frontier, link targets that re-enter the tree, root edges, the case-folded
re-entry, the snapshot repository root, and the non-ASCII check before the exact match in
`physicalTarget`. Its evidence is in `docs/build-log/2026-11-11-affsym-operand.md`,
`docs/build-log/2026-11-11-affsym-fix-round.md` and
`docs/build-log/2026-11-11-affsym-fix-round-2.md`.

`TestSymlinkOperand_V15368` was carried without change status: it labels each row's change
locally and calls `Select` with the dirty path alone; every row's expected misses are unchanged.

## Link targets: non-ASCII first, and its precision cost

`physicalTarget` runs the non-ASCII check before the exact-prefix match, so
`physicalTarget("/r/café.sh", {"/r"})` returns `"."` (from `c5e40bd79`). Review P3-a: the same
check returns `"."` and true for a non-ASCII target outside every root, such as
`/elsewhere/é.sh`. That fails closed; the cost is precision, since such a link widens the plan
to every dirty path. The comment, the `non_ascii_outside` row of
`TestSymlinksPhysicalTarget_AFPV09965` and AFP-V0-9965 now say so.

## Rollback

Revert the lane/affstack-symlink commits; the planner then matches link paths by text only, as
at the base.
