## 2026-09-26 V1-0386 DCW-V0-019: a rerun that cites nothing new keeps the committed map's bytes

Found by the V1-0019 chi smoke-006 loop control `L3c-reencoded-control`. That control rewrites the
bound `.corvint/change.cem.json` as compact JSON with identical content, commits it on top of the
bind, and reruns `corvint dogfood change BASE`. The rerun cited the same plan again. `cem cite`
publishes its output in the indent-2 encoding even when every citation is already carried, so the
sidecar was left modified. As a result `cem-status` failed `excluded-artifact-mismatch`,
`local-outcome` failed `record-index-failed`, and `dogfood check` refused `dirty-worktree`. The fix
from V1-0383 had already made the compact map bind; this defect only showed up on the rerun.

Decision: the CEM contract fixes strict JSON, not a byte layout. The cite pass in `dogfood change`
therefore reads the map before the pass. If the map it leaves decodes to the same JSON value, it
atomically restores the earlier bytes, keeping the file mode. Numbers compare by their literal text.
A pass that adds a citation still publishes the cited map. `cem cite` itself is unchanged, so its
Python-oracle byte parity holds.

A restore at the dogfood layer was chosen over making `cem cite` skip a no-op write. With a
multi-row plan, the last row writes the stage onto the map, so a no-op check inside `cem cite`
would not cover the committed map.

Evidence: `TestDogfoodChangeKeepsTheEncodingOfACommittedMapItCitesNothingInto` (built binary). The
rerun exits 1 without the fix. With the fix the rerun exits 0, the map stays byte-identical, the
worktree is clean, and check passes. The DCW-V0-019 neighbours pass. Rollback: revert the change;
a re-encoded committed map is then rewritten on the next rerun and must be recommitted.
