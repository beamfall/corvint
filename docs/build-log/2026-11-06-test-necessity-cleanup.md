# Test necessity cleanup (V1-4746)

## Decision and boundaries

The owner requested the concrete safe cleanup from the test-necessity audit. This
change simplifies fixture machinery and duplicated assertions, preserving product
behavior, supported entrypoints and security/confinement boundaries. It does not
retire a product feature. Base: `f50a0e3447a2bbadbfadbe7f8cc1019ae1636ac5`.
The builder uses Sol at medium effort for cross-layer coverage equivalence.

## Coverage mapping

- `TestCEMV1210FixtureRecomputeRefusesEveryMap` keeps stable and retired refusal
  cases. Its prior Git history, CEM preparation/citation, OCM, enrollment and Ed25519
  receipt setup was unreachable: `lrfrepo.VerifyUniverse` calls
  `refuseUniverseMap` before opening a repository. The replacement reads the
  existing neutral-criterion stable wire vector, explicitly requires successful
  `wire.ParseStable`, and checks the stable-only admission diagnostic separately
  from the retired-spec recovery line. An empty non-repository directory witnesses
  that the refusal needs no repository. This proves wire admission, not Git or
  receipt validity; those authorities are not evaluated by this interface.
- `TestBoostTestRegistryDispatch` retains runner uniqueness, public Build and Parse,
  complete passed/skipped results and exact native Build/normalized Parse equality.
  The removed failure-exit-without-failure assertion belongs to native
  `TestBoostTestBoundaryContradictions`, case `failure exit no fail`. Its `surplus
  case` removes a failed case, so the distinct registry surplus-skipped assertion
  is retained: unrequested `M::s/off` must become Unknown and incomplete.
  Independent review identified and corrected this coverage mapping before closeout.
- Each Java, C-JNI and Objective-C package retains its named
  `TestCLIInvocationFramingAndRefusal`. Their byte-identical harness is now one
  narrow helper in the existing `internal/testsupport` package. Each test still
  builds its own command from its own cwd with the same local/offline build
  environment and deadlines, then executes its exact version, family request and
  golden response, one-line framing, oversize refusal and both invalid-argv
  refusals. No entrypoint, family parser or specialized isolation test moved.
- The profile entrypoint source guards remain: output parity and worktree refusal
  cannot establish PRB-V0-010's bracket at every MCP entrypoint or PRB-V0-012's
  worker-before-bracket exclusions.
- The releasegate production-closure digest and mutation witnesses remain:
  behavioral parity alone cannot bind the counted complexity surrogate to the
  production implementation. Platform and descendant-containment tests remain.

## Corvint evidence and limitations

Canonical `make dogfood-change` ran before edits. It retained bound context,
query and positional impact receipts in this worktree's private Git metadata.
The initial no-diff report explicitly left CEM/OCM/check/outcome `NOT_PRODUCED`.
Query omitted three ranked results and withheld 22 test-path symbol candidates;
positional impact included all five initial paths with no critical misses.
The separate context packet could not resolve the native ticket ID, omitted
lexical matches at its limit and was supplemented by the ticket and original
source. These receipts do not establish semantic equivalence.

Installed build88 reports `self-binary-stale` against this base (56 Go paths
changed since its d436dff build). The final dirty affected receipt selected 160 units
and retained two `LANGUAGE_FRONTIER` uncertainties (the shared helper broadens
its conservative selection). Focused checks below supplement,
not close, that unknown; repository-wide `make gate` is not required for this
scoped issue under the owner's standing policy.

## Verification

The same focused tuple passed before and after: the frontier refusal, Boost
registry dispatch, native Boost contradiction matrix and all three analyzer
executable tests. Commands used `GOTOOLCHAIN=local script/heavy.sh go test -json
-count=1 -timeout 30m -run 'TestCEMV1210FixtureRecomputeRefusesEveryMap|TestBoostTestRegistryDispatch|TestBoostTestBoundaryContradictions|TestCLIInvocationFramingAndRefusal'`
with those six packages. Scoped `go vet` also passed, including `internal/testsupport`.
Raw baseline/final observations are `/private/tmp/v14746-baseline.jsonl` and
`/private/tmp/v14746-final-check.jsonl`; wrapper runs were 4.4s and 1.9s.
The frontier named test recorded 3.00s before and 0.00s after (Go's displayed
rounding); analyzer tests recorded 0.69/0.69/0.70s before and 0.52/0.50/0.49s
after. These are one before/after observation on a shared host with changing
cache state, not a quiet-host benchmark or a repeatable speedup claim.

Across the five changed `_test.go` files, lines fall from 490 to 142. The new
shared helper adds 93 non-`_test.go` source lines, exclusively for test support;
total Go source plus tests therefore falls by 255 lines. This explicitly counts
the moved helper rather than treating its filename as deleted test complexity.
The six named tests in the focused tuple remain six; analyzer invocations remain
five per executable and frontier refusals remain two. One registry negative
case is removed; its native semantic case remains unchanged. The unique
surplus-skipped case remains covered in the registry. No production
feature implementation changed.

`make windows-vet` passed for the main and interop modules with Go 1.27.3.
Independent review r2 passed at `535c2c28c` after the skipped-case repair
(coordinator review relay). Final CEM binding/check/seal follows this correction;
native completion remains the coordinator's closeout.

Review repair verification: the focused `TestBoostTestRegistryDispatch` passed
with the writable repository cache after the default machine cache refused access.
`make windows-vet` passed. Raw results: `/private/tmp/v1-4746-repair-test.log` and
`/private/tmp/v1-4746-repair-windows-vet.log`. Context and affected receipts are
`/private/tmp/v1-4746-repair-context.json` and
`/private/tmp/v1-4746-repair-affected.json`; affected retained the installed
binary stale-source warning. No exhaustive gate was run for this scoped repair.
