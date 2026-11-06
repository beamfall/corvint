# Unbounded verifier repair 1 — 2026-11-05

Owner intent: the unbounded lane's Verifier repair 1 instruction; base
`9db7a94169821d9b28701eaa88866aee2fec1083`. No ticket was supplied; Tasks writes are forbidden.
Requirements: AZU-V0-002, AZU-V0-9931, AFP-V0-023 and AFP-V0-088.

## Failure and repair decision

The host's existing `TestOwnershipAuditRatchet_AZU_V0_002` failed at
`3af19d3b92a2f500f4e71a05aa03065778b1583b`: 807 newly unowned ticket records and one stale
baseline record, `.taskman/releases/v0-9.json`.

The new `internal/tasks/cli` read scope granted `.taskman/tickets/`. AFP-V0-088 deliberately
keeps a named projection path unknown when any declared reader holds it. This scope therefore
made every ticket record a test input rather than exempt task state. Conversely, bounding that
package excluded the release fixture literal that previously resolved to the tracked release
record, making its existing ownership baseline entry stale. The relevant ticket/release tests
use temporary fixture repositories; this repair does not assert that they read the real store.

Remove only the newly added Tasks CLI scope entry, restoring its conservative unbounded behavior.
Do not declare fixture-only paths as real runtime readers or grant a release read merely to keep
an obsolete baseline entry. No ownership code, JSON grammar, wire shape, golden, baseline, test
assertion, error, exit or stderr contract changes. The other five added read scopes and both
comparison-only locator implementations remain intact.

**AZU-V0-9931 is PARTIAL for Tasks CLI narrowing:** the external constraint is the owner's frozen
ownership baseline/assertion contract. Separately authorizing its fixture/ownership integration
is required before narrowing this package; this round retains its widening floor. AZU-V0-9930
and the five other bounded consumers remain delivered experimentally. No new ticket or native
completion write is attempted.

This entry supersedes the Tasks CLI declaration row in
`2026-11-05-unbounded-reader-precision.md`. Its six-declaration replay results remain historical
measurements of that exact overlay, not measurements of this repaired five-declaration tree.
New replay selection is NOT_RUN because this repair round forbids `corvint test` and whole-repository
selection. No unchanged replay gain is asserted for the repaired tree.

## Verification and limits

The pre-change capture is reused as directed; no start capture was rerun. The retained
`corvint-after affected --base 9db7a94169821d9b28701eaa88866aee2fec1083` plan selects both changed
scanner packages directly, and the affected parent and read-scope checker by dependency. Scope
is UNKNOWN: 360 selections, LANGUAGE_FRONTIER=1 and UNSELECTED_BINARY_CONSUMER=2. This binary
contains the scanner changes but was built with a base/dirty stamp, so its stale-binary warning
is retained in `repair1-affected.stderr`; no source or tooling was changed after that build.

The exact failing ownership test now passes (8.310s), without any baseline or assertion edit.
Whole existing packages pass through `script/heavy.sh`, one package per command, with
`GOTOOLCHAIN=local go test -count=1 -timeout 30m`: `internal/liveverify/affected/golang`
(17.345s), `internal/gateselect` (3.870s), and `tools/test-read-scopes-check` (6.420s).
The whole `internal/liveverify/affected` package also passes (219.825s), including the unchanged
ownership ratchet. Author/committer identity variables were absent during tests. Go toolchain is `go1.27.3`;
TMPDIR is the mode-0700 `/home/russell/lanes/unbounded.tmp`. Standard heavy.env lock admission
succeeded without overrides; the Go scanner command waited 5s for a slot.

`script/check-line-citations.sh --repin` found no moved citations.
`make -s spec-requirements-check line-citations-check` passes, including its existing specindex
and focused lrfrepo checks. Its specindex test waited 95s for host load admission; no semaphore
override was used. The 180 existing untracked-basename citation skips remain visible.

Final CEM/check outcome is recorded in the lane handback.
NOT_RUN: repository-wide gates, doc-gates, selection-baseline-check, gate-affected, `corvint test`,
Windows vet (no push), seal and publication, per owner instruction. Independent review is
NOT_PRODUCED here and belongs to the orchestrator. Existing guarding-runner and coverage limits
must remain visible in the refreshed CEM report.

Raw logs remain outside the repository under `/home/russell/lanes/unbounded-evidence/`.

## Evidence digests

SHA-256; raw files are in the external evidence directory above.

- `repair1-affected.json`: `d593248356c32b2372b46e7f93003ce12f6112a0fd0085666ec8aeb007c21783` (142879 bytes).
- `repair1-ownership-focused.log`: `bf5f8bda35d6aee9cda1457f60c090d632e9be4d2e835d26b27d18ec795e4710` (794 bytes).
- `repair1-affected-whole.log`: `5d328f3a23f16e068ac7a5a45791b8b38c2b841ed5999f4393c64277c51f4a65` (837 bytes).
- `repair1-golang-whole.log`: `47a98baac4b822cdc50bc3708b2af2e5153ca3dbf417366f773c45419f22b39c` (649 bytes).
- `repair1-gateselect-whole.log`: `6a0906152b2221732410dcf206938fe8ced8f0579e98d4cd74dca9b756dbc200` (592 bytes).
- `repair1-read-scopes-whole.log`: `6b8b05092d5e0af290ed39e2f15a8a85bffc1f652f4b9a00ce333100550f3ba4` (601 bytes).
- `repair1-spec-citations.log`: `5f36cb7c328e775311f4f7514dd963bb9d3d03d184973f299fb1d85fe8c6811c` (3376 bytes).
- `repair1-repin.log`: `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133` (190 bytes).
