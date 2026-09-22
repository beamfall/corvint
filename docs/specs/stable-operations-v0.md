# Stable operations V0

Owner: Russell Lewis
Date: 2026-09-22
Requirement prefix: `SOP-V0`
Intent status: accepted (decision 0341, 2026-09-22)
Delivery status: implemented
Authoritative inputs: `../../AGENTS.md` invariants 4 and 7, `../decisions/0341-stable-operations-qualification-2026-09-22.md`,
`public-release-v0.md` (`PUB-V0-025`/`PUB-V0-026` install lifecycle and hostile store),
`release-artifact-integrity-v0.md` (archive layout and checksum manifest), `index-snapshot-v0.md`
(snapshot miss semantics), `../INSTALL.md`, `../RELEASE-RUNBOOK.md`, `../../SECURITY.md`.

## Agent digest
- Claim: Install lifecycle, corrupted-snapshot recovery and a named hostile-regression matrix run locally as release-blocking checks with NOT_COVERED gaps stated.
- Status: accepted (decision 0341, 2026-09-22) / implemented
- Exists: `script/check-install-lifecycle.sh` + `_test.sh`, `script/check-hostile-regressions.sh` + `_test.sh`, `docs/RELEASE-RUNBOOK.md`, the support window in `SECURITY.md`, the recovery contract in `docs/INSTALL.md`.
- Blocked on: `make` targets for the two checks (Makefile is outside this change); native evidence on hosts other than darwin arm64; the `memory` and `case-folds-context-index` categories have no regression.
- Read next: Requirements; Acceptance criteria and testing matrix.

## Human intent and scope

Ticket V1-0017 asks that stable operational and security expectations for the native Core close
without adding a service or permanent daemon: release artifacts survive install, upgrade, rollback,
uninstall, backup and corrupted-state recovery; hostile inputs have release-blocking regressions;
and a vulnerability policy, support window, changelog and reproducible runbook are published.

Affected user: the release owner qualifying a candidate, and an operator recovering an install or a
damaged `.corvint/index`. Measurable job: one command per concern, run from a clean checkout against
the exact candidate bytes, that prints a per-step result and a single PASS/FAIL verdict, and never
publishes, tags, signs, or writes into the repository it is run from.

## Verified current state

Measured on 2026-09-22, darwin arm64, Go 1.27.1, at the introducing commit:

- `script/check-install-lifecycle.sh` against a built binary passes all eight steps in 2.5 s; its
  wrapper test (two stamped builds, archive path, tampered checksum, usage) passes in 9.3 s.
- `script/check-hostile-regressions.sh` runs 27 matrix rows over 12 packages; all PASS in 28.9 s;
  two categories print NOT_COVERED. Its stub-driven wrapper passes in 4.5 s.
- A truncated or byte-damaged snapshot under `.corvint/index` leaves the read verb `context` at
  exit 0 with byte-identical packet output and the damaged file untouched; `corvint index --if-stale`
  reports `mutates: true` and rewrites a snapshot of the original size that the next `--if-stale`
  reports `fresh`. The rebuilt file is the same size but not byte-identical to the first build (460
  differing bytes under `cmp -l`), so the recovery contract is packet identity, not snapshot identity.
- The version string is `const version = "0.6.0"` (`cmd/corvint/main.go:30@7005974c`) and the build
  stamp is `var build = "0"` (`cmd/corvint/main.go:1499@e7212353`), set by `-ldflags -X main.build=N`.
- The archive producer writes `corvint_<goos>_<goarch>.tar.gz` for four targets, `SHA256SUMS` and
  `verification-report.json` (`release-artifact-integrity-v0.md`); the installer refuses a case-fold
  alias of an existing store entry (`internal/releasecandidate/operations.go:180@1161e39d`).

## Requirements

- `SOP-V0-001`: The install-lifecycle check MUST accept either one release archive whose single root
  directory holds `corvint` and `SHA256SUMS`, or one built `corvint` executable, and MUST verify every
  `SHA256SUMS` row against the installed bytes and run `corvint --version` before any other step. A
  mismatch MUST fail the run at `install-a` with exit 1 before any index or read command executes.
- `SOP-V0-002`: The check MUST create a committed fixture repository in its own temporary directory,
  run `corvint index` there and require `"ok":true` and exactly one snapshot under
  `.corvint/index`, then capture one read-verb packet whose bytes every later step is compared to.
- `SOP-V0-003`: An upgrade MUST install into a second store beside the first, run `corvint index
  --if-stale` and the read verb through the new store, and require byte-identical packet output. The
  first store MUST remain executable and, run again (rollback), MUST produce the same packet bytes.
  A run without `CORVINT_LIFECYCLE_UPGRADE_BINARY` MUST report the upgrade as `same-bytes`.
