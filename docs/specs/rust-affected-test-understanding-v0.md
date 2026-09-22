# Rust Affected-Test Understanding V0

Owner: Russell Lewis
Date: 2026-08-29
Intent status: proposed
Delivery status: experimental
Authoritative inputs: operator request of 2026-08-29,
`docs/specs/live-proof-carrying-verification-v0.md`, `docs/DOGFOOD.md`, and the
`internal/liveverify/affected.Language` contract

## Agent digest
- Claim: Static Cargo analysis conservatively maps Rust changes to affected test packages while unsupported surfaces widen scope.
- Status: proposed/experimental
- Exists: `internal/liveverify/affected/rust` static Cargo planning adapter and focused tests.
- Blocked on: runtime-provider qualification, cross-language ownership, and full CLI-parity evidence.
- Read next: Unit and address contract; Cross-language ownership decision required; Traceability.

## User and measurable job

Corvint should conservatively map a dirty Rust source path to the Cargo packages whose tests may be
affected, without running Cargo or treating a missing static edge as proof that a test is irrelevant.
The adapter is successful when every represented package is runner-addressable, dependency closure
uses first-party Cargo edges, and every unsupported discovery or execution condition widens scope.

This is an experimental static-planning adapter. It does not qualify a Rust runtime provider or
change Live Proof-Carrying Verification V0's `not-started` delivery status.

## Unit and address contract

One unit is one Cargo package, including one workspace member. Its stable identity is
`rust:<repository-relative member Cargo.toml>`. This granularity is the common exact boundary that
Cargo and nextest can both address and that remains stable when a function or rustdoc line moves.
Source-module edges remain inside the package unit.

For package name `<package>` and the manifest that owns the unit, the complete built-in invocation is:

```text
cargo test --manifest-path <member/Cargo.toml> -p <package> -- --include-ignored
```

`#[should_panic]` changes pass semantics but not identity. `#[ignore]` requires
`--include-ignored`. `#[tokio::test]` and other resolved `path::test` attributes expand to ordinary
libtest cases and use the same package command. Cargo includes library/binary unit tests, integration
targets, and library doctests in this package invocation.

When canonical nextest configuration is present, the corresponding package invocation is:

```text
cargo nextest run --manifest-path <member/Cargo.toml> -p <package> \
  --ignore-default-filter --run-ignored=all
```

Nextest does not run doctests. A caller using nextest must additionally run:

```text
cargo test --manifest-path <member/Cargo.toml> -p <package> --doc -- --include-ignored
```

The current `Unit` and `Selection` wire structs contain only IDs, source/test paths, imports, and a
witness. They have no command, framework, package-name, target, feature-set, ignored-state, provider,
or prerequisite field. The adapter therefore makes the command deterministically derivable from the
unit manifest but cannot carry it to a caller. Widening that frozen seam is outside this slice.

## Requirements

- `RUST-AFFECTED-V0-001`: `Owns` MUST claim `.rs` and MUST NOT claim Cargo manifests, Gherkin
  `.feature`, YAML, or other cross-language inputs.
- `RUST-AFFECTED-V0-002`: Each Cargo package MUST produce one manifest-identified unit. Nested
  packages MUST own their own source trees, and package identities, paths, imports, and frontier
  reasons MUST be sorted deterministically.
- `RUST-AFFECTED-V0-003`: Test anchors MUST include source files containing built-in `#[test]`,
  resolved async `path::test` attributes such as `#[tokio::test]`, Cargo `tests/` sources, explicit
  test targets, and runnable Rust doctest fences. A bare fence, or one whose every info token is
  `rust`, `ignore`, `no_run`, `compile_fail`, `should_panic`, or `edition20xx`, is Rust; any other
  info token makes the fence non-Rust. Fence state MUST be tracked so a non-Rust block's closing
  fence is not reinterpreted as a bare Rust opener: a close MUST use the opening marker character,
  contain at least as many consecutive markers, and have no info text. An unterminated bare or Rust
  fence remains a conservative anchor. `doctest = false` MUST suppress doctest anchors.
- `RUST-AFFECTED-V0-004`: First-party normal, dev, build, target-specific, renamed, and inherited
  workspace dependencies MUST form package edges, including those in Cargo's `dev_dependencies` and
  `build_dependencies` table aliases. Conditional edges MAY be retained conservatively but MUST also
  name their uncertainty.
- `RUST-AFFECTED-V0-005`: Inline test/doc files MUST be test anchors even when they also contain
  production code. The frozen graph's exclusive path ownership means such a file cannot appear in
  both `Sources` and `Tests`; this affects the direct witness label, not dependency reachability.
- `RUST-AFFECTED-V0-006`: A detected nextest configuration MUST retain the same package identities.
  If doctests coexist with nextest, the adapter MUST report that nextest alone cannot execute them.
- `RUST-AFFECTED-V0-007`: `thirtyfour` and `fantoccini` tests MUST retain ordinary libtest/async-test
  anchors, while their external WebDriver/browser prerequisite MUST widen scope.
- `RUST-AFFECTED-V0-008`: A `cucumber` dependency or `.feature` input MUST report unresolved
  cross-language ownership. The Rust adapter MUST NOT claim scenario coverage or `.feature` paths.
- `RUST-AFFECTED-V0-009`: Unreadable source, unresolved manifest structure, source outside a Cargo
  package, non-test cfg variation, generated/include/build-script source, generated tests, custom
  harnesses or non-default test targets, nextest/doctest mismatch, and external WebDriver runtime
  MUST produce stable frontier reasons and therefore `UNKNOWN` scope under `LPCV-V0-016`.
