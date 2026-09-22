# Initial Analyzer Candidate Profiles

Owner: Russell Lewis
Date: 2026-08-25
Intent status: proposed
Delivery status: deferred
Disposition: deferred by decision 0132; return only with an accepted qualification-and-launch profile.
Authoritative inputs: `AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`docs/specs/analyzer-capability-contract-v0.md`, and the owner's direction to support Go, Python 3.12,
JavaScript/TypeScript, .NET, Ruby, and shell through separately installed native Go plugins.
Owner amendment anchor: direct 2026-08-25 task instructions to build what Corvint needs for Beamfall
dogfood, keep language support in loadable plugins rather than one giant binary, and distinguish
language/library versions individually. The Swift/Apple candidate is separately specified in
[`swift-apple-analyzer-candidate-v0.md`](swift-apple-analyzer-candidate-v0.md); its stricter
closed coordinate vector is an experimental fifth family, not a profile acceptance.

## Agent digest
- Claim: Isolated native-Go analyzer candidates emit bounded structural facts without Core registration, selection, launch, or support.
- Status: proposed/deferred
- Exists: `internal/analyzer*` and separately built `cmd/corvint-analyzer-*` commands.
- Blocked on: an accepted, exactly reviewed qualification profile for each candidate.
- Read next: `analyzer-capability-contract-v0.md` and the family-specific candidate specs.

## User and measurable job

A Corvint developer can build and adversarially test five isolated native Go fact extractors without
adding their parsers to the Core binary or implying that Corvint can select, launch, trust, or promote
their output. Each candidate consumes only caller-supplied immutable bytes with exact handles and
digests, returns deterministic bounded candidate facts, and remains unavailable to Core until an
individually accepted exact profile closes the Analyzer Capability Contract.

The experimental job succeeds when a candidate's closed input and fact matrix passes exact-byte,
hostile-input, race, cross-platform build, non-execution, size, and allocation gates. It does not
establish runtime compatibility or user-facing language support.

## Verified current state

- Core contains the experimental resolver/profile foundation plus an isolated, unqualified
  containment boundary. It has no accepted analyzer registry, terminal repository lock, supported
  containment-backed launch, profile admission, exact verifier, or release qualification.
- Isolated branches contain initial native Go candidates for Go, Python 3.12, JavaScript/TypeScript, .NET,
  Ruby, SQLite source text, and shell. None is imported by `cmd/corvint`, registered, installed,
  selected, or launched.
- Exact review rejected the first JavaScript/TypeScript and .NET candidates for permissive parsing,
  nondeterministic tuple keys, unbounded amplification, and unsupported exact-runtime claims. Those
  branches remain non-integrated until repaired and re-reviewed.
- The locally observed toolchains are evidence for fixtures only: Go 1.27.0; Node 22.23.2; Bun
  1.3.11; .NET SDK 8.0.423 and 9.0.316; Ruby 2.6.10 and Bundler 1.17.2. Observation does not qualify
  a host/target/binary/profile/clause/verifier tuple.
- The SQL prereview lifecycle harness failed its second exact review and is excluded from this
  rebuilt analyzer slice. Its receipt, lock, and supervisor behavior is neither integrated nor
  claimed here; the analyzer candidate remains independently reviewable without that local harness.

## Requirements

- `ACP-001`: Every family MUST remain a separately buildable executable and package. Core MUST NOT
  import a candidate package, embed its parser or fixtures, register it, install it, or select it.
  A Core build at the same revision MUST have zero analyzer-package dependency edges.
- `ACP-002`: A candidate MUST consume one canonical request containing only a bounded ordered set of
  caller-owned input records using the frozen experimental envelope below. Every record MUST have a
  canonical input handle, declared closed family, scope and compilation-unit identity, exact target
  identity, SHA-256 digest, and immutable bytes. The candidate MUST verify each digest before parsing
  and bind every fact to the exact input handle and compilation unit that produced it.
- `ACP-003`: A candidate MUST emit only mechanically extracted candidate facts. A requested profile,
  local executable observation, manifest declaration, version range, alias, mutable tag, or package
  name MUST NOT be converted into an exact installed-runtime, compiler, framework, or compatibility
  fact. A required exact binding with zero matches rejects the whole request as
  `EXACT_BINDING_UNAVAILABLE`; multiple matches reject it as `AMBIGUOUS_BINDING`. Neither condition
  may be omitted, downgraded to a diagnostic, or resolved by fallback.
- `ACP-004`: Each input family MUST have a closed field, element, attribute, namespace, tuple,
  version, grammar, and context matrix. Unknown, duplicate, conflicting, dynamic, interpolated,
  globbed, absolute, escaping, control-bearing, future-version, or ambiguous data MUST fail the
  whole request before usable output. Permitted but ignored open subtrees are forbidden.
- `ACP-005`: Canonical output MUST use typed field-by-field ordering, not delimiter-concatenated sort
  keys. Input order and map iteration MUST NOT affect bytes. Control bytes are rejected before
  sorting or diagnostics. Errors MUST use the same bounded canonical JSON encoder as successes.
- `ACP-006`: Global limits MUST be consumed before allocation or append. One request permits at most
  128 inputs, 1,048,576 aggregate input bytes, 4,096 facts, 128 diagnostics, 64 scopes, 64
  compilation units, 4,096 dependency or import edges, 4,096 source files, 4,096 packages, 128-byte
  identifiers, 4,096-byte individual fact fields, and 1,048,576 output bytes including framing.
  A breached limit returns no partial fact set. Before retaining each fact, the candidate MUST add
  that fact's exact canonical encoded size plus the still-required closing/framing bytes to a
  prospective output counter and reject before append if the cap would be crossed. Family-specific
  limits may only be lower.
- `ACP-007`: Candidates MUST NOT read a repository or ambient file, invoke Git or a language
  toolchain, consult `PATH`, execute a subprocess or shell, load a dynamic library, evaluate source,
  access credentials, use the network, download, restore, install, mutate input, or emit semantic
  `PASS`, authority, admission, compatibility, policy, or exact-verifier state.
- `ACP-008`: The candidate request and output are an experimental fact-extractor envelope, not the
  accepted ACC-V0 launch protocol. Registry, lock, projection, protocol echo, containment, admission,
  exact verification, cache, five-warmup/100-fresh-process stage qualification, installation,
  selection, and launch remain explicitly `NOT_RUN` until a separate owner-accepted exact profile
  supplies those identities and gates.
- `ACP-009`: Each candidate MUST satisfy the absolute allocation ceilings and reproducible build
  recipe below. A causal ratchet is required only after this document names its exact reviewed
  baseline commit and measurement. Informational wall time may be recorded but MUST NOT be called
  ACC-V0-020 evidence. Production Go source for one candidate MUST remain at most 65,536 bytes, its
  standalone stripped binary at most 6,291,456 bytes, and the Core binary delta caused by the
  unreferenced candidate MUST be exactly zero bytes. The source ceiling is measured per candidate
  CLI as reachable production source: the byte total of every non-test Go production file in
  every first-party package of the CLI's dependency closure. A package hosting several
  candidate CLIs (the native bridge hosts c-jni, java, and objective-c) is counted in full for
  EACH of its CLIs — the shared closure is reachable from every executable, so dividing a
  package among its CLIs is not a measurement. A candidate MAY amend its own ceiling only through a row in
  the per-candidate ceiling table below, carrying its byte accounting in `docs/BUILD-LOG.md`; an
  amendment never changes any other candidate's ceiling.
- Per-candidate ceiling amendments:

  | Candidate | Amended source ceiling | Accounting |
  |---|---|---|
  | `javascript-typescript` | 92,045 bytes | `docs/BUILD-LOG.md` 2026-08-27 (walker consolidation 78,318 → 76,331 → 76,300 net of soundness fixes; member-name slash-state closure +68 bytes with comment reduction exhausted; arrow-parameter grammar +1,619 bytes, expression-state JSX gating +171 bytes, destructured declarations +134 bytes after the observed shadow run rejected every Playwright spec; round-1 review repairs +2,433 bytes: declarator-tail scan for later `require` bindings and `for await` control preservation; round-2 repairs +6,003 bytes: closed binding-pattern/formal-parameter grammar, spread tokenization, fail-closed later declarators; round-3 repair +2,563 bytes: boundary-aware ambiguous-JSX fail-closed in bounded scanners; round-4 repair +104 bytes: block-comment line terminators and non-JSX span overruns/unbalanced nesting fail closed; round-5 repair +12 bytes: U+2028/U+2029 block-comment line terminators fail closed); amended 2026-09-12 +888 bytes (`docs/BUILD-LOG.md` 2026-09-12): import clauses naming `require` fail closed, and export declarations and `catch` parameters go through the `require` binding rules; amended 2026-09-13 +1,229 bytes (`docs/BUILD-LOG.md` 2026-09-13): hoisted, later-declarator, method-parameter, and TypeScript `enum`/`namespace` `require` bindings fail closed; amended 2026-09-13 +175 bytes (`docs/BUILD-LOG.md` 2026-09-13): path segments may start with any segment byte, and a lock must correlate every declared workspace; amended 2026-09-13 +340 bytes (`ACP-011`): the CLI refuses unexpected argv instead of analyzing stdin; amended 2026-09-17 +6 bytes (decision 0308): the candidate profile and evidence-domain identity strings carry the Corvint name |
  | `python` | 90,197 bytes | `docs/BUILD-LOG.md` 2026-08-27 (closed header expression grammar built across six exact-review rounds; measured as reachable production source of `cmd/corvint-analyzer-python`); amended 2026-08-28 +899 bytes for `SourceSyntaxValid`, the syntax-only route that lets the OCM claim path reuse the closed grammar without the analyzer's request/fact/output capability limits, so OCM enforces its own 4 MiB blob ceiling while `MaxInputBytes` stays frozen at 1,048,576; amended 2026-08-28 +12,730 ceiling bytes for validation-only necessary grammar covering opaque expression starts, operand adjacency, keyword and compound shapes, decorator/type-alias spans, simple f-string fields, and numeric literals while preserving the 72-file repository corpus; amended again 2026-08-28 +321 bytes to reject empty statement spans, which previously panicked `SourceSyntaxValid` with an index-out-of-range on `;`, `;;`, `x = 1;;`, and `;x = 1`; amended 2026-08-28 +2,421 bytes for the operator-directed extraction of the closed grammar into the neutral `internal/pythongrammar` package, which lets Core reach the grammar without reaching the separately buildable candidate (PNC-001). The amendment is seam cost only: no grammar rule, capability, or limit changed, the reachable closure is the same code plus a package boundary (package declaration, the `Failure` type and its four shared constants, the `FactSink` output interface, the `ParsePython312Subset` entry point, and the analyzer-side `boundSink` that rebinds facts to its request envelope), and the forwarding-wrapper layer was removed so the lexical helpers export directly; restructured 2026-08-29 with NO ceiling change, reachable production source 93,612 → 77,296 bytes (12,901 under the unchanged 90,197 ceiling): `SourceSyntaxValid` and the validation-only necessary grammar moved to `internal/pythonsyntax`, which imports `pythongrammar` instead of living inside it, so the candidate no longer statically reaches a route it never executes — the analyzer has always called the fact-extraction entry point with syntax validation off (`internal/analyzerpython/analyzer.go`), yet `go list -deps` charged it all 16,210 bytes of the syntax file because the `validateSyntax` parameter branched into it. The restructure is seam cost only: no grammar rule, capability, or limit changed, and the moved code is byte-identical apart from its package clause and the qualified names it now uses. `ParsePython312Subset` lost that parameter to a sibling `ParseChecked(subject, source, collector, SyntaxCheck)` which `pythonsyntax` supplies its own check to, and the parser's shared token helpers were exported unchanged so the moved code could keep calling them |
  | native bridge `c-jni` | 76,380 bytes | shared bridge closure (76,064 bytes, measured 2026-09-13) is reachable from every bridge CLI; splitting it would duplicate reviewed source and raise the total reviewed surface; amended 2026-09-13 +29 bytes (`docs/BUILD-LOG.md` 2026-09-13) for the `c.cmake` exact-filename fix |
  | native bridge `java` | 76,382 bytes | same shared-closure accounting as `c-jni` |
  | native bridge `objective-c` | 76,395 bytes | same shared-closure accounting as `c-jni` |

  The `dotnet` candidate takes the unamended 65,536-byte base ceiling and carries no row above.
  Its measured reachable production source is 54,074 bytes across the two first-party packages of
  `cmd/corvint-analyzer-dotnet`'s closure (`internal/analyzerdotnet` 52,616 bytes over five files,
  plus the command's 1,458 bytes), leaving 11,462 bytes of headroom -- clear of the 4,096-byte
  warning band. A row here would lower this candidate's ceiling below the base and convert the
  bound into an exact ratchet at today's byte count, which is not what an amendment is for.
  Accounting: `docs/BUILD-LOG.md` 2026-08-29.
- `ACP-010`: Integration MUST require a fresh read-only exact review PASS over the precise commit.
  Integration changes only source availability for further testing. It MUST NOT change default CLI
  behavior, packaging, registry state, support documentation, or the private installed dogfood
  binary. Reverting the candidate commit MUST fully remove the experimental family.
- `ACP-011`: A candidate CLI MUST NOT silently analyze stdin when invoked with an unexpected
  command-line argument. Receiving argv beyond the bare invocation MUST reject exactly like a
  malformed request: the process writes the family's canonical `NONCANONICAL_REQUEST` rejection
  (or that family's existing read-error rejection shape) and MUST NOT attempt to read or decode
  stdin. This mirrors the `html-css`, `ruby`, `rust`, `shader`, and `shell` candidates, which
  already reject unexpected argv, and extends the same contract to every other candidate sharing
  the `corvint-analyzer-candidate/experimental` profile. The rejection frame exits 0; a nonzero exit
  is reserved for a frame that cannot be written in full (decision 0233, matching `ACP-012`).
  The only admitted non-analysis argv is exactly one `--version` argument on `html-css`, `rust`,
  and `shader`, whose family contracts declare a version receipt: it writes that receipt, reads no
  stdin, and exits 0 unless the receipt cannot be written in full. On `go`, `python`,
  `javascript-typescript`, `dotnet`, `kotlin-android`, `sql-native`, `ruby`, and `shell`,
  `--version` is unexpected argv, and `--version` followed by any argument is unexpected argv on
  every candidate (decision 0236).
- `ACP-012`: A candidate CLI that reads its request from standard input MUST classify the two
  pre-decode stdin failures identically (decision 0228). A request longer than 1,500,000 bytes
  MUST emit the fixed pre-envelope sentinel with `LIMIT_EXCEEDED`, whatever its content or
  framing. A stdin read failure (a reader error, a descriptor stat failure, or detected descriptor
  identity drift) MUST emit the fixed sentinel with `NONCANONICAL_REQUEST`; `ANALYZER_FAILURE` is not
  a permitted sentinel reason. Both frames exit 0; a nonzero exit is reserved for a frame that
  cannot be written in full. This binds `go`, `javascript-typescript`, `dotnet`, `html-css`,
  `python`, `ruby`, `shell`, `rust`, `shader`, `sql-native`, `kotlin-android`, `structured-data`,
  and the native bridge CLIs. It does not bind `swift`, which reads a `--request-file` descriptor
  under its own profile, and it does not change a library entry point's own reason for an
  oversize buffer.
## Frozen experimental envelope

This envelope is deliberately not ACC-V0's future launch protocol. Its generic literal profile is
`corvint-analyzer-candidate/experimental`; an exact closed-family override below replaces that generic
identity for its candidate bytes rather than creating a release-version ladder.

The command first reads at most 1,500,000 bytes through a bounded reader and rejects byte 1,500,001
without decoding. It then accepts exactly one UTF-8 JSON object followed by one LF and no other
bytes. Decoder depth is at most 8, JSON tokens at most 4,096, inputs at most 128, features at most
64, ordinary strings at most 4,096 bytes, one `content_base64` string at most 1,398,104 bytes, and
aggregate decoded content at most 1,048,576 bytes. Objects use
the field order shown below, arrays are already in the specified typed order, integers are base-10,
strings use the shortest RFC 8259 escape, and no insignificant whitespace is permitted. Decoding
MUST reject duplicate fields, canonicalize every unknown value without a `float64` conversion, and
require byte equality before inspecting content. An otherwise safe canonical extension produces the
bound `UNKNOWN_FIELD` envelope once the sentinel checks pass; noncanonical extension values do not
bypass canonicality, and any other unknown member is the `NONCANONICAL_REQUEST` sentinel. An
extension member follows its object's schema fields in increasing name order, and its numbers are
integers. Schema membership is an exact name match, so a member such as `"target inputs"` is an
extension, not a schema field (decisions 0221, 0222, and 0242; `TestSafeCanonicalExtensionBindsUnknownField` in `analyzergo`,
`analyzerpython`, `analyzerjs`, `analyzerdotnet`, `analyzerruby`, `analyzershell`,
`analyzerkotlinandroid`, `analyzershader`, and `analyzerswift`). `content_base64` is padded RFC 4648 base64. All
fields shown are required; there are no optional or `null` fields.

```json
{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input-1","family":"go.mod","path":"go.mod","sha256":"sha256:8d27e4f4ff6e3f8c1a8ced8f550a286065c62bff008c069e5f4c12061a00fa77","content_base64":"bW9kdWxlIGV4YW1wbGUuY29tL21vZHVsZQpnbyAxLjI3LjAK"}]}
```

`family` is exactly `go`, `python`, `javascript-typescript`, `dotnet`, `ruby`, `sqlite`, `html-css`, `shell`,
the separately specified `swift-apple`, or a separately specified experimental family. The shader
extension is defined only by
[`shader-glsl-metal-analyzer-candidate-v0.md`](shader-glsl-metal-analyzer-candidate-v0.md): its
additional exact `shader_profile`, span, and bounded literal witness fields replace this generic
fact shape for that family only. It does not alter any existing family bytes. `target.features` is a
strictly increasing, duplicate-free identifier array. Inputs are strictly increasing by the typed
tuple `(handle, family, path, sha256)` and duplicate handles or paths are rejected. `input_echoes`
preserves this exact canonical request-input order; it is never independently sorted.

A success is exactly one canonical object plus LF. Every fact field is required. `related_handle`
is `-` when the fact has only one input witness. Facts are strictly increasing by the typed tuple
`(kind,input_handle,related_handle,subject,predicate,value,instance_id,evidence_sha256)`; duplicates
are terminal rather than silently removed.

```json
{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","status":"CANDIDATE","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"input-1","sha256":"sha256:8d27e4f4ff6e3f8c1a8ced8f550a286065c62bff008c069e5f4c12061a00fa77"}],"facts":[{"kind":"go.language.declaration","input_handle":"input-1","related_handle":"-","subject":"go","predicate":"declares-language","value":"1.27.0","instance_id":"root","evidence_sha256":"sha256:d2a94a31be53d8284aed472d3335179733fa4cdfd216f0600777b5301c73fa73"},{"kind":"go.module","input_handle":"input-1","related_handle":"-","subject":"root","predicate":"declares-module","value":"example.com/module","instance_id":"root","evidence_sha256":"sha256:e8b895e223883c0f26260fdc3a46f8d9f46f8b6a8df88fb77fe946485f3bd5bc"}]}
```

`internal/analyzercap/candidate_profile_vectors_test.go` freezes these exact LF-framed bytes and
proves their decoded Go content, request digest, fact ordering, tuple values, and both 14-field
evidence digests. Every Go candidate MUST additionally prove its parser emits this success byte for
this request; the contract-vector test alone does not qualify a candidate implementation.

A failure emits no facts, echoes, source-derived value, diagnostic text, or path. It uses the fixed
sentinel until the entire echoed envelope (`family`, `request_id`, scope, unit, target, input
handles, and input digests) has passed its closed grammar and duplicate checks. The sentinel may
carry only `NONCANONICAL_REQUEST`, `INVALID_IDENTIFIER`, `INVALID_PATH`, `DIGEST_MISMATCH`,
`UNKNOWN_FAMILY`, `DUPLICATE_VALUE`, or `LIMIT_EXCEEDED` (structured-data, JavaScript/TypeScript,
Python, Kotlin/Android, Go, and native C/Objective-C/Java: `TestInvalidEnvelopeRejectionStaysUnbound`;
shader, decision 0213: `TestDuplicateHandleRejectionStaysUnbound`).

```json
{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}
```

After the complete request envelope is validated structurally, every failure binds the supplied
profile verbatim, family, request ID, scope, compilation unit, target, and every request input
digest in request-input order. A profile that is then rejected as unknown remains bound: otherwise
two complete requests with the same false input digest but different supplied profiles could
collapse to one replayable response (HTML/CSS: `TestExactFailureGoldensAndEchoEligibility`; JavaScript/TypeScript: `TestUnknownProfileRejectionStaysBound`). It is exactly:

```json
{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","status":"REJECTED","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"input-1","sha256":"sha256:8d27e4f4ff6e3f8c1a8ced8f550a286065c62bff008c069e5f4c12061a00fa77"}],"reason":"DIGEST_MISMATCH"}
```

`input_echoes` on rejection has the same closed two-field records and exact request order as
success. A rejection with only a matching commit-derived `request_id` is invalid; it cannot replay
across different inputs, scope, compilation unit, or target at that commit.

`unknown` is a reserved failure sentinel, not a request family. The closed reasons are
`NONCANONICAL_REQUEST`, `INVALID_IDENTIFIER`, `INVALID_PATH`,
`DIGEST_MISMATCH`, `UNKNOWN_FAMILY`, `UNKNOWN_FIELD`, `DUPLICATE_VALUE`, `CONFLICTING_VALUE`,
`MALFORMED_INPUT`, `UNSUPPORTED_SCHEMA`, `EXACT_BINDING_UNAVAILABLE`, `AMBIGUOUS_BINDING`,
`DYNAMIC_INPUT`, `CREDENTIAL_INPUT`, `LIMIT_EXCEEDED`, `OUTPUT_LIMIT`, and `ANALYZER_FAILURE`.

Identifiers match `[A-Za-z0-9][A-Za-z0-9._:@+~-]{0,127}`. A logical path is 1--4,096 bytes of
ASCII segments separated by `/`; a segment is 1--128 bytes from `[A-Za-z0-9._@+~-]`, but `.` and
`..` are forbidden. A path has no leading/trailing slash, empty segment, backslash, colon, control,
percent escape, or normalization alias. A segment may start with any of its permitted bytes, so `pages/_app.tsx` and `.storybook/main.ts` are paths (JavaScript/TypeScript: `TestPathSegmentsMayStartWithSegmentPunctuation`). SHA-256 values are `sha256:` plus 64 lowercase hexadecimal
digits. Fact subjects, predicates, values, and instance IDs are printable ASCII, 1--4,096 bytes,
and family grammars below may narrow them. No raw URL, credential, source line, or source body is a
fact value.

The prospective output counter starts with every canonical byte of the fixed success envelope,
profile/family/request/scope/unit/target/features, all input echoes, empty facts array, closing brace,
and LF. Before retaining a fact, a canonical size function charges its comma, keys, quotes, escapes,
values, and closing bytes; the candidate rejects if the total would exceed 1,048,576. The size
function and encoder share exact byte goldens, and final encoding rechecks equality and the cap.
Each input has a prospective 4,096-fact ceiling and the request has an independent prospective
4,096-fact ceiling; parsing emits directly into those counters and never first accumulates an
unbounded per-input fact slice.

`evidence_sha256` is not caller supplied. It is `sha256:` plus lowercase SHA-256 over this binary
preimage: the profile's exact evidence tag (the generic profile uses
`corvint-analyzer-candidate-evidence/experimental`), followed by a big-endian
uint32 field count of 14, then 14 big-endian-uint32-length-prefixed byte strings in this order:
`family`, `request_id`, `scope_id`, `compilation_unit_id`, canonical target-object JSON,
`input_handle`, that input's request SHA-256, `related_handle`, the related input's request SHA-256
or `-`, `kind`, `subject`, `predicate`, `value`, `instance_id`. Count/length overflow is
`LIMIT_EXCEEDED`. The candidate recomputes this frame only from its owned request bytes and fact;
the digest grants no authority.

## Closed family schema and fact matrices

Anything not listed in these tables is `UNSUPPORTED_SCHEMA`; an unknown field inside a listed
format is `UNKNOWN_FIELD`. A candidate may implement a strict subset only by returning
`UNSUPPORTED_SCHEMA` for the entire unimplemented input family. It MUST NOT accept and ignore it.

Closed lexical atoms used by the tables are:

- `CORE_VERSION`: `NUMBER.NUMBER.NUMBER`, where `NUMBER` is `0` or a nonzero ASCII digit followed by
  zero or more digits. No prefix, leading zero, prerelease, build, whitespace, or alternate spelling.
- `GO_VERSION`: `v` plus `CORE_VERSION`; pseudo-versions, `+incompatible`, queries, ranges, and
  replacements are outside this initial matrix.
- `GO_PATH`: slash-separated 1--128-byte ASCII segments from `[A-Za-z0-9._~-]`; no empty, `.`, or
  `..` segment, leading dot/dash, `@`, backslash, control, percent escape, or Unicode.
- `NPM_NAME`: either one lowercase segment from `[a-z0-9][a-z0-9._~-]{0,127}` or
  `@SCOPE/NAME` where both segments use that grammar. `NPM_INSTANCE_PATH` is empty for the root or
  slash-separated `node_modules/NPM_NAME` and declared workspace-path segments only.
- `NPM_INTEGRITY`: `sha512-` plus canonical padded base64 decoding to exactly 64 bytes.
  `NPM_RESOLVED` is `https://registry.npmjs.org/NPM_NAME/-/TARBALL.tgz` with exact lowercase host,
  no userinfo, port, query, fragment, percent escape, redirect, or alternate host; it is validated
  but never emitted. Dependency-map values in this initial matrix are `CORE_VERSION` only.
