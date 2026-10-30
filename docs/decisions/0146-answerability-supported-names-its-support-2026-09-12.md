# Decision 0146 — Answerability `supported` names its support

Date: 2026-09-12. Status: accepted. Authority: repository owner instruction, 2026-09-12 via delegation.

Decision 0068's accepted amendment used `supported` for every packet on which its withholding rule
did not fire. That label therefore covered both a source-backed conjunction and cases with no joint
source support, including one unknown name and known names scattered across separate sources. The
packet already computed the best-supporting sources, but rendered them only after replacing them
with the rows withheld by `unsupported-conjunction`.

The owner call: a `supported` verdict must name its support. `TCP-V0-016(d)` now reserves it for a
packet where at least one indexed source reaches the reported `required` support. Its
`nearest_claims` are the existing bounded best-supporting sources with their pinned blob hashes and
exact `supports` and `lacks` sets. When the withholding rule does not fire but no source reaches
`required`, the verdict is `not-withheld` and `nearest_claims` is empty. This decision amends
decision 0068's amendment; `unsupported-conjunction`, its withheld-row claims, and the withholding
rule are unchanged.

The analyzer schema advances from `corvint-analyzer/30` to `/31` because task-context packet bytes
change. The default recipe golden is re-captured through its package test fixture; the intended
golden delta is confined to `coverage.answerability.nearest_claims`.

Rollback: restore decision 0068's broad `supported` meaning, stop rendering computed support claims,
remove `not-withheld`, and restore the prior analyzer schema and packet golden together.
