# Upgrade Checklist V0

Owner: Russell Lewis
Date: 2026-10-10
Intent status: proposed, pending owner acceptance (V1-1142)
Delivery status: experimental
Authoritative inputs: V1-1142 / GitHub #784 owner request; AGENTS.md invariants 1, 2, 4, 7 and 8.

## Agent digest
- Claim: An offline upgrade checklist maps local merge evidence to issue references in adopter workaround files.
- Status: proposed, pending owner acceptance (V1-1142)/experimental
- Exists: `internal/upgradechanges` and the experimental `corvint changes` command.
- Blocked on: Owner acceptance; GitHub issue state and per-PR command changes are not observed.
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
  bytes MUST NOT create closure evidence. A 30-second total deadline or Git failure refuses with
  `changes-git-unavailable`; output failure refuses with `changes-output-unavailable`. No Git stderr, commit bodies or adopter text is reflected; summaries
  and paths use the existing secret screen; paths preserve whitespace. Summaries are limited to 240 bytes after screening.
- `UCL1142-V0-005`: Root and topic help MUST mark the command experimental and document the workflow,
  local evidence limits, explicit-file input, coded refusals and missing evidence. GitHub issue
  state, reopen/revert semantics and per-PR new flags/commands MUST remain `NOT_OBSERVED`.

## Boundaries and failure modes

Git and the local filesystem are untrusted evidence, not authority. No shell evaluation, remote
lookup, installed-tool mutation, timestamp or working-tree content enters the historical report.
Only explicit file arguments are supported: glob expansion, directory recursion, GitHub API issue
state, structured release metadata and flag/command diff are non-goals (`NOT_RUN`). A `Refs` mention
does not prove closure. Reverts, reopened issues, cross-repository references, squash/rebase merges
and closures before FROM are not resolved. Secret screening is heuristic, not a general detector
for arbitrary sensitive prose; raw messages and adopter lines are withheld. A bound means partial
evidence, never a complete empty result. Missing objects in shallow/partial clones refuse offline.

## Implementation, rollout and rollback

Keep compilation in `internal/upgradechanges`, CLI parsing in its own file and root dispatch/help
edits minimal. This additive experimental profile changes no Core profile or persisted format.
Rollback removes its dispatch, help and package; no data migration. Changed grammar/bounds require
spec and test changes together. Owner acceptance is required before promotion; kill the slice if
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

Acceptance requires the focused fixtures, CLI tests, documentation gates and independent review.
Repository-wide gate and final committed CEM/OCM seal are `NOT_RUN` in this no-commit builder scope.
