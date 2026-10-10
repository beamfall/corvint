# GitHub #709 Playwright affected: device spreads, discovery producer, MALFORMED reason

Decision: three proposed requirements in
`docs/specs/typescript-javascript-affected-adapter-v0.md`, all pending owner acceptance.

- `TJAA-V0-018` (V1-1065): the static `playwright-affected/0` profile already resolved
  `use: { ...devices['<known name>'] }`, but any runtime-computed `use` value (a global
  `baseURL: process.env.BASE_URL ?? ...`, a project `storageState: authFile`) marked the browser
  identity of every project unresolved. Only the identity options must now be static. That set is
  `PlaywrightUseIdentityKeys`, the list the provider's qualified reporter (#49) already resolves at
  runtime. The #49 code runs in Node at runtime, so it shares no parser with the static profile.
  What is shared is the key set: `TestQualifiedReporterIdentityKeysMatchStaticProfile` checks it
  against the embedded `qualified-reporter.cjs` instead of keeping a second copy.
- `TJAA-V0-019` (V1-1066): `corvint affected discovery --playwright-config PATH --playwright-list
  FILE` converts a caller-run `playwright test --list --reporter=json` report into the canonical
  receipt (`typescript.PlaywrightDiscoveryFromList`). It refuses filtered, sharded, errored,
  foreign-config or out-of-root listings with `unsupported-playwright-discovery`.
- `TJAA-V0-020` (V1-1067): a MALFORMED summary now names `reason`
  (`DECODE_FAILED|NON_CANONICAL_BYTES|INVALID_FIELD`) and a one-line `detail`. An oversize regular
  discovery file now reports MALFORMED instead of MISSING.

Investigation of the issue's STATIC_UNRESOLVED result (142 units only in the receipt, 0 static): it
was a Corvint defect, the one fixed by `TJAA-V0-018`. When any project's identity is unresolved,
`selectPlaywrightStatic` empties the static units, so the whole receipt appears as `onlyInReceipt`.
The reporter's config is not available, so this cause is inferred. It was reproduced on base
`1120e565` with a generic config that has a computed global `baseURL` and per-project device spreads
beside `storageState`. Result: `state=STATIC_UNRESOLVED onlyInReceipt=8 onlyInStatic=0`, with
`browser-identity-unresolved` for all four projects.

A second, independent trap: Playwright reports `file` relative to `config.rootDir` (the top-level
`testDir`), not to the repository. A hand-built receipt that copies those names gets
`UNIVERSE_MISMATCH` with every unit on both sides. The producer rewrites them to repository-relative
paths, and the spec now states that `test` is repository-relative.

Evidence:
- Failing first, before each fix:
  - The V1-1065 test reported unresolved identity for all three device-spread cases.
  - The V1-1066 and V1-1067 tests failed to compile because the producer and the reason fields were
    absent.
  - The CLI test got `unrecognized arguments: discovery`.
- Pass after the fixes:
  - `go test ./internal/liveverify/affected/typescript ./internal/jstestprovider ./internal/appflows`
  - `go test -run 'Affected|Help|E2ESafe|Playwright' ./cmd/corvint`
- The multi-project fixture is a real Playwright 1.61.1 listing (`testdata/playwright-list/multi-project.json`,
  with the root rebased). It has a setup dependency, two device projects, and a project with its own
  `testDir`. Its produced receipt reconciles to MATCHED, and the bytes are identical across runs.

Limits:
- The device table still recognizes only Desktop Chrome, Firefox and Safari. Other descriptors stay
  unresolved.
- A produced receipt is still caller-declared. The producer cannot prove the listing came from the
  bytes it binds, or that environment-driven config branches matched.
- Only one real listing shape (Playwright 1.61.1) was exercised.
- Other unknowns in the issue, such as `dynamic-source-reachability` from unresolved bare imports,
  are unchanged.
- An absent or unreadable discovery file is still MISSING.
- `make gate` was not run.
