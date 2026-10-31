# Bug lane other-w1: fifteen internal and integration tickets

Orchestrator bug lane `other-w1` (2026-10-31), branch `claude/lane-other-w1-1031`, base
`780f9e97099aa3ccd52744a0c239a744c4a30adf`. Each delivered ticket has a spec amendment committed
before its implementing commit, plus a focused regression test that failed on the base. New
requirement IDs come from the lane's 960-979 block. Forks were decided in the lane and are recorded
below.

## Delivered

- **V1-3347 (TSP-V0-960).** The shell plugin now expands a computed command word or `bash`/`sh`
  operand through the file's own assignments, `${NAME-default}` defaults and `for NAME in` lists.
  The expansion is bounded: depth 4, 16 values a name, 64 values a word.
  - A fully literal value becomes an import edge.
  - A value with an unknown file name that still ends in an owned shell suffix keeps
    `shell:foreign-invocation-unmodelled`. The completeness scan counts it as naming every matching
    declared script.
  - Fork: failing closed through `UnboundedReads` was rejected because it would select the
    caller's tests on every plan. The static one-file limit is recorded as known gap (9).
- **V1-2415 (AFU-V1 amendment).** A generic call is masked only through its balanced one-line
  type-argument list.
  - Fork: masked output changes, so the flow-docs generator moved to version 4 rather than
    silently changing version 3 bytes.
  - Lists holding `(`, `)`, `;`, `=` or a line break stay conservatively masked.
- **V1-2511.** The cem01-go hostile-command-name test now re-executes the test binary as the leader
  under the hostile argv0. The leader's lifetime no longer depends on the host `/bin/sleep`, and the
  test asserts the leader is not in state Z. The parser assertions are unchanged.
  - Host runs: on the rust-coreutils WSL host (Ubuntu 26.04) the base test fails, with the leader
    in state `Z`, and the fix passes 3 of 3. On the Unraid host (GNU coreutils) the fix passes 2 of
    2. The whole `interop/cem01-go` package passes on WSL in 47 s, and `go vet` is clean.
- **V1-2533 (TJAA-V0-960).** The general alias path reads `jsconfig.json` when no `tsconfig.json`
  is beside it, and gates non-TypeScript owners on `allowJs`. `jsconfig*.json` is now an observed
  resolution input.
  - Fork: a nil keep-set (`keep == nil`) means a blocked gate. It keeps the speculative edge for
    every owner, so TJAA-V0-059 is unchanged.
- **V1-2562 and V1-2582 (AFP-V0-960, AFP-V0-961).** These two tickets share one change.
  - The Playwright tag closure now admits three forms: an escape-checked `JSON.parse` binding,
    `Object.values` on the TJAA-V0-117 terms, and a `for (let i = N; ...)` loop counter used as an
    index.
  - Soundness rationale: none of the three can produce the key `tag`. A reviver, a copy, an
    enumeration or a rebound counter still stops the closure.
  - V1-2582 was confirmed, not just suspected. `url.parse(s, true).query` and `dotenv.parse(s)`
    built a `tag` key that the closure trusted, so key-building members of `url` and `dotenv` now
    stop the closure and name the line.
  - Fork: `node:` imports are no longer blanket-neutral. Only an allowlist of built-ins stays
    neutral, because `node:vm`, `node:querystring` and `node:util` can build keyed objects.
  - Known gap: `process.env` passed whole or spread as test details is not modelled.
- **V1-2519 (CEM-V1-960).** Stable admission now refuses a symlinked root, administrative directory
  or common directory up front.
  - Fork: the refusal keeps the frozen code `repository-object-unavailable` instead of adding a new
    code. Only its detail changes: it names the link and asks for the resolved path.
  - `repository boundary changed during verification` is now raised only on a real identity change.
  - The same repository opened by its resolved path validates.
- **V1-2369 (RUBY-AFFECTED-V0-960).** A declared frontier entry that resolves no observed site now
  raises `ruby:frontier-declaration-stale:path[:line]`.
  - Fork: there is no re-anchoring. A declaration is an unverified claim about one site, so after
    a line shift both the stale entry and the undeclared site at the new line stay visible.
- **V1-0571 (LOD-V0-960, decision 0548).** `TestProducerTraceConformanceV1Only_V10571` runs the
  actual `cmd/corvint-dashboard-snapshot` producer against the standalone verifier on a v1-only
  store. The store holds a passed row, a failed row and a non-ASCII task row sealed over the
  production ASCII JSON identity.
  - Reproduced divergence: the producer emits `ADAPTER_QUALIFIED` where the verifier and the frozen
    fixture keep `ADVISORY`, and it emits zero `usage.trace.retained` buckets.
  - Following decision 0548, both disputed qualifications are `NOT_PRODUCED` and the producer is
    unchanged.
  - The test requires every other field to agree and both differences to still exist, so it fails
    when either side moves.
  - Non-ASCII identity already agreed (`TestTraceV1NonASCIITaskMatchesProducerIdentity`).
  - New finding: an invalid v1 store also differs in `byteCount`, `contentSha256`,
    `epistemicClass` and `currency`. It is recorded as a LOD-V0-960 known gap.
