# Dedicated Ginkgo v2 native profile (experimental, runtime NOT_RUN)

Source implementation of the amended Ginkgo profile plan
(`ginkgo-profile-plan-amend1/plan.json`, sha256
`a0ff468f64d15346d5027ce5a175373f2f5d1cff6ada64c2939f465fb8267b42`) under
independent Gate A r3 (`GATE-A-REVIEW.json`, sha256
`1a1f0da5b07ba98fc7756b2d17a60d7cec4f6610df2c5c07c8affa9e4c2b66fa`,
APPROVED_WITH_CONDITIONS K1..K8), on base
`0ec12649` (after the CMocka profile landed). It adds `ginkgo-v2` as the
seventeenth native profile (57 concrete profiles) and TRE-V0-018..020. CEM
binding, live qualification and native Tasks completion remain separate steps.

## Gate A conditions

- K1: focus is exactly
  `^<QuoteMeta(SuiteDescription)> (<QuoteMeta(FullText_1)>|...)$` in one
  `-ginkgo.focus` element, because Ginkgo matches unanchored against
  `SuiteDescription + " " + FullText`. The plan's `^(...)$` form is not used;
  `SuiteConfig.FocusStrings` must equal the single string, and the unanchored
  form is a tested config mismatch.
- K2: `testdata/ginkgo-provenance.json` pins `internal/focus.go`,
  `internal/suite.go`, `internal/group.go`, `internal/spec.go`, `types/flags.go`
  and `types/enum_support.go` at `9f941496ce264d03b91f103e4ec4a19bbc75ce97`
  with sha256 NOT_OBSERVED (the read tool relays text, not bytes), beside the
  plan's five hashed pins.
- K3: enums accept only exact pinned table strings; JSON null refuses.
- K4: the Parse path enforces exactly one report and the shared per-report
  bound; zero and two reports are tested.
- K5: argv has 17 elements, or 18 with a selection; the focus element is
  bounded at 4096 bytes (accepted) and refused at 4097. The plan's 256 KiB figure
  exceeded the shared executor's per-argument limit and was not used.
- K6: the metacharacter case uses suite description `Calc (v1.2) [x]+` with
  suffix, prefix and metacharacter decoys; parser negatives cover both exit/suite
  contradiction directions, unexplained failure, special reasons with
  `SuiteSucceeded` true, enum null, and zero/two reports. Case 28 (BeforeSuite
  skip) has a source-derived model test; its runtime observation is NOT_RUN. The
  Ordered follow-on case is recorded per N5: zero attempts plus a Failure is
  NOT_ENTERED and the run is incomplete.
- K7: decisions below; re-pin below; serial landing kept (this slice is based
  on the landed CMocka profile and does not touch the shared executor).
- K8: runtime qualification NOT_RUN. No Ginkgo binary and no
  `github.com/onsi/ginkgo` module exist locally (only `github.com/bsm/ginkgo`
  v2.12.0); acquisition was out of scope.

## Decisions

AdditionalFailures: the invocation stays non-verbose. Ginkgo emits
AdditionalFailures only in verbose mode, so absence is not proof of no follow-on
failure; any present AdditionalFailure is incomplete. Enabling verbose output
would change the stream contract and needs its own qualification.

SuitePath: Build requires an absolute Root equal to its `EvalSymlinks` result.
Parse compares `SuitePath` with the cleaned executor Root exactly and reads no
filesystem, so readback is host-independent. The child's `os.Getwd` is expected
to return the resolved directory because the executor sets the working
directory and omits `PWD`; this is an inference from source reading, not an
observation.

buildinfo: the plan's `debug/buildinfo` module-sum check is NOT_IMPLEMENTED. The
verified `go.sum` h1 sum is NOT_OBSERVED and its positive path cannot be tested
without the module. Module identity rests on the caller-pinned executable digest
and strict schema parsing.

## Re-pin at base 0ec12649

Unchanged from the plan's `a5326ba6` pins: `registry/registry.go`, `types.go`,
`validate.go`, `execute_other.go`, `document.go` and
`cmd/corvint-test-runner/main.go`. Changed by the CMocka landing:
`native/native.go` (`819108aa…2e3c`), `native/README.md` (`966ae7f2…a7a5`) and
`execute_unix.go` (`ea433c82…32c3`). The changes add the CMocka profile and the
explicit argument-free TEST phase, which Ginkgo does not use.

## Evidence limits

All Ginkgo reports in tests are synthetic, shaped from the pinned source reading.
No actual pass/fail/skip/zero/lifecycle, timeout, interruption, parallel or
retry receipt exists. Rollback removes the additive profile, its tests and
provenance file; no shared executor or other profile changes.
