# Adoption setup report V0

Owner: Russell Lewis
Date: 2026-10-20
Requirement prefix: `ADS-V0`
Intent status: accepted under the owner's delegated 1.0 feature cut (decision 0548; V1-2052)
Delivery status: experimental
Authoritative inputs: V1-2052; `AGENTS.md` invariants 4 and 7; `cmd/corvint/init_adopt.go`
(the `adopt` adoption inventory verb this extends).

## Agent digest
- Claim: `corvint adopt --setup` reports index, instruction, hook and ignore setup gaps with evidence and fixes, writing nothing.
- Status: accepted under the owner's delegated 1.0 feature cut (decision 0548; V1-2052)/experimental
- Exists: `cmd/corvint/adopt_setup.go`, `--setup` dispatch in `cmd/corvint/init_adopt.go`, `cmd/corvint/adopt_setup_test.go`.
- Blocked on: user-scope host installs are not observed.
- Read next: Requirements; Failure modes; Acceptance evidence and rollback.

## Intent and scope

An adopter or agent can ask one read-only question, "what is missing from this repository's
Corvint setup?", and get one report naming each gap, the evidence that shows it and a suggested
fix. It extends the existing `adopt` verb instead of adding a parallel `doctor` verb.

## Requirements

- `ADS-V0-001`: `corvint [--root PATH] adopt --setup` MUST print one canonical JSON object
  `{ok:true, mutates:false, tool:"adopt", setup:{profile:"corvint-setup-report/0", state,
  gap_count, checks}}` and exit 0. `state` is `READY` when every check is `ok`, otherwise `GAPS`;
  `gap_count` counts checks whose status is not `ok`. Each check carries `class`, `status`
  (`ok`, `gap` or `unobserved`), a non-empty `evidence` list and, when not `ok` (including every
  `unobserved` path), a `fix` string.
- `ADS-V0-002`: `--setup` MUST NOT combine with any other `adopt` option or with `init`; such a
  request is a usage error (exit 2) before any read.
- `ADS-V0-003`: the `index-snapshot` check MUST use the read-only snapshot probe and report
  `gap` unless a snapshot matching the committed tree for this binary exists; the fix is
  `corvint --root ROOT index --if-stale`.
- `ADS-V0-004`: the `agent-instructions` check MUST report `gap` unless a root `AGENTS.md`,
  `CLAUDE.md` or `GEMINI.md` names Corvint (case-insensitive), with per-file evidence.
- `ADS-V0-005`: the `hooks` check MUST report `gap` unless repository-scoped `.claude/settings.json`
  or `.codex/hooks.json`, parsed as JSON, wires Corvint: an `enabledPlugins` key starting
  `corvint@` whose value is exactly `true`, or a `type: command` hook whose command runs
  `corvint adapter`. A disabled plugin, a substring match elsewhere and malformed JSON are gaps.
  The evidence MUST say that user-scope installs are not read.
- `ADS-V0-006`: the `ignored-paths` check MUST report `gap` for each Corvint private state path
  (`.context-corvint/`, `.corvint/index/`, `.corvint/self-observations.jsonl`,
  `.corvint/unplanned-reads.jsonl`, `.corvint/unplanned-reads.enabled`) that Git's ignore rules do
  not cover, including a path un-ignored by a negated pattern. Git exits 0 while warning about
  an ignore file it cannot read, so any `check-ignore` diagnostic makes the check `unobserved`
  with the warning as evidence.
- `ADS-V0-007`: the command MUST write nothing (invariant 4): files are read through the repository
  root as regular files of at most 1 MiB, after an `Lstat` that rejects a FIFO, device or link
  without opening it, and Git runs as one `check-ignore --no-index` with
  `GIT_OPTIONAL_LOCKS=0` and a 10 s timeout.

## Non-goals

No automatic fix, no network access, no reading of user-scope host configuration, no judgment of
instruction-file quality beyond naming Corvint, and no CI or release-channel checks.

## Failure modes

- A snapshot probe error or a missing or failing `git` reports that check `unobserved` with the
  error as evidence; the report still prints and `state` is `GAPS`.
- A hook wired only at user scope reports `gap` with evidence that says user scope is not read;
  the report can over-report this class and never under-reports it.
- An unreadable, oversized or non-regular instruction or hook file is treated as absent.

## Acceptance evidence and rollback

| Requirements | Witness |
|---|---|
| ADS-V0-001,003..007 | `TestAdoptSetupReportsEachGapClass`: one fully set-up fixture (READY), a Codex adapter hook, and one fixture per gap class (including a disabled plugin and malformed JSON), with a byte digest of the tree (including `.git`) unchanged across each run |
| ADS-V0-001,003,006 | `TestAdoptSetupUnobservedChecksCarryAFix` (snapshot probe error, git not on PATH), `TestAdoptSetupUnreadableIgnoreFileIsUnobserved` |
| ADS-V0-007 | `TestAdoptSetupRejectsFIFOWithoutOpening` |
| ADS-V0-002 | `TestAdoptSetupRefusesOtherOptions` |

Measured output is in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-20-v1-2052-adopt-setup-report.md`. Rollback: remove
the `--setup` flag, `cmd/corvint/adopt_setup.go`, its test, the help paragraph and this spec; no
state or format depends on it.
