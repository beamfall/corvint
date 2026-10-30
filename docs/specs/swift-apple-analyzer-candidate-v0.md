# Swift/Apple analyzer candidate V0

Owner: Russell Lewis
Date: 2026-08-25
Intent status: proposed
Delivery status: deferred
Disposition: return only with a general extractor rather than a byte-pinned fixed-input fixture.
Authoritative inputs: `AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`analyzer-capability-contract-v0.md`, and `analyzer-candidate-profiles.md`.

## Agent digest
- Claim: An isolated unregistered Swift extractor exists, but its byte-pinned fixed-input design is deferred pending a general extractor.
- Status: proposed/deferred
- Exists: isolated `internal/analyzerswift` and `cmd/corvint-analyzer-swift` candidate implementation.
- Blocked on: a general extractor, Core admission, exact verification, performance, and dogfood evidence.
- Read next: Job and current state; Limits, checks, and rollback; Open promotion gates.

## Job and current state

This is one separately selectable native-Go `swift-apple` extractor. It is source available only:
unregistered, unselected, Core-unreachable, uninstalled, and never launched by `corvint`. Core
continues to have no analyzer dependency edge. The command accepts exactly one
`--request-file PATH` argument and emits one LF-framed canonical response. On Darwin and Linux it
acquires that regular file with no-follow open plus before/open/after path-and-descriptor identity,
size, mode, modification-time, and double-read digest checks; any mismatch is the fixed sentinel.
Windows rejects descriptor acquisition closed. After acquisition the pure analyzer receives only
the copied immutable bytes and has no repository walk, process, toolchain, environment, dynamic
loader, or network capability.

## Requirements

- `SAC-001`: `internal/analyzerswift` and `cmd/corvint-analyzer-swift` remain isolated native Go
  source. No Core package imports either package and default commands/registers/installers do not
  mention them.
- `SAC-002`: The request is the frozen experimental envelope with family `swift-apple`, exactly
  three ordered input records (coordinate descriptor, selected Apple Swift source, and selected
  Apple-UI `Package.swift`), a Darwin/arm64/none target, empty features, canonical ordered JSON,
  bounded wire (1,500,000 bytes), bounded decoded input/output (1,048,576 bytes), and SHA-256
  verification before parsing. The descriptor acquisition contract above rejects symlinks and
  path/descriptor identity races before pure envelope analysis. The profile's canonical-extension
  rule applies (decision 0242), with schema membership by exact field name: a request whose only
  defect is extension members placed per decision 0221 (a), and whose remaining bytes are this
  closed envelope, is the bound `UNKNOWN_FIELD` rejection; every other envelope failure is
  the fixed sentinel.
- `SAC-003`: The closed coordinate descriptor binds Swift language `6.3`, SwiftPM tools `6.0`,
  Xcode object version `77`, clean `beamfall-apple` revision
  `8588cec3dedaef63bbff458e5e7c8bb6335de107`, full local `beamfall-apple-ui` revision
  `830a154d1d8f2f0b1b801a9a9a3b626bc74f6aa6`, and the selected Apple source blob. Any other
  coordinate is `EXACT_BINDING_UNAVAILABLE`; no nearest, latest, alias, range, or host observation
  is a fact.
- `SAC-004`: The only success facts are exact coordinate/dependency facts plus static top-level
  Swift imports, class/struct/enum/protocol/actor declarations, and extensions. Conditional
  compilation, macro/operator ambiguity, unknown top-level syntax, dynamic import, malformed
  comment/string/raw-string state, or duplicate facts fail the whole request; no partial output is
  usable. An attribute is admitted in one closed form -- `@` plus an identifier, optionally
  followed by one balanced parenthesised argument clause containing no brace -- and carries no
  tuple, so it is consumed as part of the declaration or import it decorates and contributes no
  fact. An attribute run that is not followed by a declaration or import, or whose argument clause
  is unbalanced or brace-bearing, is attribute-led ambiguity and rejects. Every emitted fact is
  exactly the frozen eight-field tuple in `analyzer-candidate-profiles.md`; lexer source locations
  are private parser state and never enter the fact envelope, diagnostics, or response bytes.
- `SAC-005`: Before the envelope validates, rejection is only the frozen sentinel. Afterwards a
  rejection echoes exactly profile, family, request ID, status, scope ID, compilation-unit ID,
  target, and input echoes, followed by one closed reason. Success and rejection use Go struct
  field order and one canonical JSON encoder.
- `SAC-006`: The candidate does not claim registry/lock selection, containment, launch, admission,
  compatibility, exact verification, support, or `PASS`. Those states are `NOT_RUN`.
- `SAC-007`: `BenchmarkAnalyzeCandidate` constructs the successful pinned Beamfall Apple request
  and expected canonical response before measurement. Qualification requires at least ten stable
  samples, each at or below 35,000 B/op and 250 allocs/op, plus an exact restored implementation or
  alternative failure for any causal ratchet. No such sample set or restored failure is recorded;
  Swift performance qualification and ACC-V0-020 remain `NOT_RUN`.
- `SAC-008`: Each refusal reports the one closed reason that names its actual cause.
  `DYNAMIC_INPUT` is reserved for bytes whose meaning depends on a value absent from the input:
  string interpolation, in plain, multiline, and raw literals. Conditional compilation (`#if`,
  `#elseif`, `#else`, `#endif`, `#sourceLocation`) and freestanding macros or pound literals
  (`#Preview`, `#available`, `#filePath`, `#selector`, `#warning`) are real Swift outside the
  closed matrix and report `UNSUPPORTED_SCHEMA`, as does unknown top-level syntax. Bytes that are
  not lexable Swift report `MALFORMED_INPUT`. A body over the source ceiling, a token count over
  the family token bound, and a raw-string delimiter run over the hash bound are bounded-resource
  refusals and report `LIMIT_EXCEEDED`; none of them is a statement about the grammar. The closed
  reason taxonomy in `analyzer-candidate-profiles.md` has no code that separates conditional
  compilation from macro expansion, so those two causes share `UNSUPPORTED_SCHEMA` until an owner
  amends that taxonomy.

