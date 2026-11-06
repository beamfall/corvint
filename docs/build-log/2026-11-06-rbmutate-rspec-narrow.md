# rbmutate depends on the RSpec runner family only (V1-4752, V1-4747 slice 2)

The V1-4747 scope review found that `internal/liveverify/rbmutate/container.go` was the only
Core-closure importer of `internal/testrunner/registry`, which it used only to plan and parse its
fixed `rspec` request. The registry links every runner family (dynamic, native, platform, mobile,
sql), so all of them sat in the `corvint` binary's dependency closure for one runner.

## Scope

- `internal/testrunner/dispatch.go` adds `BuildWith(request, execMemberRunners, build)` and
  `ParseWith(input, parse)`: the admission every dispatch applies before a family builds
  (`AdmitSelection`, then the TRE-V0-081 / LGP-V0-011 exec-member refusal with the same message),
  and the family parse followed by `Normalize`. `registry.Build` and `registry.Parse` now call
  them with their table lookup, so the guard has one implementation.
- `rbmutate` plans and parses through `buildRSpec` and `parseRSpec`, which call the same helpers
  with the `dynamic` family's `ExecMemberRunners`, `Build` and `Parse`, at the three former
  `registry.Build` sites (profile probe, run plan, `admitRunnerReceipt` rebuild) and the one
  `registry.Parse` site.
- Trace rows for TRE-V0-081, LGP-V0-010/011 and FPK-V0-080 name the new call path; no
  requirement text or ID changed.

Non-goals: no runner retired or moved; the `corvint-test-runner` companion keeps every family; no
registry API or behavior change; no new plugin abstraction. No binary-size or build-time saving is
claimed until measured.

## Evidence

Base `8fc0cf37642e7012f4c26671c9ed97a8706d6e09`, Go 1.27.3, Darwin.

- `go list -deps ./internal/liveverify/rbmutate | grep testrunner`: before `testrunner`, `dynamic`,
  `mobile`, `native`, `platform`, `sql`, `registry`; after `testrunner`, `dynamic`.
- `go list -deps ./cmd/corvint`: 402 packages before, 397 after; the five removed are
  `testrunner/registry`, `native`, `platform`, `mobile` and `sql`. `./cmd/corvint-test-runner`:
  236 packages and the same seven runner packages plus `testrunner/cli` before and after.
- `TestRSpecPathMatchesTheRegistry` (rbmutate): for the probe, run and cited-example requests
  `containerRunner.request` builds, and eleven refused variants (relative executable or report
  directory, option/glob/parent selector, option config, unknown formatter, bad selection version
  or matcher, exec members on a non-member runner), `buildRSpec` gives byte-identical invocation
  JSON and identity and the same error text as `registry.Build`; `parseRSpec` matches
  `registry.Parse` bytes and errors for a complete report, an exit mismatch, a malformed or
  missing report, a timeout, an overflow and an execution problem. It also checks that the
  registry resolves `rspec` to `testrunner/dynamic`.
- `TestBuildWithAppliesTheSharedAdmissionFirst` (testrunner): a refused selection or exec member
  never reaches the family build, the refusal names the admitting runners joined by `, `, and
  `ParseWith` normalizes while keeping the parse error.
- Focused tests via `script/heavy.sh`: `./internal/testrunner`, `./internal/testrunner/registry`,
  `./internal/testrunner/dynamic`, `./internal/langplugin`, `./cmd/corvint-test-runner` pass;
  `./internal/liveverify/rbmutate` passes on the Mac with host Ruby, including the FPK-V0-078 to
  FPK-V0-080 container tests through the fake CLI (only the opt-in live qualifications skip).

## Rollback

Revert the commit: `rbmutate` imports `internal/testrunner/registry` again and the registry inlines
its guard. There is no persistent state, schema or wire contract to unwind.
