# Rust Analyzer Candidate V0

Owner: Russell Lewis
Date: 2026-08-29
Intent status: accepted
Delivery status: deferred
Disposition: deferred by decision 0132; return only with an accepted qualification-and-launch profile.
Authoritative inputs: `AGENTS.md`, `docs/specs/analyzer-capability-contract-v0.md`, and `docs/specs/analyzer-candidate-profiles.md`.

## Agent digest
- Claim: An isolated unregistered Rust extractor emits bounded structural Cargo and Rust facts without Core selection or support.
- Status: accepted/deferred
- Exists: isolated `internal/analyzerrust` and `cmd/corvint-analyzer-rust` candidate implementation.
- Blocked on: admission, compatibility, host containment, support, and promotion evidence remain `NOT_RUN`.
- Read next: Why this document exists (decision 0007, D6); Evidence and rollback (implementation/evidence table).

## Why this document exists

`docs/specs/analyzer-candidate-profiles.md` permits named families and separately specified
experimental families. At proposal time, Rust needed an owner-ratified clause to use that extension;
implementation alone could not establish its legitimacy. Decision 0007, D6 ratified this document
and the `rust` family. The generic contract still expresses the extension as a separately specified
experimental family; this document supplies Rust's contract through that route, like
[`shader-glsl-metal-analyzer-candidate-v0.md`](shader-glsl-metal-analyzer-candidate-v0.md) and
[`kotlin-android-candidate-v0.md`](kotlin-android-candidate-v0.md). The addition preserves existing
family identifiers and wire shapes under `CF-V0-029`; ratification grants no supported Core profile.