- `DOTNET_NAME`: `[A-Za-z0-9][A-Za-z0-9._+-]{0,127}`. `DOTNET_TFM` is exactly `net8.0` or `net9.0`;
  `DOTNET_RID` is exactly `osx-arm64`, `linux-x64`, or `win-x64`; `DOTNET_LANG` is exactly `12.0` or
  `13.0`; `DOTNET_GUID` is `{` plus eight-four-four-four-twelve uppercase hexadecimal digits with
  hyphens plus `}`. Semicolon lists are nonempty, strictly sorted, duplicate-free lists of the
  corresponding atom. Values outside these sets are `UNSUPPORTED_SCHEMA`, not normalized.
- `PYTHON_NAME`: one ASCII Python identifier that is not a keyword. `PYTHON_PATH` is the common
  closed logical-path grammar ending in `.py`. Python source facts use only parser positions and
  printable ASCII static names; arbitrary evaluated strings, aliases, and call arguments are never facts.
- `RUBY_NAME`: `[A-Za-z0-9][A-Za-z0-9._-]{0,127}`. `RUBY_PLATFORM` uses the same atom grammar.
  `RUBY_RUNTIME_VERSION` is `CORE_VERSION` optionally followed by `p` and a nonzero-leading patch
  integer. Gem dependency versions are exact `CORE_VERSION`; ranges and pessimistic operators reject.