- `SOP-V0-004`: Uninstall is removal of the store directories. Afterwards the fixture's tracked tree
  MUST be unchanged (`git diff --quiet HEAD` and empty `git status --porcelain`) and `.corvint/index`
  MUST still exist.
- `SOP-V0-005`: A backup is an archive of `.corvint` taken while no Corvint writer runs. After the
  directory is removed and the archive restored, `corvint index --if-stale` MUST report `fresh` and
  the read verb MUST reproduce the same packet bytes.
- `SOP-V0-006`: With the snapshot truncated to zero bytes, and separately with bytes overwritten
  inside it, a read verb MUST exit 0, reproduce the same packet bytes and leave the damaged file's
  size unchanged (invariant 4: reads do not repair); `corvint index --if-stale` MUST report
  `mutates: true` and rewrite a snapshot of the original size that the next `--if-stale` reports
  `fresh`. Byte identity of the rebuilt snapshot is not required.
- `SOP-V0-007`: The hostile-regression check MUST hold a matrix of `category`, Go package and test
  functions, run each row as one `go test -count=1 -v -run '^(A|B)$' <package>` under
  `GOTOOLCHAIN=local` from the checkout root, and derive each test's result from its `--- PASS` or
  `--- FAIL` line. A test without either line MUST count as NOT_RUN, never as a pass; a `--- FAIL`
  line or a nonzero `go` exit MUST mark the row FAIL naming the test and fail the run with exit 1.
- `SOP-V0-008`: Every test function the matrix names MUST be defined in a tracked `_test.go` file,
  and the matrix MUST name the same tests the acceptance matrix below cites by anchored `path:line`.
  `--list` MUST print the matrix and the NOT_COVERED rows without running anything.
- `SOP-V0-009`: A category with no tracked regression MUST be printed as
  `category=<name> status=NOT_COVERED reason=<text>` in both `--list` and run output, MUST be listed
  in this spec, and MUST NOT be represented by a proxy test. Current entries: `memory` (no test bounds
  resident memory) and `case-folds-context-index` (no test covers tracked paths differing only by
  case in the context index on a case-insensitive worktree).
- `SOP-V0-010`: `SECURITY.md` MUST state the private reporting channel, the acknowledgment window,
  the in-scope classes, and the support window: while the version is 0.x only the latest published
  release receives security fixes, and the 1.0 support window is set by the owner at V1-0021.
- `SOP-V0-011`: `docs/RELEASE-NOTES.md` is the changelog. Each published release MUST have an entry
  naming its version, build number and full commit; the runbook step that adds it MUST precede the
  tag.
- `SOP-V0-012`: `docs/RELEASE-RUNBOOK.md` MUST be an ordered list of exact commands from a clean
  clone through toolchain check, gate, archive production, checksum manifest, reproducibility check,
  lifecycle check, hostile matrix, release-notes entry and tag, plus the rollback of a bad release,
  citing only scripts that exist in the tree. It MUST NOT authorize publication, signing or upload.

## Non-goals and simpler baseline

The simpler baseline is the prose runbook and the `internal/releasecandidate` fixture tests that
existed before this spec: they prove installer mechanics but never ran a released executable through
its own index and read path. This spec adds no service, daemon, package manager, signing, auto-update
or downgrade migration; it does not qualify Windows, does not measure resident memory, and does not
replace `script/public-release-check` or the companion release gate.

## Trust boundary, limits, and failure modes

Both checks write only private temporary directories removed on exit and spawn `git`, `tar`,
`shasum`/`sha256sum` and `go` from the caller's PATH. Inputs are the candidate bytes and the tracked
tree; neither check reads the network.

| Failure | Behavior |
|---|---|
| `SHA256SUMS` row mismatches or is missing | `step install-a: FAIL`, exit 1, nothing else runs (SOP-V0-001) |
| Packet bytes differ after upgrade, rollback, restore or corruption | that step FAILs naming the comparison; exit 1 |
| A read verb rewrote a damaged snapshot | `corrupt-*` FAILs "a read verb rewrote the snapshot" (SOP-V0-006) |
| Rebuilt snapshot size differs from the original | `corrupt-*` FAILs; a size change is a format change that needs a spec update |
| Neither `CORVINT_LIFECYCLE_ARCHIVE` nor `CORVINT_LIFECYCLE_BINARY` set, or an argument given | usage, exit 2 |
| Matrix test skipped by a build tag or `t.Skip` | counted NOT_RUN in its row; a row with no pass is `status=NOT_RUN` and does not fail the run (SOP-V0-007) |
| Matrix test renamed or deleted | the wrapper test fails naming it; the check itself reports NOT_RUN (SOP-V0-008) |
| Corrupt data outside `.corvint/index` (tickets, traces, evidence) | out of scope; `docs/INSTALL.md` requires a consistent backup |
| Host other than darwin arm64 | no native evidence in this change; NOT_RUN, not implied |

