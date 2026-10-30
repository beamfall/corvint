---
name: corvint-impact
description: Find what a change touches and which tests Corvint selects for it. Use before running tests, or when asked what a set of paths or a commit range affects.
license: Apache-2.0
---

# Corvint impact and test selection

- Changed paths: `corvint_core_read` with `{"operation":"impact","input":{"paths":["a/b.go"]}}`.
- A committed range: `{"operation":"impact","input":{"base":"<40-hex commit>"}}`.
- Test selection: `{"operation":"affected","input":{"base":"<40-hex commit>"}}`.

Base values are full 40-character commit IDs, such as a `revision` an earlier receipt reported;
an abbreviated or symbolic name is refused. The plan is advice: it lists the selected tests and
the unknowns it could not resolve, and Corvint executes nothing. Report both to the user as the
verification still owed, quoting every unknown; a selection is not a passing result.

Other Corvint capabilities: the catalog at
https://github.com/beamfall/corvint/blob/main/docs/CAPABILITIES.md gives each command, binary and
MCP tool with its maturity and refusals.
