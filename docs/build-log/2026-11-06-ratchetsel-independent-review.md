# Ratchet selection independent review repair — V1-4729

## Findings and decisions

The supplied independent review requires three repairs under AZU-V0-9900 and
AZU-V0-9901. The original pre-change capture is retained; this repair produces
no fresh capture and does not amend the frozen JSON, exit/error, stderr or
golden contracts.

AZU-V0-9900: retain the exact indexed-path membership used by selection, outside
the immutable graph, and re-read it after the final source observation before
emitting a receipt. Drift or a failed membership observation refuses through
the existing worktree-drift error and text. The regression stages an initially
untracked file from `affectedPlanSourcesObserved`: dirty path names and HEAD
stay the same, but emitting a stale plan is rejected. Its control selects the
named ratchet after staging has settled.

AZU-V0-9900: FULL discards narrowing patterns while retaining the ratchet's
named obligation and triggering path. The internal `GoSlice.FullReason` input
carries only that obligation across FULL; ordinary slice explanations continue
to disappear as before. This avoids changing unrelated frozen fallback output.
The public dry-run regression deletes the last source of another package,
leaving its directory present so AFP-V0-821 cannot close the unknown by proving
that the whole package directory disappeared. It checks the ratchet name,
path, reason, whole-package unit and absence of `-run`. The sessiontest regression
also checks independently reached whole-package obligations and input immutability.

AZU-V0-9901: use the existing `affected --format text` diagnostic channel for
unconditional declaration guidance on every unowned-path line. Existing committed
unowned content edits receive guidance with and without the ratchet; JSON hints
retain their compatibility boundary. Text preserves the reason and names all
three declaration files with their uses. Help, catalog and spec document this
channel. No requirement exemption or owner amendment is needed. The previous
general-guidance implementation gap is resolved; outstanding host qualification
and prohibited native task completion remain explicit.

Rollback removes these three repairs, restoring the reviewed drift, missing FULL
explanation and missing text guidance. It requires no migration. No existing test
assertion, ownership baseline, committed golden or core-freeze output is changed.

## Verification and limits

Selection: the tree-built binary's `affected --base
9db7a94169821d9b28701eaa88866aee2fec1083` selects all three qualified packages.
Scope remains UNKNOWN: three LANGUAGE_FRONTIER and two UNSELECTED_BINARY_CONSUMER
entries. The initial installed-binary stale notice is retained; the current-tree
receipt has no such notice. Toolchain is go1.27.3; every explicit Go test uses
heavy.sh, one package per command, the prescribed private TMPDIR and no identity
exports or semaphore overrides.

The first new regression run reproduces index drift and missing text guidance.
Its FULL fixture mistakenly removed the entire package directory, permitting
AFP-V0-821 attribution and NARROWED scope; it does not establish the FULL defect.
The corrected fixture retains a non-source file in that directory. Focused CLI
ownership, public FULL dry-run and frozen-profile checks then pass (3.036s), as
do focused sessiontest checks (0.010s). Those focused runs preceded the final
restriction to explicit FullReason obligations; the following whole-package
runs cover the final code and every existing package test without a filter.

| Whole package | Result | Package time |
| --- | --- | ---: |
| `internal/sessiontest` | PASS | 4.499s |
| `internal/liveverify/affected` | PASS | 199.532s |
| `cmd/corvint` | FAIL | 356.835s |

The CLI run retains exactly the prior round's 45 top-level failing test names,
with no added or removed failure name and no ownership or frozen-profile failure.
Observed limits include ancestor Git discovery below `/home/russell/lanes`,
private scratch/source qualification, and the legacy migration-record digest
mismatch. Earlier evidence proves three representative sandbox failures on the
base, not all 45. Host qualification remains required. The local outcome stays
**failed**; implementation repair is not a passing whole CLI qualification.

`make spec-requirements` reports no generated changes.
`script/check-line-citations.sh --repin` after staging moves four help citations
in two documents with identical content hashes; a second invocation finds no
further move. `make -s spec-requirements-check line-citations-check` passes,
including its own heavy-wrapped specindex and lrfrepo checks. The latter queues
95 seconds under the unchanged host load policy. An earlier unstaged repin
only reports excluded inputs; it is superseded by the staged runs. Gofmt and
staged diff whitespace checks pass.

Final CEM binding/checking follows this evidence with no seal. Guarding-runner
receipts, whole-repository qualification, model-backed review, publication,
native task writes and historical replay reruns are NOT_RUN by charter. Existing
pre-change receipts, CEM/OCM unknowns and transient replacement failures remain
visible in the bind report. Independent review and host verification belong to
the orchestrator. V1-4729 remains open for qualification and the prohibited native
completion write, with no remaining implementation gap from these three findings.

Raw artifacts remain outside the repository at
`/home/russell/lanes/ratchetsel-evidence/independent-*`.

| Evidence | SHA-256 |
| --- | --- |
| `independent-affected-current.json` | `39c5cb1561d50921cbceec5ded027615e4ec40313738c863a6ba0b18edc882a5` |
| `independent-before.log` | `1a2d468cc2bf74fef51037b55db7849c828a60d594487cf93f3e20b26c0cafa9` |
| `independent-cli-focused.log` | `61795ca2c4d87c120b9f67e2d64d608f9552e651911215e83a97de9ab3418fce` |
| `independent-cli-focused-final.log` | `7b7bb48a3d2b3c60221655ba56ce679124c0e2561c58ded3e505e8be63e89b8c` |
| `independent-session-focused.log` | `87e42164f0035565805680cbbbb6745cc8ee11fd7981cb906fe6b5ba5f3eb7c7` |
| `independent-session-whole.log` | `710d6f991a0f0c6a483a014cb0744e4f1bba9eb6d26bb9ff80e22cb7e2f3b417` |
| `independent-affected-whole.log` | `3254fb213e41b687c80045f20cd9bdcadcded98c7d73c5d7a1d820b75aa78efb` |
| `independent-cli-whole.log` | `4267a5d674e333b61131bab0bacedb3d16c695b66095b9d3d753c7fcdbb71ded` |
| `independent-spec-citations.log` | `ccbf05477ebafdccb733750cdfa91e8139a49713cb2e427565d29e60b9a6df41` |
| `independent-repin-staged.log` | `0c1ebc1b084290a0c6c51b2d447f8adf3b8fad7219870ef06ae7daa3d1f82638` |
| `independent-repin-second.log` | `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133` |