## Limits, checks, and rollback

Production source is capped by the shared 65,536-byte limit. The canonical benchmark uses the
successful pinned vector through `BenchmarkAnalyzeCandidate`; its 35,000 B/op and 250 allocs/op
ceilings are unqualified until the `SAC-007` sample and restored-failure evidence exists. Focused
tests cover exact bytes, sentinel/scoped errors, descriptor no-follow FIFO/device/path-replacement
fail-closed behavior, ambiguous Swift rejection, extra-byte rejection, output/retention limits,
and pinned coordinate values. Race, vet, and Darwin/Linux/Windows builds are required on the exact
commit. Revert this commit to remove the entire family; no lock, registry, package, or Core binary
depends on it.

## Traceability

| Requirement | Implementation/evidence |
|---|---|
| `SAC-001` | `internal/analyzerswift`, `cmd/corvint-analyzer-swift`; dependency/diff checks |
| `SAC-002,005` | `analyzer.go`, `envelope.go` `extension`; exact-vector/error tests, `TestSafeCanonicalExtensionBindsUnknownField` |
| `SAC-003,004` | `parser.go`, frozen tuple matrix, pinned-vector, attribute-admission, and ambiguity tests |
| `SAC-008` | `parser.go` `poundReason`/`lexSwift`; `TestParseSourceRejectionReasonsAreExact` |
| `SAC-006` | no Core integration; explicit `NOT_RUN` boundary above |
| `SAC-007` | `BenchmarkAnalyzeCandidate`; qualification explicitly `NOT_RUN` |

## Open promotion gates

The signed registry, terminal lock, descriptor-safe external filesystem acquisition with no-follow
identity rechecks, process containment, admission, exact verifier, qualifying stable performance
samples and restored-failure evidence, 100-fresh-process ACC-V0-020 baselines, independent review,
and any real Beamfall dogfood run remain `NOT_RUN`.
