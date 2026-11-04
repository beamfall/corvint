# Lane Verification Tooling V0

Owner: Russell Lewis
Date: 2026-11-03
Requirement prefix: `LVT-V0`
Intent status: accepted under the owner's standing lane auto-accept (2026-10-11)
Delivery status: experimental
Authoritative inputs: owner lanetools charter (2026-11-03), V1-4046, V1-4252, V1-3570,
V1-4319; `AGENTS.md`; `daily-change-evidence-workflow-v0.md`; `corvint-self-development-v0.md`.

## Agent digest
- Claim: Lane verification reserves requirement IDs, batches immutable Git reads, and cleans private host-test scratch safely.
- Status: accepted under the owner's standing lane auto-accept (2026-10-11) / experimental
- Exists: requirement-ID scripts, static gate scripts, heavy admission wrapper and native Pi host tests.
- Blocked on: independent review and terminal dogfood evidence.
- Read next: Requirements; Acceptance and evidence; Failure modes and rollback.

## User job and baseline

The owner needs fast, sound local and CI verification. At base
`6f12f4704235dc8f9bea19fc12995bfb68c746c3`, range checks reject four-digit IDs,
no reservation survives until a lane commits definitions, several checks spawn Git once per
file or history commit, and Pi test scratch survives interrupted runs. The workflow spec has
256 obligations and self-development has 254. This companion uses a new prefix under the
charter's explicit capacity exception; it does not expand either parent beyond 256.

## Requirements

- `LVT-V0-001`: (experimental under the standing lane auto-accept, 2026-11-03; V1-4046)
  Requirement block checks MUST accept endpoints 0..9999, spell numbers below 1000 with three
  digits and larger numbers without padding, and compare complete canonical ID tokens. A four-digit
  ID MUST NOT occupy its three-digit prefix; noncanonical zero padding MUST NOT occupy a canonical ID.
- `LVT-V0-002`: (experimental under the standing lane auto-accept, 2026-11-03; V1-4046)
  Block and whole-family availability MUST check origin/main (or explicit REF), unmerged local
  branches, remote-tracking branches and worktree heads sharing its history. Missing or malformed
  required evidence MUST refuse rather than declare availability. Whole-family checks cover versions.
- `LVT-V0-003`: (experimental under the standing lane auto-accept, 2026-11-03; V1-4046)
  `requirement-ids-reserve` MUST serialize check-and-reserve through an exclusive directory lock
  and atomic ledger replacement; reserve only IDs passing base and open-ref checks, reject another
  owner's overlap, and make identical owner reservations idempotent. The private shared ledger is
  `git-common-dir/corvint/requirement-id-reservations.tsv`, overridable with
  `REQUIREMENT_ID_RESERVATIONS`. Release removes only matching owner rows. Lock contention refuses
  immediately. This ledger grants no product evidence, ranking or authority.
- `LVT-V0-004`: (experimental under the standing lane auto-accept, 2026-11-03; V1-4046)
  Availability MUST also check the reservation ledger read-only; absent means no reservations,
  `OWNER` excludes only that owner's reservations. Whole prefixes overlap every version of their
  family; ranges overlap within one version. Malformed or unreadable ledgers refuse. Outputs are
  `used`, `free`, `reserved`, `unreserved`, `reserved-ok`, `already-reserved`, and `released N`.
  Refusals exit 2 with usage/recovery: `bad-arguments`, `bad-prefix`, `bad-owner`, `bad-range`,
  `missing-owner`, `missing-base`, `unknown-command`, `not-a-checkout`, `git-dir-unavailable`,
  `ledger-unreadable`, `ledger-malformed`, `ledger-dir-unwritable`, `ledger-unwritable`,
  `ledger-locked`, `scratch-unavailable`, `bad-family`, `bad-block`, and `bad-argument`.
- `LVT-V0-005`: (experimental under the standing lane auto-accept, 2026-11-03; V1-4252)
  Host package version history MUST batch immutable blob reads without changing first-parent
  field-change verdicts. Unreadable batches MUST fall back to the previous per-commit algorithm;
  an unbumped shipped change MUST still fail. Fixture Git spawns MUST drop by at least 50%
  against the same fixture suite using the baseline implementation.