## Acceptance criteria and testing matrix

Lifecycle steps are `install-a`, `first-index`, `upgrade-b`, `rollback-a`, `uninstall`,
`backup-restore`, `corrupt-truncate`, `corrupt-overwrite`; the wrapper cases are numbered in
`script/check-install-lifecycle_test.sh`.

| Requirement | Evidence |
|---|---|
| SOP-V0-001 | wrapper case 1 (binary), case 2 (archive root `corvint_test_host`), case 3 (tampered `SHA256SUMS` fails at `install-a`, no `first-index` line, exit 1) |
| SOP-V0-002 | step `first-index` in cases 1 and 2; snapshot miss/hit semantics `internal/contextindex/snapshot_test.go:47@9ea4ec57` |
| SOP-V0-003 | case 1 asserts `upgrade-b` ran `(build 2)` and no `same-bytes`; case 2 asserts `same-bytes`; installer coexistence `internal/releasecandidate/install_test.go:16@90e57d5e`, `internal/releasecandidate/operations_test.go:44@5dca69d1` |
| SOP-V0-004 | step `uninstall` in cases 1 and 2 |
| SOP-V0-005 | step `backup-restore` in cases 1 and 2; round trip `internal/contextindex/snapshot_test.go:180@4e378161` |
| SOP-V0-006 | steps `corrupt-truncate` and `corrupt-overwrite`; unit-level miss `internal/contextindex/pack_test.go:206@b2b8420d` |
| SOP-V0-007 | hostile wrapper case 2 (exact delegated argv, one `go` per row), case 3 (skipped test NOT_RUN, run passes), case 4 (row NOT_RUN), case 5 (`--- FAIL` names the test, exit 1) |
| SOP-V0-008 | hostile wrapper case 1 (`git grep` of every listed `func Test...(` in tracked `_test.go`; every category listed) |
| SOP-V0-009 | hostile wrapper cases 1 and 2 (NOT_COVERED rows present in `--list` and in a run) |
| SOP-V0-010 | review of `SECURITY.md` "Supported versions" |
| SOP-V0-011 | review of `docs/RELEASE-NOTES.md` and runbook step 9 |
| SOP-V0-012 | review of `docs/RELEASE-RUNBOOK.md`; every cited script resolves under `script/` or `conformance/` |

Hostile-regression matrix (`script/check-hostile-regressions.sh --list` prints the same rows):

| Category | Package | Tests |
|---|---|---|
| hostile-repository | `internal/genesis` | `internal/genesis/repository_test.go:146@5817975f`, `internal/genesis/repository_test.go:158@a2e88456`, `internal/genesis/repository_test.go:196@a1e12d68` |
| hostile-repository | `internal/contextindex` | `internal/contextindex/history_test.go:115@671cf1e3` |
| paths | `internal/releasegate` | `internal/releasegate/releasegate_test.go:141@e18871cf` |
| paths | `internal/companionrelease` | `internal/companionrelease/companionrelease_test.go:620@5ef76c7b` |
| paths | `internal/trace` | `internal/trace/migration_test.go:80@77e27ccc` |
| paths | `internal/doccompiler` | `internal/doccompiler/compiler_test.go:211@b9d5a61d` |
| symlinks | `internal/contextindex` | `internal/contextindex/index_test.go:163@6bb9aead`, `internal/contextindex/index_test.go:403@22de943f`, `internal/contextindex/blob_shards_open_test.go:91@9e01d46f`, `internal/contextindex/blob_shards_open_test.go:52@137ff6a7`, `internal/contextindex/snapshot_test.go:494@22f1c7f2`, `internal/contextindex/snapshot_test.go:541@ee7924d9` |
| symlinks | `internal/trace` | `internal/trace/store_test.go:222@ee958993`, `internal/trace/store_test.go:937@69176005` |
| symlinks | `internal/releasecandidate` | `internal/releasecandidate/operations_test.go:105@141529a4` |
| symlinks | `internal/worktreeimpact` | `internal/worktreeimpact/hostile_unix_test.go:12@0eab7357` |
| case-folds | `internal/companionrelease` | `internal/companionrelease/companionrelease_test.go:190@4d21a51f`, `internal/companionrelease/companionrelease_test.go:390@7f54b5f5` |
| case-folds | `internal/mcp/testvaliditybridge` | `internal/mcp/testvaliditybridge/testvaliditybridge_test.go:162@e7747668` |
| case-folds | `internal/releasecandidate` | `internal/releasecandidate/operations_test.go:105@141529a4` (`case-alias` subcase) |
| bounded-output | `internal/procgroup` | `internal/procgroup/process_overflow_test.go:13@8b1b8267`, `internal/procgroup/process_overflow_test.go:93@e6067a5b` |
| bounded-output | `internal/contextindex` | `internal/contextindex/git_execution_unix_test.go:151@b46f5300`, `internal/contextindex/extractionnote_test.go:63@d2c61a7e` |
| bounded-output | `internal/trace` | `internal/trace/record_test.go:390@cc18800d` |
| bounded-output | `internal/releasecandidate` | `internal/releasecandidate/operations_test.go:189@0595c93c` |
| time | `internal/procgroup` | `internal/procgroup/process_test.go:52@028e61d3`, `internal/procgroup/process_test.go:62@704c3abb` |
| time | `internal/contextindex` | `internal/contextindex/local_completion_context_test.go:185@1bcaa713` |
| interruption-cleanup | `internal/procgroup` | `internal/procgroup/process_test.go:519@9aa96910`, `internal/procgroup/process_test.go:95@0a8f0be2`, `internal/procgroup/process_hook_test.go:13@f0458815` |
| interruption-cleanup | `internal/releasecandidate` | `internal/releasecandidate/operations_posix_test.go:18@d8182271`, `internal/releasecandidate/operations_test.go:238@eab69ef3` |
| interruption-cleanup | `internal/contextindex` | `internal/contextindex/fifo_unix_test.go:36@5098dd43` |
| interruption-cleanup | `internal/trace` | `internal/trace/store_test.go:497@139a1f88` |
| secret-screening | `internal/secretscreen` | `internal/secretscreen/secretscreen_test.go:35@ac274de5`, `internal/secretscreen/secretscreen_test.go:164@c325b643` |
| secret-screening | `internal/trace` | `internal/trace/record_test.go:282@382ada39` |
| secret-screening | `internal/contextindex` | `internal/contextindex/history_test.go:144@b551779f` |
| corrupted-derived-state | `internal/contextindex` | `internal/contextindex/pack_test.go:206@b2b8420d`, `internal/contextindex/blob_shards_test.go:84@b82cb217`, `internal/contextindex/termtable_test.go:170@263288af`, `internal/contextindex/analyzer_schema_test.go:93@f88f0dea` |
| memory | — | NOT_COVERED: no regression bounds resident memory; the input-size bounds under bounded-output are a proxy, not a memory limit |
| case-folds-context-index | — | NOT_COVERED: no regression covers tracked paths that differ only by case in the context index on a case-insensitive worktree |

