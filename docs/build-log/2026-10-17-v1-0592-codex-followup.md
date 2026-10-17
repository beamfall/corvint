# 2026-10-17 V1-0592: review follow-up, Node file-event collision and Storybook matrix correction

Independent review of PR #1003 found two defects. Both are corrected here; the totals in
`2026-10-17-v1-0592-dynamic-qualification.md` are superseded by this entry.

Node file-level event collision (TRE-V0-060, amended). The `node-test` filter treated any
top-level event at 1:1 named after its file as Node's synthetic file-level event. A real test has
the same shape when it is called first and named after the file
(`test(import.meta.filename, ...)` before a hoisted `import`, or `test("a.test.mjs", ...)`).
Actual Node 22.23.3 runs through `corvint-test-runner` (`node-test/filename-collision`,
`filename-collision-fail`, `basename-collision` under `/private/tmp/cem10-build/dynamic-2026-10-17/`)
showed that a real passing test was dropped and reported as `no-tests`, and a real failing test
was reported as `collection-or-hook-error`. In the profile's reported events (`test:pass`,
`test:fail`, `test:summary`) the real test and the synthetic event have the same fields: name,
nesting 0, `testNumber` 1, `details.type` `test`, line 1, column 1 and the same summary counts.
A diagnostic run with every reporter event showed the two differ only in internal event
multiplicity: the real test adds a second `test:enqueue`/`test:dequeue` pair and a second
`test:complete`, and a Node file-level failure carries `exitCode` on its error. Both are internal
behaviour, not a contract, so the adapter does not rely on them. It fails closed instead. Such an
event now records `ambiguous-test-identity` and leaves the run incomplete. It never becomes a test
or a no-tests or collection verdict, and it still reconciles against the summary counts. Re-runs
with the repaired binary (`*-fixed`) are incomplete with `ambiguous-test-identity`; `mixed-fixed`
stays complete with 3 passed, 1 failed and 2 skipped. A Node file with no tests now reports
`ambiguous-test-identity` and the generic `no-tests`, not a file-specific `no-tests`.
`TestNodeFileLevelEventsAreNotTests` pins the three collision reports and the zero and collection
reports. All four mutants were killed: silently dropping the problem, disabling the guard,
restoring the old `no-tests` verdict, and removing basename matching.

Storybook matrix correction. The matrix claimed that every Z, C and T witness produced an
incomplete receipt. That is false. `storybook-test-runner/collection` and `timeout` are complete
FAILED runs with no problems, and so are `jest/hook` and `rspec/hook`. None of them became a pass.
- storybook-test-runner C is now WF, a witnessed complete FAILED run. The runner reports the
  module-load error as the indexed story's failed `smoke-test` ("The story was missing", attempt
  failure kind UNKNOWN). This is the runner's own verdict and is not a pass. The limit is that the
  receipt cannot tell a module-load error from a render failure.
- storybook-test-runner T is now I, interrupt-only. The pinned config's 15 s Jest test timeout
  ended the run before the 60 s executor timeout (`timedOut: false`). The missing executor-timeout
  witness remains a release blocker (V1-1544).

Corrected totals: W 135 (4 W!, 1 WF), I 1, parser-only 0, none 7, NA 11.

Rollback: revert this change. The original runs and the collision witnesses remain retained
evidence.
