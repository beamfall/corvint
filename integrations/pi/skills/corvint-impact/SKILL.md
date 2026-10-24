---
name: corvint-impact
description: Find what a change touches and which tests Corvint selects for it. Use before running tests, or when asked what a set of paths or a commit range affects.
license: AGPL-3.0-or-later
---

# Corvint impact and test selection

- Changed paths: `corvint_core_read` with `{"operation":"impact","input":{"paths":["a/b.go"]}}`.
- A committed range: `{"operation":"impact","input":{"base":"<40-hex commit>"}}`.
- Test selection: `{"operation":"affected","input":{"base":"<40-hex commit>"}}`.

Base values are full 40-character commit IDs; resolve them first with `git rev-parse`.
The plan is advice: it lists tests to run and the unknowns it could not resolve. Run the selected
tests yourself, and run broader checks for every unknown the plan names. Corvint executes nothing.
