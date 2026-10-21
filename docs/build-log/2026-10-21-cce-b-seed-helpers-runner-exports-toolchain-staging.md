# cce-b: seed-parity helpers, literal runner exports, staged Go driver and a stale bounded-reader entry

Date: 2026-10-21. Lane: claude/cce-b-1021 (base c9ca8beff). Tickets: V1-1908, V1-1925, V1-2029,
V1-2197. The new requirements TSA-V0-644, TSA-V0-645 and BBF-V0-214 are accepted under the
owner's standing auto-accept (2026-10-11). They use lane-reserved numbers (next free + 180).

## Problems

- V1-1908: seed parity (TSA-V0-100..109) reported setup shapes for an example whose setup also
  ran through a helper method or function the reading never saw. The reported parity was not
  evidenced, which breaks invariant 2.
- V1-1925: unselected-test runner reading (TSA-V0-141/142) had two problems:
  - It read a JS/TS configuration whose export was not a literal object, such as
    `module.exports = require('./base')` or a factory call, as if it had no selection keys.
  - Mocha hidden-path matching was looser than minimatch `dot: false`: a dot segment anywhere in
    the pattern exempted every hidden segment.
- V1-2029 (suspected TOCTOU, now confirmed by code reading and a test): the go-test provider
  checked the Go driver's digest and then ran it from its original path. The comment said this was
  needed because Go resolves GOROOT from its own path.
- V1-2197: `.corvint/unbounded-readers.json` still listed `script/pre-push-windows-vet_test.sh`
  after that script stopped being an unbounded reader.

## Decisions

- TSA-V0-644 (RSpec):
  - A statement-head bare name that is a `def` in the group chain or at file level is followed.
    Its body joins the scope one level deeper, and its parameters are read as non-literal.
  - A followed helper must have exactly one unconditional site at the example's depth.
  - Any other unknown statement-head name abstains with `seed-helper-unresolved`. Known names are
    lets, canonical names, setup words, and RSpec, Rails, Capybara and FactoryBot words.
- TSA-V0-644 (JS/TS): a statement-head call to an unknown unqualified function abstains. JS/TS
  helpers are not followed.
- In both languages only statement heads count, not names in argument position. Counting argument
  names broke valid TSA-V0-102/103/107 cases.
- TSA-V0-645:
  - Every JS/TS export must be an object literal, `defineConfig({`, or a const/let/var bound to
    one of those. Anything else is `runner-config-export-dynamic`.
  - Mocha hidden matching is segment-exact. `**` crosses only non-dot segments, and a dot segment
    needs a pattern segment that starts with a literal dot.
- BBF-V0-214:
  - The Go driver is read once and digest-checked, then exactly those bytes run from a private
    copy, with `GOROOT` pinned to the in-place resolution root. A root without `pkg/tool` is
    refused, and a declared `GOROOT` is refused at plan time.
  - Retained limit: the compiler, linker and standard library under GOROOT, and the RSpec
    toolchain, still run in place. The RSpec toolchain is often a wrapper that resolves paths
    from its own location.

## Evidence

- New tests, each shown to fail against the pre-change sources:
  - `TestTSAV0644SeedHelpers`
  - the `required export`, `function export` and `mocha hidden segment` cases in
    `TestUnselectedAbstainsTSAV0142` and `TestUnselectedRunnerSemanticsTSAV0141`
  - `TestBBFV0214ToolchainRunsVerifiedBytes`
- Existing Seed, Unselected and BBF provider tests pass. The BBF provider tests use the real
  staged Go toolchain.
- `make -s unbounded-readers-check` is clean.

## NOT_RUN

- `make gate`: per lane policy.
- Live RSpec qualification (BBF-V0-033): no toolchain change on that path.

## Rollback

Revert the lane's commits. The changes are additive reasons, an abstention and a staging step,
with no wire or stored-state migration.
