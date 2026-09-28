# Restore the Tasks scope boundary

Owner decision on 2026-09-28: restore decision 0397's existing import boundary and conservative
scope fallback; defer automatic derivation. PR #312, merged into #311, imported Core packages
from Tasks, breaking both `TestImportDirection` and the standalone Tasks source subset. Its
pack-only loader changes also invalidated `TestAnalyzerSchemaInputs` without updating the audit.
Both failures were reproduced before editing.

The Tasks CLI now leaves a nil deriver to the existing `NoScopeDeriver` fallback, including
`claim --next`. Explicit and declared scope precedence remains intact. The injected deriver and
selected-ticket binding remain available for conformance; automatic index integration is deferred.
The removed pack integration and its Core loader additions are rolled back to the pre-#312
source. The preexisting analyzer-schema export, schema ID and audit digest are unchanged.
The updated CLI regression covers default and pack-opt-in fallback, explicit/next claims,
requested paths and declared paths. Import, analyzer identity, scope and collision tests passed.

The shared PR-stack fixture repair supplies a test identity to `git commit-tree` and disables
identity guessing; the output helper retains stderr. This fixes the reproduced CAL-V0-017 CI
failure without changing its refusal assertions. The final selected check covers completion too.

The owning spec, index and README explicitly retain the automatic-derivation gap. No wire
format, ranking or authority changes. Rollback restores the removed integration only after an
owner-accepted boundary/bundle design and its gates; simply weakening the import test is not a fix.

Corvint query, tracked-path impact and affected were used; original receipts and unknowns remain
in the private Git directory and /tmp/corvint-pr-fixes-20260928. Initial dogfood-change at an empty
range could not produce a CEM/OCM and had no outcome input; it was not a passing gate.
Full `make gate` is NOT_RUN under scoped-work policy. Frozen retrieval evaluations are not
applicable because analyzer inputs are restored exactly and no ranking change is introduced.
Standalone Tasks build/dependency checks and independent review cover the packaging boundary.
Live host/process providers and paired cost measurement are not applicable; billed token/cost
and paired savings are NOT_OBSERVED. Existing CAL-V0-022 intent retains the deferred work.
