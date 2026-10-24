---
name: corvint-review
description: Review a committed change against its evidence, proofs and gaps. Use before opening or approving a pull request, or when asked whether a range is proven.
license: AGPL-3.0-or-later
---

# Corvint review

All calls use `corvint_core_read`; `base` is a full 40-character commit ID.

1. `{"operation":"review","input":{"base":"<base>"}}`: affected advice and branch overlap hints.
2. `{"operation":"witness","input":{"base":"<base>"}}`: the unwitnessed surface of the range.
   The range always ends at the checked-out HEAD: `head`, when given, must be that same commit,
   and any other commit is refused. A dirty worktree is refused, not guessed.
3. Proof state, when the change has a CEM map: `cem-status` (`map`, `base`, `target`), then
   `ocm-status` and `frontier` as the receipt directs.

Quote each receipt's open items, refusals and `NOT_RUN` entries. A tool success is not a review
verdict, and no read grants completion or authority.