- **V1-0382, partial (seal not delivered).** `benchmarks/daily-loop-v1/harness.py` steps now get a
  fresh `HOME` from `tempfile.mkdtemp`.
  - The step environment drops inherited `GIT_*`, `DOGFOOD_*`, `CORVINT_*` and `XDG_CONFIG_HOME`.
  - The Go caches are pinned from `go env -json`, so moving `HOME` does not move them.
  - A scratch check found five leaks with the old environment: DOGFOOD, CORVINT, XDG, HOME and
    `~/.gitconfig`. It found none with the new one.
  - Defects 2 to 4 were already fixed at the 1.0 snapshot (PCCO-V0 row 185).
  - The preregistration seal (AC1, AC5) is not delivered. Sealing fixes the thresholds and a
    held-out attestation, both owner-owned bars, and decision 0430 puts new seals in the next
    numbered preregistration.
- **V1-1495 (envelope recorded, decision 0082).** No wait without a deadline was found in
  `conformance/cli-parity-v0`.
  - Every child process runs through `procgroup.Run` with a per-case timeout (at most 30 s) and a
    2 s shutdown. Fixture git runs at 30 s plus 2 s. `TestGPKV0002ManifestReplay` bounds the
    candidate build and the 136-case replay with one 9-minute context.
  - Measurement at host load 18-20 with `GOMAXPROCS=1 GOFLAGS=-p=1`: the whole package passed in
    253 s wall. The replay took 100.46 s and the in-process index test 11.41 s.
  - Forced dump: the 30-minute detector was cut to `-timeout 60s` with `GOMAXPROCS=1`. The
    goroutine dump names the single waiter as `procgroup.run` (`process.go:418`) under
    `executeCase` (`runner.go:358`), `replayParityCase` and `runReplayJobs`. That is the bounded
    per-case wait.
  - Post-deadline work test: the replay context was cut to 20 s as an uncommitted scratch edit. The
    test failed at 20.32 s, so cases do not keep running after the deadline.
  - Conclusion: the package's worst case is the 9-minute replay bound plus the bounded fixture
    tests. A 30-minute panic at load 65-76 (about 6x oversubscription of 12 cores) is CPU starvation
    of the test binary and the go toolchain, not a hang. Decision 0082 already classifies `-timeout
    30m` as a per-package hang detector, not a performance budget.
  - The 2026-10-16 base `5b6b85441` predates the clean 1.0 history, so its bounds could not be
    compared.

## Already delivered

- **V1-2223.** `corvint affected` reports 51 unresolved packages at the base, down from 104. None of
  them is `jstestprovider`, `worksource` or `postmergeconnector`. All three are declared in
  `.corvint/test-read-scopes.json` and `.corvint/unbounded-readers.json` (AFP-V0-023).

## Not delivered

- **V1-2929.** This needs a live OpenCode 2.2.0 host that renders the OpenTUI 0.7 sidebar under
  `qualify-opencode --inspector`. No such host run was available to the lane.
- **V1-0554.** This needs complete-root public calibration runs and an independent review of the
  chosen design. Reviews are run by the orchestrator, not by lanes.
- **V1-1269.** This needs a context-ranking change, or withheld-with-reason packet entries, plus
  the frozen held-out context evaluation gates. That did not fit the remaining lane budget.

## Review fixes

The independent review found three defects in the lane's own changes. Each was reproduced by a
failing regression test before it was fixed.

- **TSP-V0-960 value bound.** Two nine-word `for` loops that run `sh "./scripts/${x}${y}.sh"`
  build 81 names. Expansion kept 64 of them and dropped the rest without a trace, so a change to a
  dropped script could skip the test. Now `expand`, `braced` and `named` report when the bound cuts
  values. `observe` then marks the unit `shell:dynamic-source`, the same as a computed `source`
  (NSA-V0-009), and the kept values stay edges. The spec amendment is in TSP-V0-960. Test:
  `TestComputedInvocationValueBoundIsUnbounded_TSP_V0_960`.
- **AFP-V0-960 shadowed counter.** Inside `for (let i = 0; ...)`, the destructuring
  `for (const [i] of [['ta' + 'g']])` passed as a whole-index read of the counter. That let
  `rows[i]` pass the literal-tag check. `playwrightCounterUse` now refuses `const [NAME]`,
  `let [NAME]` and `var [NAME]`. The spec already said a shadowed counter stops the closure, so it
  was not changed. Test: row `counter shadowed by destructuring` in
  `TestPlaywrightTagClosureParseValuesCounter_V12562`.
- **AFU-V1-043 keyword before `<`.** `return<div>(required) save(text)</div>` was read as a
  generic call, because the keyword `return` ends in identifier bytes. That left the JSX text
  unmasked. An expression keyword is no longer treated as a generic callee. This stays within
  generator version 4, which has not shipped yet. The analyzer input digest is repinned with no
  schema bump, matching the V1-2415 precedent, because the flow mask does not change index
  encoding. Test: the `Render` case in `TestFlowDocsGenericCallMaskedOnlyThroughTypeArguments`.
