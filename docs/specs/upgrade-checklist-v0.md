# Upgrade Checklist V0

Owner: Russell Lewis
Date: 2026-10-10
Intent status: accepted (decision 0518; V1-1142)
Delivery status: experimental
Authoritative inputs: V1-1142 / GitHub #784 owner request; AGENTS.md invariants 1, 2, 4, 7 and 8.

## Agent digest
- Claim: An offline upgrade checklist maps local merge evidence to issue references in adopter workaround files.
- Status: accepted (decision 0518; V1-1142)/experimental
- Exists: `internal/upgradechanges` and the experimental `corvint changes` command.
- Blocked on: GitHub issue state comes only from an operator-supplied file; per-PR command changes and reopen/revert semantics are not observed.
- Read next: Requirements; Boundaries and failure modes; Delivery.

## User and verified baseline

The adopter tags a workaround with an issue number, then compares two installed build commits in
the Corvint source checkout. At `55aa6bb9f1e9d8db81f8185b5d185fc8c1bbc821`, the CLI has no
`changes` command. The simpler baseline is manually reading Git merge messages and searching files.
Success is a repeatable local checklist with issue, closing PR and exact file/line, without changing
the checkout or contacting a service. This is evidence to review, not permission to remove a hold.

## Requirements

- `UCL1142-V0-001`: `corvint [--root SOURCE] changes --from SHA --to SHA` MUST resolve hexadecimal
  commit IDs locally, pin full IDs, and require FROM to be an ancestor of TO. Equal
  endpoints yield an empty range. Missing commits refuse with `changes-build-unavailable`; invalid
  arguments refuse with `changes-invalid-arguments`; incompatible ancestry refuses with
  `changes-range-unavailable`. No fetching, lazy fetching, repository writes or trace writes occur.
- `UCL1142-V0-002`: The deterministic range MUST use first-parent commits in FROM..TO. Only merge
  commits whose subject is `Merge pull request #N from ...` yield PR rows. Inspect each such merge
  message and commits introduced relative to its first parent, restricted to FROM..TO; map
  case-insensitive Closes/Fixes/Resolves/Refs #N evidence to that carrying PR. Explicit closure
  keywords yield `closed_by_commit_message`; Refs yields `unknown`, never closure. Unassociated
  commits, unsupported merge forms and unmentioned issue state remain explicit unknowns. PR
  summaries use the first nonempty merge-body line (or the PR number); issue summaries use the
  evidence commit subject. These are source-derived summaries, not GitHub issue titles.
- `UCL1142-V0-003`: Repeatable `--refs-file FILE` MUST scan explicit regular files relative to SOURCE
  (or absolute paths), reject symlink components and non-regular files, and emit one hint per
  matching occurrence with file, one-based line, issue and closing PR. Only explicitly closed
  issues from the selected range qualify. The hint is `workaround may be removable`; file text is
  never emitted. File scanning is supported on macOS/Linux; other platforms refuse. Unreadable/unsafe paths refuse with `changes-refs-unavailable`.
- `UCL1142-V0-004`: Output MUST be one deterministic compact JSON document with profile
  `corvint-upgrade-checklist/0`, fixed field order, sorted PR/issue/hint rows, full endpoint IDs,
  explicit unknowns and a boolean `truncated`. Bound history to 256 first-parent commits, 256
  introduced commits per PR, 64 KiB per message, 4 MiB total Git output, 16 files of 1 MiB each,
  4096 scanned references per file, and 1024 issue/hint rows each. Any omitted evidence MUST set `truncated`; incomplete message
  bytes MUST NOT create closure evidence. Git that cannot run or a repository that cannot be read refuses with
  `changes-git-unavailable`; the range bound and the 30-second total deadline refuse as `UCL1142-V0-008` defines (amendment recorded under the 2026-10-11 standing auto-accept, V1-1699); output failure refuses with `changes-output-unavailable`. Cancellation or the deadline MUST close an open `--refs-file` descriptor so its reader is released rather than kept until process exit; only a read the kernel cannot interrupt may hold it until that system call returns (amendment accepted, decision 0519 (V1-1155)). No Git stderr, commit bodies or adopter text is reflected; summaries
  and paths use the existing secret screen; paths preserve whitespace. Summaries are limited to 240 bytes after screening.
