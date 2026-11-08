<!-- SPDX-License-Identifier: Apache-2.0 -->
# Mergeprev merge-forward onto c3b29cbb54d3

Owner intent: merge the existing mergeprev lane forward onto pinned mainline
`c3b29cbb54d31c8526d3cce4ad74356b6fb9b943`, preserve both sides, and bind the
whole lane against that mainline. The lane was unsealed and must remain unsealed.
Original lane head: `978b955e9702f8c33d795d0823c020ed7b7b2868`.
Merge commit: `80dfb2e45722af706736bd20d06cc72cb8dceea7`.

Conflict resolutions:

- `Makefile`: keep the lane's three merge-driver installation recipes and
  mainline's spec-claims-check/spec-claims-test targets.
- `docs/specs/daily-change-evidence-workflow-companion-v0.md`: union the digest's
  Exists entries, retaining merge drivers plus rebind, seal review and handback.
- `docs/specs/compat-replay-runner-v0.md`: retain the citation's original content
  anchor and relocate its Makefile line after combining both sets of targets.
- `docs/specs/eol-policy-gate-v0.md`: retain both citation anchors and relocate
  their Makefile ranges after combining both sets of targets.

Generated `docs/specs/REQUIREMENTS.tsv` and `docs/specs/INDEX.json` were restored
from the merged mainline before the merge commit. The post-merge owning target
`make spec-requirements` generates REQUIREMENTS.tsv, INDEX.json and README.md
in one pass; it restores all six lane DWC-V0-7700..7705 rows. README entries from
both sides remain. No lane requirement ID collides with mainline; no renumbering.
The whole-range map also covers REQUIREMENTS.tsv's temporary projection resolution.

No Go file needed conflict resolution or regeneration, so the owner's focused
Go-package test set for this round is empty. The requested final checks are
`make -s spec-requirements-check line-citations-check`; broad selections, doc-gates,
Tasks writes, publication and sealing are excluded by the owner's round instructions.
Independent review of this composition and additional gates belong to the host
verifier after this round. Prior independent review and its parser repair remain
in the existing lane history. No new independent review is claimed here.

Binding uses the staged lane citation evidence, refreshed and reviewed for the
new base without removing evidence. All NOT_PRODUCED and NOT_OBSERVED reasons
remain in the dogfood report and the round handback. Exact command return codes,
final head and bind SHA are retained outside the tracked change in
`/home/agent/lanes/mergeprev.merge.handback.md`; raw round evidence is under
`/home/agent/lanes/mergeprev-evidence/merge-*`.

Rollback: revert the merge and this regeneration as a new change; preserve the
pre-round lane commits. No implementation or authority boundary changes here.
