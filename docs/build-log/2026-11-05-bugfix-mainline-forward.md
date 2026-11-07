<!-- SPDX-License-Identifier: Apache-2.0 -->
# Bugfix mainline merge-forward

The owner requested merging mainline cdeed7ee6396a8d6bf6225eb5b2b4cb4d601b419
into lane head 5e06aba141c7776a5d335a466a55ad478d56e901, retaining both sides'
behavior and rebinding against that mainline commit. Merge commit 5ff27fb6e
preserves both parents. No earlier commit is rewritten; no publication, Tasks
writes or seal is authorized in this round.

Conflict resolutions:

- `cmd/corvint/pi_adapter.go`, `integrations/compatibility.json`,
  `integrations/pi/compatibility.json`, `integrations/pi/package.json`,
  `integrations/pi/runtime.js`, and `docs/specs/pi-workflow-v0.md` retain the
  lane's synchronized Pi adapter 0.3.14 pins over mainline's 0.3.13 pins.
- `docs/SPEC-TOOLCHAIN-INTEGRATION.md` and
  `docs/specs/FRONTIER-DECISION-BRIEF-2026-08-29.md` retain the lane's relocated
  citations to the combined live-verification spec. Both prose intents survive.
- `docs/specs/live-proof-carrying-verification-v0.md` retains the lane's stronger
  LPCV-V0-7601 child-build proof and counter-receipt distinction and mainline's
  surrounding requirements. Overlapping newly indexed requirement IDs denote
  shared additions, not unrelated requirements; no renumbering is needed.
- `tools/linecov/watch_linux.go` combines the lane's recursive watch restoration
  and conservative lost-event handling with mainline's Git-ignored non-Go output
  filtering. Filtering uses the directory map before refresh; watches are restored
  before a relevant notification is published. Ignored Go source remains relevant.
  `tools/linecov/watch_linux_test.go` retains both sides' regression tests.

The generated requirement and spec index files were taken from mainline before
committing the merge. `make spec-requirements` owns REQUIREMENTS.tsv, INDEX.json
and README.md and passed; commit 081d2f846 restores the lane's registry rows.
The merged README retains both sides' entries.

The first focused tools/linecov package run failed to compile: the automatic
merge duplicated the execfile import in deadline_test.go. This repair removes
only that duplicate; both executable fixture changes remain. Verification logs,
exact final commit IDs, local outcome and missing-evidence reasons are retained
in the operator-requested `~/lanes/bugfix.merge.handback.md` and private scratch
artifacts. Scope is the two packages holding resolved Go files and doc gates;
repository-wide and live qualification are not produced by this round.
