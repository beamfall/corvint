# rspt11py merge-forward to 9761ea1f

The requested merge-forward preserves lane head
`974af48f39bc826934e5b8a86d0aee543967dec6` and merges mainline
`9761ea1f459c0d3b3c963953e24a76eaeda16370` in merge commit
`935d446c87b91998e2e66db0ee297da788657281`. The old base was
`8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734`; subsequent dogfood commands
use the merged mainline commit as BASE (docs/DOGFOOD.md merge-forward route).
The lane was unsealed, so this round must stop after a passing dogfood-check
and must not seal, publish, rewrite prior commits or write Tasks state.

## Resolution and derived files

- `docs/specs/core-compatibility-freeze-v1.md`: the sole Git content conflict
  contains three citation rows with unchanged enum contracts. Keep both
  sides' citation moves: affected.go lines 228 and 251 from the lane, and
  prove.go line 1791 from mainline. The existing content hashes still match.
- `docs/specs/REQUIREMENTS.tsv`: select the mainline version for the merge,
  then regenerate with `make spec-requirements BASE=9761ea1f459c0d3b3c963953e24a76eaeda16370`.
  The generator restores the runner-selection requirements and language-plugin
  amendment while retaining mainline requirements.
- `docs/specs/INDEX.json`: select the mainline version for the merge, then
  regenerate with that same owning target. It generates both files and the
  README table atomically; the README already preserves both sides' entries.

The last two paths are also reported by `script/dogfood-merge-base.sh --paths`
because the committed merge deliberately selected mainline derived content.
No requirement-ID collision occurred and no ID was renumbered. No Go file
conflicted or was regenerated. Inspection of the automatic affected.go merge
confirms both the lane runner-shadow advice and mainline import-set changes
remain. Existing commits and sealed maps remain intact.

## Observed checks and limits

- `corvint affected --base 9761ea1f459c0d3b3c963953e24a76eaeda16370`: rc 0.
  The installed binary reports `self-binary-stale` (built from the old base);
  its scope is its own result, not evidence for the current binary.
- `make spec-requirements BASE=9761ea1f459c0d3b3c963953e24a76eaeda16370`: rc 0,
  including its required obligation-bound check. This is also the Makefile's
  spec-index generation target; there is no separate index target.
- `make -s spec-requirements-check line-citations-check`: rc 0. Its specindex
  and focused lrfrepo checks pass. Existing exclusions remain: 180 basename
  citations skipped, and no Darwin runtime evidence from the Linux host.
- Conflict-resolved/regenerated Go package tests: NOT_RUN, because there are
  no conflict-resolved or regenerated Go files. No whole selection or
  repository-wide gate was run; the host verifier owns further validation.
- Initial `make dogfood-change BASE=9761ea1f459c0d3b3c963953e24a76eaeda16370`:
  rc 2, `sealed-cem-in-change`, constituent CEM policy state `unsupported`,
  code `unsupported-process-containment`, bind
  `3f32d7ad7ca93f9328cb492d53f3f08dc347b1bb`, path
  `.corvint/changes/3f32d7ad7ca93f9328cb492d53f3f08dc347b1bb.cem.json`.
  This is the same containment refusal retained by the preceding repair.
  It occurs before preparing or citing this round's CEM. Fresh binding and a
  passing final check are NOT_PRODUCED; the inherited map is not removed or
  weakened. Final attempts and exact return codes belong in the external
  merge handback. No local outcome is promoted from this refusal.

Raw round evidence is retained under `/home/agent/lanes/rspt11py.tmp/` as
`merge-affected.json`, `merge-affected.stderr`, `merge-regenerate.log`,
`merge-focused-gates.log`, and `merge-change-initial.log`. The requested
handback records final command outcomes and commit IDs. Rollback can revert
this round's new commits, including the merge with its lane parent selected;
no earlier lane commit needs rewriting. Verification must resume on a host
that can establish process containment, with the same pinned mainline BASE.
