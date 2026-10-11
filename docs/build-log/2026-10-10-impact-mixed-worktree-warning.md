# V1-1141 / GitHub #783: MIXED impact binding disclosure

Intent: proposed, pending owner acceptance (V1-1141). Delivery: experimental.
Base: `55aa6bb9f1e9d8db81f8185b5d185fc8c1bbc821` (origin/main at start).
Owning contract: `docs/specs/revision-cache-dirty-worktree-v0.md`, DIRTY-CACHE-014..015.

## Scope and implementation

Path impact compares its normalized requested paths with the complete captured dirty set. When
that set is nonempty and does not contain every requested path, the context receipt's top-level
`warning` (`context.warning` in the CLI wrapper) names the bound commit, dirty count and mutable
content exclusion. The warning participates in ordinary packet budgeting and byte accounting.
Clean and all-dirty requests retain their existing shape and freshness metadata.

Experimental `impact --require-clean` returns an empty NEEDS_WIDENING receipt on MIXED, before ranking
or external provider reads. It retains freshness, warning and explicit `mixed-worktree` abstention.
It exits 0 as a structured receipt; invalid arguments exit 2. It shares ordinary path/limit
validation. The flag is limited to path impact, including its non-Go syntax profile. Rollback is
removal of the flag and additive warning together; no stored-data migration is introduced.

`affected` is deferred as a follow-up proposal: its separate mutable-worktree graph already selects
Git-visible dirty paths plus range changes. It does not share the immutable path-impact compiler.
An AFP amendment would need to define strict behavior across normal, range and snapshot modes.
No new task was filed because this builder is explicitly prohibited from writing `.taskman`.

## Evidence and review

- Requirement allocation checked live origin/main, all 17 open PR file lists, and remote
  `refs/heads/codex/w5-*` (none returned). No owning-spec collisions; IDs 014 and 015 were free.
- Existing mixed rename, fresh-process clean/mixed impact and budget baseline passed in
  `internal/contextindex` and `cmd/corvint` before source edits.
- Focused regressions cover disjoint/partial/all-dirty sets, duplicate and CLI-normalized paths,
  full dirty-set membership beyond display sampling, rename endpoints, unknown commit identity,
  staged/unstaged/untracked dirtiness, committed-source canary exclusion, minimum budget,
  read-only repository contents, strict clean parity and provider bypass.
- Independent native reviewer found an early-abstention validation bypass: negative limits could
  panic and excessive limits/path counts were not rejected. Shared request validation repairs it;
  explicit invalid-limit and path-bound regressions retain the failure. Re-review confirmed the
  repair, with no remaining findings. The final state uses existing NEEDS_WIDENING to preserve
  the closed CCF-V1-007 enum; independent review also checked its budget compaction.
- Initial test development exposed the wrong CLI test helper signature and use of a nonnormalized
  path in the lower-level API; both fixture errors were repaired before the focused pass.
- Final focused tests passed with GOTOOLCHAIN=local, GOMAXPROCS=3 and the requested TMPDIR:
  `go test -count=1 -timeout 30m ./internal/contextindex ./cmd/corvint -run
  'TestImpact|TestFreshProcessImpact|TestEvalImpact|TestMinPacketBytes'`.
  Final package times: 27.797s and 22.162s. Log: `/private/tmp/v1-1141-tests-final.log`.
- All requested documentation gates passed: spec-requirements-check,
  requirement-definitions-check, traceability-tests-check, decision-numbers-check,
  line-citations-check, error-code-ownership-check, unbounded-readers-check,
  use-case-receipts-check and use-case-receipts-test. The first run caught shifted source
  citations; exact unchanged source spans were relocated with their original content anchors,
  then the gates passed. No new decision file or accepted-intent change was introduced.
  Final gate validation uses `/private/tmp/v1-1141-validation`; its Git index tree is checked
  unchanged across the complete run: `b8f707856ae54bcb6c09a431c0e12437e7b01700` before and after,
  exit 0. The earlier alternate-index log is retained as an
  invalidated validation attempt, not final gate evidence.
- `sh script/repin-use-case-receipts.sh` repinned the three receipts naming main.go and their
  ledger hashes. `sh script/repin-use-case-receipts.sh --check` reports all 18 receipts current;
  the repin test reports five cases passed. `git diff --check` passed.
- The affected plan retains language-frontier and unowned-document unknowns. Its generic full-gate
  advice is NOT_RUN under the owner's explicit no-make-gate/no-exhaustive-test instruction;
  focused results are not represented as equivalent exhaustive coverage.

## Local process limits and self-use

Used Corvint query for the task and affected against the immutable base before tests; outputs are
retained locally at `/private/tmp/v1-1141-query.json`, `/private/tmp/v1-1141-affected-baseline.json`
and `/private/tmp/v1-1141-affected.json`. Advice is not exhaustive coverage. Source reads and the
focused test selection resolve this bounded change; optional provider, learning, mutation and
release qualification routes are not applicable. No claim of external outcome validation is made.

Installed Core: 1.0.0-rc.3 build 407; Tasks: 0.0.0-tcp01-unverified+build.407. Official release
listing showed Core v1.0.0-rc.2 and Tasks tasks-dev-20260929.2. `corvint-update check --allow-network`
failed its version smoke with `process-after-start-failed: descendant snapshot unavailable`;
freshness remains UNKNOWN and no update or downgrade was attempted. Account usage telemetry was
not available through exposed tools (NOT_OBSERVED). Parent model/effort was inherited for this
bounded builder and one independent review; no cost or live-setting control is claimed.

Snapshot refresh and `make dogfood-change` failed because their shared Git-directory write paths
are outside this worktree's writable sandbox. The supported query fallback succeeded. Post-commit
CEM/OCM binding and sealing are NOT_PRODUCED: this builder is expressly prohibited from committing.
No ticket completion, publication, decision file or exhaustive gate was attempted.

Actual `git add` also failed at the external worktree index lock. The initial explicit disposable
index under `/private/tmp/v1-1141-git` was invalidated when a gate fixture inherited GIT_INDEX_FILE
and rewrote it. A final readback caught this; that combined run is not final evidence. Final
spec/REQUIREMENTS generation and all documentation gates instead use a disposable shared clone
at `/private/tmp/v1-1141-validation`, detached at the exact base with the task files copied and
staged normally, without Git environment overrides. Its full index tree is checked before and
after the run. Generated REQUIREMENTS.tsv is copied back into the requested worktree.
Actual staging remains an integration step for the owning coordinator. macOS mktemp ignores
TMPDIR for an implicit template here, so a temporary PATH wrapper supplies the approved temp
path for no-argument calls. No validation workaround grants extra filesystem permissions.
