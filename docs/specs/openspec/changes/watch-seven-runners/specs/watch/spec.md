# Spec delta: watch

The lane copies each statement below into `watch-v1.md`, inside its `## Requirements` section, as
a numbered definition line that begins `(proposed 2026-11-09; delivery not-started)`.

## ADDED Requirements

### Requirement: WCH-V1-001 Seven runners answer on save

Source: G7, G1, owner decision R7 (2026-11-09). A running `corvint watch` MUST answer each save
with the selected tests of every affected runner among Go, Vitest, Jest, Playwright, pytest, RSpec
and Minitest. Each answer MUST name the runner instance (runner name, root and config, RNC-V1-014)
and MUST identify each test by its canonical test ID (RNC-V1-012).

#### Scenario: Mixed repository

- **GIVEN** a repository with Go, Jest and pytest tests and a running watch
- **WHEN** the user saves a Python source file
- **THEN** watch lists the selected pytest tests and names no unaffected runner

#### Scenario: Two Vitest instances

- **GIVEN** a pnpm workspace with Vitest configs in `packages/a` and `packages/b`
- **WHEN** the user saves a file reached only by tests in `packages/a`
- **THEN** the answer names the `packages/a` Vitest instance and not the other one

### Requirement: WCH-V1-002 List by default, run on request

Source: G7. The default answer MUST be the list of selected tests with one ready-to-run command per
runner, and watch MUST run tests only with `--run`.

#### Scenario: Default mode

- **WHEN** watch starts without `--run` and a file is saved
- **THEN** it prints the list and the command and starts no test process

### Requirement: WCH-V1-003 Each answer carries its tier

Source: G7, G2, owner decisions R8 and R16 (2026-11-09). Each selected test in an answer MUST carry
its tier (`line`, `file`, `flow`, or `full` when widened), the `exact` flag of its runner's argv
(RNC-V1-003) and a one-line reason. pytest answers carry tier `file` (owner decision R8
(2026-11-09)).

#### Scenario: Widened answer

- **WHEN** a runner's state is stale
- **THEN** its tests carry tier `full` and the reason names the stale state

#### Scenario: pytest answer

- **WHEN** a saved Python source file reaches pytest tests
- **THEN** each selected pytest test carries tier `file`

### Requirement: WCH-V1-004 Save-to-answer gate

Source: G7. For each of the seven runners, on each named reference machine and the named reference
repo, the time from the save event to the written answer in a warm, running watch MUST be at most
1.0 s at p95 and 300 ms at p50, and the release MUST be blocked when a runner misses either bound.

#### Scenario: Gate run

- **WHEN** the gate saves 100 recorded edits per runner on the Linux reference machine
- **THEN** the report lists p50 and p95 per runner, and the gate fails if any exceeds its bound

### Requirement: WCH-V1-005 macOS proven by a real run

Source: G7. The save-to-answer gate MUST be run with the kqueue event source on the macOS reference
machine, and its retained result MUST be part of the release evidence.

#### Scenario: Release evidence

- **WHEN** the release evidence is assembled
- **THEN** it contains a macOS kqueue run with per-runner p50 and p95

### Requirement: WCH-V1-006 Event-source failure is explicit

Source: G7, invariant 2. When the event source fails or cannot watch the filesystem, watch MUST exit 2
with a typed error naming the fallback `--events-stdin`, and MUST NOT keep running silently.

#### Scenario: WSL Windows mount

- **WHEN** watch starts on `/mnt/c` under WSL
- **THEN** it exits 2 and names `--events-stdin`

### Requirement: WCH-V1-007 Warm state without the snapshot unless measured

Source: G25 item 5, G7. Watch MUST load each runner's planning state once at start and reuse it, and
MUST NOT depend on the `contextindex` snapshot unless a recorded measurement on the reference
machines shows the bar is missed without it and met with it.

#### Scenario: Snapshot removed

- **WHEN** the snapshot path is absent from the binary
- **THEN** watch still meets WCH-V1-004 or the gate report names the missing runner

### Requirement: WCH-V1-008 The answer path is read-only

Source: invariant 4, owner decision R10 (2026-11-09). Without `--run`, watch MUST NOT write to the
repository or the trace state, and MUST NOT run `corvint inventory refresh` or any lister.

#### Scenario: Listing session

- **WHEN** watch answers ten saves without `--run`
- **THEN** `git status` shows only the user's own edits

### Requirement: WCH-V1-009 Kill switch

Source: G10, owner decision R13 (2026-11-09). `CORVINT_SELECTION=full`, globally or as
`full:<runner>[,<runner>]`, MUST make watch answer with each named runner's full command, on every
instance of that runner, with tier `full`. The grammar is owned by `narrowing-measurement`
(NMS-V1-008); watch parses it through that one implementation and does not define its own. Watch
is a CLI surface: an unknown value MUST exit 2 at start with a message naming the accepted forms.

#### Scenario: Global switch

- **WHEN** the switch is set globally
- **THEN** every answer lists each affected runner's full command with tier `full`

#### Scenario: Unknown value

- **WHEN** watch starts with `CORVINT_SELECTION=fulll`
- **THEN** it exits 2 with a message naming the accepted forms

### Requirement: WCH-V1-010 Diff basis and line-shift mapping

Source: G7, invariant 2 (testing M5). The changed lines of each answer MUST be computed from the
saved working-tree file against its blob at the current HEAD. Deleted and modified lines MUST map
to their HEAD line numbers, and inserted lines MUST map to the HEAD lines directly above and below
the insertion. A runner whose planning state was built at a commit other than the current HEAD
MUST answer its full command with tier `full` and reason `head-changed` until the state is reloaded
for the current HEAD. Untracked files MUST count as wholly changed.