- `RUST-AFFECTED-V0-010`: The adapter MUST satisfy the common language-seam witness, exclusion,
  widening, composition, and determinism tests and an opt-in real-repository direct-test gate.

## Detection and known static limits

Built-in and async tests are detected from Rust attributes; `tests/` and explicit `[[test]]` paths
use Cargo discovery conventions. Doctests are detected from runnable Rust fences, opened with either backticks or tildes, in actual line
or block doc comments; fence state and the closed Rust info-token set in `RUST-AFFECTED-V0-003`
prevent a non-Rust closing fence from becoming an opener, and apparent doc comments inside
quoted/raw strings are ignored. An indented
code block (four columns of spaces, or a tab) is also a doctest anchor, because rustdoc tests it like an
unannotated fence. It counts only where Markdown can open one: at the start of the doc comment or after
a blank doc line, heading, or closed fence, and not inside a fence. Indentation is measured before
rustdoc removes the comment's common indent, so this can add an anchor but cannot drop one. A quote opens a
char or byte literal only when exactly one rune or one escape sequence precedes the closing quote,
so a lifetime such as `'static` cannot pair with a later apostrophe and hide the test
attributes that `RUST-AFFECTED-V0-003` anchors. nextest is
detected from root or nested-workspace `.config/nextest.toml` or `nextest.toml`. WebDriver and
cucumber signals come from Cargo dependency names; `.feature` presence is observed without
ownership. A dependency is renamed only by an inline-table `package` key; the word `package` inside a
path, version, or other string value leaves the dependency key as the package name. Likewise a
dependency inherits a workspace entry only through a `workspace = true` key; the word `workspace`
inside a path cannot redirect the edge to a same-named workspace dependency. The deprecated `[project]`
manifest table is read as `[package]`. Whether every current cargo edition still accepts it is not
asserted; reading it can only add a package unit and its edges, never drop one.

The following remain explicit Unknown rather than silent omissions:

- macro/procedural/reflection-generated tests without statically recognizable anchors; unknown
  attributes and non-standard macro expansion conservatively raise the generated-test frontier;
- cfg, feature, target, `required-features`, custom-harness, or nextest profile filtering without a
  fixed execution profile;
- `include!`, `include_str!`, `OUT_DIR`, build-script generation, external doc inclusion, and custom
  target layouts the bounded manifest reader cannot resolve;
- renamed or re-exported procedural test attributes whose expansion cannot be established;
- rustdoc's function-level doctest names, which contain source line numbers and are not stable under
  unrelated insertions; and
- nextest use present only in an external invocation or skipped CI/configuration path.

Package granularity ensures a recognized package command still runs every dynamically expanded test
that its harness discovers. The frontier is nevertheless required because static discovery cannot
prove the eligible set complete or produce stable child identities.

## Cross-language ownership decision required

Gherkin `.feature` files are plain-text scenario assets bound to steps in Rust or another host
language. Maestro `.yaml`/`.yml` flows are schema-defined tests rather than source in the language of
the application. Appium test bodies belong to their host-language plugin, while shared capabilities,
fixtures, and data may not. Extension-wide ownership by every interested plugin is not sound.

If two plugins put the same path in unit `Sources` or `Tests`, graph construction fails with
`ErrDuplicateOwner`. If no plugin claims it, a dirty path becomes `UNOWNED_DIRTY_PATH`; if plugins
claim the extension without indexing the path, it becomes `UNINDEXED_SOURCE_PATH` without expressing
which host/scenario association was intended. Quietly choosing Rust as the owner would also prevent a
Python or Java step implementation from representing the same feature honestly.

A resolution must define a neutral asset-owner or association contract, many-to-many edges from an
asset/scenario to host-language units, stable scenario and Scenario Outline row identities, exact
provider addressing (including duplicate names and custom parsers/runners), collision precedence,
and invalidation for asset/config changes. YAML ownership must be schema-specific rather than
extension-wide. Until that decision is accepted, cucumber scenario selection is unsupported and
widens scope.

## Resource, rollback, and acceptance

Observation is local, read-only, bounded by the existing repository walk and source-size limits, and
does not execute Cargo, rustc, nextest, browsers, drivers, or test code. Rollback removes the Rust
package, its seam fixture, and this experimental spec; no persisted format or runtime receipt changes.

Acceptance evidence:

1. focused Rust and common affected-selection tests pass;
2. two clean real Git repositories report all direct test attributes anchored, with the numerator
   and denominator stated separately from macro-expanded runtime discovery;
3. the full Go build/test/vet and Python compile/test gates pass;
4. `src/**` remains byte-untouched; and
5. full `conformance/cli-parity-v0` output remains green using a freshly built, path-proven
   `corvint` binary.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `RUST-AFFECTED-V0-001..002`, `RUST-AFFECTED-V0-004..009` | `internal/liveverify/affected/rust/rust.go` | Rust focused tests and frontier assertions |
| `RUST-AFFECTED-V0-003` | `runnableDocComment`, `docFence`, `runnableDocFence`, `rustDocAttributes` | `TestFencedDoctestLanguageAndClosingFence` (including different-character and shorter nested markers inside a four-marker non-Rust fence) plus Rust test-form and indented-doctest tests |
| `RUST-AFFECTED-V0-010` | common seam fixture and opt-in ground-truth gate | affected-selection suite and real-repository runs |
| `LPCV-V0-013/014/016/019` | unchanged common `Build` / `Select` path | shared witness, exclusion, widening, determinism tests |
