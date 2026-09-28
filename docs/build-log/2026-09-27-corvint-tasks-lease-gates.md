
## 2026-09-27 CAL-V0-015..017 and CAL-V0-024: `submit`, `gate run` and `complete` (S5)

An `external-agent` attempt can now finish its ticket without a supervisor. `submit --tree`
records the candidate tree after the CAL-V0-024 scope check. It moves `RUNNING` to `BUILT`, and a
resubmit from `BUILT` or `CHECKING` replaces the tree. `gate run --gate` runs one policy `COMMAND`
gate in the caller's clean worktree at the candidate tree. It records a `taskman-gate-result/0`
under `evidence/`, and the store observes the exit status itself. `complete --commit` checks the
ticket and the attempt. It then completes the ticket `VERIFIED` with a
`taskman-completion-manifest/0` evidence record, fences the generation and frees the reservation,
all in one transaction.
Amendment A13 records the narrowings:
- Only `COMMAND` gates with no shared resource or inputs are run.
- A result's staleness is its tree binding. No stored result is rewritten.
- The supervisor, lane, spawn and review checks do not apply to this runtime.
- A completion receipt names its ticket, because it writes the ticket file (the A9 wording).

Two defects were found and fixed while building. First, a gate with empty output posted a nil
evidence body, which the store reads as a deletion. Second, replay rejected a completion receipt
whose resulting revision was non-null, so a lease `complete` now counts as a ticket operation.

Live CLI qualification ran in a real Git repository with a fixture policy (gate `verify` =
`sh -c "printf ok"`). The run was `init`, then `ticket create`, then committing the store exports
on `main`, then a claim from a separate `git worktree`. Results:
- `submit` of a tree adding `stray.go` was refused `OUT_OF_SCOPE`, naming `stray.go`.
- `submit` of the in-scope tree reached `BUILT`.
- `complete` before merging was refused `STALE_TREE` ("not reachable from main").
- `gate run` with an untracked file was refused `DIRTY_WORKTREE`. Clean, it passed with the output
  digest of `ok`.
- `complete` after a fast-forward merge left the ticket `COMPLETED`/`VERIFIED` and the attempt
  `COMPLETED`/`FENCED`, with the manifest and gate result as evidence. A repeat replayed.
- `receipt audit` reported `CONSISTENT`/`AGREES` at head 6.

The first live attempt was refused `OUT_OF_SCOPE`, correctly. Its agent commit included
`.taskman/queue.json`, which `ticket create` had re-exported into the same checkout. Store exports
have to be committed before a claim's base, and an agent should not work in the primary worktree.

Finding for adoption: Beamfall's live `.taskman/policy.json` declares `full-gate`,
`interop-gate`, `companion-release`, `focused-docs` and `release-checklist` as `required`, all
with `env: []`. Every completion therefore needs all five to pass. Each would also run without
`PATH`, so a recipe that calls `go` fails. That policy needs `required` narrowed to what every
ticket must pass, and `PATH` (plus the Go variables) declared, before agents complete through it.

An independent review before binding found seven issues, and each was fixed:
- The scope check diffed only against the base tree. A candidate rebased onto a moved `main`
  would have been charged with other tickets' paths, which dead-ends completion. A path now counts
  only when it differs from both the base tree and the intent branch tip. The price, recorded in
  A13, is that a path committed straight to `main` before `submit` is not counted.
- A13 said an `APPROVAL_REQUIRED` completion needs a `RUN` grant. The code requires `COMPLETE`.
- A gate whose children held its output pipe open could hang `gate run`. It now has a 5-second
  wait delay after the process group is killed.
- A gate's `cwd` and reducer were ignored. They are now refused `UNSUPPORTED` unless the `cwd` is
  `WORKTREE` and there is no reducer.
- An interrupt of `gate run` was recorded as a result. It now cancels the run and records nothing.
- An attempt with no lease reached the gate runner.
- An attempt that changed while its gate ran could panic on the missing record. It is now refused
  `TICKET_STATE`.
The live flow was rerun on the fixed binary with the same results.