Related installer evidence outside the matrix, because it is not hostile-input: probe failure
cleanup `internal/releasecandidate/operations_test.go:160@155b3bc9`.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| SOP-V0-001 | `install`, `verify_sums`, `install_from_archive` in `script/check-install-lifecycle.sh` | wrapper cases 1–3 |
| SOP-V0-002 | `make_fixture`, `index`, `read_packet`; `TestSnapshotHitAndMissUseStatusDirtyPaths` | wrapper cases 1–2 |
| SOP-V0-003 | steps 3–4 of the same script; `TestPUBV0025VersionedInstallCoexistsAndNeverReplaces`, `TestPUBV0025RecoveryLifecycle` | wrapper cases 1–2 |
| SOP-V0-004 | step 5 | wrapper cases 1–2 |
| SOP-V0-005 | step 6; `TestSnapshotRoundTripAppliesDirtyPathsAndMissesOnANewTree` | wrapper cases 1–2 |
| SOP-V0-006 | `corrupt_and_recover`; `TestPackSnapshotRefusesCorruptionTruncationAndOutOfRangeAsAMiss` | wrapper cases 1–2; measured 460-byte non-identity recorded above |
| SOP-V0-007 | the run loop in `script/check-hostile-regressions.sh` | hostile wrapper cases 2–5 |
| SOP-V0-008 | `matrix()` and `--list`; the matrix table above | hostile wrapper case 1 |
| SOP-V0-009 | `not_covered()` and `print_not_covered` | hostile wrapper cases 1–2 |
| SOP-V0-010 | `SECURITY.md` | review |
| SOP-V0-011 | `docs/RELEASE-NOTES.md`; runbook step 9 | review |
| SOP-V0-012 | `docs/RELEASE-RUNBOOK.md` | review |

## Rollout, rollback, and drift

Both checks are additive scripts with no Go change; rollback is deleting the four scripts and this
spec's rows. The wrapper tests fail when a matrix test is renamed, so a rename edits the matrix, this
spec's table and the anchor in the same change. A snapshot format change that alters the rebuilt
size fails `corrupt-*` and must update `SOP-V0-006`. The `make` targets `install-lifecycle-test`,
`hostile-regressions-check` and `hostile-regressions-test` are proposed, not present, until the
Makefile owner adds them.

## Unresolved

Native runs on linux amd64/arm64 and darwin amd64 are NOT_RUN; the `memory` and
`case-folds-context-index` categories need new tests before they leave NOT_COVERED; whether a
rebuilt snapshot should be byte-identical to its first build is a question for `index-snapshot-v0.md`.