- `UCL1142-V0-005`: Root and topic help MUST mark the command experimental and document the workflow,
  local evidence limits, explicit-file input, coded refusals and missing evidence. Reopen/revert
  semantics and per-PR new commands MUST remain `NOT_OBSERVED`; GitHub issue state is
  `NOT_OBSERVED` unless `UCL1142-V0-006` supplies it, and per-PR flags follow `UCL1142-V0-007`
  (amendment accepted, decision 0519 (V1-1158)).
- `UCL1142-V0-006`: (accepted, decision 0519 (V1-1158)) Optional `--issue-state-file FILE`
  (at most once, relative to SOURCE or absolute) MUST be read with the `UCL1142-V0-003` no-follow,
  regular-file, 1 MiB reader and decoded as one JSON array of objects, each with a positive integer
  `number` and a `state` of `OPEN` or `CLOSED` (case-insensitive), the shape `gh issue list --state
  all --json number,state` produces. Other members are ignored and never emitted. More than 4096
  rows, a malformed, `null` or truncated document, an invalid row or repeated member, conflicting states for one number, or an
  unreadable/unsafe path refuse with `changes-issue-state-unavailable`. Corvint MUST NOT query
  GitHub or any network service. Each issue row and hint carries `github_state`: the supplied
  state, or `NOT_OBSERVED` when no file is given or the number is absent. A hint whose supplied
  state is `OPEN` MUST NOT say `workaround may be removable`; it says `closing evidence, but the
  supplied issue state is OPEN; review`. The unknowns name the state as operator-supplied and not
  verified by Corvint. The file is caller-reported evidence, never authority, and does not resolve
  reopen/revert semantics or closures before FROM.
- `UCL1142-V0-007`: (accepted, decision 0519 (V1-1158)) Each PR row MUST carry
  `new_flags_basis`, `new_flags` and `new_commands` (replacing `new_flags_commands`). `new_flags` lists,
  sorted and `--`-prefixed, flag names that appear on added lines and on no removed line of the
  local diff from the merge's first parent to the merge, restricted to non-test `cmd/corvint/*.go`
  files: names declared through the standard `flag` package by string literal and `"--name"` /
  `"--name="` literals. Git runs in forced text mode with no external diff, textconv, color, rename
  detection or configured prefix/relative path; quoted header paths are decoded, and a binary patch or
  undecodable path is treated as over a bound. The diff is bounded to 256 KiB per PR within the 4 MiB total, and
  at most 64 names; over a bound, `new_flags` is empty, `new_flags_basis` is `NOT_OBSERVED` and the
  report is `truncated`. Otherwise `new_flags_basis` is `cmd-corvint-go-source-diff`. This is a
  source-literal heuristic, not a verified CLI surface diff; `new_commands` stays `NOT_OBSERVED`.
