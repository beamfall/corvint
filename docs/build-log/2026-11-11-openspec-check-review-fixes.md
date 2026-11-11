# 2026-11-11: openspec-check review fixes, round 1 (V1-5399)

Change `openspec-check`; requirements OSC-V1-001, OSC-V1-003 and OSC-V1-011 in
`docs/specs/openspec-check-v1.md`. An independent review of the first delivery
(`2026-11-11-openspec-check.md`) returned FIX. This entry records the decisions and the fixes.

## Decision: OSC-V1-003 applies only to commits that opt in to a change

- Finding. With `--base b55635262~50` the check failed 865 of 1023 non-merge commits: ordinary
  bug-fix commits carry `Corvint-Ticket:` for tickets no proposal names, and each one failed as
  "binds to no change".
- Decision (orchestrator, owner-delegated, 2026-11-11). A commit is in scope only when it opts in
  to a change: it touches a path under `docs/specs/openspec/changes/<name>/` (or under the archived
  `changes/archive/YYYY-MM-DD-<name>/`), carries a `Change: <name>` trailer, or carries a
  `Corvint-Ticket:` naming a ticket that some proposal's `Ticket:` header names. Every other
  commit gets no finding. OSC-V1-003 and the change delta are amended to match.
- Consequence. A path opt-in binds the commit to that change, so the propose stage (OSC-V1-004)
  applies to a commit that adds a change folder and code together even without a trailer. The
  generic "binds to no change" failure remains only for a commit whose opted-in folders no active
  or archived folder holds at HEAD; a `Change:` trailer naming no folder keeps its own finding.
- Measured. Base `b627141d7`, the 51st non-merge commit before `b55635262` (81 non-merge commits
  to this branch's head): 66 OSC-V1-003 findings before, 0 after. Base `b55635262~50` (1029
  non-merge commits): 865 before, 0 after; 0 OSC-V1-004 findings either way. The commits in the
  first range that opt in and touch code are `4cea40c19` (agent-packages), `945730c88`
  (ticket-taxonomy), `c8bb373da` (affected-plan-kinds, e2e-inventory-discovery,
  flows-status-report), `17050e432` and `f9529aa07` (tasks-store-cutover), `792c4be29` and
  `6d1bbbb5c` (ticket-brief-context), and this change's `0e911bcf1`, `73e83b7cd`, `5051acb60` and
  `ac660bde7` (through `Corvint-Ticket: V1-5399`). Each binds to a folder held at HEAD whose
  proposal its first parent holds, so none fails.

## Decision: the baseline lists findings, not subjects

- Finding. One `change X` line hid every future finding of X.
- Decision. Each line is `<rule ID> <subject>: <key>`; the key is the finding message with the
  line number of a `file.md:N:` locator removed, so an edit that only moves a line keeps its
  entry. An entry hides only the finding with the same rule, subject and key; a new finding on a
  listed subject fails. `baseline.txt` was regenerated from the BASELINE lines of the
  subject-format run: 363 entries over the same 318 subjects (30 change folders, 288 specs), so it
  hides nothing the subject format did not.

## Fail-closed Git reads

- `exists` and `showFile` read the tree with `git ls-tree`: an empty listing is "absent at the
  commit"; any Git failure (an unresolvable rev, a missing object) is a `history-unavailable`
  refusal, exit 2. The archive-history tree comparison uses the same reader.
- `baselineGrowth` propagates Git errors; a baseline file absent at the base makes every entry
  count as added. So `make openspec-check OPENSPEC_BASE=<rev>` fails for any base older than the
  commit that adds `baseline.txt`, including `origin/main` until this change lands (363 "added
  since" failures at `b627141d7`).
- The ID rule propagates a failure to read `REQUIREMENTS.tsv` at the folder's parent, and the
  archive rule propagates a failing `git status --porcelain`.

## Task lines

- A bullet (`-`, `*` or `+`) whose text starts with `[` in a form other than `- [ ] N` or
  `- [x] N` fails as malformed, naming the line: `* [x]`, `+ [ ]`, `- [X]`, `- [-]`, `-[ ]`. No
  existing `tasks.md` holds such a line.

## Evidence

- `go test -count=1 ./tools/openspec-check/`: ok. New tests: `TestOSCV1003OptIn`,
  `TestOSCV1011BaselinePerFinding`, `TestOSCV1011BaselineGrowthGitErrors`,
  `TestOSCV1011GitAbsentVersusFailure`, `TestOSCV1001MalformedTaskLines`; `TestOSCV1003CommitBinding`
  and `TestOSCV1011Baseline` updated to the new scope and format.
- `make openspec-check`: PASS (0 failures, 363 baselined findings in 318 subjects).
