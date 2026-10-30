# Shader GLSL/Metal Analyzer Candidate V0

Owner: Russell Lewis
Date: 2026-08-25
Intent status: proposed
Delivery status: deferred
Disposition: leaf assets provide no routing value; do not advance this candidate without a new
authority-backed use case.
Authoritative inputs: `AGENTS.md`, `docs/specs/analyzer-capability-contract-v0.md`, and `docs/specs/analyzer-candidate-profiles.md`.

## Agent digest
- Claim: An isolated unregistered extractor emits bounded GLSL and Metal structural facts but has no authority-backed routing value.
- Status: proposed/deferred
- Exists: isolated `internal/analyzershader` structural extractor and pinned dogfood corpus tests.
- Blocked on: an authority-backed routing use case, accepted ACC profile, and promotion evidence.
- Read next: Scope and boundary; Pinned dogfood corpus and evidence; Acceptance and rollback.

## Requirements

This is an unregistered native-Go structural fact extractor. It receives only one bounded
canonical JSON/LF frame of caller-owned bytes/digests. It never opens repositories or ambient
files, reads PATH, invokes a compiler/process, uses a network or dynamic loader, changes CWD/HOME,
or emits admission, compatibility, authority, compile, or semantic `PASS` facts. Core has no
dependency edge to it and it is unselectable pending a separately accepted ACC-V0 profile.

- `SHA-V0-001`: accept only exact finite language/version/toolchain/stage tuples, never a nearest
version: GLSL ES `3.00` with frozen `glslang-16.4.0`/`SPIRV-Cross-2026-07-06T12:43:32` structural
tuple for `beamfall-visual-shaders@a2c968b0d8cf4eb3e5998fa32885b626266236ee`; GLES `3.00` with
`beamfall-android-gles-contract-1.0.0` for
`beamfall-android-ui@6e379d7feec88128439d1753325bcfb22194fdfc`; and Metal `3.1` with the explicit
`metal-3.1-structural-lexer-v0` tuple for vertex, fragment, and compute separately at
`beamfall-apple-ui@830a154d1d8f2f0b1b801a9a9a3b626bc74f6aa6`. The Metal tuple names this
extractor's structural grammar only; it does not claim an Apple compiler invocation.

- `SHA-V0-002`: use the experimental `shader` family extension: a request has the fixed transport
profile, a `shader_profile` exact ID, explicit `shader_language`, `shader_version`, and
`shader_toolchain` fields, and only one matching `shader.glsl`, `shader.gles`, or `shader.metal`
source family. An unknown or mismatched tuple is `EXACT_BINDING_UNAVAILABLE`; no tuple is inferred
from source text or host tooling.

- `SHA-V0-003`: facts carry an inclusive/exclusive byte span (zero-based bytes, one-based byte columns),
literal bounded base64 witness bytes, and witness SHA-256. Typed facts remain ordered and bind the
whole request through a domain-separated evidence digest. Every post-envelope success and rejection
echoes its exact tuple, target, ordered input handle/digest set, and a SHA-256 of the canonical
request bytes. Witnesses are at most 4,096 bytes; no source-derived rejection text is retained.

- `SHA-V0-004`: lexer/preprocessor handling is fail-closed. Unterminated comments/strings, unsupported
conditionals/pragma/extensions, nonliteral includes, malformed macros, unknown directives,
line continuations, macro replacements outside the closed empty/single-identifier/decimal-integer
grammar, unbalanced delimiters, unsupported bytes, token or byte-limit exhaustion, and a second
well-formed GLSL `void main(){` entry point in one compilation unit reject the full request.
The extractor may emit GLSL version/precision/layout/uniform/in/out/entry and Metal
include/namespace/resource/attribute/stage-entry structure; it never asserts compilation. A
`layout(...)`-qualified `in`/`out`/`uniform` declaration emits both the layout fact and the same
interface fact a plain declaration of that qualifier would -- the layout qualifier decorates the
binding, it does not withhold it.

