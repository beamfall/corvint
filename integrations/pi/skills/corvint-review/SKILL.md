---
name: corvint-review
description: Review a committed change against its evidence, proofs and gaps. Use before opening or approving a pull request, or when asked whether a range is proven.
license: Apache-2.0
---

# Corvint review

All calls use `corvint_core_read`; `base` is a full 40-character commit ID.

1. `{"operation":"review","input":{"base":"<base>"}}`: affected advice and branch overlap hints.
2. `{"operation":"witness","input":{"base":"<base>"}}`: the unwitnessed surface of the range.
   The range ends at the checked-out HEAD unless `head` names an earlier commit; that report is
   read from the Git object store and says `headBinding: COMMIT-PINNED`. A head that does not
   descend from `base` is refused. A dirty worktree refuses a checked-out head, not guessed.
3. Proof state, when the change has a CEM map: `cem-status` (`map`, `base`, `target`), then
   `ocm-status` and `frontier` as the receipt directs.

Quote each receipt's open items, refusals and `NOT_RUN` entries. A tool success is not a review
verdict, and no read grants completion or authority.

Other Corvint capabilities: the catalog at
https://github.com/beamfall/corvint/blob/main/docs/CAPABILITIES.md gives each command, binary and
MCP tool with its maturity and refusals.
