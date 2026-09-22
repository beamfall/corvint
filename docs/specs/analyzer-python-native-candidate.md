# Native Python 3.12 analyzer candidate

Intent status: proposed
Delivery status: deferred
Disposition: deferred by decision 0132; return only with an accepted qualification-and-launch profile.

Status: proposed candidate, deferred by decision 0132. It is unregistered, unselected,
Core-unreachable, and `NOT_RUN` for installation, launch, qualification, and
support. `cmd/corvint-analyzer-python` is separately buildable; neither
`cmd/corvint` nor its dependency graph imports `internal/analyzerpython`.

## Agent digest
- Claim: An isolated native-Go Python 3.12 extractor emits bounded structural facts without Core registration, selection, launch, or support.
- Status: proposed/deferred
- Exists: `internal/analyzerpython` and `cmd/corvint-analyzer-python`.
- Blocked on: an accepted qualification-and-launch profile.
- Read next: `analyzer-capability-contract-v0.md` and `analyzer-candidate-profiles.md`.

## Closed profile coordinates

| Coordinate | Literal value |
|---|---|
| Profile | `corvint-analyzer-candidate/experimental` |
| Family | `python` |
| Command | `./cmd/corvint-analyzer-python` |
| Parser | owned bounded lexer and closed native-Go Python 3.12 subset in `internal/analyzerpython` |
| Grammar accepted for facts | bounded Python 3.12 lexical source and the named closed statement families only |
| Source coordinate | logical path, one-based `path:line:column` |
| Toolchain/runtime | no Python interpreter, process, shell, network, repository path, CWD, HOME, or ambient configuration is used |

## Hermetic build provenance

The candidate has no non-standard Go module requirements, no `go.sum`, and no
`vendor/` tree. `script/check-analyzer-python-offline-build.sh ROOT` starts with
an empty `GOMODCACHE`, forces `GOPROXY=off`, builds with `GOFLAGS=-mod=mod`, and
proves that Core excludes `internal/analyzerpython`.

The grammar is owned native Go, not an interpreter or external-parser claim.
It accepts bounded comments, ordinary/triple/raw/f/bytes strings, balanced
delimiters and indentation, imports/relative imports, decorators,
functions/async functions/classes, PEP 695 type aliases without defaults,
compound suites, and direct expression-statement calls. Other balanced
expression spans are opaque and cannot yield a fact. Unclosed strings,
delimiters, indentation, or named fact-bearing statement forms reject as
`MALFORMED_INPUT`; unrepresentable static names reject as `DYNAMIC_INPUT`.
Python 3.13 type-parameter defaults, Python 3.14 template-string prefixes, and
PEP 758 unparenthesized exception lists (including explicit continuation
`except A \\` newline `, B:`) reject as `UNSUPPORTED_SCHEMA`.

Structural suites consume a matching `INDENT` after a non-inline recognized
header and matching `DEDENT` before returning to its parent; unexpected layout
rejects. Function and async-function headers require a closed parameter list,
optional closed type parameters, optional nonempty return annotation, and a
header colon; class headers permit only optional closed type parameters and
bases before that colon. Parameter separators, defaults, annotations, `/`,
`*`, and `**` obey their closed ordering productions. Decorators are exactly a
static dotted target or one call of that target; the call's comma-separated
arguments obey the closed expression grammar (`TestDecoratorCallArgumentsFailClosed`),
preserve Python's cross-argument positional, keyword, `*`, and `**` ordering
(`TestDecoratorCallArgumentOrdering`), and trailing dynamic expression syntax
rejects as `DYNAMIC_INPUT`. F-string replacement fields use typed
delimiter matching, reject lone literal `}`, and reject direct newlines in
non-triple f-strings. Bytes-literal source characters are ASCII; a non-ASCII
code point rejects as `MALFORMED_INPUT`, while its ASCII escape spelling remains
accepted (`TestPythonBytesLiteralSourceIsASCII`). Type-parameter default
detection matches the full outer bracket before testing top-level `=`.

Statement placement follows CPython 3.12, so no fact describes code Python
would not parse (`PNC-003`, `PNC-004`). Indentation is compared at tab size 8
and tab size 1; an indent or dedent whose ordering differs between the two
rejects as `MALFORMED_INPUT`, as CPython raises `TabError`. A pending decorator
does not cross a `DEDENT`. Outside a string, `\\` is valid only as an immediate
LF continuation; every other bare backslash rejects (`TestPythonBareBackslashRejects`).
After `;` only a simple statement may follow, so a
decorator, definition, or compound header there rejects. In `from M import`,
`*` is only the sole unparenthesized target without `as`, an empty `()`
rejects, and a parenthesized name list may close directly after its last name
without a trailing comma. A block `case` is valid only directly within a
`match` suite, whose direct statements are one or more block `case` clauses;
ordinary `case` and `match` soft-keyword assignments remain simple statements
(`TestPythonMatchCasePlacementFailsClosed`). Evidence also includes the
`TestPNC00*` tests in `internal/pythongrammar/statementplacement_test.go`.

