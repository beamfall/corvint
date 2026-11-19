# Go qualification control exclusions (V1-5709)

Under the experimental GLS-V0-060 rule, the index commit fixes the declaration,
its observed root-module package keys and the root files it can cover. The
checkout must reproduce the committed declaration before the baseline runs.
An untracked declaration or a misspelled reader otherwise lets the same index
qualify a different set of controls. The gate selector already validates
package ownership against a commit tree. Qualification reuses that validation
with the root module as owner and binds the parser blob under GLS-V0-034.

Each GLS-V0-033 report with exclusions records the sorted package and reason
pairs in `excludedControls`. Chunks from one run must agree on that list before
their summary can supply the digest for a qualification record. Older reports
without the field decode with an empty list. New source bindings require fresh
qualification evidence.