- `SHA-V0-005`: decode, base64, total input, token, witness, fact, and prospective output limits are
checked before retaining data. Base64 uses strict decoding plus exact re-encoding. Every echoed
target/digest field and the longest prospective rejection frame are bounded before rejection may
retain request bindings; otherwise the minimal fixed sentinel is emitted. The sentinel reason set is
the analyzer-candidate-profiles list (decision 0213): a frame that does not decode into the closed
schema is `NONCANONICAL_REQUEST`, except that an otherwise safe canonical extension is the bound
`UNKNOWN_FIELD` once the echo checks pass (decision 0222, `TestSafeCanonicalExtensionBindsUnknownField`),
and a duplicate input handle is the `DUPLICATE_VALUE` sentinel
(`TestDuplicateHandleRejectionStaysUnbound`). An unbounded echoed field and an oversize rejection
frame use listed reasons (decision 0221): nil features `NONCANONICAL_REQUEST`, over 64 features or an
oversize frame `LIMIT_EXCEEDED`, a bad target or feature identifier `INVALID_IDENTIFIER`, unsorted
features `DUPLICATE_VALUE`, and a bad digest `DIGEST_MISMATCH`
(`TestUnboundedEchoedFieldsUseListedSentinelReasons`). `reject` independently
enforces the output ceiling. Output is one complete LF frame. The reason taxonomy is closed; an
impossible implementation reason reduces to
`ANALYZER_FAILURE`, never a new string.

- `SHA-V0-006`: this candidate provides neither CEM nor OCM production. Its version receipt names both
states `UNSUPPORTED`; no local CEM/OCM closure artifact or promotion claim is emitted for this
unregistered extractor.

## Pinned dogfood corpus and evidence

Tests vendor and execute the complete 62-file GLSL and 49-file Metal byte corpora from the immutable
revisions above. Each embedded byte stream is recomputed as its literal Git blob identity before
analysis, then compared with a frozen expected candidate/rejection/fact-count result. The raw
`visuals/pulse.glsl` control remains correctly rejected without its wrapper, and Android's generated
`VERTEX_300` slice remains a bounded structural control. Runtime receives no repository path or
fixture. The built CLI has 1,024 distinct canonical request vectors with a frozen aggregate output
SHA-256, full-write/error/permutation checks, and poisoned PATH/HOME/CWD/environment/network probes;
the source guard has live process/shell/network/loader/descriptor/filesystem positive controls.
Exact OS syscall tracing and semantic compilation remain `NOT_RUN`; CEM/OCM are `UNSUPPORTED`.

## Acceptance and rollback

Focused package tests provide exact tuple/rejection boundaries, full-request replay binding,
under/at/over prospective candidate/rejection output, echoed target/digest, witness, token, wire,
and input-count limits; strict-base64 and continuation-forgery vectors; CLI read/full-write error
paths; plus an actual pinned `Pulse.metal` allocation ratchet whose constrained source alternative
fails. The exact Core integration parent `718dfc7db3e9162f045669309e2f28dc73487aff` has fixed
`cmd/corvint` and `internal/analyzercap` tree identities and no analyzer dependency edge.

Rollback is the exact seven-commit candidate range
`393ef14427a40e42bf854142bc20fd811441f296^..25650f846f2ec78bedd452962ed87a38b3ce69f8`, not
a single candidate commit. After first reverting any descendant repair commit, revert this sequence
newest-to-oldest: `25650f846f2ec78bedd452962ed87a38b3ce69f8`,
`1a44aac2c34d76d7fb9ce11030bf3e9bb1eee06f`,
`e791e69def23b136f01bde439b7fe51a82b8e37c`,
`141ce7e6e9ff25afcd1744db84072254327738f0`,
`8466b61b895e67446a5144615385e2592d60c168`,
`586f9c2b95d7dfeefd84ac89e550e43c95a20977`, then
`393ef14427a40e42bf854142bc20fd811441f296`. No registry/lock/install state exists.

| Requirement | Implementation | Evidence |
|---|---|---|
| `SHA-V0-001..002` | `internal/analyzershader/analyzer.go` | exact tuple/replay/corpus tests |
| `SHA-V0-003..005` | analyzer, lexer, GLSL, Metal, command | frozen corpus, boundary, CLI, and isolation tests |
| `SHA-V0-006` | command version receipt | explicit `UNSUPPORTED` CEM/OCM test |
| promotion | ACC-V0 containment/admission/exact verifier | `NOT_RUN` |
