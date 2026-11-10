# Design: hub-read-scopes

## Context

The planner selects a Go unit on every change when the unit is listed in
`.corvint/unbounded-readers.json` (rule (d)). It also selects a unit when a path literal in its
test files names a changed path (rule (c)). A declared read scope (AFP-V0-023) removes a unit from
both rules for paths outside the scope, and full CI confines the unit to the scope (AFP-V0-161).
The hub packages cannot declare a scope today because a few of their tests read the whole
repository.

## Decisions

1. Move tests, do not add planner rules. A proof for literals joined to a temporary root was
   built and measured (AFP-V0-9810): it frees no hub. A read scope is an existing, enforced
   mechanism, so the change adds no new soundness argument.
2. One sibling package per hub holds every whole-repository test of that hub. The sibling stays
   an unbounded reader. It runs on every change, which keeps today's coverage of those tests.
3. The list of tests to move comes from observation, not from the recorded reason. The recorded
   reasons name examples only. The lane runs the hub under the Landlock wrapper with the
   candidate scope and moves every test that fails there because of a read outside the scope.
4. A test that needs unexported names moves only when a narrow test hook keeps the production
   API unchanged. Otherwise the hub stays unbounded (AFP-V0-109).
5. The scope is sized from observed reads, then declared. `tools/test-read-scopes-check` and the
   confined run are the proof (AFP-V0-108).
6. Two stages. Stage 1 takes the two hubs with few whole-repository tests. Stage 2 takes
   `cmd/corvint`, whose tests run `go list` over the command closure and build a sibling command;
   whether they can move is `NOT_OBSERVED`.

## Risks

- A whole-repository test is missed and stays in a scoped hub. The confined run fails, so the
  scope is not declared. The risk is cost, not an unsound selection.
- A test passes confined only because it skips when a read fails. The lane checks the confined
  run for skipped tests and treats a new skip as a failure.
- The sibling packages grow over time. `unbounded-readers-check` already reports the set.

## Rollback

Restore the hub's entry in `.corvint/unbounded-readers.json` and delete its scope entry. The
moved tests can stay where they are.