#### Scenario: Line shift in a dirty file

- **GIVEN** a planning state built at HEAD and a source file with 10 lines inserted above a function
- **WHEN** the user then edits a line inside that function and saves
- **THEN** the answer selects the tests that cover that function's HEAD line, not the tests of the
  line now at the same working-tree number

#### Scenario: Branch switch

- **WHEN** HEAD changes to another commit while watch runs
- **THEN** every runner answers its full command with reason `head-changed` until its state is
  reloaded, and no answer uses the old state

Failure mode: the HEAD blob cannot be read (shallow clone, corrupt object): that runner answers its
full command with reason `head-blob-unreadable`. Acceptance: fixture tests for both scenarios and
for an untracked file. Rollback: revert the commit; watch then widens every dirty-file answer to
file level (the pre-change behaviour) until fixed.

### Requirement: WCH-V1-011 Debounce, coalescing and ignore rules

Source: G7 (testing M5). Watch MUST coalesce all events that arrive within one quiet window of at
most 50 ms into one answer, MUST ignore events for paths under `.git/` and paths matched by the
repository's `.gitignore` files, and MUST treat more than 500 changed paths in one window as a HEAD
or bulk change, answering each affected runner's full command with reason `bulk-change`.

#### Scenario: Formatter rewrites five files

- **WHEN** a formatter writes five tracked files within 20 ms
- **THEN** watch prints one answer covering all five files

#### Scenario: Ignored build output

- **GIVEN** `node_modules/` and `dist/` listed in `.gitignore`
- **WHEN** a package install writes 10,000 files under them
- **THEN** watch prints no answer

Failure mode: a `.gitignore` file cannot be parsed: watch treats no path as ignored, prints a
warning once, and keeps answering (over-selection, never a missed save). Acceptance: fixture tests
for the three cases above, plus a timing test showing the window adds at most 50 ms. Rollback:
revert the commit; each event then produces its own answer.

### Requirement: WCH-V1-012 Linux inotify watch limit

Source: G7, invariant 2 (testing M5). On Linux, when adding an inotify watch fails because
`fs.inotify.max_user_watches` is reached, watch MUST exit 2 with the typed error
`watch-inotify-limit`, printing the current limit, the number of directories it could not watch and
the fallback `--events-stdin`, and MUST NOT keep running with part of the tree unwatched.

#### Scenario: Limit reached

- **GIVEN** a Linux host with `max_user_watches` set below the repository's directory count
- **WHEN** watch starts
- **THEN** it exits 2 with `watch-inotify-limit`, the limit, the unwatched count and `--events-stdin`

Failure mode: the limit cannot be read from `/proc/sys/fs/inotify/max_user_watches`: the error
prints `limit unknown` and still exits 2. Acceptance: a Linux test that lowers the limit in a user
namespace or injects the failure through the event-source seam. Rollback: revert the commit; the
generic event-source failure (WCH-V1-006) still exits 2.

### Requirement: WCH-V1-013 Where save-to-answer timing starts and ends

Source: G7 (testing M5). For WCH-V1-004 and WCH-V1-005, the save-to-answer time MUST start when the
harness's write of the saved file returns (after the file is closed), and MUST end when the last
byte of the answer is written to watch's output; both MUST be taken from one monotonic clock. The
time therefore includes event delivery and the WCH-V1-011 quiet window.

#### Scenario: Timing record

- **WHEN** the gate harness saves one recorded edit
- **THEN** its record holds the write-return time, the answer-written time and their difference, all
  from the same monotonic clock

Failure mode: the harness cannot observe the answer's last byte (output buffered by a pipe): the
sample is discarded and counted as failed, and more than 1 percent failed samples fails the gate.
Acceptance: the harness source and one retained report per reference machine showing the three
fields. Rollback: revert the commit; WCH-V1-004 then has no valid measurement and the gate reports
NOT_RUN, which blocks the release.

### Requirement: WCH-V1-014 Absent or stale inventory in watch

Source: owner decision R10 (2026-11-09), review T-M4. Watch MUST read the committed inventory
(change `test-inventory`) and MUST NOT refresh it. When the inventory is absent, each runner MUST
answer its declared full command (RNC-V1-013) with tier `full` and reason `inventory-absent`; when
it is stale, the answer MUST select all tests of the affected runners with reason
`inventory-stale`. A later `corvint inventory refresh` run outside watch is picked up at the next
save.

#### Scenario: No inventory

- **WHEN** watch runs in a repository with no committed inventory and a file is saved
- **THEN** each affected runner answers its full command with reason `inventory-absent`

#### Scenario: Refresh outside watch

- **WHEN** the user runs `corvint inventory refresh` in another terminal and then saves a file
- **THEN** the next answer uses the refreshed inventory, and watch itself wrote nothing

## MODIFIED Requirements

### Requirement: LPCV-V0-1086 Watch loop

Watch is no longer Go-only and no longer runs tests by default (WCH-V1-001, WCH-V1-002). The event
sources (inotify, kqueue, `--events-stdin`) and `--once` are kept; `--dry-run` is removed with no
alias (settled S2).

#### Scenario: No dry-run flag

- **WHEN** watch is started with `--dry-run`
- **THEN** it exits 2 as an unknown flag, and listing is the default without it

### Requirement: LPCV-V0-1087 Event-source error

Kept: exit 2 on an event-source error, now with the typed fallback named by WCH-V1-006 and
WCH-V1-012.

#### Scenario: Event-source error

- **WHEN** the event source fails
- **THEN** watch exits 2 and names `--events-stdin`