**Intent is accepted** by [decision 0007, D6](../decisions/0007-omnibus-ratification-2026-08-29.md#d6--rust-analyzer-family-ratified). Rust remains an experimental candidate, not a supported family.
Ratification does not grant Core selection, qualification, or admission, however complete its code is.
The separately accepted ACC-V0 profile and its evidence are still required.

## Scope and boundary

This is an unregistered native-Go structural fact extractor. It receives only one bounded canonical
JSON/LF frame of caller-owned bytes and digests. It never opens a repository or ambient file, reads
PATH/HOME/CWD or the environment, invokes `cargo`, `rustc`, a build script, a proc macro, a shell,
or any process, uses a network or dynamic loader, or emits admission, compatibility, authority,
compile, or semantic `PASS` facts. Core has no dependency edge to it and it is unselectable pending
a separately accepted ACC-V0 profile.

## Requirements

- `RAC-001`: the wire family is exactly `rust` and the transport profile is the generic
`corvint-analyzer-candidate/experimental`. Unlike the shader extension, this family **reuses the
generic request envelope, the generic eight-field fact shape, and the frozen fourteen-field evidence
digest verbatim**. It adds no request field, no fact field, and no domain separator of its own, so it
alters no existing family bytes and introduces no new cryptographic surface. Its success and
rejection frames are byte-identical in shape to the `go` vectors frozen in
`internal/analyzercap/candidate_profile_vectors_test.go`.

- `RAC-002`: facts have exactly one input witness, so `related_handle` is always `-` and its evidence
digest slot is `-`. A fact is one **observation**: the same observation made twice in one input —
two `#[derive]` attributes in a file, two `cfg`-gated items — is recorded once rather than emitted
twice, which is what keeps the output strictly increasing at the source. The post-sort duplicate
check remains behind that as a terminal invariant, so a duplicate that reaches the output is fatal
and never silently removed. No span, witness, or source-text field is emitted: this family does not
replace the generic fact shape, so it does not carry the shader family's positional fields.

- `RAC-003`: the input record families are exactly `rust.manifest`, `rust.toolchain`, `rust.lock`, and
`rust.source`. The declared family selects the grammar; nothing is inferred from the path suffix,
and the path must independently agree with the family before any byte is parsed.

- `RAC-004`: source extraction is lexical only. A construct outside the closed static forms is
**inert** — it yields no fact — matching the `js.source` and `ruby.source` rules. Rejection is
reserved for lexical failure, unsupported bytes, and dynamic external input. Nothing asserts that
any input compiles, resolves, or links. `const` directly after `*`, `<`, or `,` is a raw-pointer
type or const-generic parameter and declares nothing. `#[test]` marks only the item it annotates:
a `;`, `{`, `}`, `mod`, `use`, or `extern` read before that item's `fn` name clears the mark.

- `RAC-005`: the lexer is fail-closed on the constructs where Rust differs from C-family languages.
Block comments **nest** (`/* /* */ */`) and only close at depth zero. Raw strings
(`r"…"`, `r#"…"#`, `br#"…"#`, `cr#"…"#`) close only on a quote followed by exactly the opening hash
run; a longer immediate hash run remains body text rather than closing at its matching prefix. A
leading `r#` is a raw string when a quote follows the hashes and a raw identifier when an
identifier start follows exactly one hash. `'` is never a string delimiter: a complete character or
byte literal is consumed as a literal, and an escape cannot consume a literal LF. Every other `'`
form (`'a`, `'static`, `'_`, `'outer:`) is a lifetime or label. An unterminated comment, string, raw string, or character literal, an
unbalanced delimiter or a closer that does not match the innermost open `{`, `[`, or `(`, a `CR` or `NUL` byte, a non-UTF-8 byte, a non-ASCII identifier, or an
unsupported punctuation byte rejects the whole request.

- `RAC-006`: `include!`, `include_str!`, and `include_bytes!` name content the caller did not supply.
They are dynamic external-input directives and reject the request as `DYNAMIC_INPUT`, exactly as
`//go:embed` does in the `go.source` rule. Every other macro invocation and each
`macro_rules! NAME` definition is inert as one complete delimited token tree: a macro body is
arbitrary token soup and none of its tokens can yield a closed fact.

- `RAC-007`: conditional compilation is **never resolved**. The exact target coordinates are only
`darwin/arm64/none` and `linux/amd64/none`, and `target.features` must be empty; every other OS,
architecture, ABI, or feature triple rejects as `UNSUPPORTED_SCHEMA`. The target binds the receipt;
it does not select source. A `#[cfg]` or `#[cfg_attr]` attribute emits a `rust.cfg.unevaluated` fact
so a consumer can see that the declaration set was not target-resolved, and an attribute contributes
only its leading path — never its arguments — so no derive list, cfg predicate, or arbitrary token
payload can become a fact value. The scanner consumes the complete bracketed attribute before it
resumes, so tokens in a path tail or argument payload cannot be reinterpreted as source facts.

- `RAC-008`: a `use` tree that is grouped, glob, or aliased is not a closed atom. It emits a
`rust.use.unevaluated` fact naming only its head segment rather than silently emitting nothing,
preserving explicit uncertainty over silence. A static import needs exactly one adjacent `::`
between segments (a leading `::` is allowed) and a terminating `;`; a single, spaced, tripled, or
trailing colon separator, or a tree cut off by end of input, is likewise unevaluated.

- `RAC-009`: decode, base64, total input, row, token, fact, and prospective output limits are checked
before data is retained. Base64 uses strict decoding plus exact re-encoding, and the supplied digest
is verified before any byte reaches a grammar. The prospective output charge is the **encoded** byte
count of the empty success envelope and of each complete fact, not an arithmetic model of it. Output
is one complete LF frame; the reason taxonomy is closed and an impossible implementation reason
reduces to `ANALYZER_FAILURE`, never a new string.

- `RAC-010`: rejection is two-stage. Until the entire echoed envelope — family, request ID, scope,
compilation unit, target, and every input handle and digest — has passed its closed grammar and
duplicate checks, a failure emits the minimal fixed sentinel carrying only `NONCANONICAL_REQUEST`,
`MALFORMED_INPUT`, `UNKNOWN_FIELD`, `UNKNOWN_FAMILY`, `INVALID_IDENTIFIER`, `INVALID_PATH`,
`DUPLICATE_VALUE`, or `LIMIT_EXCEEDED`, and no request binding. After that point every failure binds
the complete request in request-input order, so a rejection cannot replay across different inputs,
scope, compilation unit, or target. A bound rejection that would itself exceed the output ceiling
falls back to the sentinel. No failure emits a fact, echoes a source-derived value, or retains
diagnostic text or a path.

- `RAC-011`: this candidate provides neither CEM nor OCM production. Its version receipt names both
states `UNSUPPORTED`; no local CEM/OCM closure artifact or promotion claim is emitted.

## Closed input matrix

| Family | Input record | Closed accepted schema |
|---|---|---|
| Rust | `rust.manifest` | exactly `Cargo.toml` or `*/Cargo.toml`. UTF-8 LF rows of a closed TOML subset, not a TOML parser. Sections are exactly `[package]`, `[dependencies]`, `[dev-dependencies]`, and `[build-dependencies]`, each at most once. `[package]` keys are exactly `name:RUST_NAME`, `version:CORE_VERSION`, `edition:RUST_EDITION`, and `rust-version:CORE_VERSION`; `name` is required. Dependency rows are exactly `RUST_NAME = "CORE_VERSION"`. Rows are `key = value` with single spaces around the equals. Inline tables, arrays, multi-line/literal strings, dotted keys (`edition.workspace`), array-of-tables headers, `[target.'cfg(…)'.dependencies]`, datetimes, non-decimal or underscored numerals, indentation, escapes, unknown sections, unknown keys, duplicate sections, and duplicate keys reject. Ranges, caret/tilde/wildcard operators, and git/path/workspace sources are not exact versions and reject, matching the Ruby rule. |
| Rust | `rust.toolchain` | exactly `rust-toolchain.toml` or `*/rust-toolchain.toml`, with exactly one `[toolchain]` section carrying exactly one `channel` row whose value is `CORE_VERSION` or exactly `stable`, `beta`, or `nightly`. Every other key, section, and profile/component/target row rejects. |
| Rust | `rust.lock` | not implemented by the initial candidate; reject as `UNSUPPORTED_SCHEMA` until an exact `Cargo.lock` grammar — including its multi-line `dependencies` arrays, `[[package]]` identity, and checksum/source value rules — is added here. |
| Rust | `rust.source` | any `RUST_PATH` ending `.rs`, UTF-8 LF without CR or NUL, wholly consumed by the bounded lexical-state scanner of `RAC-005`. Comments, nested block comments, strings, raw strings, byte/C strings, character and byte literals, lifetimes, labels, raw identifiers, attributes, and numeric literals are recognized and are inert except where the fact matrix names them. |

The **manifest rule** — the analogue of the `ruby.version` rule — is the `rust.toolchain` row above:
exactly one `[toolchain]` section with exactly one `channel` value, no prefix, range, component list,
profile, date-suffixed channel, or host-triple alias. A pinned `CORE_VERSION` channel is compared
with a declared `rust-version`; a channel older than the declared minimum is `CONFLICTING_VALUE`. A
named channel is not a version and never conflicts.

## Atom grammars

- `RUST_IDENT`: `[A-Za-z_][A-Za-z0-9_]{0,127}`. ASCII only. A non-ASCII identifier is outside this
  closed grammar and rejects as `UNSUPPORTED_SCHEMA` rather than being accepted in its tail. A raw
  identifier `r#NAME` yields `NAME`, and `r#crate`, `r#self`, `r#super`, and `r#Self` reject because
  Rust forbids them.
- `RUST_PATH_ATOM`: `::`-separated `RUST_IDENT` segments, with `crate` or `self` permitted only as
  the head and `super` permitted in any leading run. A segment that is any other Rust keyword, or
  that carries generics, turbofish, braces, a glob, or an alias, is not a nameable path.
- `RUST_NAME`: `[A-Za-z0-9][A-Za-z0-9_-]{0,63}` — a Cargo package name. Hyphen and underscore
  spellings are distinct textual identities and are never normalized into one another.
- `RUST_EDITION`: exactly `2015`, `2018`, `2021`, or `2024`. Any other value is `UNSUPPORTED_SCHEMA`,
  not normalized.
- `CORE_VERSION` is the shared atom from `analyzer-candidate-profiles.md`, and `RUST_PATH` is the
  shared closed logical-path grammar.

Fact subjects, predicates, values, and instance IDs are printable ASCII within the shared field
ceiling, narrowed to `RUST_IDENT`, `RUST_PATH_ATOM`, `RUST_NAME`, `CORE_VERSION`, `RUST_EDITION`, or
a logical path. No raw URL, credential, source line, literal body, or source body is a fact value.

## Closed fact matrix

| Fact kind | Subject | Predicate | Value |
|---|---|---|---|
| `rust.package` | scope | `declares-package` | `RUST_NAME` |
| `rust.package.version` | scope | `declares-version` | `CORE_VERSION` |
| `rust.edition` | `rust` | `declares-edition` | `RUST_EDITION` |
| `rust.language.declaration` | `rust` | `declares-language` | `CORE_VERSION` |
| `rust.toolchain.declaration` | `rust` | `declares-toolchain` | channel |
| `rust.dependency` | `RUST_NAME` | `requires`, `requires-for-development`, `requires-for-build` | `CORE_VERSION` |
| `rust.module` | source path | `declares-module` | `RUST_IDENT` |
| `rust.import.static` | source path | `imports` | `RUST_PATH_ATOM` |
| `rust.use.unevaluated` | source path | `imports-unevaluated` | head `RUST_IDENT` |
| `rust.extern.crate` | source path | `links-crate` | `RUST_IDENT` |
| `rust.declaration` | source path | `declares-fn`, `-struct`, `-enum`, `-trait`, `-union`, `-type`, `-const`, `-static` | `RUST_IDENT` |
| `rust.test` | source path | `declares-test` | `RUST_IDENT` |
| `rust.attribute` | source path | `carries-attribute` | attribute path `RUST_IDENT` |
| `rust.cfg.unevaluated` | source path | `declares-conditional-compilation` | `cfg` or `cfg_attr` |

No resolved dependency, resolved module file, workspace membership, inherited field, edge,
relationship, target, feature, build-script, doc-test, installed-tool, compatibility, runtime,
support, or `PASS` fact is in this matrix. An `impl` block declares no single name and therefore
emits no declaration.

## Bounds

Request bytes 1,500,000; aggregate decoded input bytes and LF-inclusive output bytes each 1,048,576;
inputs 128; features 64; facts 4,096; JSON nesting 8; JSON tokens 4,096; input base64 strings
1,398,104; fact fields 4,096; identifiers 128; logical paths 4,096 with 128-byte segments; source
bytes 262,144; source tokens 262,144; manifest bytes 65,536; manifest rows 8,192; delimiter and
block-comment nesting 64; raw-string hashes 255.

## Deliberate divergences from the behavioural reference

The Python reference (`src/beamfall_corvint/rust_indexing.py` on `codex/corvint-rust`) is a reference,
not an oracle, for a family that has no ratified spec. Where it and this specification disagree,
**this specification wins**, and each divergence below is deliberate.

- **Raw identifiers are read correctly.** The reference captures `fn r#match` as the name `r`, drops
  `mod r#type` entirely, and drops `extern crate r#foo`. This candidate yields `match`, `type`, and
  `foo`. The reference's behaviour is a defect, not a contract.
- **`edition` and `rust-version` are read directly from `[package]`.** The reference's `[package]`
  whitelist silently discards both, reaching an edition only through workspace inheritance. Reading a
  declared scalar from the manifest that declares it is the same rule `go.mod`'s `go X.Y.Z` follows.
- **`Cargo.lock` is declared but unimplemented.** The reference never reads a lockfile at all. Rather
  than invent an unreviewed grammar, this candidate takes the `js.bun-lock-v1` and
  `dotnet.packages-lock-v1` route and rejects it as `UNSUPPORTED_SCHEMA`.
- **No cross-file resolution.** The reference resolves `use` paths and `mod` declarations to files,
  derives workspace membership from globs and a path-dependency BFS, enrols implicit members,
  topologically sorts for cycles, and emits `imports`/`tests` edges. None of that is carried: an
  analyzer candidate emits per-input facts and Core owns resolution. Carrying it would also require
  ambient multi-file reasoning this boundary forbids.
- **Diagnostics become facts or rejections.** The reference has a large `analysis_truncated` /
  `*_uncertain` diagnostic taxonomy and always partially analyses a `.rs` file. This candidate has no
  diagnostic channel: a lexical failure rejects the whole request, and the two uncertainties worth
  preserving are carried as the explicit `rust.cfg.unevaluated` and `rust.use.unevaluated` facts.
- **No positions.** The reference records line numbers. The generic fact shape has no span field and
  this family does not replace it, so no position is emitted.
- **Dependency kinds are distinct predicates.** The reference tags a dependency record with a kind;
  here `dev-` and `build-` dependencies carry distinct predicates so they can never be conflated.

Behaviours carried from the reference, each of which its three adversarial review rounds established
the hard way: nested block comments; variable-hash raw strings; the raw-string/raw-identifier
disambiguation; `'` as a lifetime rather than a string delimiter; byte and C-string prefixes;
unterminated forms failing rather than leaking; `mod` never matching an identifier merely ending in
`mod`; dynamic `include!` targets refusing to be guessed; hyphen and underscore crate spellings kept
distinct; and explicit uncertainty preferred over silence.

## Evidence and rollback

Focused package tests provide the closed envelope and canonicality boundaries, the exact
fourteen-field evidence digest against the frozen generic construction, fact ordering and duplicate
terminality, prospective-output accounting at and over the ceiling, the full rejection taxonomy, the
hostile-lexer corpus for every construct in `RAC-005`, the manifest and toolchain grammars including
their conflict rule, and CLI read/write/argument paths. The candidate's reachable production source
is measured by `internal/analyzercap/source_ceiling_test.go` and sits under the unamended ACP-009
base ceiling, so no per-candidate ceiling amendment is claimed. `go list -deps` shows the executable
reaches exactly its own package and `cmd/corvint-analyzer-rust`, with no Core dependency edge.

A read-only observation over 64 genuine Rust files accepted 59 and produced 1,354 facts with no
lexical false positive; the five rejections were all genuine `include!`, `include_str!`, or
`include_bytes!` dynamic external input. This is a coverage observation on one incidental corpus, not
a pinned dogfood corpus and not a qualification claim: a byte-pinned corpus with frozen expected
results remains `NOT_RUN`.

Exact OS syscall tracing, semantic compilation, admission, exact verification, installation, launch,
host containment, and support remain `NOT_RUN`; CEM and OCM are `UNSUPPORTED`. This specification
makes no claim that any promotion condition is satisfied.

Rollback is reverting the candidate commit: `internal/analyzerrust/`, `cmd/corvint-analyzer-rust/`,
this document, and its index row. No registry, lock, or install state exists, and no existing
candidate's bytes, ceiling, or family list entry is touched.

| Requirement | Implementation | Evidence |
|---|---|---|
| `RAC-001..003` | `internal/analyzerrust/analyze.go` | envelope, canonicality, evidence-digest, and input-matrix tests |
| `RAC-004..008` | `rust_lexer.go`, `rust_source.go` | hostile-lexer corpus and source fact tests, including `TestMacroBodiesAreInert`, `TestAttributePayloadIsInert`, `TestRawStringTerminatorRequiresExactHashRun`, `TestCharacterEscapeCannotConsumeLineFeed`, `TestTypePositionConstDeclaresNothing`, and `TestTestAttributeBindsOnlyTheNextItem` |
| manifest rule, atoms | `cargo.go` | manifest, toolchain, conflict, and atom-boundary tests |
| `RAC-009..010` | `analyze.go`, `facts.go` | bound, ordering, output-accounting, and two-stage rejection tests |
| `RAC-011` | `cmd/corvint-analyzer-rust/main.go` | explicit `UNSUPPORTED` CEM/OCM receipt test |
| ratification | owner acceptance of this document | decision 0007, D6; qualification remains `NOT_RUN` |
