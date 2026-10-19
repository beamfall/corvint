# V1-1968: review packet from a configured source repository

Date: 2026-10-19
Ticket: V1-1968 (GitHub issue beamfall/corvint#1217).
Requirements: ERG-V0-022 (`docs/specs/corvint-tasks-external-reviews-v0.md`), with a one-sentence
pointer in ATR-V0-141 (`docs/specs/corvint-tasks-attempt-runner-v0.md`). Accepted under the owner's
standing auto-accept of lane requirements (2026-10-11).

## Decisions

- **Named sources live in the dispatch configuration.** A program names each source checkout once
  (`sourceRepositories: {NAME: /abs/path}`, at most 16); a role selects one with
  `reviewPacket.sourceRepository`, and `gate packet --source NAME --config FILE` reads the same
  table. `--source` without a configured name is a path. No new store, policy field or ticket
  field: the task store is unchanged.
- **Search order: source, then task repository.** The revision resolves in the first repository
  that holds it, so a task-repository commit still works with a source set; absent from all, the
  refusal is MISSING_EVIDENCE with `COMMIT_NOT_FOUND:` naming every repository searched.
- **Refusal codes reuse the closed set.** The §11 code set is closed, so, like
  `CANDIDATE_CLEARED_BY_CLAIM:`, the new reasons are detail prefixes: `SOURCE_REPOSITORY_UNKNOWN:`
  (MALFORMED) for a name the configuration lacks or a path that is not a worktree top level, and
  `COMMIT_NOT_FOUND:` (MISSING_EVIDENCE).
- **No candidate branch with a source reviews the source HEAD.** Such programs submit no candidate
  branch in the task repository, so the dispatch source (which passes no `--commit`) would otherwise
  always refuse. The packet labels this `revisionSource: SOURCE_HEAD`; it is the checkout's HEAD at
  read time, not a submitted candidate.
- **Every packet carries `repository`.** Kind, root, optional name, requested and resolved revision,
  revision source and the searched roots, so a reviewer can cite which repository the diff came
  from. `readOnDemand.diff` adds `git -C ROOT` for a source.
- **Base selection is unchanged.** Passed trees are content addresses, so the review history still
  finds a passed ancestor in the source; the task-repository attempt base is used only when it is an
  ancestor of the commit in the source.

## Residuals

- The packet records the source's local root, not a remote identity; it is not an ownership proof.
- `SOURCE_HEAD` depends on the checkout's state when read; an operator who needs a pinned revision
  passes `--commit`.
- Live use in a two-repository program is NOT_RUN.
