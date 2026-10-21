# tools-1021c: SNR admitted toolchains and a structural JWT branch

Date: 2026-10-21. Lane: tools-1021c. Tickets: V1-2120, V1-1704 (delivered); V1-2122, V1-0450
(already delivered); V1-1091, V1-0288 (skipped). The requirement changes (SNR-V0-003 amendment,
LTA-V0-016) are accepted under the owner's standing auto-accept of lane requirements (2026-10-11).

## Problem

- V1-2120: `script/check-sql-native-ratchets.sh` pinned one Homebrew `go1.27.0` (binary, receipt
  and GOROOT digests plus `GOVERSION`). The repository pin is `go1.27.2` and decision 0547 admits
  `go1.27.3`, so every run failed a bare `test` under `set -eu` with no output, and the fixture
  test hard-coded a `/opt/homebrew/Cellar/go/1.27.0/bin` PATH.
- V1-1704: the writer secret screen's JWT branch matched any three dotted base64url-ish segments
  case-insensitively, so dotted adopter obligation ids (`A.B.C.R00001`) were refused as
  `SECRET_DETECTED`.

## Decisions

- SNR keeps content pinning (decision 0104). The fixed digests become an `admitted_toolchains`
  table keyed by `GOVERSION` and go-binary SHA-256; the matched row supplies the receipt-tool and
  toolchain-root pins. Every toolchain refusal is now typed: `status=REJECTED
  reason=TOOLCHAIN_MISMATCH check=<...>`, exit 78. The source-byte ceiling runs before the
  toolchain checks so its fixture is toolchain independent. Rows were measured on darwin/arm64 for
  the official module `go1.27.2` and Homebrew `go1.27.3`.
- The writer JWT branch now requires compact-JWS header structure: `\b` plus case-exact `ey`/`ew`
  (base64url of `{"` or `{` plus whitespace). A real JWS always starts that way, so detection of
  real tokens is not weakened. `StoredV1Pattern` is unchanged (schema-v1 rows keep their recorded
  classification); the dotted-triple prefilter remains a necessary condition. The parity fixture's
  synthetic "JWT" became a real header-shaped token. `internal/secretscreen` is an analyzer input,
  so `script/analyzer-schema.sh bump` moved the schema to `corvint-analyzer/136`.

## Evidence

- `sh script/check-sql-native-ratchets_test.sh` exit 0 under `go1.27.3` and `go1.27.2`; new test 5
  zeroes the digests and asserts exit 78 with `check=go-binary-digest`. `go1.27.1` refuses with exit
  78. A real `sh script/check-sql-native-ratchets.sh` on `go1.27.3` exited 0 (`source_bytes=64763
  binary_bytes=2832850 goroot_entries=17299 core_binary=identical candidate_max_allocs_op=393`,
  causal parent `NOT_RUN` as before).
- `TestLTAV0016JWTBranchRequiresHeaderStructure` fails without the prefilter change (the alias and
  other negatives matched) and passes with it; `./internal/secretscreen` passes.
  `TestTOLV0055_AliasLimit128` with a dotted alias fails without the fix (`SECRET_DETECTED`) and
  passes with it; `TestTOLV005[5-9]` pass.
- V1-2122: the failing nightly (run 39258757168, head 714393264) predates 7f46e42dc (V1-1780, keeps
  the helper's lock file reachable so GC cannot release the flock). Run 39416257984 at f3ca72248
  passed `internal/tasks/cli` on ubuntu-24.04.
- V1-0450: delivered by 88f63535b (`untracked-impact-path` code, recorded at info by the OpenCode
  adapter; `cmd/corvint/harness_untracked_test.go` and `integrations/host-adapters.test.mjs`).

## Skipped

- V1-1091: the in-process half was delivered by 05fa42d95 (`TestV11091IdentityStableAcrossBtimeJitter`).
  The remaining criteria need a GitHub Linux runner measurement and an owner compatibility decision
  for persisted identities (±1 s tolerance versus a `boot_id` migration).
- V1-0288: honouring `safe.directory` for the root relaxes Git's ownership protection across 63
  sanitized-environment builders; that security posture change needs an owner decision.

## NOT_RUN

- Linux toolchain digests for SNR: `NOT_OBSERVED`; Linux hosts refuse with the typed reason.
- `make gate`: not run (lane charter: focused tests only).

## Rollback

Revert this change. For SNR, restore the single pinned row; for LTA-V0-016, restore the previous
writer alternative and source digest. No stored state depends on either.