## Requirements

- `PNC-001`: The candidate MUST remain separately buildable and unreachable from Core.
- `PNC-002`: The scanner MUST prospectively bound canonical request members before decode or retention.
- `PNC-003`: Accepted closed inputs MUST emit only fully typed, deterministic candidate facts.
- `PNC-004`: Python syntax outside the closed 3.12 profile MUST reject without source facts.
- `PNC-005`: Unrepresentable static source targets MUST reject as `DYNAMIC_INPUT`, never be omitted.
- `PNC-006`: Padded base64 MUST be strict RFC 4648, including canonical pad bits.
- `PNC-007`: The exact 126-file pinned Beamfall corpus MUST remain literal-object based and accepted.
- `PNC-008`: Fresh-process permutations MUST bind to independent full-response byte oracles.
- `PNC-009`: The command MUST not use ambient process, filesystem, or network channels.
- `PNC-010`: The identity-pinned restored parent MUST fail the corpus and candidate allocation growth MUST stay linear.

## Real Beamfall dogfood corpus

`TestLiteralPinnedBeamfallPythonFixtures` reads bytes only with `git show
REV:path`, never from a fixture checkout worktree. Before every run it requires
the external checkout clean, the literal commit object present and resolving
exactly, the exact Python blob count, and this SHA-256 over sorted
`path || NUL || sha256(blob-bytes) || NUL` records:

| Consumer | Revision | Python blobs | Aggregate SHA-256 |
|---|---|---:|---|
| Beamfall Core | `da38c59eb30b2121cbac37b912485b30b2e54841` | 114 | `da71de79a7d9c14341061337fbfca548a0d3ee7ec0564db04460a7f303723b11` |
| Beamfall Apple | `8588cec3dedaef63bbff458e5e7c8bb6335de107` | 10 | `212224660edfbc7ff7621378ade8c77fa3e972dfe06c88d54ab525a60b510609` |
| Beamfall relay | `9723152fdd4ead36b32553a171d98749e4fc23e9` | 1 | `8c2c1259831d7dbd39d2592e4edc9ff20b9a7926750b68b268a8af5293d3321a` |
| Beamfall plugin SDK | `5536ecde6d12214d6786a6832e537bd5d4feca02` | 1 | `5e3c8519a5d4e0c8f2919a37433d6e9e2f00e500d4847852aba31fbca950c141` |

Every one of those 126 literal blobs is independently framed as `py.source`
and must produce `CANDIDATE` plus its exact `path:1:1` source fact. The corpus
therefore covers the active Beamfall tooling profile, not a substitute toy
subset. The pins are fixture identities only; they do not claim an installed
Python, host qualification, support, verifier, or launch protocol.

## Closed inputs and extracted facts

The common experimental envelope in
[`analyzer-candidate-profiles.md`](analyzer-candidate-profiles.md) applies
unchanged. The Python family accepts only these records, strictly ordered by
`(handle,family,path,sha256)` with globally unique handles and paths:

| Input family | Exact path | Closed contents | Facts |
|---|---|---|---|
| `py.project` | `pyproject.toml` | exactly `requires-python = "==3.12.0"\n` | `python.language.declaration` |
| `py.toolchain` | `.python-version` | exactly `3.12.0\n` | `python.toolchain.declaration` |
| `py.requirements` | `requirements.txt` | strictly increasing `lower-name==X.Y.Z` LF rows, each at most 512 bytes | `python.dependency.pinned` |
| `py.source` | any closed logical `.py` path | UTF-8, LF, no CR/NUL, closed native Python 3.12 subset | `python.source`, `python.import.static`, `python.definition`, `python.decorator`, `python.call.static` |

`py.source` accepts the closed construct families named above. It extracts only
static imports, static decorator targets, definitions, and bare
expression-statement static calls; opaque expression spans cannot fabricate a
fact. Nested suites are lexically walked. A dynamic/unrepresentable source name
or fact atom rejects the complete request as `DYNAMIC_INPUT`.

Every fact is a fully typed tuple in deterministic field-by-field order:
`(kind,input_handle,related_handle,subject,predicate,value,instance_id,evidence_sha256)`.
`instance_id` is the exact source coordinate for source-derived facts, including
`path:1:1` for the parse witness. Every non-evidence fact field is validated
before evidence generation, then the generated evidence digest is validated
before append. A derived coordinate above the 4,096-byte fact-field limit
rejects as `LIMIT_EXCEEDED`; it is never hashed or emitted. No
delimiter-derived ordering key is used.

