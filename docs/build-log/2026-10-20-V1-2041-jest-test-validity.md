# 2026-10-20 V1-2041: Jest test files and never-executed assertions in test-validity

V1-2041 (GitHub #1293) asks that Jest TypeScript tests get the JS reader's verdict axes, or that
the spec name each axis that cannot apply. Requirements `TSA-V0-388..390` in
`docs/specs/test-source-analysis-v0.md` are agent-drafted and accepted under the owner's standing
auto-accept of lane requirements (2026-10-11).

## Finding

Jest `.test.ts` and `.spec.ts` files already took the V1-1858 reading (`TSA-V0-073..083`). The gap
was naming and one axis:

- a file under Jest's default `__tests__` directory with a plain name (`__tests__/cart.ts`) failed
  `IsTestFile`, so `tests validity` and receipt discovery returned `not-a-js-test-file`, the
  "not analysed" result in the issue;
- no JS rule read an assertion that never runs before the example ends.

## Decisions

- A validity-only predicate, `IsJSValidityTestFile`, adds the `__tests__` form. `IsTestFile` stays
  unchanged, so lint, role locators, teardown and the other readers keep their scope.
- The never-executed axis is the static `JS-ASSERTION-NOT-AWAITED`: an assertion in a `.then`,
  `.catch` or `.finally` callback whose promise statement the parameterless example neither awaits
  nor returns. It is advisory, since a later flush or an unparsed construct can still run it.
  Timer callbacks are left out because fake timers run them inside the example.
- Measured strength, run outcome and freshness cannot apply. `corvint-js-test-provider` runs only
  Vitest and Playwright, so no Jest receipt exists to project (`TSA-V0-390`).

## Evidence

`TestJestValidityTestFile`, `TestJestAssertionNotAwaited` and `TestJestValidityAxes` in
`internal/testsource`, run through the host semaphore at base `f3ca72248`. Real-suite
qualification is NOT_RUN.