- `LVT-V0-006`: (experimental under the standing lane auto-accept, 2026-11-03; V1-4252)
  Fixed-temp-path, Mkfifo and line-citation checks MUST batch staged or immutable blob reads,
  retaining path/line diagnostics,
  exemptions and build-constraint decisions. A failed read MUST fail closed. Git spawn count MUST
  stay constant as candidates grow; unstaged fixes MUST NOT change staged verdicts.
  Fixed-temp-path, Mkfifo, error-code ownership and ordinary line-citation checks MUST use at
  most three Git spawns. Line-citation historical content reads MUST share its existing batch.
- `LVT-V0-007`: (experimental under the standing lane auto-accept, 2026-11-03; V1-4252)
  Directory-local error-code discovery MUST read its existing staged scratch tree without one
  Git process per declaring directory and preserve emitted sites and ownership failures.
  Both the staged enumeration and read failures MUST fail closed.
- `LVT-V0-008`: (experimental under the standing lane auto-accept, 2026-11-03; V1-3570)
  Remote heavy execution MUST retain package parallelism bounds and explicit caller/environment
  GOMAXPROCS precedence. A changed default MUST be justified by same-build measurements; unavailable
  remote qualification MUST remain NOT_PRODUCED and retain the existing default.
- `LVT-V0-009`: (experimental under the standing lane auto-accept, 2026-11-03; V1-4319)
  Native Pi host scratch MUST be removed on normal exit and catchable INT/TERM/HUP. On startup,
  safely sweep only private, same-user, nonsymlink directories with a valid harness owner marker
  older than 24 hours whose owner process is demonstrably absent. Live, ambiguous, malformed,
  unmarked, foreign or symlinked directories MUST survive. SIGKILL cannot run cleanup; sweep is
  recovery. Tests MUST use a temporary sweep root, never delete arbitrary /tmp prefixes.
- `LVT-V0-010`: (experimental under the standing lane auto-accept, 2026-11-03; owner friction)
  Dogfood finish static precheck MUST use explicit CORVINT_BIN, otherwise SKIP with a reason;
  it MUST NOT let an unrelated installed binary refuse a tree change. Prechange impact failure
  MUST retain the failing command's path diagnostic. A cheap worktree spec-Claim check MUST name
  file and line and reject Claims longer than 160 characters before generation.

## Non-goals and authority

No test-plan narrowing, CI policy change, daemon, network service or ticket mutation. These
checks do not claim semantic completeness. Reservation state is host local, not a distributed lock.

## Acceptance and evidence

| Requirement | Witness |
|---|---|
| LVT-V0-001 | `script/lane-scripts_test.sh` requirement IDs group; `script/requirement-id-collisions_test.sh` |
| LVT-V0-002 | `script/requirement-id-collisions_test.sh` remote block/family/collision controls |
| LVT-V0-003, LVT-V0-004 | `script/requirement-id-reservations_test.sh` overlap, release, worktree and malformed controls |
| LVT-V0-005 | `script/check-host-package-versions_test.sh` history growth and unbumped-change injection |
| LVT-V0-006 | `script/check-fixed-tmp-paths_test.sh`; `script/check-mkfifo-build-tags_test.sh`; `script/check-line-citations_test.sh` |
| LVT-V0-007 | `script/check-error-code-ownership_test.sh` staged ownership violations |
| LVT-V0-008 | `script/lane-scripts_test.sh` explicit overrides and invalid-value control; measurements in build log |
| LVT-V0-009 | `integrations/pi/host-scratch.test.mjs` ownership and interrupted-exit fixtures |
| LVT-V0-010 | `script/lane-scripts_test.sh` static group; `TestPrechange`; `script/check-spec-claims_test.sh` |

Retain same-build before/after git counts and timings on real recent origin/main ranges in
`docs/build-log/2026-11-03-lanetools.md`. Every skip/reuse keeps its negative control and fault
injection. Missing remote, native-host or independent-review evidence remains explicit.

## Failure modes and rollback

Unreadable Git evidence fails closed or uses the previous full algorithm. Locks never imply free
IDs. Owner-PID reuse conservatively preserves stale scratch. Sweep refuses ambiguous ownership and
permissions. Revert each script change independently; reservations may be released explicitly, and
unrecognized scratch stays for manual inspection. Promotion needs focused checks, doc gates,
independent review and post-commit dogfood-check; the lane stops before seal per owner instruction.