## Bounds and failure framing

Before JSON decoding or request allocation/retention, the native scanner
canonical-decodes JSON member names, then consumes depth (8), every key/value
token (4,096), input count (128), target feature count (64), and raw string
length prospectively: ordinary strings are
at most 4,096 bytes and only a `content_base64` member may reach 1,398,104 bytes.
It takes slices of the caller-owned request frame rather than copying strings.
After canonical decoding and validation, each base64 record is length-checked
before allocation, decoded one at a time against the 1,048,576-byte aggregate,
then its strict RFC 4648 base64 field and decoded bytes are discarded before the next input;
noncanonical pad bits reject.
Native token spans and source strings are local to one input. Facts are charged by
their exact canonical JSON bytes (including comma/framing) before append and
the final canonical output is rechecked against 1,048,576 bytes. Inputs are at
most 128 and facts at most 4,096; every breach returns no partial facts.
Requirements rows are scanned one line at a time and reject above 512 bytes;
they never split an unbounded source into a retained row slice.
Before native grammar analysis, the lexical preflight caps source atoms at
65,536, delimiter nesting (including f-string expressions) at 256, and
indentation nesting at 256 while skipping comments and string bodies.
The f-string scanner shares the source atom counter through recursive
replacement fields and format specifications, so an expression cannot bypass
the atom ceiling. The PEP 758 closure continues through comments and newlines
while delimiters remain open, then rejects a depth-zero exception-list comma.
Explicit-continuation indentation is not structural nesting. The owned lexer and
closed statement grammar are the only grammar authority.

Malformed/noncanonical input before a valid `family` and `request_id`, including
`UNKNOWN_FAMILY`, has only the fixed minimal sentinel, as does any invalid echoed
identifier, target, input handle, malformed input digest (`DIGEST_MISMATCH`), or
duplicate handle (`TestInvalidEnvelopeRejectionStaysUnbound`). An otherwise safe canonical
extension is the bound `UNKNOWN_FIELD` once those checks pass, and any other unknown member is the
`NONCANONICAL_REQUEST` sentinel (decision 0222, `TestSafeCanonicalExtensionBindsUnknownField`).
Once the complete envelope
binds, every other rejection
uses the deterministic full object containing profile, family, request ID,
scope, compilation unit, target, every ordered input echo, and one closed
reason. `TestRejectionEchoesEveryBoundInputAndReason` and the built-command
literal rejection vector freeze this byte shape.

## Boundary, isolation, and performance evidence

The built command owns only inherited stdin/stdout. It does no descriptor path
open, so a nofollow/open/after-identity sequence is inapplicable: caller bytes
arrive only on stdin. It reads at most byte 1,500,001 and `writeAll` loops over
partial writes, treating zero/invalid short writes as an error. The command
tests freeze literal success/rejection stdout bytes, test partial/zero writers,
and run 1,001 identical outputs plus 1,024 distinct permutations as fresh
compiled processes with exact independent literal full-response oracles and
replay checks.

The all-channel isolation test supplies PATH traps for Python/Git/shell,
separate CWD/HOME/TMP/XDG snapshots, and a localhost listener. It proves the
built candidate hits none; a test-helper negative control deliberately invokes
the trap, writes all three locations, and dials the listener, proving each spy
detects a violation.

`BenchmarkRepresentativeCandidate` uses the feature-rich fixture above;
`BenchmarkRestoredParentScanner` and `BenchmarkRestoredParentRegexp` retain
the parent comparison path. Under the receipt toolchain with `GOMAXPROCS=1`,
the restored scanner reproduced its exact 1,776 B/op and 12 allocs/op and the
regexp baseline its exact 2,001 B/op and 15 allocs/op; this host measured
0.59–0.61 µs/op and 1.69–1.79 µs/op respectively, rather than treating earlier
0.79–1.68 µs/op and 1.94–1.97 µs/op timings as portable facts.
`TestPerformanceRatchets` proves the exact restored parent
`90795d7750405933227962633b23c5441ef79ae9` line parser rejects at least one
input from the real pinned 126-file corpus while this candidate accepts every
input, preventing a faster reduced-semantics substitution. Five private raw
allocation samples process that corpus and each must remain at most 600,000
allocations; a doubled-corpus allocation ratchet rejects superlinear growth.
The test pins the exact restored production parser source before requiring that
it reject a corpus input. Wall time is benchmark-only local evidence and is
never a test assertion or portability claim.

`script/check-analyzer-python-ratchets.sh ROOT BINARY` checks source, command,
and stripped-binary ceilings. `script/check-analyzer-python-offline-build.sh
ROOT` proves the clean-cache offline candidate build and Core dependency
separation. Focused `go test`, `go vet`, race, and cross-build receipts are
required before a fresh independent review; this document supplies no activation
or support claim.
