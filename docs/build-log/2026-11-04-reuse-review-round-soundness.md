# reuse: review-round soundness corrections (environment, closure modules, assembly, replay freshness)

Base: `6f12f4704235dc8f9bea19fc12995bfb68c746c3`. Tickets: V1-1097, V1-1099. Requirements amended
in place (IDs stable): AFP-V0-2321, AFP-V0-2322, AFP-V0-2333, AFP-V0-2346 in
`docs/specs/affected-plan-reuse-v0.md`. An independent review of `fa1155543` returned three P1 and
one P2 finding; each is fixed with a regression test that fails against the reviewed
implementation.

## Findings and decisions

1. **Runtime environment (P1, AFP-V0-2322).** The identity bound only resolved Go environment
   values, so a test that skips unless a variable is set (for example
   `CORVINT_TRUSTED_NAV_PYTHON`) could reuse a pass made while it was skipped. Decision: bind the
   SHA-256 of the whole supplied process environment (`runtimeEnvDigest`), last value wins,
   names case-folded on Windows, unset distinct from empty; values are never written to the
   record. An allowlist was rejected: a test or a child process can read any name, and the
   review's alternative (refusing packages with unmodelled environment reads) has no sound
   detector. Cost: any environment difference between record and decision now gives
   `IDENTITY_CHANGED`. A caller that wants reuse across invocations must run tests, records and
   decisions under one normalized environment. Hosted CI carries per-run variables, so CI
   reuse would need that normalization before AFP-V0-2327's hold is lifted; the replay's
   measured savings assume it (the replay uses one fixed environment).
   Evidence: `TestRuntimeEnvironmentEnablesSkippedFailure` (skipped-to-enabled: unchanged
   environment reuses, empty and broken values give `IDENTITY_CHANGED`, and the enabled test
   really fails) and `TestRuntimeEnvironmentCanonicalization`.
2. **Closure-module manifests (P1, AFP-V0-2321).** Only root module files were hashed. Every
   non-standard closure member now binds its effective module's `go.mod` (from `go list`'s
   `Module`) and the adjacent `go.sum` or an `absent` marker. Missing metadata, or a module
   directory or manifest outside the repository or not the directory's `go.mod`, is
   `DIGEST_ERROR` `module-input`. Evidence: `TestDependencyModuleLanguageFault` (an in-repository
   replacement whose `go.mod` moves from Go 1.22 to 1.21 is `DIGEST_CHANGED` and the real test
   build fails on integer range; an untracked or dirty manifest or checksum is `DIGEST_ERROR`).
3. **Assembly includes (P1, AFP-V0-2321).** `go list` does not name headers an assembly file
   includes from outside the package. Decision: refuse any non-standard closure member with
   `SFiles` (`DIGEST_ERROR` `assembly-input`) rather than parse `#include` directives. The
   repository has no non-standard assembly, so nothing reusable is lost. Evidence:
   `TestAssemblyIncludedHeaderFault`: the assembly package and its consumer are refused before
   and after the escaping header changes, and a forced fresh build (`-a`, since Go's build cache
   also misses that header) fails.
4. **Replay freshness (P2, AFP-V0-2333 / AFP-V0-2346).** The replay loaded records, decisions and
   job plans whenever their files existed. Every invocation now removes and recomputes them; only
   records computed in the same invocation are memoized, and a failed recomputation stays
   `NOT_PRODUCED` with no file left behind. Evidence: `ReplayFreshnessTest` in
   `tools/ci-reuse-replay/replay_test.py` (changed binary, environment, source attempt and
   declaration each rerun the commands and expose an injected violation).

## Negative controls

Run against the reviewed `observe.go`/`testreuse.go` and `replay.py` with the new tests:
all three Go regressions fail with `RECORD_DIGEST_EQUAL` where they want `IDENTITY_CHANGED`,
`DIGEST_CHANGED` or `DIGEST_ERROR`; both replay tests fail (`0 != 2` and `0 != 1` recomputations
or violations). With the fixes, `go test ./internal/testreuse/` and the focused
`TestAffectedJobsCLI|TestTestReuseCLI` CLI tests pass, as do both replay tests.

## Limits

The full-environment binding lowers precision for any changed or volatile variable. That is by
design and is never relaxed for savings. The replay acceptance numbers in
`2026-11-03-reuse-verified-results.md` were not rerun. Hosted CI reuse remains held.
