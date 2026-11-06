# V1-4729: self-review repair

Date: 2026-11-06
Base: `9db7a94169821d9b28701eaa88866aee2fec1083`.
Repair input: three self-review findings at `7e7982b9e03759aec87d59b2da2d8ca98e946df8`.

## Decisions and requirement coverage

AZU-V0-9901 / AFP-V0-511: remove the global fixture-path suppression. A selected
Go host does not prove the absence of a computed reader in another language.
Every unowned path now reaches the existing conservative classifier, which
decides closure per plugin and frontier reason. Dynamic imports, path-open bounded sites,
unclassified reasons and plugins without `FrontierReach` stay open. Resolution
inputs, foreign fixture content and independently reached frontiers retain
their existing protections. The closure bounds fixture now declares a genuine
fixture-closed runner reason; its positive and negative assertions remain.
New computed-`require` and classification regressions cover the unsound cases.

AZU-V0-9900: compare indexed membership separately from filesystem existence.
An indexed addition removed only from the worktree still selects the named
ownership ratchet. Missing tracked worktree files remain a separate trigger;
untracked-only changes remain excluded. The membership table covers the exact
staged-addition/worktree-deletion combination.

AZU-V0-9900 takes precedence over general AZU-V0-9901 guidance for content-only
edits. Declaration hints require a committed path whose membership changed, as
well as the repository's ratchet. The compatibility regression now covers both
owned Go source and an existing committed unowned file, comparing serialized
plans and the compact digest used by `test --dry-run`. The graph-level no-change
regression also observes committed paths without membership changes.

V1-4729 remains PARTIAL for general guidance that conflicts with the frozen
serialization contract, host qualification and native completion. No JSON
shape, error/exit contract, stderr text, golden, baseline or existing acceptance
assertion is weakened. Rollback restores the three reported defects; no data
migration or wire revision is introduced.

## Verification and evidence

Raw evidence stays under `/home/russell/lanes/ratchetsel-evidence/self-review-*`.
The three CLI regressions fail on the pre-repair implementation with exactly
the missing ratchet, changed content-only plan and lost computed-import frontier
reported by review. The existing pre-change capture is reused.

Selection used `corvint affected --base 9db7a94169821d9b28701eaa88866aee2fec1083`
and selected both touched Go packages. Its installed-binary stale notice is
retained: that receipt establishes installed-binary selection only. Scope was
UNKNOWN, with three language-frontier and two unselected-binary-consumer unknowns.
A subsequent tree binary built through `heavy.sh` selects the same two touched
packages with those same unknown counts and no stale-binary notice
(`self-review-affected-current.json`); no wider test-coverage claim is made.

Focused CLI ownership and frozen-profile checks pass (2.027 seconds); focused
ownership checks in the affected package pass (6.278 seconds). The complete
affected package passes without a filter (131.747 seconds), including existing
performance and ratchet checks. `make -s spec-requirements-check line-citations-check`
passes. The citation repin moves one `select.go` anchor by three lines with its
content hash unchanged; a subsequent repin reports no further move.

The first complete CLI run fails in 368.807 seconds with the same 45 top-level
failure names as `repair1-cli-whole.log`, no added or removed failure name, and
no ownership-test failure. That run overlapped finalizing the compatibility fixture to keep the original
substantive Go edit alongside the new unowned-file case, so a second complete
CLI run qualifies the final test source rather than reusing that run as
final-source evidence.

The final complete CLI run fails in 291.527 seconds with exactly those same 45
failure names. No ownership test or frozen-profile test fails. Retained failures
include ancestor Git discovery and profile paths under `/home/russell/lanes`,
private scratch/executable qualification, and the pre-existing migration-record
digest mismatch. Prior evidence reproduces three representative sandbox failures
at the base, not all 45. Host rerun remains required; these unrelated checks and
assertions are preserved. The local outcome is **failed**, while all three
reported repair findings are fixed and their focused regressions pass. Earlier
historical replay measurements are not requalified by this repair; no prior
post-change frontier count is presented as a current result.

All Go tests use `script/heavy.sh`, one package per command, the prescribed
mode-700 TMPDIR and no identity exports or semaphore override. Whole-repository
selection/tests/vet, doc-gates, model-backed review, native task writes,
publication and sealing are NOT_RUN under the charter. The orchestrator owns
independent review and deferred host qualification. No new pre-change capture
is produced; the existing capture is retained. Final binding/checking follows
verification, with no seal and no CEM claim that structural closure proves
passing tests or ticket completion.

| Evidence | SHA-256 |
| --- | --- |

| `self-review-regressions-before.log` | `bbd0ba3bb67979f4d8caf3108ecd0c06a608f756888995bea1ce83c89c7d344c` |
| `self-review-cli-focused.log` | `ad065f03ea1c4e634e464169d5819ce9d3ad1793ef7b4ac7a8e1e3655dc8bcf4` |
| `self-review-affected-focused.log` | `1a86d063fcd69a55e36c50851796914c764c93bdb320135acb363ad66c9ab4b6` |
| `self-review-affected-whole.log` | `1bc5d2d6c1796448a774913c9324acd431db2a113acff98b1d64bdbb927373a3` |
| `self-review-spec-citations-check.log` | `a83d2cde330a8edaa586b933b0a002bbc622d7b2ebb0a8373c21da7e377eaa8e` |
| `self-review-cli-whole.log` | `9d130d0920b09882e10f92511b4f137076d64a3d13b8b01327fb1bb409bd40f4` |
| `self-review-affected-current.json` | `11b851f95a021874fa610363f28e72791c3e8993f3d4f4762b217ca3fd34c228` |
| `self-review-cli-whole-final.log` | `5b88e4fa9f668f2235747f254478894f0dac73a8b8ff8ea211eb883d9964a96b` |