| Family | Input record | Closed accepted schema |
|---|---|---|
| Go | `go.mod` | UTF-8 LF lines: one `module PATH`; one `go X.Y.Z`; optional one `toolchain goX.Y.Z`; `require PATH VERSION` lines or one parenthesized require block. In this closed profile `X.Y` is exactly `1.26` or `1.27` and `Z` is a decimal patch number `0`--`99` written without leading zeros, so real released patch versions (`1.26.5`, `1.26.6`, `1.27.0`) are admitted; a toolchain directive is `go1.26.Z` or `go1.27.Z` over the same patch range. Every other minor series, including `1.25.0` and `1.28.0`, and any patch outside `0`--`99`, rejects as `UNSUPPORTED_SCHEMA`. The patch component does not alter the directive grammar this analyzer reads, which is why it is admitted (decision 0007, D7). `replace`, `exclude`, `retract`, comments after tokens, unknown directives, and noncanonical spacing are rejected. |
| Go | `go.sum` | unique `PATH VERSION h1:BASE64` or `PATH VERSION/go.mod h1:BASE64` lines strictly ordered as the Go command writes them: by path, then numerically by `vX.Y.Z` version (`v0.9.0` before `v0.10.0`), then the archive line before its `/go.mod` line; an out-of-order line is `DUPLICATE_VALUE`; path/version/hash use the Go 1.27 module grammar and canonical base64. |
| Go | `go.work` | UTF-8 LF lines: one `go X.Y.Z`; optional one `toolchain goX.Y.Z`; `use PATH` lines or one parenthesized use block. The same closed `1.26.Z`/`1.27.Z` language and `go1.26.Z`/`go1.27.Z` toolchain tuple set applies, over the identical `0`--`99` patch range. When both inputs are supplied, `go.work` may equal or be newer than `go.mod`; an older workspace declaration is `CONFLICTING_VALUE`. Module and workspace `toolchain` directives are independent suggestions and may differ. `replace` and unknown directives are rejected. |
| Go | `go.source` | path ends `.go`; Go 1.27 parser accepts the whole file. Selection uses `go/build.Context` with `GOOS=target.os`, `GOARCH=target.architecture`, `Compiler="gc"`, `CgoEnabled=false`, empty `BuildTags` and `ToolTags`, and `ReleaseTags` exactly `go1.1` through `go1.27`; request features must be empty. Filename selection precedes opening source bytes, as in `go/build.MatchFile`: a hidden or target-excluded filename returns `EXACT_BINDING_UNAVAILABLE` without its body changing the result. The header is the one `go/build` reads: blank lines, `//` comments, and `/* */` comments before the first other text. A `//go:build` line counts anywhere in that header outside a block comment, and a legacy `+build` line counts only when a blank line follows it before the first non-`//` line. At most one canonical `//go:build`; a counted legacy line must use the canonical `// +build ` spelling, since any other spelling `go/build` honours (such as `//+build`) is `MALFORMED_INPUT`, and must be equivalent to it. Filename suffixes use that context and are read, as `go/build` does, from the base name before its first `.`, so `x_linux.pb.go` is a `linux` file and `x.linux_amd64.go` has no suffix; generated state is `ast.IsGenerated`. `//go:embed` and `//go:generate` are dynamic external-input directives and reject as `DYNAMIC_INPUT`; an import URI with userinfo rejects as `CREDENTIAL_INPUT`. |
| Python 3.12 | `py.project` | exactly `pyproject.toml` with `requires-python = "==3.12.0"\n`; this is a declaration fact only. |
| Python 3.12 | `py.toolchain` | exactly `.python-version` with `3.12.0\n`; this is a declaration fact only. |
| Python 3.12 | `py.requirements` | exactly `requirements.txt` with strictly increasing unique `lower-name==CORE_VERSION` LF rows of at most 512 bytes. |
| Python 3.12 | `py.source` | any `PYTHON_PATH`, UTF-8 LF without CR/NUL, parsed wholly by the native Go Python grammar. Comments, indentation, escaped/raw/f/triple/bytes strings, imports/relative imports, decorators, definitions/classes/async, calls, comprehensions, match, and type aliases are accepted. Python 3.13 type-parameter defaults and 3.14 template strings reject; only static import/decorator/definition/bare-call facts are extracted. |
| JavaScript/TypeScript | `js.package` | object fields only: `name:NPM_NAME`, `version:CORE_VERSION`, `workspaces:logical-path[]`, and four `NPM_NAME`-to-`CORE_VERSION` maps (`dependencies`, `devDependencies`, `peerDependencies`, `optionalDependencies`). Every field yields a declaration/workspace edge; unknown metadata, scripts, engines, package-manager hints, ranges, aliases, URLs, Git/path specs, and mutable tags reject. |
| JavaScript/TypeScript | `js.npm-lock-v3` | root fields only: `name:NPM_NAME`, `version:CORE_VERSION`, `lockfileVersion:number(3)`, `requires:boolean`, `packages:object`. Keys are unique `NPM_INSTANCE_PATH`. Package entries allow only `name:NPM_NAME`, `version:CORE_VERSION`, `resolved:NPM_RESOLVED`, `integrity:NPM_INTEGRITY`, five booleans (`link`, `dev`, `optional`, `peer`, `inBundle`), and the four `NPM_NAME`-to-`CORE_VERSION` maps. Exact facts require version, integrity, one stable instance path, and exact manifest correlation. Every workspace the root manifest declares needs its own lock entry, whether or not its manifest is supplied; otherwise the request rejects as `EXACT_BINDING_UNAVAILABLE` (`TestDeclaredWorkspaceWithoutLockEntryRejects`). |
| JavaScript/TypeScript | `js.tsconfig` | not implemented by the initial candidate; reject as `UNSUPPORTED_SCHEMA` until exact option enums, path ownership, references, and no-glob semantics are added here. |
| JavaScript/TypeScript | `js.source` | path suffix is `.js`, `.jsx`, `.mjs`, `.cjs`, `.ts`, `.tsx`, `.mts`, or `.cts`; a bounded lexical-state tokenizer admits static import/export specifiers and bare `require("literal")` only. Strings, comments, templates, regex, property calls, computed calls, and dynamic import cannot yield facts. A `require` call whose argument is not a single string literal, and a method or property named `require`, yields no fact and is lexed as ordinary tokens (`TestLexerNonLiteralRequireYieldsNoFact`). A `require` call yields a fact only while no binding named `require` is in scope. An import clause that names `require` rejects the source as `UNSUPPORTED_SCHEMA`, because import bindings are hoisted over every call in the module. An `export` of a `const`, `let`, `var`, `class`, `function`, or `async` declaration, and a `catch (...)` parameter, follow the same binding rules as the unexported declaration and function parameters (`TestLexerModuleAndCatchBindingsOfRequire`). Because the tokenizer has no scope model, it rejects as `UNSUPPORTED_SCHEMA` every `require` binding it cannot scope: a `var`, `function`, `function*`, `enum`, or `namespace` named `require` (hoisted over earlier calls); a `const`, `let`, or `class` named `require` after a `require` fact was emitted in its block; and a bare `require` identifier, neither called nor a member base, that follows `,`, `{`, `[`, `:`, `...`, or `(`, which covers later declarators and method, constructor, and setter parameters. An object-literal key `require:` directly inside braces stays admitted. Skipped trivia that holds a line break, including a `/* */` comment whose body spans lines (LF or CRLF), opens a statement start as a newline does; a comment on one line does not, and an unterminated comment rejects the source (`TestLexerBlockCommentLineBreakOpensStatement`). A `//` comment ends before the first `\n`, `\r`, U+2028, or U+2029, so a `require` after a lone `\r` is lexed as code, and one after U+2028 or U+2029 rejects the source as `MALFORMED_INPUT` like any other non-ASCII code (`TestLexerLineCommentEndsAtEveryLineTerminator`). |
| JavaScript/TypeScript | `js.bun-lock-v1` | not implemented by the initial candidate; reject as `UNSUPPORTED_SCHEMA` until an exact owner-reviewed Bun tuple matrix is added here. |
| .NET | `dotnet.global-json` | root object has only `sdk`; `sdk` has only `version`, exactly `8.0.423` or `9.0.316`. Values are declarations, not installed-SDK facts. |
| .NET | `dotnet.sln` | not implemented by the initial candidate; reject as `UNSUPPORTED_SCHEMA` until exact header, project, GUID, and global-section grammars are added here. |
| .NET | `dotnet.slnx` | XML namespace is empty; root `Solution` permits only `Project Path="RELATIVE"` children. No other element, attribute, text, namespace, comment, PI, DTD, entity, or XInclude is accepted. |
| .NET | `dotnet.project` | XML namespace is empty or exactly `http://schemas.microsoft.com/developer/msbuild/2003`. `Project` permits only optional `Sdk:DOTNET_NAME`; `PropertyGroup` and `ItemGroup` permit no attributes. Scalar children are `TargetFramework:DOTNET_TFM`, `TargetFrameworks:DOTNET_TFM-list`, `RuntimeIdentifier:DOTNET_RID`, `RuntimeIdentifiers:DOTNET_RID-list`, `LangVersion:DOTNET_LANG`, `IsTestProject:true|false`, and `EnableDefaultItems:true|false`; they permit no attributes/children. `PackageReference` permits required `Include:DOTNET_NAME` and optional `Version:CORE_VERSION`, `PrivateAssets:DOTNET_NAME`, `GeneratePathProperty:true|false`; `ProjectReference` permits only required relative `Include`; `Compile` and `None` permit exactly one of relative `Include`, `Update`, or `Remove`. No element accepts text except named scalar properties. Conditions, imports, globs, expansion, unknowns, and duplicates reject. |
| .NET | `dotnet.central-packages` | same XML namespace rule. `Project`, `PropertyGroup`, and `ItemGroup` have no attributes. `ManagePackageVersionsCentrally` has no attributes/children, text `true` or `false`, and at most one declaration across all `PropertyGroup`s (a second is `DUPLICATE_VALUE`); `PackageVersion` facts are withheld unless it is `true`; `PackageVersion` has no text/children and exactly `Include:DOTNET_NAME` plus `Version:CORE_VERSION`. |
| .NET | `dotnet.packages-lock-v1` | not implemented by the initial candidate; reject as `UNSUPPORTED_SCHEMA` until exact package type, request, resolution, content-hash, and dependency-value grammars are added here. |
| .NET | `dotnet.directory-build` | same closed `dotnet.project` XML subset; filename is exactly `Directory.Build.props` or `Directory.Build.targets`. MSBuild evaluates its items against the importing project's directory, which this candidate never learns, so a well-formed input decodes but mints no fact of any kind, `dotnet.source` included (decision 0174). |
| .NET | `dotnet.nuget-config` | not implemented by the initial candidate; reject as `UNSUPPORTED_SCHEMA` until a credential-safe exact source matrix is added here. |
| Ruby | `ruby.version` | exactly one LF-terminated `X.Y.Z` line with no prefix, range, engine alias, whitespace, or comment. |
| Ruby | `ruby.gemfile` | LF lines from the closed static grammar: `source "https://rubygems.org"`, `ruby "X.Y.Z"`, and `gem "NAME", "X.Y.Z"`; comments and blank lines are allowed but yield no facts. Blocks, method calls, interpolation, variables, alternate URLs, Git/path sources, groups, and ranges reject. |
| Ruby | `ruby.gemfile-lock` | exact LF grammar and indentation: `GEM`, then `  remote: https://rubygems.org/`, `  specs:`, one or more `    RUBY_NAME (CORE_VERSION)` rows each followed by zero or more `      RUBY_NAME (CORE_VERSION)` dependency rows; `PLATFORMS` with `  RUBY_PLATFORM` rows; `DEPENDENCIES` with `  RUBY_NAME (= CORE_VERSION)` rows; `RUBY VERSION` with one `   ruby RUBY_RUNTIME_VERSION`; and `BUNDLED WITH` with one `   CORE_VERSION`. Sections occur exactly once in that order; rows are typed-sorted and unique; unknown, duplicate, conflict, alternate source, and extra text reject. |
| Ruby | `ruby.source` | `.rb` path; bounded lexical states exclude comments/strings/heredocs. A pending heredoc body owns every line through its terminator, so a body line reading `=begin` or `__END__` is text, never a block comment or data section. `%=` opens a `=`-delimited percent literal only where a value may start (the regex position rule); after a value it is modulo assignment. Under the same position rule `?` followed by one non-space, non-identifier character (or `\` and one character) is a character literal, so `?"` opens no string; after a value `?` is the ternary operator. A `<<` glued to a preceding identifier, closing bracket, or closing quote is a shift (`a<<b`), unless the glued word is a keyword (`return<<A`). One trailing `\r` is ignored when matching `=begin`, `=end`, and heredoc terminators, so CRLF sources close these states. Only exact static `require "rails"`, `RSpec.describe`, and `Cucumber` tokens may yield non-promotable source/test observations, never a framework-version fact. |
| Ruby | `ruby.gemspec`, `ruby.bundle-config`, `ruby.rails-marker` | not implemented by the initial candidate; reject as `UNSUPPORTED_SCHEMA` until their exact matrices are added here. |
| SQLite 3.51 source text | `sqlite.migration` | Canonical logical path under `internal/store/migrate/`, UTF-8 LF bytes, and only closed SQLite 3.51 source-grammar productions for `CREATE TABLE` (including virtual), `CREATE INDEX`, `CREATE TRIGGER`, `CREATE VIEW`, `ALTER TABLE`, and `DROP TABLE|INDEX|TRIGGER|VIEW`. Query, write, CTE, pragma, and transaction families reject. The parser is lexical only: no SQL executes and no database, shell, network, repository, or installed SQLite version is consulted. |
| SQLite 3.51 source text | `sqlite.query` | Canonical `.sql` logical path under `internal/store/` or `testdata/antennapod/`; the migration grammar plus closed SQLite 3.51 source-grammar productions for `SELECT`/CTE, `INSERT`, `UPDATE`, `DELETE`, `PRAGMA`, and transactions. Qualified and quoted identifiers, comments, strings, numeric/blob literals, constraints, and foreign keys are accepted only in their exact lexical productions. The virtual-table subset accepts only a module name plus closed column, `UNINDEXED`, or one-token option arguments; punctuation cannot become opaque module payload. Attach/detach, extension loading, dynamic parameters, foreign-dialect forms, malformed controls/comments/strings, and every unknown or trailing form reject the whole request as `UNSUPPORTED_SCHEMA` or `MALFORMED_INPUT`. |
| Swift/Apple | `swift.apple.coordinates`, `swift.source`, `swift.apple-ui.package` | separately specified closed Darwin/arm64 coordinate descriptor, selected Apple source, selected Apple-UI `Package.swift`, and static Swift source subset; all other SwiftPM/Xcode/project syntax is unsupported. |
| Shell | `shell.posix-bash` | UTF-8 LF source at a logical `.sh` or `.bash` path. The exact target coordinates are only `darwin/arm64/none` and `linux/amd64/none`; other OS, architecture, or ABI triples reject. The dialect is `posix-shell/2018` when `target.features` is empty, or `bash/5.2` when its sole feature is `bash-5.2`; `source`, `local`, `declare`, and `typeset` are Bash-only. A bounded token stream retains quote and escape provenance before recognizing static command words, leading assignments, `$NAME`/`${NAME}`, `$0`, literal `.`/`source PATH`, `trap 'literal' SIGNAL`, static ordinary redirections, bounded heredocs, static groups, and `if`/`elif`/`else`/`fi`. A `<<` delimiter requires quote or escape provenance; only `<<-` permits an unquoted delimiter and strips leading tabs for comparison. `unset` permits only unquoted, unescaped identifier operands; assignment-shaped operands reject. Unquoted/unescaped Bash 5.2 reserved words are either parsed only for the listed group/conditional syntax or rejected before static command fact collection; quoted/escaped spellings remain ordinary words. Comments, single quotes, and heredoc bodies are inert. Command substitution, arithmetic expansion, backticks, process substitution, positional parameters other than `$0`, indirect/parameter operators, dynamic source or redirection, `|&`, malformed/adjacent separators, unbalanced quote/heredoc/group/conditional depth, unsupported controls, and every unlisted form reject. |

SQLite comments may contain valid UTF-8 and the permitted SQL whitespace bytes but no NUL or other
disallowed C0 controls. `load_extension` is forbidden as a callable name under every identifier
quoting form while remaining an ordinary quoted identifier in non-callable positions. Trigger-body
DML targets are unqualified; `INSERT ... DEFAULT VALUES` and a directly prefixed `WITH` statement
are excluded. The pinned SQLite 3.51 oracle confirms the target and default-values restrictions and
the positive unqualified-DML complement. It also confirms that direct `WITH ... SELECT` is valid
SQLite; rejecting that form is an intentional narrower profile boundary, not a compatibility claim.

For the JavaScript/TypeScript candidate only, every post-envelope `input_echoes` record is the
closed descriptor `(handle, family, path, sha256)`, in request order. This binds a rejection to its
input family and logical path as well as its handle and content; different canonical descriptors
therefore produce different rejection bytes. The common Go vectors above retain their frozen
two-field echo schema until their own contract revision.

The only permitted fact tuples are below. `Input` and `Related` freeze witness cardinality;
`-` means `related_handle` is exactly `-`. `path` means the already validated logical input path,
`scope` means `scope_id`, and `unit` means `compilation_unit_id`.

| Kind | Input | Related | Subject | Predicate | Value | Instance ID |
|---|---|---|---|---|---|---|
| `go.module` | `go.mod` | `-` | `scope` | literal `declares-module` | `GO_PATH` | `scope` |
| `go.language.declaration` | `go.mod` or `go.work` | `-` | literal `go` | literal `declares-language` | `CORE_VERSION` | `scope` |
| `go.toolchain.declaration` | `go.mod` or `go.work` | `-` | literal `go` | literal `declares-toolchain` | literal `go` plus `CORE_VERSION` | `scope` |
| `go.dependency.locked` | `go.mod` | exact `go.sum` | required `GO_PATH` | literal `locked-at` | `GO_VERSION` | `GO_PATH@GO_VERSION` |
| `go.workspace.use` | `go.work` | `-` | `scope` | literal `uses` | logical path | logical path |
| `go.package` | `go.source` | `-` | Go package identifier | literal `declares-package` | Go package identifier | `unit` |
| `go.source` | `go.source` | `-` | `path` | literal `classifies` | exactly `source`, `test`, or `generated` | `path` |
| `go.import.static` | `go.source` | `-` | `path` | literal `imports` | `GO_PATH` or standard-library import path | `path` |
| `go.build.constraint` | `go.source` | `-` | `path` | literal `selected-for` | canonical Go 1.27 build expression | `unit` |
| `python.language.declaration` | `py.project` | `-` | literal `python` | literal `declares-language` | exactly `3.12.0` | `path:1:1` |
| `python.toolchain.declaration` | `py.toolchain` | `-` | literal `python` | literal `declares-toolchain` | exactly `3.12.0` | `path:1:1` |
| `python.dependency.pinned` | `py.requirements` | `-` | lower package name | literal `pinned-at` | `CORE_VERSION` | `path:line:1` |
| `python.source` | `py.source` | `-` | `path` | literal `parses-as` | exactly `python-3.12.0` | `path:1:1` |
| `python.import.static` | `py.source` | `-` | `path` | literal `imports` | static absolute/relative module | exact `path:line:column` |
| `python.definition` | `py.source` | `-` | `path` | literal `defines` | `function:PYTHON_NAME`, `async-function:PYTHON_NAME`, or `class:PYTHON_NAME` | exact `path:line:column` |
| `python.decorator` | `py.source` | `-` | `path` | literal `decorates` | static decorator plus `->` plus definition value | exact `path:line:column` |
| `python.call.static` | `py.source` | `-` | `path` | literal `calls` | static callee | exact `path:line:column` |
| `js.workspace` | `js.package` | `-` | root `NPM_NAME` | literal `declares-workspace` | logical path | logical path |
| `js.package.locked` | `js.npm-lock-v3` | exact `js.package` | `NPM_NAME` | literal `locked-at` | `CORE_VERSION` | `NPM_INSTANCE_PATH` or literal `root` |
| `js.dependency.locked` | `js.npm-lock-v3` | exact `js.package` | parent instance ID | literal `depends-on` | child instance ID | parent instance ID |
| `js.source` | `js.source` | `-` | `path` | literal `classifies` | exactly `javascript`, `typescript`, or `test` | `path` |
| `js.import.static` | `js.source` | `-` | `path` | literal `imports` | printable ASCII literal specifier | `path` |
| `dotnet.sdk.declaration` | `dotnet.global-json` | `-` | literal `dotnet-sdk` | literal `declares` | exactly `8.0.423` or `9.0.316` | `scope` |
| `dotnet.solution.project` | `dotnet.slnx` | `-` | `scope` | literal `contains-project` | logical path | logical path |
| `dotnet.target.framework` | `dotnet.project` | `-` | `path` | literal `targets` | `DOTNET_TFM` | `unit` |
| `dotnet.runtime.identifier` | `dotnet.project` | `-` | `path` | literal `targets-runtime` | `DOTNET_RID` | `unit` |
| `dotnet.language.version` | `dotnet.project` | `-` | `path` | literal `declares-language` | `DOTNET_LANG` | `unit` |
| `dotnet.package.central` | `dotnet.central-packages` | `-` | `DOTNET_NAME` | literal `central-version` | `CORE_VERSION` | `scope` |
| `dotnet.project.reference` | `dotnet.project` | `-` | `path` | literal `references-project` | logical path | logical path |
| `dotnet.source` | `dotnet.project` | `-` | logical path | literal `classifies` | exactly `source`, `test`, or `generated` | logical path |
| `ruby.version.declaration` | `ruby.version` or `ruby.gemfile` | `-` | literal `ruby` | literal `declares-version` | `CORE_VERSION` | `scope` |
| `ruby.bundler.version.locked` | `ruby.gemfile-lock` | `-` | literal `bundler` | literal `locked-at` | `CORE_VERSION` | `scope` |
| `ruby.gem.locked` | `ruby.gemfile-lock` | exact `ruby.gemfile` | `RUBY_NAME` | literal `locked-at` | `CORE_VERSION` | `RUBY_NAME@CORE_VERSION` |
| `ruby.gem.dependency` | `ruby.gemfile-lock` | exact `ruby.gemfile` | parent gem instance ID | literal `depends-on` | child gem instance ID | parent gem instance ID |
| `ruby.platform.locked` | `ruby.gemfile-lock` | `-` | literal `ruby` | literal `locks-platform` | `RUBY_PLATFORM` | `scope` |
| `ruby.source.identity` | `ruby.gemfile` or `ruby.gemfile-lock` | `-` | literal `rubygems` | literal `source-digest` | `sha256:` plus digest of literal `https://rubygems.org/` | `scope` |
| `ruby.source` | `ruby.source` | `-` | `path` | literal `classifies` | exactly `source` or `test` | `path` |
| `ruby.test.static` | `ruby.source` | `-` | `path` | literal `observes-test-token` | exactly `rspec` or `cucumber` | `path` |
| `sqlite.schema.table`, `sqlite.schema.virtual_table`, `sqlite.schema.index`, `sqlite.schema.trigger`, `sqlite.schema.view`, `sqlite.schema.column`, `sqlite.schema.drop` | `sqlite.migration` or `sqlite.query` | `-` | closed ASCII identifier | literal `declares-*`, `adds-column`, or `drops-*` | closed ASCII identifier | scope |
| `sqlite.query.read`, `sqlite.query.write` | `sqlite.query` | `-` | literal `query` | literal `uses-statement` | exactly `select`, `pragma`, `insert`, `update`, or `delete` | `scope:statement-ordinal` |
| `swift.language.coordinate` | `swift.apple.coordinates` | `-` | literal `swift` | literal `declares-language` | literal `6.3` | literal `beamfall-apple` |
| `swiftpm.tools.coordinate` | `swift.apple.coordinates` | `-` | literal `swiftpm` | literal `declares-tools-version` | literal `6.0` | literal `beamfall-apple` |
| `xcode.project.coordinate` | `swift.apple.coordinates` | `-` | literal `xcode` | literal `declares-object-version` | literal `77` | literal `beamfall-apple` |
| `apple.project.revision` | `swift.apple.coordinates` | `-` | literal `beamfall-apple` | literal `pins-revision` | literal `8588cec3dedaef63bbff458e5e7c8bb6335de107` | literal `beamfall-apple` |
| `swiftpm.package.dependency` | `swift.apple.coordinates` | exact `swift.apple-ui.package` | literal `beamfall-apple` | literal `depends-on` | literal `beamfall-apple-ui@830a154d1d8f2f0b1b801a9a9a3b626bc74f6aa6` | literal `beamfall-apple` |
| `swift.import.static` | `swift.source` | `-` | `path` | literal `imports` | literal `SwiftUI` | `path` |
| `swift.symbol.declaration` | `swift.source` | `-` | `path` | literal `declares-enum` | literal `A11yID` | `unit` |
| `swift.symbol.extension` | `swift.source` | `-` | `path` | literal `extends` | literal `View` | `unit` |
| `shell.import.static` | `shell.posix-bash` | `-` | `path` | literal `imports` | logical path | `path` |
| `shell.command.static` | `shell.posix-bash` | `-` | `path` | literal `invokes` | ASCII command identifier | `path` |
| `shell.env.read` | `shell.posix-bash` | `-` | `path` | literal `reads` | shell identifier | `path` |
| `shell.env.write` | `shell.posix-bash` | `-` | `path` | literal `writes` | shell identifier | `path` |
| `shell.trap.static` | `shell.posix-bash` | `-` | `path` | literal `traps` | `EXIT`, `ERR`, `HUP`, `INT`, `QUIT`, or `TERM` | `path` |
| `shell.script.entry` | `shell.posix-bash` | `-` | `path` | literal `declares-entry` | literal `main` | `path` |

Runtime, installed-toolchain, compatibility, framework-version, admission, exact-verifier, and PASS
facts are absent from the taxonomy and therefore invalid.

### HTML/CSS candidate (proposed, unregistered)

`internal/analyzerhtmlcss` and `cmd/corvint-analyzer-html-css` are an isolated native-Go candidate
only. They are not imported by Core, registered, installed, selected, or reachable from
`corvint`. The literal family is `html-css`; its source coordinates are
`beamfall-core@da38c59eb30b2121cbac37b912485b30b2e54841` and
`beamfall-web@b924fe0ae0d36af08b2a1d50be9383381f809cc0`; fixture-toolchain coordinate is
`go1.27.0`. These are reproducibility coordinates, not support, selection, or qualification
claims.

Its only inputs are canonical `html.document` and `css.stylesheet` records. HTML accepts ASCII,
balanced `html`, `head`, and `body` structure plus closed, double-quoted forms of: `a[href]`,
`link[rel="stylesheet"][href]`, `script[type="module"][src]`, `img[src]`, `source[src]`,
`audio[src]`, and `video[poster]`. CSS accepts ASCII `@import "RELATIVE";` or
`@import url("RELATIVE");`, simple selector blocks, exact `--NAME: ATOM;` custom-property
declarations, and exact `background`, `background-image`, `content`, or `src: url("RELATIVE");`
asset declarations. A relative reference is a non-dot logical path. All other tags, attributes,
selectors, syntax, URL forms, comments, whitespace/control forms, dynamic expressions, escaping,
or ambiguity reject the complete request as `UNSUPPORTED_SCHEMA`; no browser or CSS recovery is
attempted.

The closed facts are `html.link.static`, `html.stylesheet.link`, `html.module.import`,
`html.asset.reference`, `css.import.static`, `css.asset.reference`, and
`css.custom-property.declaration`. They bind their supplied input handle and logical input path;
they never assert that a target exists, was loaded, or is compatible. After a canonical envelope
is available, every rejection includes the exact request `profile`, `family`, `request_id`,
`scope_id`, `compilation_unit_id`, `target`, and input echoes, then `status:"REJECTED"` and one
closed reason. Before that point it uses only the frozen `unknown` family/request sentinel. The
candidate bounds request reads at 1,500,000 bytes, decoded input bytes at 1,048,576, facts at
4,096, and final framed output at 1,048,576; it verifies every digest before parsing.

The CLI reads only stdin through its inherited descriptor, caps the read, compares pre/post
descriptor identity, performs no path lookup or ambient read, and has no network, subprocess,
shell, dynamic-loading, or source-evaluation path. `--version` is the only non-analysis option
and exposes the fixed coordinates above. This remains an experimental implementation candidate:
profile registration, selection, admission, launch, exact verification, and dogfood are all
`NOT_RUN` (decision 0132: this document is `deferred` and no accepted registry/launch profile
exists for this family).

The envelope's closed family set is exactly `go`, `python`, `javascript-typescript`, `dotnet`,
`ruby`, `sqlite`, `html-css`, `shell`, `swift-apple`, and separately specified experimental
families; therefore the frozen envelope text and the candidate family table agree. A request whose
family is in that set but is not `html-css` echoes its envelope and rejects `UNSUPPORTED_SCHEMA`. Only a
family outside the set uses the `UNKNOWN_FAMILY` sentinel (`TestExactFailureGoldensAndEchoEligibility`). For this
candidate, `SIMPLE_SELECTOR` is exactly `:root` or an ASCII 1--128 byte atom made from
`[A-Za-z0-9._#-]`; `CSS_ATOM` is that atom; `CUSTOM_PROPERTY` is `--` plus `CSS_ATOM`; and
`RELATIVE` is the existing non-dot logical-path atom. HTML is exactly
`<html><head>HEAD</head><body>BODY</body></html>` with no whitespace or text nodes: HEAD contains
only closed `link` and module-script forms; BODY contains only closed anchor, module-script, image,
source, audio, and video forms. All names are lowercase ASCII; attributes are double quoted with
no escaping, entity, comment, or raw-text form. CSS is a concatenation of zero or more imports
before one or more rules, with no whitespace, comments, escapes, nested blocks, directives, or
optional semicolons. Imports alone are rejected: there must be one or more rules. A rule has one
`SIMPLE_SELECTOR` and a nonempty sequence of terminal-semicolon declarations. Import values may be
quoted or `url("RELATIVE")`; asset declarations accept only `url("RELATIVE")`. Repeating one
declaration name anywhere in one stylesheet, in the same rule or a later one, with the same value is
`DUPLICATE_VALUE`; repeating it with a different value is `CONFLICTING_VALUE`. The scope is the
stylesheet because no CSS fact tuple carries a selector, so two rules declaring one custom property
would otherwise emit indistinguishable conflicting facts for one path. This is a closed subset, not a browser/CSS parser.

The HTML/CSS tuple matrix is exact: every HTML fact has `input_handle` equal to its source record,
`related_handle:"-"`, `subject:path`, and `instance_id:path`; `html.link.static` uses
`predicate:"links"`, `html.stylesheet.link` uses `predicate:"imports-stylesheet"`,
`html.module.import` uses `predicate:"imports-module"`, and `html.asset.reference` uses
`predicate:"references-asset"`; each value is its `RELATIVE` attribute. CSS import/asset facts use
the same source-handle/path instance shape with predicates `imports-stylesheet` and
`references-asset`; their value is `RELATIVE`. `css.custom-property.declaration` has subject
`CUSTOM_PROPERTY`, predicate `declares`, value `CSS_ATOM`, and instance `path`. No other tuple is
valid.

HTML and CSS are independently versioned closed parsers within this combined candidate: the HTML
input identity is `html.document/closed-v1` and the CSS input identity is
`css.stylesheet/closed-v1`. They are reported separately by `--version`; neither is an installed
browser, stylesheet engine, compatibility, selection, or support claim. For this candidate only,
every `input_echoes` row is the four-field canonical input identity
`{handle,family,path,sha256}` in request order. `content_base64` is intentionally not replayed:
the echoed SHA-256 binds its immutable bytes. The rejection envelope therefore binds the complete
request identity rather than projecting only a handle/digest pair.

The HTML/CSS evidence frame is correspondingly stricter than the original generic candidate
example: it has 19 length-prefixed fields — request family, request ID, scope ID, compilation-unit
ID, canonical target JSON, SHA-256 of the complete LF-framed canonical request, own handle/family/
path/SHA-256, related handle/family/path/SHA-256 (or four `-` atoms), then kind, subject, predicate,
value, and instance ID. Thus changing an input family, path, bytes, target, scope, unit, or request
identity changes every derived evidence digest. This is a candidate-local experimental wire version,
not a change to the unselected generic candidates.

Parser selection is deliberately negative: Go's `html/template` is an output escaper, not a source
fact parser; `encoding/xml` rejects normal HTML void-element rules; and recovery-oriented HTML/CSS
parsers accept or normalize precisely the ambiguity this profile must reject. The candidate's
native closed scanner is therefore the viable implementation for this bounded grammar. Its
four-input/32-fact benchmark is an allocation ratchet, not a latency claim: local timings are
diagnostic and load-sensitive. The candidate remains below its committed 105,000 B/op and 635
alloc/op fixture ceilings; no comparison to a different fixture is a promotion result.

The typed fact ordering ratchet is causal rather than a latency claim. At exact parent
`404f2773b650749a0bb1c1c7bd4a9e241fece69e`, descending 4,096-fact input used typed insertion
ordering and therefore takes 8,386,560 comparisons. The production `sortFacts` function returns
its exact comparator-work count while it performs typed O(n log n) ordering; `Analyze` ignores that
metric, while `TestTypedFactOrderingWorstCaseRatchet` calls that production function and requires no
more than 196,608 comparisons. Its restored-parent negative control verifies the precise ancestor
contains and fails with the original insertion loop. `BenchmarkSortFactsWorstCase4096` measures the
same worst case only diagnostically; it publishes no wall-clock, throughput, or latency claim.

### Proposed Shell profile (experimental only)

`shell` is a separately buildable candidate only. Its coordinates are closed:
`posix-shell/2018` is represented by `target.features:[]`; `bash/5.2` is represented by exactly
`target.features:["bash-5.2"]`. They are distinct profiles and no other shell, feature, target,
or version is accepted. Each profile permits only `darwin/arm64/none` or `linux/amd64/none`.
Inputs are an ordered, unique set of `shell.posix-bash` records; each is a
strict logical `.sh`/`.bash` path and one content digest. The shell-specific fact cap is 3,000;
this lower family cap keeps a complete fact response inside the global output envelope. Duplicate handles, paths, tuples, null
arrays, noncanonical JSON, unknown fields, malformed identities, controls, limits, and input digest
fail before a full echo is permitted. Thereafter all rejections echo the complete target and every
input `{handle,sha256}` in request order, preventing replay against a changed unit, target, scope,
or input set.

The closed errors are the frozen envelope errors plus `DYNAMIC_INPUT` (substitution, dynamic
source, indirect parameter, unsupported expansion, or process substitution), `UNSUPPORTED_SCHEMA`
(unlisted shell form/family/dialect), and `OUTPUT_LIMIT`. Static fact coordinates are exactly the
Shell rows above: subject and instance are the input logical path, related handle is `-`, predicates
are the listed literals, and values are only the listed identifiers, logical paths, signals, or
`main`. Repeated fact tuples are terminal `DUPLICATE_VALUE` rejections before canonical typed
ordering; conflicting coordinates reject. This candidate neither sources nor executes its input.

## Qualification boundaries

Every `NOT_RUN` boundary below stays that way for the same reason: decision 0132 froze this whole
document as `deferred`, and no accepted registry, lock, admission, launch, or exact-verifier surface
exists in Core for any family to be qualified against (see Verified current state). None is a test
that could be executed here; each awaits a separate owner-accepted qualification-and-launch profile.

- Go 1.27.0 on Darwin/arm64 is a fixture target only. Host binary, target, clause, registry, lock,
  containment, admission, and verifier qualification remain `NOT_RUN`.
- Python 3.12 is source/declaration grammar only. No Python executable, package environment,
  interpreter ABI, framework, host/target, compatibility, admission, or verifier claim is made.
- JavaScript, TypeScript, and TSX are separately classified source grammars; React is only a module
  specifier unless a separate exact locked profile says otherwise. Node 22.23.2, npm 10.9.8, and Bun
  1.3.11 are distinct observed fixture values only. TypeScript, React, Jest, Vitest, node:test,
  bun:test, and every framework/runtime/package-manager tuple require separate exact locked evidence
  and accepted qualification; none is implied by this candidate.
- .NET SDK 8.0.423 and 9.0.316 are declaration fixtures only. MSBuild, NuGet, target framework, RID,
  host/target, binary, clause, admission, and verifier qualification remain `NOT_RUN`.
- Ruby 2.6.10 and Bundler 1.17.2 are observed fixture values only. Rails and every Ruby/gem/platform/
  host/target/binary/clause/verifier tuple remain `NOT_RUN`.
- SQLite source text has no installed-runtime claim. The only closed candidate profile is
  `corvint-analyzer-candidate/sqlite-3.51.0-source-v1`: dialect label `sqlite-3.51.0`, Go toolchain
  label `go1.27.0`, no feature states (`features` is exactly `[]`, never `null`), and source
  coordinates restricted to `internal/store/migrate/**/*.sql`, `internal/store/**/*.sql`, and
  `testdata/antennapod/**/*.sql`. Its evidence tag is
  `corvint-analyzer-candidate-evidence/sqlite-3.51.0-source-v1`. Its frozen literal witnesses are Beamfall Core
  `da38c59eb30b2121cbac37b912485b30b2e54841` and Beamfall Podcasts
  `fbfba6b56ed2d5e308e083bb0e06374cb2c33ce2`; Core selection, launch, qualification, and support
  remain `NOT_RUN`.
- Swift 6.3, SwiftPM tools 6.0, Xcode object version 77, and the two pinned Apple revisions are
  closed source coordinates only. Swift compiler/runtime, Objective-C, C, Metal, host/target,
  binary, clause, admission, and verifier qualification remain `NOT_RUN`.

## Reproducible size and allocation gates

All measurements use the regular-file Go executable
`/opt/homebrew/Cellar/go/1.27.0/libexec/bin/go`, SHA-256
`71c4991041d8e44975c882e4f72005719c958013d3340dc665a3808b72ddf702`, with the complete GOROOT
tree receipt below, `GOMAXPROCS=1`, `CGO_ENABLED=0`, network/module lookup disabled, and fresh private
cache/home paths. The benchmark fixture is constructed before timing and has exactly one scope, one
compilation unit, four inputs, 32 facts, and no rejection. Each package exposes
`BenchmarkAnalyzeCandidate`.

`cmd/corvint-go-toolchain-receipt` is the reviewed streaming tree verifier. Its canonical preimage is
the raw ASCII tag `corvint-go-toolchain-tree/v1`, the GOROOT directory's big-endian uint32 stable
mode, a big-endian uint32 entry count, then every GOROOT descendant sorted by its slash-separated
UTF-8 relative-path bytes. Each entry contributes a
uint32-length-prefixed path, a big-endian uint32 stable mode, and one type byte. Stable mode is the
exact Go `os.FileMode` bit representation masked to `ModePerm|ModeSetuid|ModeSetgid|ModeSticky`; the
entry type is framed separately. A directory uses `D`; a symlink uses `L` plus its
uint32-length-prefixed uninterpreted link target; a regular file uses `F` plus its big-endian uint64
size and exact streamed contents. The root cannot be a symlink and its identity, complete mode,
size, and modification time are rechecked after collection and after hashing. Unsupported entry
types, invalid paths, count/length overflow, read errors, or entry/root identity, mode, size, or
modification-time drift reject. Directory entries make addition, removal, type changes, empty
directories, permissions/special modes, link targets, executables, compiler/linker tools,
standard-library sources, headers, VERSION, and every other live GOROOT input part of one receipt.
Mutation tests cover content, ordinary and special mode, symlink target, entry addition, initial
symlink root, and accepted-root identity substitution.

The exact verifier source at this document's commit MUST first build to stripped Darwin/arm64
SHA-256 `f8337b642a5ffaf4ee44e29fec2b8053450a46c549e7d61187a7897487eb3da6`. That binary MUST report
exactly 17,277 entries and tree SHA-256
`94b2c0ed86c9f62518348cc7fc3e4442e4e40a0194d10cfc8f6bc1be98447c33` immediately before and after
every benchmark or candidate/Core build. A mismatch aborts without recording a measurement. This
is a reproducibility guard for an owner-controlled local toolchain, not an adversarial trust root;
an accepted release profile still requires its independently trusted binary/toolchain provenance.

```console
$ measure_root=$(mktemp -d /private/tmp/corvint-profile.XXXXXX)
$ trap 'rm -rf "$measure_root"' EXIT INT TERM
$ test "$(shasum -a 256 /opt/homebrew/Cellar/go/1.27.0/libexec/bin/go | awk '{print $1}')" = \
    71c4991041d8e44975c882e4f72005719c958013d3340dc665a3808b72ddf702
$ env -i HOME="$measure_root/receipt-home" PATH=/usr/bin:/bin \
    GOROOT=/opt/homebrew/Cellar/go/1.27.0/libexec GOPATH="$measure_root/receipt-gopath" \
    GOMODCACHE="$measure_root/receipt-modcache" GOENV=off GOWORK=off GOPROXY=off GOSUMDB=off \
    GOTOOLCHAIN=local GOMAXPROCS=1 CGO_ENABLED=0 GOCACHE="$measure_root/receipt-cache" \
    /opt/homebrew/Cellar/go/1.27.0/libexec/bin/go build -trimpath -buildvcs=false \
    -ldflags='-s -w -buildid=' -o "$measure_root/toolchain-receipt" \
    ./cmd/corvint-go-toolchain-receipt
$ test "$(shasum -a 256 "$measure_root/toolchain-receipt" | awk '{print $1}')" = \
    f8337b642a5ffaf4ee44e29fec2b8053450a46c549e7d61187a7897487eb3da6
$ check_toolchain() { \
    test "$("$measure_root/toolchain-receipt" /opt/homebrew/Cellar/go/1.27.0/libexec)" = \
      '{"Entries":17277,"SHA256":"94b2c0ed86c9f62518348cc7fc3e4442e4e40a0194d10cfc8f6bc1be98447c33"}'; \
  }
$ check_toolchain
$ env -i HOME="$measure_root/home" PATH=/usr/bin:/bin \
    GOROOT=/opt/homebrew/Cellar/go/1.27.0/libexec GOPATH="$measure_root/gopath" \
    GOMODCACHE="$measure_root/modcache" GOENV=off GOWORK=off GOPROXY=off GOSUMDB=off \
    GOTOOLCHAIN=local GOMAXPROCS=1 CGO_ENABLED=0 GOCACHE="$measure_root/gocache" \
    /opt/homebrew/Cellar/go/1.27.0/libexec/bin/go test -run '^$' \
    -bench '^BenchmarkAnalyzeCandidate$' -benchmem -count=10 ./internal/PACKAGE
$ check_toolchain
$ check_toolchain
$ env -i HOME="$measure_root/build-home" PATH=/usr/bin:/bin \
    GOROOT=/opt/homebrew/Cellar/go/1.27.0/libexec GOPATH="$measure_root/build-gopath" \
    GOMODCACHE="$measure_root/build-modcache" GOENV=off GOWORK=off GOPROXY=off GOSUMDB=off \
    GOTOOLCHAIN=local GOMAXPROCS=1 CGO_ENABLED=0 GOCACHE="$measure_root/build-gocache" \
    GOOS=darwin GOARCH=arm64 /opt/homebrew/Cellar/go/1.27.0/libexec/bin/go build \
    -trimpath -buildvcs=false -ldflags='-s -w -buildid=' \
    -o "$measure_root/candidate" ./cmd/COMMAND
$ check_toolchain
```

| Family | Package | Command | Maximum B/op | Maximum allocs/op |
|---|---|---|---:|---:|
| Go | `analyzergo` | `corvint-analyzer-go` | 12,000 | 200 |
| Python 3.12 | `analyzerpython` | `corvint-analyzer-python` | 150,000 | 900 |
| JavaScript/TypeScript | `analyzerjs` | `corvint-analyzer-js` | 24,000 | 300 |
| .NET | `analyzerdotnet` | `corvint-analyzer-dotnet` | 130,000 | 900 |
| Ruby | `analyzerruby` | `corvint-analyzer-ruby` | 35,000 | 1,000 |
| SQLite source text | `experimental/analyzers/sqlnative` | `corvint-analyzer-sql-native` | 98,304 | 400 |
| HTML/CSS | `analyzerhtmlcss` | `corvint-analyzer-html-css` | 105,000 | 635 |
| Swift/Apple | `analyzerswift` | `corvint-analyzer-swift` | 35,000 | 250 |
| Shell | `analyzershell` | `corvint-analyzer-shell` | 130,000 | 900 |

No allocation row exists for Kotlin/Android, Shader GLSL/Metal, structured-data, or native/JVM
bridge. Those candidates carry ACP-006/ACP-009 obligations but may claim no allocation ceiling or
qualification until this umbrella assigns an explicit row; this table must not be extended by
inventing measurements.

The maximum of all ten allocation samples must satisfy the table. Source bytes use the ACP-009
per-executable recipe: the byte total of every non-test production `.go` file in every first-party
package of the candidate CLI's `go list -deps` closure at the candidate commit, as enforced by
`TestCandidateSourceCeilingPerExecutable`; for a CLI with no transitive first-party dependency this
equals the named package plus its command. Binary
size is the exact output of the build command above. To prove zero Core binary growth, build
`./cmd/corvint` with that command in separate clean worktrees at the exact parent and candidate
commits, using separate empty caches but identical environment/output path length; SHA-256 and byte
count must match. One qualification run creates one `measure_root`; the benchmark and every
candidate, parent, and Core build use distinct empty cache/home/GOPATH/module-cache subpaths within
it. Each parent/candidate/Core build repeats the exact three-command bracket `check_toolchain`,
isolated build, `check_toolchain` in its own clean worktree; no stateful step occurs inside a
bracket. The two adjacent checks in the example make the benchmark's immediate post-check and the
candidate build's immediate pre-check explicit. VCS stamping is disabled. A causal ratchet may
replace an absolute ceiling only after recording its exact parent commit, fixture digest, command,
and restored-parent failure in this document. When the exact integration parent predates an
isolated candidate package, the only permitted alternative is an explicitly labelled constrained
absolute-ceiling control; it must not name a nonancestor baseline or claim an A/B improvement.

The Swift/Apple exception uses `BenchmarkAnalyzeCandidate` with the successful canonical response
for its three-input, eight-fact pinned Beamfall Apple request constructed before timing. Its ten or
more samples must all fit the Swift row's B/op and allocs/op ceilings. No Swift sample, causal
ratchet, restored implementation failure, or ACC-V0-020 qualification is recorded at this freeze;
its performance state remains `NOT_RUN` and the ceilings do not imply readiness.

## Deterministic acceptance matrix

| Gate | Required evidence before integration |
|---|---|
| Canonical bytes | independent exact success/error goldens; 1,024 distinct checked-in requests run through fresh pinned production processes, with frozen request/response digests; input-order permutations emit one digest |
| Closed parsing | exhaustive positive and hostile tables for every named family; every unknown/duplicate/ambiguous/future form fails |
| Bounds | just-under/at/over cases for every global count/byte bound; amplification fails before retained growth or output write |
| Non-execution | static import audit plus positive-controlled arbitrary file/environment/process/network observers and sandbox denial; the independently golden production process records zero attempts; for HTML/CSS, a Darwin denial spy observes zero candidate launches/channels and a Go-runtime positive control exercises filesystem, network, subprocess, and descriptor paths that must fail the same candidate-clean assertion |
| Portability | focused tests, race, vet, and cgo-disabled Darwin/arm64, Linux/amd64, and Windows/amd64 builds |
| Modularity | Core dependency graph and stripped binary are byte-identical before/after an unreferenced candidate integration |
| Performance | all ten samples satisfy the absolute ceiling; any named causal ratchet fails at its exact parent, while a parentless isolated package uses only the labelled constrained control; no latency or ACC-V0-020 claim |
| Review | fresh Sol/high exact review of the final candidate commit returns PASS with `NOT_RUN` boundaries intact |

## Rollout, rollback, and drift

Passing candidates may be integrated as unregistered experimental source commands only. They are
excluded from default builds, release archives, installers, the private `corvint-next` candidate,
and user-facing support tables. A candidate is rolled back by reverting its isolated integration
commit; no repository lock, registry, cache, or installed state may depend on it.

Any new language, runtime, SDK, lock format, framework, compiler option, host, target, ABI, binary,
or parser schema changes the candidate matrix and requires a spec/test/review update. It does not
inherit qualification from a similar version.

## Traceability and promotion criteria

| Requirements | Planned implementation | Evidence state |
|---|---|---|
| `ACP-001,007,008,010` | separate `cmd/corvint-analyzer-*` and `internal/analyzer*` packages | `TestRunExactLFAndRejectsFileArguments`, `TestBuiltCLILiteralGoldensAndFreshProcessPermutations`, and `TestBuiltCLIAmbientEffectSpiesWithPositiveControls`; selection/launch `NOT_RUN` |
| `ACP-002..006` | family-specific strict request, parser, fact, and canonical-output packages | shell: `TestExactCanonicalFacts`, `TestFreshCanonicalRuns`, `TestPreEnvelopeDuplicateNeverEchoes`, `TestShellDialectCoordinatesAndSeparators`, `TestTokenStreamPreservesQuoteEscapeAndExpansionProvenance`, `TestTokenStreamFailsClosedForGrammarAndSupportsStaticRedirections`, `TestControlWordsRequireUnquotedUnescapedProvenanceAndBranchesAreNonempty`, `TestDoubleQuoteBackslashAndExpansionFormsFailClosed`, `TestEnvelopeAndEvidenceBindingsAreMetamorphic`, `TestClosedRejectionsCoverEveryReachableReason`, `TestPublicBoundsHaveJustUnderAtAndOverWitnesses`, and `TestPinnedShellCorpusIsLiteralAndBound` |
| `ACP-009` | per-family allocation/output/source/binary ratchets | shell four-input/32-fact absolute-ceiling evidence (`TestHardAllocationOutputAndSourceRatchets`); causal ratchet `NOT_RUN` |
| `ACP-012` | stdin read and size classification in every stdin-reading `cmd/corvint-analyzer-*` `run` (kotlin-android in `internal/analyzerkotlinandroid` `readBounded`/`readStableDescriptor`; native bridge in `internal/analyzernativebridge` `readWire`) | go `TestRunAlwaysFramesReadAndSizeFailures`; js `TestRunClassifiesStdinFailureAsNoncanonical`, `TestRunRawRequestBoundary`; dotnet `TestRunRejectsOverlongInputWithoutDecoding`, `TestRunReportsAReadFailureAsTheFixedSentinel`; html-css `TestAnalyzeStdinPreEnvelopeFailuresUseClosedSentinel`, `TestBuiltCLILFOverflowAndStableDescriptor`; python `TestRunFramesReadAndSizeFailures`; ruby `TestRunPinsInvocationFramingAndRefusals`; shell `TestRunWritesCompleteReadFailureResponse`, `TestRunReportsOversizeAsLimitExceeded`; rust `TestOversizeStdinIsBounded`, `TestReadFailureIsNoncanonicalSentinel`; shader `TestRunReadFailureWritesNoncanonicalSentinel`; sql-native `TestRunReportsReadFailureAsNoncanonical`; kotlin-android `TestRunReportsReadFailureAsNoncanonical`, `TestProspectiveLimitsAndReaderError`, `TestStableDescriptorRejectsSameInodeMutation`; structured-data `TestBuiltCLIReportsReadFailureAsNoncanonical`, `TestRunReturnsNonzeroWhenFrameIsNotWritten` (write failure exits nonzero); native bridge `TestPreflightLimitAndMalformedSentinels`, `TestCLIInvocationFramingAndRefusal` |
| `ACP-011` | `run(args []string, ...)` argv guard in `cmd/corvint-analyzer-go`, `cmd/corvint-analyzer-python`, `cmd/corvint-analyzer-js`, `cmd/corvint-analyzer-dotnet`, `cmd/corvint-analyzer-kotlin-android`, and `cmd/corvint-analyzer-sql-native`; `cmd/corvint-analyzer-shell` and `cmd/corvint-analyzer-ruby` (exit 0 per decision 0233); the sole `--version` receipt in `cmd/corvint-analyzer-html-css`, `cmd/corvint-analyzer-rust`, and `cmd/corvint-analyzer-shader` (decision 0236) | `TestRunRejectsUnexpectedArgv` in each of the six packages' `main_test.go` (go also pins `--version` as unexpected argv); shell `TestRunExactLFAndRejectsFileArguments`; ruby `TestRunPinsInvocationFramingAndRefusals`, `TestBuiltCLIEmitsExactLFAndRejectsFileArguments`; version receipt: rust `TestVersionReceiptNamesUnsupportedCEMOCM` and `TestUnexpectedArgumentsReject` (`--version extra`), shader `TestVersionDeclaresCEMAndOCMUnsupported` and `TestBuiltCLIVersionKeepsCEMAndOCMUnsupported`, html-css `TestBuiltCLILFOverflowAndStableDescriptor`; write failure exits nonzero: go, js, html-css, and sql-native `TestRunReturnsNonzeroWhenFrameIsNotWritten` |
| Go matrix | `internal/analyzergo`, `cmd/corvint-analyzer-go` | candidate review in progress. The 2026-09-12 fact-correctness hunt repaired five points. The D7 `1.26.Z`/`1.27.Z` patch range and minor-then-patch workspace ordering are covered by `TestClosedGoDeclarationTuplesRejectUnsupportedFutures` and `TestClosedGoRootAndChecksumConflictBoundaries`. Semantic `go.sum` line order is covered by `TestClosedLockAndEnvelopeBounds`. Header parity with `go/build`, first-dot filename-suffix parity, excluded-filename read ordering, and noncanonical legacy rejection are covered by `TestGo127BuildSelectionMatchesMatchFileOracle`, `TestFilenameExcludedSourceIsNotRead`, and `TestGoWorkSourceAndBuildMatrix` |
| Python 3.12 matrix | `internal/analyzerpython`, `cmd/corvint-analyzer-python` | exact review PASS (round 6) and integrated as experimental source; every activation and promotion boundary `NOT_RUN` |
| JavaScript/TypeScript matrix | `internal/analyzerjs`, `cmd/corvint-analyzer-js` | exact review PASS (round 7) and integrated as experimental source; every activation and promotion boundary `NOT_RUN` |
| .NET matrix | `internal/analyzerdotnet`, `cmd/corvint-analyzer-dotnet` | isolated candidate; `TestFrozenSuccessVector`, `TestEvidenceDigestBindsEveryFrameField`, `TestXMLSurfaceIsClosed`, `TestProjectSchemaRejections`, `TestCentralPackagesRejections`, `TestSourceClassificationRequiresDefaultItemsOff`, `TestFamilyLaunderingIsRefused`, and `TestAllocationAndOutputRatchets`; `dotnet.sln`, `dotnet.packages-lock-v1`, and `dotnet.nuget-config` reject wholesale as the matrix requires. Not imported by Core, registered, installed, selected, or reachable from `corvint`; selection/launch/promotion `NOT_RUN` |
| Ruby matrix | `internal/analyzerruby`, `cmd/corvint-analyzer-ruby` | implemented as an unregistered candidate; closed grammars, fact matrix, source ceiling (49,651/65,536, unamended base), and allocation ratchet (33,825 B/op, 246 allocs/op) evidenced in `docs/BUILD-LOG.md` 2026-08-29; heredoc-body precedence over `=begin` and `%=` modulo assignment pinned by `TestLexerNestingAndMultipleHeredocs` and `TestRegexVersusDivisionDisambiguation`, which on 2026-09-13 move the source ceiling to 49,772/65,536 and the allocation sample to 33,831 B/op, 246 allocs/op; the `?` character literal, glued `<<` shift, and CRLF directive and terminator matching, pinned by the same two tests on 2026-09-13, move the source ceiling to 51,036/65,536 (unamended base) and the allocation sample to 33,830 B/op, 246 allocs/op; exact review, registry/lock/launch, and dogfood `NOT_RUN` |
| SQLite source-text matrix | `experimental/analyzers/sqlnative`, `cmd/corvint-analyzer-sql-native` | unregistered candidate; focused bytes, replay, no-follow, parser, and ratchet checks only |
| HTML/CSS matrix | `internal/analyzerhtmlcss`, `cmd/corvint-analyzer-html-css` | isolated candidate exact-review repair; canonical extension, zero-allocation ordering, Go-runtime Darwin isolation, and portability evidence required; selection/launch `NOT_RUN` |
| Swift/Apple matrix | `internal/analyzerswift`, `cmd/corvint-analyzer-swift` | unregistered candidate; registry/lock/launch/dogfood `NOT_RUN` |
| Shell matrix | `internal/analyzershell`, `cmd/corvint-analyzer-shell` | candidate repair in progress; Core remains unreachable |

Promotion to supported requires a separate owner acceptance for one exact profile, then complete
ACC-V0 registry, lock, containment, request/echo, admission, exact-verifier, rollback, and
five-warmup/100-fresh-process evidence. Until then every family is unavailable to Core.

## Unresolved decisions and kill criteria

- Owner acceptance of the first exact family/profile tuple and its exact verifier.
- Signed local registry and repository-lock formats, trust roots, revocation, and retention.
- Host containment and enforceable CPU/memory/descendant controls.
- Package/install layout and release-archive exclusion for experimental candidates.

Immediately quarantine a candidate after nondeterministic bytes, fabricated exact facts, accepted
unknown schema, escaped bounds, repository or ambient reads, any subprocess/network action, Core
dependency growth, partial output, analyzer-authored authority, or a false support claim.
