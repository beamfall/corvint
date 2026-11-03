# Selector rule (c) honours declared read scopes (TSP-V0-980, V1-4357)

The planner bounds a declared package's path-literal reads by its read scope (AFP-V0-161), but the
PR-tier selector (`internal/gateselect`) did not: any test naming a path literal that matched a dirty
path selected its package on a rule (c) `reader` line, even when `.corvint/test-read-scopes.json`
declares that package and `.github/testconfine` confines its tests to a scope that excludes the path.

`boundedReaders` now drops such declared readers, reusing `declaredBears` so a changed declaration
still selects under AFP-V0-050. Undeclared packages are unchanged.

Measured on probe #70's plan (a comment-only change under `internal/betarung`): selected cost fell
from 5,815 s to 4,020 s of the 9,703 s suite (31%), and reader lines from 41 to 13. The remaining
cost is planner-selected UNBOUNDED_READER packages (`cmd/corvint`, `internal/tasks/cli`,
`internal/companionrelease`, `internal/liveverify/affected`, `internal/contextindex` and others whose
tests climb to the repository root) and three declared packages whose scopes include `internal/`;
those are the next precision round. CI tools build from the compared `main` commit (CIN-V0-011), so
the change takes effect in CI after it merges.