- `UCL1142-V0-008`: (accepted under the 2026-10-11 standing auto-accept, V1-1699 / GitHub #1083)
  Before reading any commit message, parent list or diff, Compile MUST count every commit in
  `FROM..TO` with one bounded `git rev-list --count`. A count above 512 refuses at once with
  `changes-range-too-large`, and the CLI refusal names `limit` (512) and `commits` (the count).
  A count that cannot be read or parsed refuses with `changes-git-unavailable`. Reaching the
  command's own 30-second total deadline refuses with `changes-timeout`, naming `limitMs` and
  `elapsedMs`; a caller's cancellation and every other Git failure keep `changes-git-unavailable`.
  Whichever of the two fires first decides: a caller cancelling after the command's own deadline
  fired, while Git is still being reaped, MUST still refuse with `changes-timeout` (amended
  2026-10-18 after the PR #1096 review).
  Help lists both codes and says a wide range is checked hop by hop over narrower build pairs.
  Paging and a `--max-commits` override are non-goals.

## Boundaries and failure modes

Git and the local filesystem are untrusted evidence, not authority. No shell evaluation, remote
lookup, installed-tool mutation, timestamp or working-tree content enters the historical report.
Only explicit file arguments are supported: glob expansion, directory recursion, GitHub API issue
lookup, structured release metadata and command diff are non-goals (`NOT_RUN`); issue state enters
only through the `UCL1142-V0-006` operator file. A `Refs` mention
does not prove closure. Reverts, reopened issues, cross-repository references, squash/rebase merges
and closures before FROM are not resolved. Secret screening is heuristic, not a general detector
for arbitrary sensitive prose; raw messages and adopter lines are withheld. A bound means partial
evidence, never a complete empty result. Missing objects in shallow/partial clones refuse offline.
The 512-commit range bound is measured, not derived: on the Corvint checkout a 666-commit range
completed in about 26 s and a 920-commit range reached the 30-second deadline (V1-1699), so a range
within the bound can still time out on a slower host and then refuses with `changes-timeout`.

## Implementation, rollout and rollback

Keep compilation in `internal/upgradechanges`, CLI parsing in its own file and root dispatch/help
edits minimal. This additive experimental profile changes no Core profile or persisted format.
Rollback removes its dispatch, help and package; no data migration. Changed grammar/bounds require
spec and test changes together. Intent is accepted (decision 0518); promotion beyond
experimental needs owner approval; kill the slice if
the local fixture cannot reproduce hints safely and deterministically. External adoption outcomes
remain `NOT_RUN` and are not a prerequisite for this locally testable experimental slice.

## Delivery

| Requirement | Implementation | Focused evidence | Status |
|---|---|---|---|
| UCL1142-V0-001 | `internal/upgradechanges` | `TestCompileRangeAndRefs`, `TestCompileRefusals`, `TestCompileGitConfiguration`, `TestCompileRefusesShallowCheckout` | experimental |
| UCL1142-V0-002 | `internal/upgradechanges` | `TestCompileRangeAndRefs` | experimental |
| UCL1142-V0-003 | `internal/upgradechanges` | `TestCompileRangeAndRefs`, `TestCompileUnsafeRefs`, `TestReadRefsRejectsFIFO` | experimental |
| UCL1142-V0-004 | `internal/upgradechanges` | `TestCompileBounds`, `TestCompileRangeAndRefs`, `TestCompileGitConfiguration`, `TestCappedOutput`, `TestCompileReferenceAndIssueCaps`, `TestCompileRefsReadHonoursContext`, `TestCompileShallowCheckCancellationKeepsGitUnavailable` | experimental |
| UCL1142-V0-005 | `cmd/corvint/changes.go`, `docs/INSTALL.md` | `TestChangesCLI` | experimental |
| UCL1142-V0-006 | `internal/upgradechanges` (`readIssueStates`), `cmd/corvint/changes.go` | `TestCompileIssueStateFile`, `TestChangesCLI` | accepted (decision 0519; V1-1158); experimental |
| UCL1142-V0-007 | `internal/upgradechanges` (`newFlags`) | `TestCompileNewFlags` | accepted (decision 0519; V1-1158); experimental |
| UCL1142-V0-008 | `internal/upgradechanges` (`Compile`, `RangeTooLargeError`, `TimeoutError`), `cmd/corvint/changes.go` | `TestCompileRangeBoundAndTimeout`, `TestChangesCLI` | accepted (standing auto-accept 2026-10-11; V1-1699); experimental |

Acceptance requires the focused fixtures, CLI tests, documentation gates and independent review.
Repository-wide gate and final committed CEM/OCM seal are `NOT_RUN` in this no-commit builder scope.
