# Test Claim Qualification V0

Owner: Russell Lewis
Frozen: 2026-08-23
Intent status: proposed
Delivery status: implementation-candidate; promotion evidence NOT_RUN
Freeze note: the 60-edge labelled corpus and reporter-compatibility runs are deferred until a WP6
authority root is specified; absent that root, the corpus would measure a relation nobody may act on.
Authoritative inputs: `docs/specs/ocm-v0-dogfood.md`,
`docs/specs/cem-0.2-canonical-binding.md`, `docs/specs/change-frontier-v0.md`,
`docs/PRODUCT.md`, `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/BUILD-LOG.md`

## Agent digest
- Claim: TCQ deterministically qualifies selected OCM test anchors and caller-supplied JUnit matches without proving adequacy or correctness.
- Status: proposed/implementation-candidate; promotion evidence NOT_RUN
- Exists: deterministic reference implementation and vectors for selected OCM claim qualification; an optional declared environment variant per observation and a shared same-revision flake rule (`TCQ-V0-048..050`); a per-hunk patch coverage witness from one local coverprofile with a visible reviewer-report downgrade (`TCQ-V0-051..054`); a bounded per-hunk mutation discrimination witness reusing the `prove --mutate` runner, with survivors as a visible downgrade that never fails the build (`TCQ-V0-055..058`); a fixed sandbox-unavailable not-run reason when the macOS launcher refuses a nested profile (`TCQ-V0-059`, accepted by decision 0463); an offered "cite the guarding test" producer step whose cited span alone names the tests the consumer hunk-revert falsifier runs (`TCQ-V0-060..061`, V1-1266); one compiled-in runner contract and fold for the Go and Ruby `discriminate` arms, and an execution-only same-change test witness (`TCQ-V0-067..071`, experimental, V1-2798).
- Blocked on: a WP6 authority root, 60-edge labelled corpus, reporter compatibility, and promotion gates.
- Read next: Threat model and claim boundary; Requirements; Acceptance and adversarial matrix.

## User and job

Before the Verified Absence Frontier can use a selected OCM test claim, a maintainer needs to know
whether that exact source anchor belongs to one safely delimited, runnable-shaped test unit and
whether a caller-supplied JUnit report contains one row matching that unit-derived execution key at
the target.

Test Claim Qualification V0 (`tcq/0`) is a deterministic, source-body-free matcher over one
verified OCM, only its selected claim anchors, their exact target-tree test units, and optionally
one command plus one JUnit observation. It emits one result per selected `(obligationId, claimId)`
edge. It does not decide semantic relevance, test adequacy, code coverage, requirement
satisfaction, implementation correctness, report authenticity, or frontier closure.

This specification remains proposed WP4 intent. A reference implementation and deterministic
reference vectors exist, but the labelled corpus, reporter-compatibility, and ten-change promotion
gates remain `NOT_RUN`. WP4 alone closes neither an OCM obligation nor the Verified Absence
Frontier. WP5 may compose the narrow relation below with separately delivered canonical CEM binding
and WP3 relevance; no structural OCM state is upgraded here.

## Threat model and claim boundary

V0 detects malformed, noncanonical, mismatched, stale, and resource-exhausting supplied artifacts.
It does **not** resist a malicious or compromised same-UID caller or task agent. That caller can
fabricate a command artifact, JUnit report, observation, or clean-target statement. V0 has no
portable authority, authentication, signature, trusted runner, or execution attestation.

The only positive relation is `test-report-matched-v0`. It means:

> At the target revision, this OCM-selected requirement-reference anchor is exactly associated
> with this test unit, and the caller supplied a report containing exactly one passing row matching
> the unit-derived execution key under this command while attesting that the target was clean.

Every result edge carries exact `authorityClass: CALLER_REPORTED`. It does not mean that the
test proves the requirement, covers changed code, ran in a trustworthy environment, or passed under
an independently controlled harness. In particular, key matching does not establish which dynamic
Python body executed when source contains duplicate runtime names; the exact body is source identity
and review context only. Signing caller-controlled bytes does not upgrade that class. A future WP6
MAY add a harness-controlled, authenticated authority root and relation under new profile names. It
MUST NOT upgrade V0 artifacts in place.

## Definitions

- **selected claim edge**: one `(obligationId, claimId)` reference in a structurally valid linked
  OCM row;
- **selected anchor**: the exact claim span re-extracted from the target blob with the OCM claim
  extractor;
- **test unit**: the one exact enclosing Python test function, Go test function, or admitted Go
  table-case leaf associated with a selected anchor;
- **runnable-shaped**: a supported, safely delimited body that is non-empty and not statically
  known to be unconditionally skipped; it does not mean adequate or correct;
- **command artifact**: a content-addressed declaration of the exact argv, cwd, runner, target, and
  V0 environment-omission policy supplied by the caller;
- **expected base** and **caller target**: required invocation inputs supplied independently of all
  artifacts and resolved locally to exact commit OIDs through the CEM 0.2 repository boundary;
- **observation**: a content-addressed, body-free projection of one caller-supplied JUnit report,
  its target, command ID, command exit code, deterministic rows, and an optional declared
  environment variant;
- **environment variant**: the caller-declared name/value pairs under which one observation ran,
  in the ResultDB variant-key shape; an observation that declares none has an unknown environment,
  never an empty one;
- **prior observation**: an earlier verified-shape observation of the same target supplied by the
  caller so that TCQ can compare outcomes of one test across runs; it is never re-derived from a
  report and never adds a relation;
- **operational failure**: invalid, unavailable, unsafe, noncanonical, mismatched, unsupported, or
  exhausted invocation input; it emits no partial TCQ artifact;
- **claim abstention**: a valid selected edge that V0 cannot associate or classify; it remains a
  canonical claim result with null unit and execution identity.

## Requirements

### Boundary, profiles, and exact selected set

- `TCQ-V0-001`: TCQ MUST accept exactly one canonical `ocm/0.1-experimental` artifact and its
  canonical `cem/0.2` artifact plus independent caller expected-base and target inputs. It MUST pass
  the same bounded raw byte copies and both revision inputs through the current OCM verifier, which
  in turn invokes the shared CEM 0.2 canonical verifier. The resolved expected base MUST equal CEM
  `baseRevision`; the resolved caller target MUST equal OCM `targetRevision`. TCQ MUST bind both
  resolved OIDs in its result and read source only from that target tree through the inherited
  bounded, sanitized, no-fetch Git boundary. Worktree bytes, sibling repositories, implicit fetch,
  and additional-repository discovery are forbidden.
- `TCQ-V0-002`: legacy `cem/0.1` is not a TCQ input. It MUST fail operationally as
  `unsupported-cem-profile`; it cannot produce an abstention or matched relation. TCQ therefore
  cannot ship before the independently specified CEM 0.2 canonical binding is available.
- `TCQ-V0-003`: TCQ MUST emit exactly one claim result for each selected claim edge and no result
  for an unknown OCM obligation or unselected claim. The set is derived by traversing OCM
  obligations in OCM order and selecting every claim ID in each `linked` row. Results retain that
  obligation order and then sort by claim ID. A claim referenced by two obligations produces two
  separate edge results. Duplicate references inside one row retain the inherited OCM structural
  failure.
- `TCQ-V0-004`: only OCM extractor `corvint-test-claim/1` is accepted. The verifier MUST re-run that
  extractor against the target blob and selector, re-derive the exact span bytes and anchor profile
  from source grammar, and verify the claim ID. It MUST NOT trust a caller-supplied language,
  anchor kind, span, source, runtime name, or profile label.
- `TCQ-V0-005`: profile dispatch is exactly this table. Unknown extractor IDs are
  `unsupported-claim-extractor`; unsupported anchor profiles are valid per-claim abstentions with
  `unsupported-anchor-profile`. There is no extension fallback.

  | Re-derived anchor profile | Unit profile | V0 disposition |
  |---|---|---|
  | `python-test-name/1` | `python-ast/1` (Python 3.9 grammar) | associate exact named function |
  | `python-docstring/1` | `python-ast/1` (Python 3.9 grammar) | associate exact enclosing function |
  | `go-test-name/1` | `go-lexical/1` | associate exact named function |
  | `go-table-case/1` | `go-lexical/1` | associate exact parent/case leaf |
  | any other profile | none | abstain |

### Independent axes and association

- `TCQ-V0-006`: every claim result exposes independent `associationState`, `hygieneState`, and
  `reportState` axes. Their exact values are:

  | Axis | Values |
  |---|---|
  | `associationState` | `ASSOCIATED|ABSTAINED` |
  | `hygieneState` | `ELIGIBLE|INELIGIBLE|ABSTAINED` |
  | `reportState` | `NOT_MATCHED|PASSED|FAILED|ERROR|SKIPPED|AMBIGUOUS` |

  One axis MUST NOT rewrite another. No axis is a confidence, coverage, proof, or generic success
  score. `associationKind` is null or exactly `PYTHON_DOCSTRING_FUNCTION`,
  `PYTHON_TEST_FUNCTION`, `GO_TEST_FUNCTION`, or `GO_TABLE_CASE`. Every selected edge has exact
  `authorityClass: CALLER_REPORTED`, including an abstention; no input can select another class.
- `TCQ-V0-007`: association is per selected claim edge. TCQ MUST inspect only the selected anchor
  and the smallest enclosing construct in the same verified blob. It MUST NOT extract or borrow a
  sibling anchor, claim, association, requirement reference, role, semantic term, or hygiene
  evidence. The sole exception is one bounded scan of supported test units per distinct selected
  target blob, used only to derive execution keys and their collision counts under `TCQ-V0-021`.
  That scan emits no sibling claim or unit and supplies no derived fact except the internal,
  non-persisted collision cardinality for a selected edge's own key.
- `TCQ-V0-008`: a Python `python-test-name/1` anchor associates only when its exact span is the
  identifier span of one `FunctionDef` or `AsyncFunctionDef` named `test_[A-Za-z0-9_]+`. A
  `python-docstring/1` anchor associates only when its exact span is within the leading docstring
  statement of exactly one such enclosing function. Its association kind is respectively
  `PYTHON_TEST_FUNCTION` or `PYTHON_DOCSTRING_FUNCTION`, and both use that exact enclosing
  function's runtime name. A legal Python name need not and generally cannot contain a hyphenated
  obligation ID.
- `TCQ-V0-009`: a Go `go-test-name/1` anchor associates only when its exact span is the identifier
  of one validated `TestX` function. A `go-table-case/1` anchor is a leaf component: it associates
  only when its span is one admitted case literal inside exactly one validated parent `TestX`; its
  runtime name is exactly `TestX/<case>`. Association kind is respectively `GO_TEST_FUNCTION` or
  `GO_TABLE_CASE`.
- `TCQ-V0-010`: the selected anchor remains a caller-authored requirement reference. Exact
  enclosure permits the surrounding unit to be matched; it does not turn anchor text into proof of
  that requirement. Multiple requirement IDs in one docstring remain separate selected edges and
  separate caller assertions. Each may associate with the same unit, but none inherits another
  edge's identity, state, reasons, or relation.
- `TCQ-V0-011`: association with no candidate yields `ABSTAINED` and
  `claim-association-missing`; more than one candidate yields `ABSTAINED` and
  `claim-association-ambiguous`. Unsupported profiles use `unsupported-anchor-profile`. All three
  use `associationKind: null`, `testUnitId: null`, `executionKeySha256: null`,
  `hygieneState: ABSTAINED`, `reportState: NOT_MATCHED`, empty row IDs, and a null relation. A
  supported re-derived anchor profile is retained for missing, ambiguous, and unparseable
  association, including unsupported Python grammar and offset mismatch; `anchorProfile` is null
  only for an unsupported profile. They are not invocation failures.
- `TCQ-V0-012`: WP3 owns semantic relevance. TCQ performs no stopword, identifier-overlap,
  assertion-presence, requirement-entailment, or claim-quality classification. Source association
  and runnable shape alone cannot close a frontier item.

### Python and Go runnable shape

- `TCQ-V0-013`: Python profile `python-ast/1` is frozen to the Python 3.9 `exec`-mode grammar with
  type comments disabled; it never uses a parser's default/latest grammar. It accepts only
  a UTF-8 `.py` target blob. A unit is a `FunctionDef` or `AsyncFunctionDef` whose complete name
  matches `test_[A-Za-z0-9_]+` and whose direct lexical parent is the module or a class; class methods
  are allowed and nested local functions are not. The body span begins at the first body node and
  ends at the function end offset using original-source UTF-8 byte offsets. Python source rejected
  by the frozen grammar abstains as `unsupported-python-grammar`; there is no fallback host parse.
- `TCQ-V0-014`: for Python hygiene, remove one leading docstring. The body is `empty-body` when
  every remaining statement is `pass`, a standalone constant or ellipsis expression, or a bare
  `return` with no value. A function containing only a bare return is empty. `return <expression>`
  and calls are non-empty without implying an assertion or adequate test.
- `TCQ-V0-015`: Python is `unconditional-skip` when the function or enclosing class has exact
  syntactic `unittest.skip(...)`, `pytest.mark.skip` with or without a call, literal
  `unittest.skipIf(True,...)`, `unittest.skipUnless(False,...)`, or
  `pytest.mark.skipif(True,...)`, or when its first non-no-op statement is unconditional
  `self.skipTest(...)`, `pytest.skip(...)`, or `raise unittest.SkipTest(...)`. Aliases and dynamic
  conditions are not resolved.
- `TCQ-V0-016`: Go profile `go-lexical/1` accepts only a path ending `_test.go` and a bounded token
  scan of a top-level, no-receiver `func TestX(<name> *testing.T) { ... }`, where the function name
  matches `Test[A-Z][A-Za-z0-9_]*`. The scanner pairs comments, interpreted and raw strings, rune
  literals, parentheses, and braces. Comment/string lookalikes, wrong signatures, and lexical
  failure produce `unparseable-test-unit` abstention.
- `TCQ-V0-017`: a Go body with no effective token, or only semicolons and one bare `return`, is
  `empty-body`. A first effective `<parameter>.Skip(...)`, `.Skipf(...)`, or `.SkipNow()` is
  `unconditional-skip`. Other non-empty bodies remain only runnable-shaped.
- `TCQ-V0-018`: a Go case is a table entry's `name`/`testName`/`test_name` field or the first
  argument of an identifier's `.Run(` call (`t.Run("...")`, decision 0029); it is admitted only
  when its selected anchor is a simple unescaped ASCII string literal matching
  `[A-Za-z0-9_-]{1,96}` inside one validated parent test body. The
  case unit uses the parent function's body span and body digest. Dynamic, escaped, concatenated,
  out-of-body, or ambiguous cases abstain. Both the claim extractor and the unit scanner admit the
  `.Run(` form (Go evidence: `TestGoRunCaseAssociatesAsTableCase`). The selector's parent MUST be
  the validated parent test whose body holds the anchor; a selector naming a parent the unit scanner
  did not admit, such as a `func TestX(` inside a string, abstains `claim-association-missing`
  (Go evidence: `TestGoCaseSelectorMustNameTheScannedParent`).
- `TCQ-V0-019`: a safely delimited unit is `INELIGIBLE` if `empty-body` or
  `unconditional-skip` applies, otherwise `ELIGIBLE`. Both reasons are retained when both apply.
  Unsupported profile, parse failure, or missing/ambiguous association is `ABSTAINED`. JavaScript,
  JSX, TypeScript, TSX, MJS, and CJS are wholly deferred. A future profile MUST classify
  comment/semicolon-only callbacks and a callback whose sole statement is bare `return` as empty
  before support is advertised.

### Source and execution identities

- `TCQ-V0-020`: each associated unit binds exactly `extractorProfile`, normalized target `path`,
  target `blobOid`, half-open `bodySpan`, raw `bodySha256`, and `executionKeySha256`. All are
  re-derived from the target blob. Raw source, runtime names, and anchor text are never persisted in
  TCQ output.
- `TCQ-V0-021`: runtime name is the exact Python function name, Go `TestX`, or admitted
  `TestX/<case>`. Execution key is:

  ```text
  SHA-256(UTF8("corvint-test-execution-key/0") || 0x00 ||
          canonical-json-value({"name":runtimeName,"path":normalizedPath}))
  ```

  `canonical-json-value` is canonical JSON without a terminal LF. If multiple safely delimited
  supported test units in that same target blob derive one execution key, every selected edge whose
  own unit derives that key uses `reportState: AMBIGUOUS`, `execution-identity-ambiguous`, empty row
  IDs, and no relation, even when only one colliding unit is selected and the report has exactly one
  matching row. No sibling body, hygiene, anchor, claim, or requirement data participates.
- `TCQ-V0-022`: unit ID is `test-unit:sha256:` plus:

  ```text
  SHA-256(UTF8("corvint-test-unit/0") || 0x00 || canonical-json-value({
    "blobOid":blobOid,
    "bodySha256":bodySha256,
    "bodySpan":{"end":end,"start":start},
    "executionKeySha256":executionKeySha256,
    "extractorProfile":extractorProfile,
    "path":normalizedPath
  }))
  ```

  Claim identity, association kind, hygiene, report state, and relation do not participate in unit
  identity.

### Canonical command artifact

- `TCQ-V0-023`: dynamic matching requires one canonical `test-command/0.1-experimental` artifact:

  ```json
  {"argv":["python3","-m","unittest"],"cleanTarget":"CALLER_ATTESTED_CLEAN","cwd":".","environment":{"entries":[],"policy":"OMITTED"},"id":"test-command:sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","runner":{"identity":"python-unittest","version":"3.13.7"},"spec":"test-command/0.1-experimental","targetRevision":"0123456789abcdef0123456789abcdef01234567"}
  ```

  All shown fields are required and no others are allowed. `argv` is an array, never a shell
  string: 1..64 UTF-8 strings, each 1..1,024 bytes, at most 8,192 bytes total, with no NUL or ASCII
  control. `cwd` is `.` or a normalized repository-relative directory under the CEM path grammar,
  at most 512 bytes. After caller-target resolution, `.` denotes the target root; every other `cwd`
  component MUST resolve inside that target Git tree and the final object MUST be a tree. A missing
  object, blob, symlink, gitlink, or unsupported mode fails redacted `command-cwd-unavailable`.
  Worktree filesystem directories are never cwd authority. Runner identity and version are each
  1..128 bytes and match `[A-Za-z0-9][A-Za-z0-9._/+:-]{0,127}`.
- `TCQ-V0-024`: V0 environment policy is exactly `{"entries":[],"policy":"OMITTED"}`. No
  environment names or values are persisted or inferred. A caller needing an environment-sensitive
  claim must wait for a future explicit profile. `cleanTarget` is exactly
  `CALLER_ATTESTED_CLEAN|NOT_ATTESTED`; it is an unauthenticated caller statement, not verifier
  evidence. Only the former can participate in the V0 relation.
- `TCQ-V0-025`: command ID is `test-command:sha256:` plus:

  ```text
  SHA-256(UTF8("corvint-test-command/0.1-experimental") || 0x00 ||
          canonical-json-value(command artifact without id))
  ```

  The command is sensitive repository metadata: argv may contain secrets. TCQ neither executes it
  nor claims fallible heuristic secret screening. It never persists argv, implicitly commits or
  exports it, or echoes it, and stores only `commandId` in the TCQ result. Users are responsible for
  not supplying secrets. Exact argv remains in the caller-supplied artifact so it is inspectable and
  may support best-effort re-invocation; it does not make execution reproducible or the omitted
  environment complete. `commandId` is an integrity digest, not encryption or a secrecy transform,
  and may permit guessing low-entropy arguments.

### Strict JUnit report and observation

- `TCQ-V0-026`: before constructing an XML parser, TCQ MUST use the private report-byte copy to
  enforce the 4 MiB byte ceiling, strip at most one UTF-8 BOM, and perform a byte preflight. It MAY
  remove one leading declaration only when it is exactly `<?xml version="1.0"?>`,
  `<?xml version='1.0'?>`, or either form with one exact
  ` encoding="UTF-8"|"utf-8"|'UTF-8'|'utf-8'` clause using the declaration's quote style. After
  removing that declaration it rejects any `<?` byte sequence and rejects `<!DOCTYPE` or
  `<!ENTITY` anywhere, all case-sensitively. Only after that preflight passes may it decode UTF-8
  and construct a parser with DTD loading, DTD-defined entity declaration and expansion, external
  resolution, network access, and XInclude processing disabled. XML's predefined entities remain
  ordinary syntax. If the selected parser cannot mechanically disable those capabilities, V0 fails
  `invalid-junit` rather than parse. The parser and all later traversal consume only the same
  in-memory copy.
- `TCQ-V0-027`: raw JUnit has no namespace. The document has exactly one root element,
  `testsuites` or `testsuite`; a second top-level element or non-whitespace text outside the root
  invalidates the report (Go evidence: `TestJUnitRefusesContentOutsideTheSingleRoot`). A
  `testsuites` element's direct element children may only be `testsuite`. A `testsuite` element's direct element
  children may only be `properties`, `testsuite`, `testcase`, `system-out`, or `system-err`. A
  `testcase` element's direct element children may only be `properties`, `failure`, `error`,
  `skipped`, `system-out`, or `system-err`. `properties` may contain only `property`; `property`,
  `failure`, `error`, `skipped`, `system-out`, and `system-err` may contain no element children. Any
  unknown element or descendant, including XInclude or a nested `testcase`, invalidates the report.
  Comments are ignored. Unknown attributes are bounded and ignored. A repeated attribute name on
  one element, which XML forbids but a lenient parser may admit, invalidates the report; a
  namespace-prefixed attribute is an unknown attribute and never supplies `file`, `name`, `status`,
  or `disabled` (Go evidence: `TestJUnitRefusesAmbiguousStatusAttributes`). Text, properties, suite
  counters, aggregate status, time, failure messages, stdout, and stderr never contribute to
  identity or output. Terminal status uses only direct `failure|error|skipped` children of
  `testcase`. Depth, element, attribute, and testcase counters are enforced during this single
  strict traversal, not after building an unbounded tree. Because a decoder materializes a start
  tag's attributes before returning it, the per-element attribute count is bounded on the report
  bytes before the parser runs: an `=` outside a quoted value inside a tag, comments and CDATA
  excluded, counts one attribute (Go evidence: `TestJUnitBoundsAttributesBeforeDecoding`).
- `TCQ-V0-028`: testcase identity requires non-empty `file` and `name`, each at most 1,024 UTF-8
  bytes without NUL or ASCII control. Replace `\` with `/`, remove at most one leading `./`, enforce
  the CEM path grammar, and require a tracked target-tree blob in a supported mode. Absolute,
  drive-prefixed, empty, dot, dot-dot, unavailable, or unsupported paths are unkeyed. Unavailable
  means the target tree has no such entry. A failed target-tree lookup, for this `file` or for the
  command `cwd`, is not an unavailable path: it stops evaluation with its inherited repository or
  Git failure (Go evidence: `TestTreeLookupFailureIsNotAnAbsentPath`). `classname`
  and suite names never participate. For a keyed row, use the exact `name` as runtime name and
  derive `executionKeySha256` from that name and the normalized `file` using `TCQ-V0-021`. Raw names
  are not persisted. Unkeyed rows retain only a count.
- `TCQ-V0-029`: testcase status uses this complete, case-sensitive table. Any other combination,
  repeated terminal child, or multiple terminal kinds invalidates the report.

  | Direct terminal child | `status` | `disabled` | Result |
  |---|---|---|---|
  | none | absent, `run`, or `passed` | absent or `false` | `PASSED` |
  | none | absent, `notrun`, `disabled`, or `skipped` | `true` | `SKIPPED` |
  | none | `notrun`, `disabled`, or `skipped` | absent or `false` | `SKIPPED` |
  | one `failure` | absent or `failed` | absent or `false` | `FAILED` |
  | one `error` | absent or `error` | absent or `false` | `ERROR` |
  | one `skipped` | absent, `notrun`, `disabled`, or `skipped` | absent, `false`, or `true` | `SKIPPED` |

  Exit zero with any keyed or unkeyed `FAILED|ERROR` testcase is
  `report-command-inconsistent` and invalidates the dynamic invocation.
- `TCQ-V0-030`: testcase ordinal is zero-based depth-first document order over every `testcase`
  encountered while traversing the admitted tree, before keyed rows are sorted. Each keyed row
  stores exactly `executionKeySha256`, `id`, and `status`. Row ID is `test-row:sha256:` plus:

  ```text
  SHA-256(UTF8("corvint-junit-row/0") || 0x00 || canonical-json-value({
    "executionKeySha256":executionKeySha256,
    "ordinal":zeroBasedDocumentOrdinal,
    "reportSha256":reportSha256,
    "status":status
  }))
  ```
- `TCQ-V0-031`: one observation has this exact shape:

  ```json
  {"commandId":"test-command:sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","exitCode":0,"id":"test-observation:sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","report":{"bytes":24,"format":"junit-xml/corvint-v0","sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},"rows":[],"spec":"test-observation/0.1-experimental","targetRevision":"0123456789abcdef0123456789abcdef01234567","unkeyedRows":0}
  ```

  All fields are required. The only other member allowed is the optional `TCQ-V0-048`
  `environment` object; no further member is admitted. `exitCode` is `0..2147483647`. Rows sort by
  `(executionKeySha256,id)` and are strictly unique. Observation ID is `test-observation:sha256:`
  plus `SHA-256(UTF8("corvint-test-observation/0.1-experimental") || 0x00 ||
  canonical-json-value(observation without id))`; a declared `environment` is part of that preimage.
- `TCQ-V0-032`: observation verification requires the exact raw report and command artifact. It
  re-parses the entire report and re-derives byte count, digest, ordinals, row IDs, rows, and unkeyed
  count. Missing report, altered rows, digest mismatch, target mismatch, or command mismatch is an
  operational failure. The observation is a caller-reported projection, never authorization.

### Matching and exact result wire

- `TCQ-V0-033`: input combinations are exact:

  | Command | Observation | Raw report | Result |
  |---|---|---|---|
  | absent | absent | absent | valid static TCQ; all associated report states are `NOT_MATCHED` |
  | present | present | present | valid dynamic TCQ after full verification |
  | any other combination | any other combination | any other combination | `invalid-tcq-input` |

  Prior observations (`TCQ-V0-050`) are admitted only alongside the dynamic tuple; with the static
  combination any prior is `invalid-tcq-input`.

  The three artifacts are deliberately separate: a command is a reusable pre-run declaration, an
  observation projects one report under that command, and TCQ recomputes per-claim source
  associations. Their lifecycles differ, and merging them would either duplicate argv or make a
  cached result unverifiable against its exact command and report inputs.
- `TCQ-V0-034`: after source association, matching uses only exact execution keys. Zero matching
  keyed rows is `NOT_MATCHED`; use `row-identity-unavailable` instead of `test-not-matched` when the
  report has one or more unkeyed rows. Exactly one matching row yields its status. Two or more
  matching rows always yield `AMBIGUOUS` and `repeated-test-rows`; V0 defines no reporter retry or
  deduplication profile. A skipped, failed, or errored row never becomes passing. A row match states
  only equality with the source-derived key; it never identifies which colliding dynamic body ran.
- `TCQ-V0-035`: `test-report-matched-v0` is emitted if and only if all of these hold: canonical CEM
  0.2 and OCM 0.1 bindings verify; association is supported and exact; hygiene is `ELIGIBLE`;
  execution identity is unique; exactly one caller-supplied report row matches the unit-derived
  execution key and is `PASSED`; command,
  observation, OCM, and resolved caller target revisions are identical; resolved expected base
  equals CEM `baseRevision`; command clean target is `CALLER_ATTESTED_CLEAN`; and exit code is zero.
  Its authority remains `CALLER_REPORTED`. The relation is otherwise null and no weaker positive
  relation is substituted. Exact `bodySpan` and `bodySha256` identify source context only and are
  never execution evidence.
- `TCQ-V0-036`: a TCQ document has this exact top-level shape:

  ```json
  {"baseRevision":"0123456789abcdef0123456789abcdef01234567","claims":[],"commandId":null,"id":"tcq:sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","issues":[],"observation":null,"ocmSha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","profile":"tcq/0","targetRevision":"0123456789abcdef0123456789abcdef01234567","units":[]}
  ```

  `baseRevision` and `targetRevision` are the independently resolved invocation OIDs, not values
  learned from artifacts. Static output uses the null fields shown. Dynamic output requires
  `commandId` and exactly this observation-summary shape, with values copied from the verified
  observation:

  ```json
  {"commandId":"test-command:sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","exitCode":0,"id":"test-observation:sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","reportSha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
  ```

  No other conditional shape is valid, except that an observation which declares a `TCQ-V0-048`
  `environment` has that object copied verbatim into the summary as a fifth member `environment`;
  an undeclared environment adds no member, and the frozen vectors carry none.
- `TCQ-V0-037`: every claim item has exactly this shape:

  ```json
  {"anchorProfile":null,"associationKind":null,"associationState":"ABSTAINED","authorityClass":"CALLER_REPORTED","claimId":"claim:sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","executionKeySha256":null,"hygieneState":"ABSTAINED","obligationId":"TCQ-V0-001","reasons":["unsupported-anchor-profile"],"relation":null,"reportState":"NOT_MATCHED","rowIds":[],"testUnitId":null}
  ```

  Every item requires exact `authorityClass: CALLER_REPORTED`. Associated items require
  `associationState: ASSOCIATED` and non-null supported `anchorProfile`, `associationKind`,
  `testUnitId`, and `executionKeySha256`. Abstained items retain a supported `anchorProfile` when one
  was re-derived but have null association kind, unit ID, and execution key. `relation` is null or
  exactly `test-report-matched-v0`. `rowIds` contains exactly the matching row IDs, sorted by ID; it
  is empty for no match and execution-identity ambiguity. No executable identity is permitted on an
  abstention.
- `TCQ-V0-038`: every unit item has exactly the fields in the unit-ID preimage plus `id`. `units` is
  the unique set referenced by associated claim items, sorted by unit ID; no orphan unit is allowed.
  `claims` is exactly the selected-edge set. Every claim unit reference resolves within `units`;
  every row ID resolves within the verified observation; static results have no row references.
  `issues` is exactly one object `{"claimId":...,"obligationId":...,"reason":...}` for every claim
  reason and no others, sorted by OCM obligation order, claim ID, then reason order.
- `TCQ-V0-039`: reason order and vocabulary are exactly:

  1. `unsupported-anchor-profile`
  2. `claim-association-missing`
  3. `claim-association-ambiguous`
  4. `unsupported-python-grammar`
  5. `python-offset-mismatch`
  6. `unparseable-test-unit`
  7. `empty-body`
  8. `unconditional-skip`
  9. `execution-identity-ambiguous`
  10. `row-identity-unavailable`
  11. `test-not-matched`
  12. `repeated-test-rows`
  13. `test-skipped`
  14. `test-failed`
  15. `test-error`
  16. `target-cleanliness-not-attested`
  17. `command-failed`
  18. `test-flaky`

  Association abstention uses exactly its one applicable association reason. Associated claims add
  all applicable hygiene reasons. Report projection adds exactly one of: no dynamic input or zero
  keyed matches gives `test-not-matched` (except that any unkeyed rows substitute
  `row-identity-unavailable`); repeated matches give `repeated-test-rows`; a unique non-passing row
  gives its exact status reason; a unique passing row gives neither. A unique passing row also adds
  `target-cleanliness-not-attested` and/or `command-failed` when applicable. A claim with one or
  more matched rows also adds `test-flaky` when `TCQ-V0-050` holds for its execution key; the
  reason is reachable only through prior observations, so the frontier shim, which supplies none,
  never emits it. Reasons are unique and sorted above.
- `TCQ-V0-040`: TCQ ID is `tcq:sha256:` plus
  `SHA-256(UTF8("corvint-tcq/0") || 0x00 || canonical-json-value(TCQ document without id))`. Complete
  command, observation, and TCQ JSON artifacts use UTF-8, lexicographically sorted object keys,
  minimal JSON, and one terminal LF. Duplicate keys, floats, invalid Unicode, insignificant
  whitespace, or absent/extra fields are noncanonical.

### Resource, input, failure, and inspection contract

- `TCQ-V0-041`: raw limits are checked with a bounded, string-aware scanner before semantic JSON
  parsing. The scanner rejects nesting or aggregate token/member limits before allocating an
  unbounded tree. Limits are:

  | Input | Raw bytes | JSON depth | Aggregate object members plus array items |
  |---|---:|---:|---:|
  | inherited CEM | CEM 0.2 bound | CEM 0.2 bound | CEM 0.2 bound |
  | inherited OCM | 1 MiB | 16 | 20,000 |
  | command | 64 KiB | 8 | 256 |
  | observation | 4 MiB | 8 | 30,000 |
  | TCQ result verification | 4 MiB | 12 | 50,000 |

  JUnit is at most 4 MiB, 50,000 elements, depth 32, 64 attributes per element, 1,024 UTF-8 bytes
  per attribute, and 10,000 testcase elements. TCQ reads at most 512 selected edges, 512 unique
  units, 4,096 issues, 1 MiB per referenced source blob, 16 MiB source bytes total, 100,000 Python
  syntax nodes per blob, 100,000 Go tokens per blob, 10,000 collision-scan test-unit candidates per
  blob, and 20,000 collision candidates total. Exceeding any TCQ-owned limit is
  `tcq-resource-exhausted` and yields no partial output.
- `TCQ-V0-042`: operational validation stops at the first failing stage in this order:

  1. library call shape, immutable-bytes artifact type, and input-combination presence;
  2. one private copy of each raw artifact, raw byte ceilings, and JSON depth/token preflight,
     completed for every artifact before stage 3 parses any of them (Go evidence:
     `TestPreflightPrecedesStrictParse`);
  3. strict JSON UTF-8, duplicate-key, closed-schema, canonical-byte, exact-profile, ID-grammar,
     and self-ID checks for OCM, CEM, command, observation, every prior observation, and an
     optional TCQ result;
  4. inherited CEM 0.2/OCM dispatch and validation precedence, normatively defined by
     `cem-0.2-canonical-binding.md`, using the same CEM/OCM bytes and independent revisions;
  5. TCQ's local command target equality, target-tree cwd resolution, and remaining command
     validation;
  6. observation target, command ID, and declared report byte-count/digest equality against the
     copied report bytes, then every prior observation's target equality;
  7. JUnit byte/token safety preflight, disabled-capability parser construction, strict traversal,
     projection, report/command consistency, and observation-row equality;
  8. selected-edge association, hygiene, execution matching, and reference closure;
  9. TCQ output bounds, canonical encoding, expected-base/target binding, and TCQ ID.

  Inherited precedence remains normative; TCQ adds only command, observation, JUnit, association,
  and output ordering after it. Command target mismatch precedes cwd failure; observation target
  mismatch follows command checks.

  Inherited `expected-base-required`, `target-required`, `base-revision-mismatch`,
  `invalid-target-revision`, `target-mismatch`, CEM, OCM, repository, and Git failures retain their
  current redacted meanings. TCQ adds exactly `invalid-tcq-input`, `unsupported-cem-profile`,
  `unsupported-ocm-profile`, `unsupported-claim-extractor`, `invalid-command`,
  `noncanonical-command`, `command-target-mismatch`, `command-cwd-unavailable`, `invalid-observation`,
  `noncanonical-observation`, `observation-target-mismatch`, `observation-command-mismatch`,
  `report-digest-mismatch`, `invalid-junit`, `report-command-inconsistent`, `invalid-tcq`,
  `noncanonical-tcq`, and `tcq-resource-exhausted`. Error assignment is exact:

  | Failure | Code |
  |---|---|
  | forbidden input combination or non-bytes raw artifact | `invalid-tcq-input` |
  | unsupported but well-formed CEM, OCM, or extractor profile | its exact `unsupported-*` code |
  | malformed UTF-8/JSON, duplicate key, schema/type/range/ID-grammar error, extra/missing field, or content-address mismatch | `invalid-command`, `invalid-observation`, or `invalid-tcq` for that artifact |
  | otherwise valid artifact bytes not in the frozen sorted-key/minimal/terminal-LF encoding | the artifact's `noncanonical-*` code |
  | required/mismatched revision, command, cwd, report digest, or report/command relation | the exact named code above |
  | XML grammar or status violation | `invalid-junit` |
  | any exceeded TCQ-owned bound | `tcq-resource-exhausted` |

  Operational errors emit no partial artifact and no unverified path, OID, digest, selector, count,
  command argument, XML value, or derived identity.
- `TCQ-V0-043`: a TCQ result is a recomputable local cache, not an authority token or execution
  record. Cache verification requires independent expected-base and target inputs, exact raw
  canonical CEM and OCM, and target Git objects. Dynamic cache verification additionally requires
  the exact raw canonical command, observation, and JUnit report. Verification re-derives all
  source associations, profiles, units, rows, reasons, references, and IDs from those byte copies. A
  result, signature, command ID, or observation ID alone is insufficient. The retained command is
  inspectable and at best re-invocable; V0 cannot reproduce the original environment, filesystem,
  process state, dependencies, runner behavior, or execution outcome.
- `TCQ-V0-044`: every raw CEM, OCM, command, observation, report, and TCQ-result input is required to
  be one caller-supplied immutable bytes value. TCQ checks that artifact's byte ceiling, makes one
  private copy, and hashes, parses, compares, and traverses only that copy. A mutable buffer, path,
  path-like object, open file, descriptor, pipe, socket, iterator, callback, or blocking/asynchronous
  stream is invalid `invalid-tcq-input`; TCQ opens no raw-artifact filesystem path. Git target-tree
  reads remain exclusively inside the inherited CEM/OCM repository boundary.
- `TCQ-V0-045`: TCQ V0 is library and wire conformance only. It adds no `corvint tcq` CLI, raw-artifact
  path API, default artifact path, shell runner, next action, artifact persistence, daemon, database,
  or network service. A future CLI MAY reuse an existing separately hardened bounded reader and pass
  its immutable bytes into TCQ, but that reader and its filesystem authority remain outside the TCQ
  wire, verifier, error vocabulary, and V0 implementation scope.
- `TCQ-V0-046`: WP5 MUST expose `authorityClass: CALLER_REPORTED` without aliasing it to
  harness-observed, authenticated, or mechanically proved execution. Default frontier policy MUST
  NOT use this class to close an item. An explicit permissive policy MAY acknowledge it, but the
  caller-reported item remains visible, the frontier remains non-empty, and output must distinguish
  policy acknowledgement from closure. Only a future WP6 authority root and new relation/profile
  can define stronger behavior; signatures over V0 caller-controlled bytes do not qualify.
- `TCQ-V0-047`: any parser or runtime MAY implement `python-ast/1` when it implements the exact
  frozen Python 3.9 grammar and passes the shared golden-vector set before advertising support. No
  implementation name or runtime version is required or persisted. A parser unable to implement the
  grammar or source rejected by it abstains the affected Python edge as
  `unsupported-python-grammar`; Go processing remains available. Location mapping treats syntax-tree
  line numbers as one-based and column offsets as UTF-8 byte offsets into the original line; it
  preserves LF/CRLF source bytes and validates every name, docstring, body, and function boundary
  against an independent bounded 3.9 lexical-token scan. Missing/out-of-range positions, a location
  not on the expected token boundary, or any parser-dependent span difference abstains that edge as
  `python-offset-mismatch`. The golden set covers non-ASCII prefixes, LF and CRLF, tabs, decorators,
  multiline signatures, multiline docstrings, async functions, class methods, duplicate top-level
  and class-method names, nested functions, and trailing comments. Per edge, validation order is
  frozen grammar support/parse then offset/token validation; only the first applicable reason is
  emitted. Parser/runtime identity never enters unit identity.

### Environment variants and same-revision flake qualification

- `TCQ-V0-048`: an observation MAY declare the environment variant it ran under as one optional
  top-level member `environment`: a JSON object of at most 32 members whose keys match the ResultDB
  variant-key grammar `^[a-z][a-z0-9_]{0,63}$` and whose values are strings of at most 256 bytes.
  Any other shape is `invalid-observation`. TCQ never infers, reads, or persists a process
  environment; the member records only what the caller declared, and `TCQ-V0-024`'s omitted
  command environment policy is unchanged. An observation without the member has an unknown
  environment: TCQ reports it as undeclared through the library result and never as an empty
  variant. A declared variant enters the observation ID preimage (`TCQ-V0-031`) and is copied
  verbatim into the dynamic observation summary (`TCQ-V0-036`); an undeclared one adds no member,
  so a consumer that ignores unknown members reads a new document exactly as before and every
  frozen conformance vector stays byte-identical.
- `TCQ-V0-049`: one shared flake rule applies to every Corvint test provider: a test observed at
  one target revision under one environment variant is flaky when its terminal statuses across two
  or more runs contain more than one distinct value of `PASSED`, `FAILED`, and `ERROR`. `SKIPPED`
  never contributes, one status repeated never diverges, and a single run is never flaky. The
  rule is `tcq.Flaky`; a provider MAY map its own attempt vocabulary onto these statuses but MUST
  NOT substitute its own divergence rule, and a reporter's own flaky label MAY only add the
  qualification, never remove it, because missing attempt evidence upgrades nothing.
- `TCQ-V0-050`: a caller MAY supply at most 16 prior observations alongside the dynamic tuple.
  Each is copied, bounded, preflighted, and parsed exactly as the current observation; more than
  16 is `tcq-resource-exhausted`, any prior with the static combination is `invalid-tcq-input`,
  and a prior whose `targetRevision` differs from the resolved caller target is
  `observation-target-mismatch`. Priors are never re-derived from reports. A prior is comparable
  with the current observation only when their declared variants are byte-identical, two
  undeclared variants being equal and a declared variant never equalling an undeclared one. Per
  execution key, TCQ pools the row statuses of the current observation and every comparable prior;
  the key is flaky when at least two observations contribute rows to it and `TCQ-V0-049` holds.
  A claim whose execution key is flaky and whose own projection matched at least one row adds
  `test-flaky`, keeps the report state its own rows determine, and by `TCQ-V0-035` receives no
  relation. Priors only remove relations; they never add rows, change the observation summary, or
  enter the TCQ ID preimage other than through the reasons they add. `TCQ-V0-034` repeated rows
  inside one observation remain `AMBIGUOUS`, not flaky.

### Patch coverage witness

A test claim says a test exercises a hunk; a coverage witness records whether one named test run
reached the hunk's added lines. The witness is decision 0347
(`docs/decisions/0347-patch-coverage-witness-from-a-local-coverprofile-2026-09-22.md`): it is
ingested from one operator-named local coverprofile, never discovered or produced by Corvint, and
it qualifies a claim without proving adequacy or correctness.

- `TCQ-V0-051`: `corvint cem cover --map MAP --coverprofile PATH --test-run TEXT [--output PATH]`
  reads exactly one local coverprofile (Go's line format; `TCQ-V0-065`) named by the operator, bounded at the gorunner
  coverage bound, through the `internal/cem/coverprofile` parser (`Parse`); Corvint never
  discovers profiles, runs tests, or accepts a second parser. The witness records the profile's
  SHA-256 and the operator's `--test-run` identity (1..256 bytes, no control characters)
  verbatim; Corvint never derives, verifies, or normalizes either. A hunk path matches a profile
  path that equals it or ends in `/` plus it; two distinct matching profile paths (whatever their
  block counts, `TCQ-V0-097`), a malformed
  profile, an empty or oversized test-run identity, and a non-canonical (`cem/0.1`) map are each
  refused as `invalid-arguments` and leave the map unchanged.
- `TCQ-V0-052`: the witness is one optional hunk member `coverage` admitted only on `cem/0.3`:
  `{profileSha256, testRun, mode ∈ {set,count,atomic}, state ∈ {covered,uncovered},
  covered: [{start,count}...]}` with closed keys. `covered` ranges are one-based new-side lines,
  ascending, non-adjacent, `count ≥ 1`, and inside the hunk's `newRange`; `state` is `covered`
  exactly when `covered` is non-empty, and any disagreement is `invalid-field`. `cem/0.1` and
  `cem/0.2` reject the member as `unknown-field`, so a consumer that predates the witness reads
  every existing map exactly as before; `cover` declares `cem/0.3` on a canonical map the way a
  structural reason does (CEM-SM-006). (proposed, decision 0398) Like a structural `mark`, `cover` and
  `discriminate` never write that `cem/0.3` map over their `cem/0.2` input or to
  `.corvint/change.cem.json`; either output is refused as `invalid-arguments` before the map is
  written (and, for `discriminate`, before any mutant runs), leaving the input unchanged.
- `TCQ-V0-053`: `cover` writes a witness on every hunk of the map, never on a subset. The covered
  ranges are the hunk's added lines (`+` body lines, not context) intersected with the lines of
  profile blocks whose count is positive, folded into maximal ranges; a hunk with no such line
  carries `state: uncovered` with an empty `covered` array. An absent witness and an uncovered
  witness are distinct states, and neither is ever read as tested.
- `TCQ-V0-054`: the reviewer report (`corvint cem report`) lists every hunk whose basis cites a
  `test-claim` relation under `## Test claims`. A hunk whose witness is `covered` is `tested`
  with its test run and profile digest shown; a hunk with no witness is downgraded with reason
  `no-coverage-witness`, and a hunk whose witness is `uncovered` with reason
  `coverage-witness-uncovered`. The downgrade is rendering only: dispositions, counts, the
  worklist, and the `status`/`verify` envelopes are unchanged, and a map without test claims
  renders exactly as before.
- `TCQ-V0-065`: (experimental under the owner's standing lane auto-accept of 2026-10-11; V1-2798,
  GitHub #1895 ask 7) the coverprofile is a line format, not a Go-only input: `cover` MUST record
  the same witness for a hunk in any language whose line coverage the operator writes in that
  format (`mode: set|count|atomic`, then `PATH:L.C,L.C STATEMENTS COUNT` per span, `PATH` equal to
  the hunk path or ending in `/` plus it), and `cem cover --help` MUST say so. The witness stays
  coverage only. Non-goals: Corvint ships no per-language converter and adds no non-Go execution
  or hunk-revert witness; those need the proposed language-neutral runner contract below. Ruby
  discrimination is `TCQ-V0-066`. Failure modes: a converted profile
  whose paths match no hunk yields `uncovered` witnesses, never `covered`; a malformed line is the
  `TCQ-V0-051` refusal.
- `TCQ-V0-091`: (experimental under the owner's standing lane auto-accept; V1-2921, GitHub #1927)
  the `cover` result MUST distinguish a hunk whose path no profile block names (any count) from
  an instrumented hunk no positive block reaches: it adds `notInstrumented` (a count) and
  `notInstrumentedHunks` (the hunk IDs, in map order) beside the unchanged `covered` and
  `uncovered` counts, and `notInstrumented` counts a subset of `uncovered`. The map witness stays
  `state: uncovered` for both, because `TCQ-V0-052`'s closed state vocabulary is a frozen map wire
  contract; a `not-instrumented` map state needs a new map profile version, and the additive
  `cem-coverage-states/1` document (`CAX-V0-010..011`) records it beside the map. Path matching is the
  `TCQ-V0-051` rule. Non-goal: a converter from Ruby or JavaScript line-coverage JSON (the
  `TCQ-V0-065` non-goal stands). Failure mode: a reader that summed the result's members sees two
  more. Rollback: drop the two result members.
- `TCQ-V0-097`: (experimental under the owner's standing lane auto-accept of 2026-10-11; V1-2990,
  V1-2994) `cover` MUST keep each profile path's positive-count blocks as sorted, merged line
  intervals and intersect the hunk's added lines with them directly, never expanding a block line
  by line: the parser admits line numbers up to 4294967295, so a block such as
  `1.1,4294967295.2` would otherwise cost one step per line. The `TCQ-V0-051` ambiguity check MUST
  consider every profile path the profile names, including one whose blocks all have a zero
  count, so `a/pkg/a.go` with count 0 beside `b/pkg/a.go` with count 1 refuses hunk path
  `pkg/a.go` as ambiguous instead of silently taking the covered one. `cover` checks its context
  before each hunk and refuses `git-cancelled` once it is cancelled, leaving the map unchanged.
  The covered-range semantics of `TCQ-V0-053` and every unambiguous profile's witness are
  unchanged. Non-goals: no parser change, no new witness member, no change to path matching.
  Failure modes: a profile that previously resolved a hunk path only because the other matching
  path had no positive count now refuses `invalid-arguments` naming both paths; the operator
  removes the stray path or runs `cover` from a profile that names one. Rollback: restore the
  per-line set in `coveredLinesByPath` and the positive-count-only path registration.

### Mutation discrimination witness

A coverage witness says a test reached a hunk; a discrimination witness records whether the
cited tests notice when the hunk is changed. The witness is decision 0353
(`docs/decisions/0353-hunk-mutation-discriminates-witness-2026-09-22.md`): it reuses the
`prove --mutate` runner (`internal/liveverify/mutate`: `Open`, `Export.Judge` with `Complete`,
and the additive `Report.Survivors` export) on the map's changed hunks only, and it qualifies a
claim without proving adequacy or correctness. A hunk whose mutants all die is discriminated by
its tests; a surviving mutant is a change the cited tests cannot tell from the original.

- `TCQ-V0-055`: `corvint cem discriminate --map MAP --target REV [--max-hunks N]
  [--max-mutants N] [--wall-time DURATION] [--output PATH]` runs bounded mutation on at most
  `--max-hunks` hunks (default 8), at most `--max-mutants` mutants per hunk (default 8), inside
  one `--wall-time` budget of whole seconds (default `10m`) that covers the export and every run.
  A non-positive bound, a sub-second or fractional wall time, a missing `--target`, a
  non-canonical (`cem/0.1`) map, and a `--target` whose canonical patch from the map base does
  not carry the map's `patchSha256` are refused (`invalid-arguments`, or `patch-digest-mismatch`
  for the target) and leave the map unchanged with no mutant run. The run is pinned to the
  resolved `--target` object ID (`treeRevision`) and to `selectionSha256`, the SHA-256 of the
  sorted unique selected test paths each terminated by a newline; both are derived by Corvint,
  never operator-supplied.
- `TCQ-V0-056`: the witness is one optional hunk member `discriminates` admitted only on
  `cem/0.3`: `{treeRevision, selectionSha256, mutants, killed, survived, survivors:
  [{operator, line, description}...], bounds: {maxHunks, maxMutants, wallTimeSeconds}, state ∈
  {discriminates, survived, not-run}, detail}` with closed keys at every level. Counts are
  non-negative with `killed + survived ≤ mutants` (a mutant that did not compile is neither);
  `survivors` has exactly `survived` entries, each with a non-empty operator, a `line` inside the
  hunk's `newRange`, and a 1..512-byte description; bounds are positive; `detail` is at most
  512 printable bytes. `state` is `discriminates` exactly when `killed ≥ 1` and `survived = 0`,
  `survived` exactly when `survived ≥ 1`, and `not-run` exactly when `mutants = 0`; any
  disagreement is `invalid-field`. `cem/0.1` and `cem/0.2` reject the member as `unknown-field`;
  `discriminate` declares `cem/0.3` on a canonical map the way `cover` does.
- `TCQ-V0-057`: candidates are, in map order, the hunks whose path is Go source (not a
  `_test.go` file) and whose basis cites at least one `_test.go` file with the `test-claim`
  relation; the first `--max-hunks` candidates are selected. A selected hunk is mutated only on
  its `newRange` lines against each cited test file; a mutant survives only when every cited
  test lets it live. `discriminate` writes a witness on every hunk of the map: a hunk that is not
  a candidate, is past the hunk limit, adds no lines, ran out of wall time, or that the runner
  could not judge (no sandbox, no mutant, budget exceeded, runner failure) carries `state:
  not-run` with zero counts and a `detail` naming the reason. An absent witness and a `not-run`
  witness are distinct states, and neither is ever read as discriminated. Ruby source hunks citing
  `_spec.rb` files are candidates too (`TCQ-V0-066`).
- `TCQ-V0-058`: the reviewer report appends the witness to each `## Test claims` line: a
  `discriminates` witness adds `discriminates (killed K of M mutants)`, a `not-run` witness adds
  `mutation not-run (reason)`, and a `survived` witness downgrades the claim with reason
  `mutants-survived` and lists every surviving mutant as `operator at path:line`. A survived
  mutant outranks a missing or uncovered coverage witness as the downgrade reason. The downgrade
  is rendering only: dispositions, counts, the worklist, the exit status, and the
  `status`/`verify` envelopes are unchanged, so a surviving mutant never fails the build and is
  never silent.
- `TCQ-V0-059`: (accepted 2026-10-08, decision 0463; ticket V1-0651) a runner failure whose whole output is
  the macOS sandbox launcher's own refusal to apply its profile, exactly one line
  `sandbox-exec: sandbox_apply: ` followed by non-empty printable ASCII errno text and a newline,
  at most 160 bytes, as `/usr/bin/sandbox-exec` writes (exit 71) before `go test` starts when an
  enclosing sandbox forbids nesting, MUST surface as the fixed error
  `mutate.ErrSandboxUnavailable`, whose text is
  `mutate: sandbox unavailable: the host refused to apply the sandbox-exec profile`, so the
  witness `detail` names it instead of `mutate: run ended without a verdict`. Neither error
  echoes any output byte, including the errno text. Any other output, including the refusal
  line preceded or followed by other bytes or carried inside a test event, stays the generic
  error, so repository-controlled output can at most move a not-run reason between these two
  fixed texts and never reach a verdict. Only the error text changes: the witness stays
  `not-run` with zero counts, `prove --mutate` still exits 2 `unsupported-prove-mutation`
  with its fixed message, and every other not-run, downgrade, and verdict path is unchanged.
  Linux `bwrap` launcher failures are classified only as `TCQ-V0-064` states.
- `TCQ-V0-064`: (proposed 2026-10-24 under the owner's standing lane auto-accept of 2026-10-11;
  V1-2569) a runner failure whose whole output is one Linux `bwrap` setup refusal line, recorded
  on the linux/WSL2 runner box (bubblewrap 0.11.1), MUST surface as
  `mutate.ErrBubblewrapUnavailable`, whose fixed text is
  `mutate: sandbox unavailable: the host refused to set up the bwrap sandbox` and which
  `errors.Is` reports as `mutate.ErrSandboxUnavailable`. The admitted lines are
  `bwrap: Failed to make / slave: ` (what bwrap writes, exit 1, under test-confine's Landlock
  domain) and `bwrap: Creating new namespace failed: ` (inside an enclosing bwrap
  `--disable-userns`), each followed by non-empty printable ASCII reason text and a newline,
  under the same 160-byte whole-output bound as `TCQ-V0-059`. The same match drives the
  `FPK-V0-142` nesting preflight, so such a host reports every claim `UNSUPPORTED` with the fixed
  nesting reason once per export and runs no `go test`; since 2026-10-28 (V1-2940) the preflight
  retries one refusal once and appends the launcher's quoted line, because the
  `Creating new namespace failed: ` line is also what `bwrap` writes for a transient `EAGAIN`
  from `clone(2)` on a loaded host, which the per-run match cannot tell from a nesting refusal
  (a per-run transient refusal still surfaces as this error). Failure modes: any other `bwrap` line,
  such as `bwrap: execvp go: No such file or directory`, a refusal line with other bytes before
  or after it, an empty or non-printable reason, or an overlong output stays
  `mutate: run ended without a verdict`; no output byte is echoed; a refusal never becomes a
  verdict. The sandbox-exec text and every other path are unchanged. Acceptance:
  `TestTCQV0064InconclusiveNamesABubblewrapSetupRefusal`,
  `TestTCQV0064PreflightFindsABubblewrapRefusal`, and
  `TestDiscriminateRecordsWitnessAndReportDowngrades` skipping with the runner's own reason under
  test-confine (`-race`) on the linux/WSL2 box and passing there unconfined.
- `TCQ-V0-066`: (experimental under the owner's standing lane auto-accept of 2026-10-11; V1-2798,
  GitHub #1895 ask 7) `cem discriminate` MUST also admit, as `TCQ-V0-057` candidates, the hunks
  whose path is Ruby source (`.rb`, not `_spec.rb`) and whose basis cites at least one `_spec.rb`
  file with the `test-claim` relation, and MUST judge them on the same export with the existing
  RSpec arm of the `prove --mutate` runner (`internal/liveverify/rbmutate`, `FPK-V0-068..081`):
  `bundle exec rspec` beside a `Gemfile`, else `rspec` on `PATH`, under the same host sandbox. A
  mutant is killed when any cited spec file kills it and survives only when every cited spec
  file lets it live; a mutant that did not load, timed out, or whose report was not admitted and
  that no spec killed is neither, as an uncompilable Go mutant is. Every other `TCQ-V0-055..058`
  rule holds unchanged: the witness wire, its bounds, `selectionSha256` over the selected test
  paths, the not-run reasons (a runner that cannot start, a failing baseline or a spent budget is
  `not-run` with the runner's reason), and the rendering-only downgrade. A hunk that is neither
  Go nor Ruby source reads `not a Go or Ruby source file`; a Ruby hunk citing no spec file reads
  `no test claim cites a _spec.rb file`. Non-goals: no Python, JavaScript or other runner, no
  containerised RSpec profile (`FPK-V0-078`) for `discriminate`, no RSpec example selection, and
  no change to the `cem/1.0` wire or any vector. Failure modes: a host without `ruby`, `rspec` or
  a sandbox leaves Ruby hunks `not-run` with the reason; a spec file that does not pass at the
  target is `not-run` at the baseline stage, never a kill.

Measured cost (this host, macOS `sandbox-exec`, `TestDiscriminateRecordsWitnessAndReportDowngrades`
fixture: one Go module, one selected hunk, 4 mutants, `--max-mutants 6`, `--wall-time 5m`): one
bounded run took 5.7 s on a quiet host and 30.5–32.8 s while seven other agents were building and
testing on the same host; the run is dominated by one `go test` per mutant inside the sandbox.
The refusal path runs no mutant and completes in under a second per refusal.

### Language-neutral execution runner contract (GitHub #1895 asks 6 and 7)

Status: accepted by the owner on 2026-10-27 (V1-2798) and experimental under the owner's standing
lane auto-accept of 2026-10-11: `TCQ-V0-067..071`. Design entry
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-27-cem-adopter-chain-design.md`. The contract fixes the rules every
`cem discriminate` arm already had to follow, so the Go and Ruby arms share one fold and one set of
not-run reasons; it adds no language. Both open questions are decided: a runner is compiled into
the binary and never declared by the repository under review, and no second new language is
chosen in this step.

- `TCQ-V0-067`: (experimental under the owner's standing lane auto-accept of 2026-10-11; V1-2798,
  GitHub #1895 ask 7) Each `cem discriminate` runner MUST declare, for one language, its language
  name, the source-file suffix a candidate hunk needs, and the test-file suffix a cited or
  same-change test needs (`cemdiscriminate.Runners`: Go `.go` and `_test.go`, Ruby `.rb` and
  `_spec.rb`); a test file is never a candidate source. The workflow's candidate rule MUST agree
  with the declarations, so a path is a candidate exactly when a declared runner judges it and
  its tests carry that runner's suffix. Runners are compiled in; a repository cannot declare,
  replace or configure one, because it would then be repository-controlled code choosing its own
  verdict source. Non-goals: no Python, JavaScript or other runner, and no runner plugin or
  configuration surface. Failure mode: a path no runner judges is `not-run` with
  `no runner judges PATH`, as `TCQ-V0-066` already reads for the candidate rule.
- `TCQ-V0-068`: (experimental under the owner's standing lane auto-accept of 2026-10-11; V1-2798) A
  runner MUST reach a verdict for a mutant only from its own parsed run output (the Go arm's
  classified `go test` output, which separates a build failure from a failing test; the Ruby arm's
  admitted RSpec JSON report), each run executing in the existing host sandbox over the disposable
  export of the target (`TCQ-V0-059`, `TCQ-V0-064`). An exit status alone never kills: a mutant that
  does not build or load, a run that times out, and a report that is not admitted are neither killed
  nor survived. Failure mode: a run whose every mutant is neither is `not-run` with `no mutant
  compiled` (Go) or `no mutant loaded` (Ruby), never a kill.
- `TCQ-V0-069`: (experimental under the owner's standing lane auto-accept of 2026-10-11; V1-2798) A
  complete runner report MUST carry one outcome per planned mutant (`mutate.Report.Outcomes`,
  additive and read only by `discriminate`; `rbmutate.Report.Results`), and one fold MUST judge
  every arm: a mutant is killed when any selected test kills it, survives only when every selected
  test lets it live, and is otherwise neither. This corrects two earlier Go-arm behaviours: a mutant
  that did not compile against one test and survived another is no longer credited as killed, and
  one test whose mutants all fail to compile no longer leaves the hunk `not-run` when another test
  judges them.
- `TCQ-V0-070`: (experimental under the owner's standing lane auto-accept of 2026-10-11; V1-2798)
  The baseline MUST pass at the target before any mutant is judged, and a run that cannot complete
  MUST make the hunk `not-run` with its reason rather than a partial witness: a runner error
  (`mutation runner failed for TEST: ...`), a failed baseline, a spent budget or an empty plan (the
  runner's lowercased verdict and detail), a report without one outcome per mutant (`incomplete
  mutation report for TEST`), runs that plan different mutants (`runs disagree on the mutant plan`),
  and a survivor outside the hunk (`survivor at line N lies outside the hunk`). Every witness keeps
  the frozen `TCQ-V0-056` wire, its bounds and its state rules.
- `TCQ-V0-071`: (experimental under the owner's standing lane auto-accept of 2026-10-11; V1-2798,
  GitHub #1895 ask 6) `cem discriminate --same-change-test HUNK=PATH[,HUNK=PATH...]` MUST let a test
  file the change itself adds witness a hunk through an observed run only. HUNK names a candidate
  source hunk of the map; PATH is a repository-relative file with that hunk's runner's test suffix
  that the map shows the change adding lines to. The test joins only that hunk's selection, merged
  with its cited tests, and `selectionSha256` covers it. It is not evidence, is never cited, and
  changes no basis, disposition, support tier, count or exit status; the hunk is `discriminates`
  only when the run passes the test at the target and a mutant of the hunk fails it, and the witness
  `detail` begins `same-change test (execution only): PATH`. A named test that never runs (the hunk
  limit, the budget, a runner error) leaves the hunk `not-run`. Non-goals: no `cem/1.1`, no sidecar,
  no new wire member, and no citation of a target-only span. Failure modes: a malformed pair, an
  unknown hunk, a hunk no runner judges, a path of another language, a path that is not a test, a
  test the change does not add lines to, a path that escapes the repository and more than 64 pairs
  each refuse `invalid-arguments` before any run.
- `TCQ-V0-092`: (experimental under the owner's standing lane auto-accept; V1-2918, GitHub #1924)
  the exported copy every `discriminate` arm and `prove --mutate` run in MUST reproduce a tracked
  symlink (mode `120000`) whose link text cannot resolve outside the copy: a non-empty relative
  path of at most 4096 bytes with no NUL or backslash whose `..` components form one leading run
  no deeper than the link's directory. Symlinks are created after every regular file, and a
  changed (mutated) file that is itself a symlink is unsupported, so no file is ever written
  through a link. Any other symlink, an absolute one included, link text outside 1..4096 bytes,
  and a link the host cannot create (a name clash, or Windows without the privilege) still fail
  the export, and the refusal names the path and `sandbox-unpack-symlink`. When the runner cannot open, the
  `discriminate` result adds `runnerUnavailable`: `sandbox-unpack-symlink` for that refusal, else
  `runner-unavailable`; per-hunk witnesses stay `not-run` with their detail and the map wire is
  unchanged. Non-goals: following a link to read content outside the copy; a per-hunk reason
  member in the witness. Failure modes: a host that cannot create symlinks (Windows without the
  privilege) still fails the export, now named `sandbox-unpack-symlink`; on Windows a link
  whose target is a later directory link may be created as a file link and not resolve, which
  breaks the run but never escapes the copy; `runnerUnavailable` is classified from the refusal
  text, so any open failure whose message carries that code is reported under it; a test that
  depends on an escaping link stays unjudged. Rollback: restore the symlink refusal in
  `layoutEntry` and drop `runnerUnavailable`.
- `TCQ-V0-093`: (experimental under the standing lane auto-accept (GitHub #1942)) the hunk side
  of each `cem discriminate --same-change-test SELECTOR=PATH` pair MUST resolve exactly as
  `cem cite --hunk` and `cem mark --hunk` resolve their selector: a canonical one-based decimal
  ordinal into the map's hunk list, or a full derived hunk ID. An ordinal and the ID of the same
  hunk select the same run. The usage line and help name `SELECTOR=PATH`. Every other
  `TCQ-V0-071` rule is unchanged. Non-goals: no range or glob selector, no path selector, and no
  change to the witness wire or its detail. Failure modes: a non-canonical ordinal (`0`, `02`,
  `+1`), an ordinal past the hunk list and an unknown ID each refuse `invalid-arguments` naming the
  pair, before any run; an ordinal is positional, so it names a different hunk after the map's
  hunk list changes, exactly as it does for `cite`. Rollback: restore the ID-only lookup in
  `parseSameChangeTests` and the `HUNK=PATH` usage line; IDs keep working throughout.
- `TCQ-V0-094`: (experimental under the standing lane auto-accept (V1-2963, GitHub #1949)) the
  RSpec arm's runner probe MUST name its cause. A runner-stage detail keeps its FPK-V0-069 text
  and appends ` (CAUSE: probed `COMMAND --version`, exit status S)`, the command by base names and
  `S` the exit status or `none: REASON` when nothing ran. `CAUSE` is `runner-not-found` when
  `rspec` is not on `PATH`, the probe did not start, or it exited 126 or 127 inside the sandbox;
  `bundle-unavailable` when `bundle` is not on `PATH` beside a `Gemfile`, `bundle exec rspec
  --version` exits otherwise non-zero, or its gems are missing offline; and `runner-unavailable`
  for a present `rspec` that exits otherwise non-zero. `rbmutate.Report.Cause` carries it. An
  export whose launcher refused to nest (FPK-V0-142) still resolves the command on the host
  `PATH`, which needs no sandbox: an absent `rspec` or `bundle` reads `DETAIL; SANDBOX-REASON` with
  its cause, so a confined Linux host names the same cause as an unconfined one; a present
  command keeps the sandbox reason alone and nothing runs (amended 2026-10-28, GitHub #1949:
  under CI's Landlock confinement `bwrap` refuses to nest, and the runner reported only the
  sandbox reason with no cause). When
  any judged hunk's Ruby arm stops there, the `cem discriminate` result MUST add
  `runnerUnavailable` (the first such cause) and `runnerUnavailableHunks` (how many hunks it left
  `not-run`); a runner that could not open keeps TCQ-V0-092 and counts every selected hunk. `cem
  discriminate --help` lists the toolchain prerequisites. Non-goals: installing a toolchain; a
  wire change (the witness detail is the existing bounded string). Failure modes: a shim that
  exits 127 for another reason reads `runner-not-found`; a container runner names no cause.
  Rollback: drop `probeDetail`, `Report.Cause`, `Judge.RunnerUnavailable` and the two members.

Measured cost (linux/amd64 WSL2 box, go1.27.3, `TestTCQV0071SameChangeTestWitnessesByExecution`
fixture: one Go module, one judged hunk, `--max-mutants 6`): one bounded run took 18.8 s. The
contract's fold and refusal tests run no runner and complete in under a second.

### Guarding-test citation for the hunk-revert falsifier

Status: accepted; `TCQ-V0-060..062` under the owner's standing auto-accept (2026-10-11), V1-1266;
`TCQ-V0-063` under the same standing auto-accept, V1-1453. Decision 0523 P2. `TCQ-V0-057` selects only hunks that already cite a
`_test.go` span as `test-claim`, and before this step no dogfood map named a test that way, so the
consumer falsifier `cem verify --hunk-revert` (`CEM-V1-088..089` in `cem-stable-v1.md`) had
nothing to run.

- `TCQ-V0-060`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1266) The
  dogfood producer contract (`docs/DOGFOOD.md` section 4) MUST offer a "cite the guarding test"
  step: for a hunk of non-test Go source that a base test pins, add a citation-plan row
  `HUNK<TAB>PATH_test.go<TAB>START:END<TAB>test-claim` whose span contains that test's
  declaration header, from `func` through its parameter list. The step is offered, not forced: a plan without it is admitted unchanged, a
  hunk no test guards stays cited or `unknown` as before, and no producer, check or seal refuses
  or downgrades a map for lacking it.
- `TCQ-V0-061`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1266) The
  consumer MUST name a guarding test only from declarations the cited span holds: it parses the
  cited base file as Go and names each top-level `func TestX(t *testing.T)` declaration whose
  header, from `func` through its parameter list, lies inside the span and whose name `go test`
  runs (`Test` followed by nothing or by a rune that is not a lower-case letter), sorted, run in the
  cited file's package directory. Declaration-shaped text in a comment or a string is never a
  declaration, and a file that does not parse names none. A span that declares none, such as a
  helper, a comment that mentions a test or a subtest body, is reported `not-run` with reason
  `no-named-test`; no test is inferred from the file name, the rest of the file or the hunk.
- `TCQ-V0-062`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1266) The
  producer MUST be able to find the guarding test it cites by observation, not by guess:
  `cem verify --guarding-tests` adds the envelope member `guardingTests` (profile
  `cem-guarding-tests/0`, runner `go-test`) beside the unchanged `stableVerification` result, null
  unless the result is `ACCEPT`. It searches every hunk of non-test Go source that is not
  `mechanical` and whose basis cites no `_test.go` span as `test-claim`. A hunk's candidates are
  the top-level tests of the `TCQ-V0-061` shape declared at the base in the `_test.go` files of the
  hunk's own directory whose target bodies mention, by identifier name, a top-level declaration
  holding a line the hunk adds (for a pure deletion, the target line on either side), or a
  declaration of that directory, test helpers and other tests included, that transitively mentions
  one. Lines are physical lines; a `//line` directive moves neither a declaration nor a citation. The candidates run under the `CEM-V1-088`
  revert-and-run classification in a disposable export of the target: a guard is a candidate the
  search observed pass at the target and fail with only that hunk reverted, and the row is
  `guarded`; `unguarded` when every eligible candidate still passes reverted; otherwise `not-run`
  with its `CEM-V1-089` reason (a revert that breaks the build and a candidate that fails without
  the revert guard nothing), or `no-candidate-test` when a modification hunk has no candidate,
  even when the total bound is spent after every hunk was named (a bound spent while naming leaves
  the row `total-limit`, since a base read it cut short reads as an absent test). For a guarded row
  whose first guard by name has base header lines, `func` through the parameter list, that the
  frozen drift classifier keeps stable or relocated at the target, the member's `plan` carries one
  citation-plan row `HUNK_ID<TAB>PATH_test.go<TAB>START:END<TAB>test-claim` naming those base
  lines, which `cem cite` and the dogfood citation plan admit as written. The search writes
  nothing: no map, witness, support tier or count changes, `held` still comes only from the
  consumer's own `--hunk-revert` run (`CEM-V1-088`), and a printed row is guidance the producer may
  cite or drop (`TCQ-V0-060`). `status` and `report` refuse the flag `invalid-arguments`, and
  `verify` refuses it beside `--hunk-revert`, so one run never both suggests a test and credits it.
- `TCQ-V0-063`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1453) A
  `TCQ-V0-062` plan row MUST respect the `cem cite` span bound, one shared constant
  (`wire.MaxCitationSpanBytes`, 1 MiB) that `cem cite` and the search both read: a guard whose base
  header span exceeds it is skipped at emission and the next guard by name whose header fits and is
  stable or relocated is cited instead; with none, the row stays `guarded` with no citation and no
  plan row is printed. A header exactly at the bound is cited.

Recorded limits of `TCQ-V0-062`: candidates match identifier names, so they over-approximate and
the revert run decides; only tests in the hunk's own directory are candidates, so a test elsewhere
that exercises the hunk (for example in `cmd/corvint`) is never found; tests added by the change
are not candidates because `cem cite` needs base evidence. Only `go test` runs; no other runner is
supported or qualified (an rc.3 limit). The bounds are the falsifier's (300 seconds per run, 1200
seconds in total), and a package whose candidate run exceeds them is `not-run: timeout` or
`total-limit`; the first real change measured spent the whole 1200 seconds on a package with
394 candidates. Reverting one hunk of a change whose hunks depend on each other often breaks the
build, and such a hunk is never guarded. The search executes the target's tests unconfined with the caller's privileges,
exactly as `--hunk-revert` does, and like it creates the disposable export under the system
temporary directory through the shared helper that refuses one inside the repository (`CEM-V1-250`
in `cem-stable-v1.md`, V1-1452). The plan row respects the `cem cite` span bound (`TCQ-V0-063`,
V1-1453).

## Canonical conditional-state table

Every row below carries `authorityClass: CALLER_REPORTED`.

| Condition | Association fields | Hygiene | Report | Rows | Relation |
|---|---|---|---|---|---|
| unsupported/missing/ambiguous/unparseable association | supported anchor profile retained; executable identity null | `ABSTAINED` | `NOT_MATCHED` | empty | null |
| associated empty or unconditional skip | exact identity | `INELIGIBLE` | independently derived | exact matches | null |
| associated eligible, no observation/no keyed match | exact identity | `ELIGIBLE` | `NOT_MATCHED` | empty | null |
| duplicate target execution identity or repeated rows | exact identity | independently derived | `AMBIGUOUS` | empty for target ambiguity; all matches for repeated rows | null |
| one keyed skipped/failed/error row | exact identity | independently derived | exact row status | one | null |
| one passing row, nonzero command or target not attested clean | exact identity | independently derived | `PASSED` | one | null |
| matched rows whose execution key diverges across comparable observations (`TCQ-V0-050`) | exact identity | independently derived | own rows' status | own matches | null |
| every `TCQ-V0-035` condition | exact identity | `ELIGIBLE` | `PASSED` | one | `test-report-matched-v0` |

## Acceptance and adversarial matrix

Before implementation may claim experimental delivery, an independent conformance suite MUST cover
at least these genuine-pass/fabricated-fail/no-input classes:

| Class | Required adversarial fixtures |
|---|---|
| profile/binding | CEM 0.1, missing/mismatched expected base, missing/mismatched caller target, wrong command/observation target, noncanonical OCM, unknown extractor, spoofed anchor kind, selector drift |
| Python grammar/association | every advertised parser/runtime passes shared goldens; Python 3.9 syntax passes; 3.10-only `match` abstains; correct and forged non-ASCII/CRLF offsets; exact name/docstring; sibling laundering; nested/class function; two IDs; ambiguous span |
| Python execution collision | two top-level functions with one duplicate test name, only one selected plus one matching JUnit row, is `AMBIGUOUS`; two classes with one duplicate test-method name under the same conditions is also `AMBIGUOUS`; neither emits a relation or sibling claim/unit |
| Python hygiene | docstring-only, pass, constant, ellipsis, sole bare return, return value, call, each exact skip form, dynamic skip |
| Go association | exact TestX, exact parent/case runtime, duplicate case, escaped/dynamic/out-of-body case, comment/string lookalike, wrong signature |
| Go hygiene | empty, semicolon-only, sole bare return, first Skip/Skipf/SkipNow, later/dynamic skip, non-empty body |
| deferred languages | JS/TS descriptions, empty callback, comment/semicolon callback, sole bare return callback all abstain |
| raw input | immutable bytes at the exact cap; over-cap bytes; rejected bytearray, path/path-like, file, descriptor, pipe, iterator, callback, and blocking/asynchronous stream; one-copy hash/parse equality |
| command | shell string, empty/oversized or secret-bearing argv, no implicit argv output/export, hostile cwd, target-tree blob/symlink/gitlink/missing cwd, env entry, wrong target, invalid runner, forged clean-target statement remains caller-reported |
| XML grammar | size rejection before parser, DTD/ENTITY/PI preflight, external resolver/network/XInclude disabled, both roots, nested suites, unknown element at every depth, nested testcase, terminal grandchild, namespace, deep/wide/large input |
| row identity/status | hostile path, classname collision, absent file/name, every status cell, invalid combinations, keyed/unkeyed failure with exit zero |
| matching | zero/one/two identical rows, same name different path, duplicate target key, nonzero exit, not-attested clean target |
| environment variant | undeclared variant leaves observation bytes and summary unchanged and reads as unknown; declared variant changes the observation ID and is copied into the summary; bad key grammar, non-string value, and non-object member are `invalid-observation` |
| flake qualification | divergent current/prior outcomes at one target and variant add `test-flaky` and drop the relation while the current report state stands; same variant, other variant, declared-vs-undeclared, and both-undeclared comparisons; priors with the static combination, at another target, and beyond 16 |
| coverage witness | `cem/0.3` accepts a covered and an uncovered witness; `cem/0.1` and `cem/0.2` reject the member; disagreeing state, out-of-range, overlapping, adjacent ranges, bad digest, empty or control-character test run, bad mode or state, surplus key; cover of a covered added line, a profile reaching only a context line (uncovered, never absent), ambiguous profile path, malformed profile, empty test run; report shows `tested`, `no-coverage-witness`, `coverage-witness-uncovered` |
| discrimination witness | `cem/0.3` accepts a discriminates, a survived, and a not-run witness; `cem/0.1` and `cem/0.2` reject the member; state disagreeing with counts, counts exceeding mutants, survivor count disagreeing, survivor line outside `newRange`, empty operator, zero bound, bad state, bad revision or selection digest, control-character detail, surplus key at each level; a strong cited test kills every mutant (`discriminates`), the hunk limit leaves the next hunk `not-run` with its reason, a test asserting nothing lets mutants survive (`survived`, each survivor described) while `status` still succeeds; zero or negative bound, sub-second wall time, missing target, and a target off the map's patch are refused with the map unchanged; report shows `discriminates (killed K of M mutants)`, `mutation not-run (reason)`, and `mutants-survived` with every survivor |
| wire/cache | duplicate JSON keys, depth before parse, extra/missing fields, reordered rows, tampered IDs/digests, missing each raw cache-verification input, byte-identical fresh-process output |
| authority/policy | every edge is `CALLER_REPORTED`, signature does not upgrade, default frontier stays open, permissive acknowledgement remains visible and non-closing |
| privacy/boundary | source/XML/output canaries absent from artifacts and errors; sibling, worktree, alternate object, and denied-object canaries absent |

Conformance and reporter compatibility are separate gates:

- Deterministic conformance MUST pass every frozen fixture byte-for-byte with zero false matched
  relations and zero source-body or boundary leaks.
- An independently labelled corpus of at least 60 selected edges, at least 30 Python and 30 Go,
  MUST reach association precision at least 0.95 and runnable-shape precision at least 0.90.
  Semantic precision is not measured because WP4 makes no semantic classification.
- Each advertised reporter/version/configuration is evaluated separately on at least three real
  repositories. Report keyed-row rate, unique-match recall, repeated-row rate, and unkeyed-row rate.
  Advertise compatibility only at exact-match precision 1.00 and unique-match recall at least 0.80.
  Reporter results cannot compensate for a conformance failure.
- Dogfood the next ten eligible Corvint changes. Record selected edges, associated/abstained,
  eligible/ineligible, each report state and reason, exact matches, reviewer-invalid associations,
  authoring latency, artifact bytes, and source bytes excluded. Promote WP4 only with zero false
  relations, no privacy/boundary failure, at most 5% reviewer-invalid associations, and median
  authoring overhead below three minutes and 10% of task latency.

## YAGNI and rollback

V0 supports Python, Go, strict JUnit, one repository, one expected base, one target, an omitted
command environment with an optional declared observation variant, bounded prior observations for
the shared flake rule, one caller-reported relation, one operator-named Go coverprofile
ingested as a per-hunk witness (`TCQ-V0-051..054`), and one bounded Go mutation run per map
through the existing `prove --mutate` runner (`TCQ-V0-055..058`; its RSpec arm for Ruby hunks,
`TCQ-V0-066`), all arms under one compiled-in runner contract (`TCQ-V0-067..071`). It adds no
JavaScript/TypeScript
classifier, semantic floor, assertion detector, sibling extraction, retry deduplication within one
observation, flake history storage or retrieval, coverprofile discovery, test execution outside the
sandboxed mutation runner, non-Go coverage formats or mutation operators, a second mutator, new
mutation operators, mutation of test files or of unchanged lines, mutation-score thresholds,
statement- or branch-level coverage thresholds, coverage- or mutation-driven disposition, count,
or exit-status changes,
path or stream raw-artifact input, artifact persistence, shell execution, daemon, database, network,
UI, signature, authenticated harness, policy service, or portable promotion claim. WP6 owns any
harness-controlled/authenticated upgrade after WP4 signal is measured.

If association precision, compatibility, privacy, boundary, or overhead gates fail, remove the TCQ
producer and consumer while retaining OCM, CEM, source fixtures, and labelled evaluation data. Do
not weaken abstention or relabel caller reports as proof. `TCQ-V0-048..050` roll back on their own
by dropping `Request.PriorObservations`, the `environment` member, and reason 18; no frozen vector
changes because none carries either. `TCQ-V0-051..054` roll back by removing the `cem cover`
action, the `coverage` hunk member from the `cem/0.3` validator, and the `## Test claims` report
section; every map written without `cover` is unchanged, and a map that carries a witness fails
closed as `unknown-field` rather than being read as tested. `TCQ-V0-055..058` roll back by
removing the `cem discriminate` action, the `discriminates` hunk member from the `cem/0.3`
validator, and the mutation note from the `## Test claims` lines; `prove --mutate` output is
unchanged by their presence or removal because the runner's additive `Report.Survivors` field is
read only by `discriminate`, every map written without `discriminate` is unchanged, and a map
that carries the member fails closed as `unknown-field` rather than being read as discriminated. `TCQ-V0-059`
rolls back by removing `ErrSandboxUnavailable` and `launcherRefused`; the refusal then reads as
`mutate: run ended without a verdict` again and no wire, state, or exit status changes. `TCQ-V0-060..061`
roll back with the V1-1266 code commit: the `DOGFOOD.md` paragraph goes, and a plan that cites a
guarding test stays an ordinary `test-claim` citation. `TCQ-V0-062` rolls back by removing the `--guarding-tests`
flag, `internal/cem/verify/guards.go` and its `DOGFOOD.md` command; no committed map, seal, golden or
public case depends on them, and a row already cited stays an ordinary `test-claim` citation. `TCQ-V0-063`
rolls back with the V1-1453 code commit: the search again prints a row over the bound, which
`cem cite` refuses.
`TCQ-V0-064` rolls back by removing the two `bwrap` entries from `launcherRefusals` and
`ErrBubblewrapUnavailable`; a `bwrap` refusal then reads as `mutate: run ended without a verdict`
again and no wire, state, or exit status changes.
`TCQ-V0-065` rolls back by restoring the Go-only `cem cover` help wording; the parser and the
wire never changed, so no map, seal or golden depends on it.
`TCQ-V0-066` rolls back by restoring the Go-only candidate rule in
`internal/cem/workflow/discriminate.go`, removing the Ruby arm (`rubyRun`) from
`internal/cemdiscriminate/cemdiscriminate.go` and the help sentence; Ruby hunks then read
`not-run` with `not a Go source file` again, and no wire, map, seal or golden depends on it.
`TCQ-V0-067..070` roll back by restoring the per-arm folds in
`internal/cemdiscriminate/cemdiscriminate.go` (the Go fold over `Report.Survivors`, the Ruby fold
over `Results`) and removing `Runners`, `RunnerFor` and `mutate.Report.Outcomes`; `prove --mutate`
output never reads `Outcomes`, and no wire, map, seal or golden depends on them. `TCQ-V0-071` rolls
back by removing `--same-change-test` from `cem discriminate`, `parseSameChangeTests` and
`sameChangeDetail`; a witness already written keeps its frozen shape and its detail stays plain
text.

## Traceability

The `src/context_corvint_test_claim*.py` citations below are historical implementation references,
not live authority: decision 0012 R0
(`docs/decisions/0012-expert-panel-ratifications-2026-09-01.md`) rules the Python implementation
non-authoritative and slated for separate removal.

| Requirement | Planned implementation | Planned evidence |
|---|---|---|
| `TCQ-V0-001..012` | `src/context_corvint_test_claim.py` | profile, binding, association, and abstention tests |
| `TCQ-V0-013..022` | `src/context_corvint_test_claim.py` | Python/Go parser, hygiene, identity, and collision tests |
| `TCQ-V0-023..032` | `src/context_corvint_test_claim_junit.py` | canonical command, strict JUnit, observation, tamper, and privacy tests |
| `TCQ-V0-033..043` | both modules | matching, wire, cache verification, precedence, bounds, and conformance vectors |
| `TCQ-V0-044..047` | both modules | bounded byte inputs, library-only surface, caller authority, and Python 3.9 grammar tests |
| `TCQ-V0-048` | `internal/tcq/observation.go`, `internal/tcq/encode.go` | `TestObservationEnvironmentIsAdditive` |
| `TCQ-V0-049` | `internal/tcq/flake.go`, `internal/jstestprovider/projection.go`, `internal/jstestprovider/playwright.go` | `TestFlakyRuleNeedsDivergentTerminalOutcomes`, `TestFlakyOutcomeIsSharedRule`, `TestParsePlaywrightJSON_MixedStates` |
| `TCQ-V0-050` | `internal/tcq/evaluate.go`, `internal/tcq/assemble.go` | `TestSameRevisionDivergentOutcomesAreFlaky`, `TestPriorObservationVariantMismatchIsNotFlaky`, `TestPriorObservationsRequireDynamicTupleAndTarget`, `TestReasonVocabularyMatchesFrontierSeam` |
| `TCQ-V0-051` | `internal/cem/workflow/cover.go`, `internal/cem/cli/cli.go`, `internal/cem/coverprofile/coverprofile.go`, `internal/liveverify/gorunner/coverage.go` | `TestCoverRefusesAmbiguousAndInvalidInputs`, `TestParseCoverProfileExportsBlocks` |
| `TCQ-V0-052` | `internal/cem/wire/map.go`, `internal/cem/workflow/commands.go` `checkSpec03Output` | `TestInheritedCoverageWitness` (since V1-1278 S2b the witness is inherited by `cem/1.0`; the `cem/0.3` upgrade and its test are retired by V1-1278 S2b (`CEM-V1-215`)) |
| `TCQ-V0-053` | `internal/cem/workflow/cover.go`, `internal/cem/workflow/workflow.go` | `TestCoverRecordsCoverageWitnessAndReportDowngrades` |
| `TCQ-V0-054` | `internal/cem/workflow/read.go` | `TestCoverRecordsCoverageWitnessAndReportDowngrades` |
| `TCQ-V0-055` | `internal/cem/workflow/discriminate.go`, `internal/cem/cli/cli.go`, `internal/cemdiscriminate/cemdiscriminate.go`, `cmd/corvint/cem_discriminate.go` | `TestDiscriminateRefusesInvalidBoundsAndTarget`, `TestDiscriminateRecordsWitnessAndReportDowngrades`, `TestDiscriminateWithoutRunnerIsNotRun` |
| `TCQ-V0-056` | `internal/cem/wire/discriminate.go`, `internal/cem/wire/map.go`, `internal/cemdiscriminate/cemdiscriminate.go` | `TestInheritedDiscriminationWitness` (since V1-1278 S2b the witness is inherited by `cem/1.0`, `CEM-V1-215`) |
| `TCQ-V0-057` | `internal/cem/workflow/discriminate.go`, `internal/cem/workflow/workflow.go`, `internal/cemdiscriminate/cemdiscriminate.go`, `internal/liveverify/mutate/mutate.go` | `TestDiscriminateRecordsWitnessAndReportDowngrades` |
| `TCQ-V0-058` | `internal/cem/workflow/read.go`, `internal/cemdiscriminate/cemdiscriminate.go` | `TestDiscriminateRecordsWitnessAndReportDowngrades` |
| `TCQ-V0-059` | `internal/liveverify/mutate/gotest.go` (`inconclusive`, `launcherRefused`, `ErrSandboxUnavailable`) | `TestInconclusiveNamesALauncherRefusalWithoutEchoingIt`, `TestRunReportsASandboxLauncherRefusalAsUnavailable`, `TestInconclusiveDoesNotExposeTestOutput`, `TestRunRefusesToJudgeARunWithoutAVerdict`; measured 2026-10-08 inside a deny-default outer `sandbox-exec` profile: `TestDiscriminateRecordsWitnessAndReportDowngrades` witness detail `mutation runner failed for pkg/calc/calc_test.go: mutate: sandbox unavailable: the host refused to apply the sandbox-exec profile` |
| `TCQ-V0-060` | `docs/DOGFOOD.md` section 4 "Cite the guarding test" | `TestCEMV1088HeldOnlyFromConsumerReexecution` (a plan citing the guarding test's declaration line yields `held`) |
| `TCQ-V0-061` | `declaredTests` and `revertCandidates` in `internal/cem/verify/hunkrevert.go` | `TestCEMV1088DeclaredTestsAreWhatGoTestRuns`, `TestCEMV1088ForgedWitnessNeverCounts` (a span declaring no test is `no-named-test`) |
| `TCQ-V0-062` | `internal/cem/verify/guards.go`, `internal/cem/workflow/stable_read.go`, `internal/cem/cli/cli.go` | `TestTCQV0062GuardingTestsNameObservedGuards`, `TestTCQV0062FlagIsVerifyOnly` (`internal/cem/workflow`); `TestTCQV0062VerifyGuardingTestsIsOptIn` (`internal/cem/cli`); `TestTCQV0062CandidatesFollowThePackageClosure`, `TestTCQV0062CitationNeedsAStableBaseHeader`, `TestTCQV0062LineDirectivesKeepPhysicalLines`, `TestTCQV0062ChangedLinesFindTheEditedDeclaration`, `TestTCQV0062SpentBudgetKeepsNoCandidateReason` (`internal/cem/verify`): on an uncited prepared map the search guards the Clamp hunk with its one asserting test only, keeps a revert that breaks the build and a test failing without the revert `not-run`, prints the plan row `cem cite` admits, and the consumer then holds the hunk |
| `TCQ-V0-063` | `goPackage.citation` in `internal/cem/verify/guards.go`; `wire.MaxCitationSpanBytes` read by `cem cite` in `internal/cem/workflow/commands.go` | `TestTCQV0063PlanRowRespectsTheCiteSpanBound` (`internal/cem/verify`): a guard header of exactly 1 MiB is cited, one byte over is skipped and the next guard by name is cited |
| `TCQ-V0-064` | `internal/liveverify/mutate/gotest.go` (`launcherRefusals`, `launcherRefused`, `ErrBubblewrapUnavailable`); `internal/liveverify/mutate/sandbox.go` (`nestingRefused`, `preflightRefused`) | `TestTCQV0064InconclusiveNamesABubblewrapSetupRefusal`, `TestTCQV0064PreflightFindsABubblewrapRefusal` (`internal/liveverify/mutate`); `TestFPKV0142PreflightRetriesATransientRefusalOnce` and `TestFPKV0142NestingRefusalCarriesTheLauncherLine` (V1-2940: a transient `EAGAIN` line retried once, the persisting line carried); measured 2026-10-24 on the linux/WSL2 box under test-confine `-race`: `TestDiscriminateRecordsWitnessAndReportDowngrades` SKIP with detail `unsupported: cited tests cannot be sandboxed: the host refused to apply the sandbox profile (an enclosing sandbox forbids nesting)`, and PASS unconfined |
| `TCQ-V0-092` | `layoutTree`, `writeBlobs`, `readLink`, `linkEscape`, `SymlinkRefusal` in `internal/liveverify/mutate/export.go`; `unlinkedRegularFile` in `mutate/gotest.go` and `rbmutate/rbmutate.go`; `runnerUnavailable` in `internal/cem/workflow/discriminate.go` | `TestOpenRefusesATreeEntryItCannotReproduce`, `TestLinkEscapeAdmitsOnlyInTreeTargets` (`internal/liveverify/mutate`); `TestDiscriminateNamesAnEscapingSymlinkRefusal`, `TestDiscriminateWithoutRunnerIsNotRun` (`internal/cem/workflow`); `TestTCQV0092SymlinkRefusalIsTheWorkflowReason` (`internal/cemdiscriminate`) |
| `TCQ-V0-093` | `parseSameChangeTests` over `resolveHunk` in `internal/cem/workflow/discriminate.go`; `cem discriminate` usage and help in `cmd/corvint/help.go` | `TestTCQV0093SameChangeTestAcceptsTheCiteSelector` (`internal/cem/workflow`): ordinals and IDs select the same hunks; non-canonical and out-of-range ordinals refuse `invalid-arguments` |
| `TCQ-V0-094` | `probeDetail`, `runnerCause`, `resolveRunner` and `findRunner` in `internal/liveverify/rbmutate/rspec.go`; `Report.Cause` and `judge` in `rbmutate.go`; `Judge.RunnerUnavailable` in `internal/cemdiscriminate/cemdiscriminate.go`; `judgeHunks` in `internal/cem/workflow/discriminate.go`; `cem discriminate` help | `TestTCQV0094RunnerProbeNamesTheCause`, `TestTCQV0094UnsandboxedCopyStillNamesAnAbsentRunner`, `TestFPKV0069RunnerUnavailableIsUnsupportedAtTheRunnerStage` (`internal/liveverify/rbmutate`); `TestDiscriminateNamesAnAbsentRSpec` (`internal/cem/workflow`): with rspec absent the hunk detail names `runner-not-found`, the probe and its status, and the result names `runnerUnavailable` with `runnerUnavailableHunks: 1` |
| `TCQ-V0-097` | `coveredIntervalsByPath`, `profileLinesFor`, `coveredRanges` and the per-hunk context check in `internal/cem/workflow/cover.go` | `TestTCQV0097CoverKeepsCoveredLinesAsIntervals`, `TestTCQV0097CoveredRangesIntersectMergedIntervals`, `TestCoverRefusesAmbiguousAndInvalidInputs` (`ambiguous-zero-count`) (`internal/cem/workflow`): a `1.1,4294967295.2` block covers added line 2 in under 2 s; a cancelled context refuses; merged intervals reproduce the line-set ranges; a zero-count matching path refuses as ambiguous |
| `TCQ-V0-091` | `profilePaths`, `profileNames` and the `notInstrumented` result in `internal/cem/workflow/cover.go`; `cem cover` help | `TestCoverRecordsCoverageWitnessAndReportDowngrades` (`internal/cem/workflow`): a profile naming another file yields `uncovered` in the map and `notInstrumented: 1`; a zero-count instrumented hunk yields `notInstrumented: 0` |
| `TCQ-V0-065` | `cem cover` help in `cmd/corvint/help.go`; the unchanged `internal/cem/coverprofile` parser and `profileLinesFor` in `internal/cem/workflow/cover.go` | `TestTCQV0065CoverAcceptsANonGoLineProfile` (`internal/cem/workflow`): a Ruby hunk gets a `covered` witness from a converted `mode: set` profile and `uncovered` from a zero count |
| `TCQ-V0-066` | `runnerTestSuffix`, `claimedTests` and `unjudgedReason` in `internal/cem/workflow/discriminate.go`; `rubyRun` and `foldRuns` in `internal/cemdiscriminate/cemdiscriminate.go`; `cem discriminate` help in `cmd/corvint/help.go` | `TestTCQV0066DiscriminationCandidatesAdmitRubySpecClaims` (`internal/cem/workflow`): a Ruby hunk citing `_spec.rb` files is a candidate with only those, Go keeps `_test.go`, and the not-run reasons name the missed rule; `TestTCQV0066JudgesARubyHunkThroughTheRSpecArm` (`internal/cemdiscriminate`): a band-edge spec discriminates, a weak spec leaves survivors on the hunk line, both cited together discriminate; it skips without a sandbox or `ruby` |
| `TCQ-V0-067` | `Runner`, `Runners` and `RunnerFor` in `internal/cemdiscriminate/cemdiscriminate.go`; `runnerTestSuffix` in `internal/cem/workflow/discriminate.go` | `TestTCQV0067RunnersDeclareSourceAndTestRules` (`internal/cemdiscriminate`): exactly the Go and Ruby arms are declared, each complete, and test files and other languages are not judged; `TestTCQV0067CandidateRuleAgreesWithTheRunnerDeclarations` (`internal/cem/workflow`): the candidate rule and the declarations agree path by path |
| `TCQ-V0-068` | `goRun` and `rubyRun` in `internal/cemdiscriminate/cemdiscriminate.go`; `classifyRun` in `internal/liveverify/mutate/gotest.go`; the RSpec report admission in `internal/liveverify/rbmutate` | `TestTCQV0068OnlyAParsedKillCounts` (`internal/cemdiscriminate`, fixture reports, no Ruby or `go test` needed): unadmitted, timed-out, unloadable and uncompilable mutants are neither and leave the hunk `not-run`; the existing `TestClassifyRunSeparatesABuildFailureFromARedTest`, `TestRunNeverCreditsAMutantThatDoesNotCompile` (`internal/liveverify/mutate`) and `TestFPKV0069UnloadableMutantIsNeverCredited`, `TestFPKV0080UnadmittedReportEarnsNoVerdict` (`internal/liveverify/rbmutate`) |
| `TCQ-V0-069` | `Report.Outcomes`, `MutantOutcome` and `judgeMutants` in `internal/liveverify/mutate/mutate.go`; `claim.credit` in `internal/liveverify/mutate/group.go`; `foldRuns` in `internal/cemdiscriminate/cemdiscriminate.go` | `TestTCQV0069FoldKillsOnAnySurvivesOnAll` (`internal/cemdiscriminate`): for both arms one fold kills on any test, survives only on all, and never credits a mutant that did not build or load in one run and lived in the other; `TestRunReportsSurvivedWhenTheTestAssertsNothing` and `TestRunNeverCreditsAMutantThatDoesNotCompile` (`internal/liveverify/mutate`) check one outcome per mutant |
| `TCQ-V0-070` | `goRun`, `rubyRun` and `foldRuns` in `internal/cemdiscriminate/cemdiscriminate.go`; the runners' baseline checks | `TestTCQV0070IncompleteRunsAreNotRun` (`internal/cemdiscriminate`): a runner error, a failed baseline, a spent budget, an empty plan, a short report, disagreeing plans, an out-of-hunk survivor and no run each give a `not-run` witness of the frozen shape; the existing `TestRunReportsUnsupportedWhenTheBaselineFails` (`internal/liveverify/mutate`) and `TestFPKV0069BaselineMustPass` (`internal/liveverify/rbmutate`) |
| `TCQ-V0-071` | `--same-change-test` in `internal/cem/cli/cli.go`; `parseSameChangeTests`, `discriminationCandidates` and `sameChangeDetail` in `internal/cem/workflow/discriminate.go`; `cem discriminate` help in `cmd/corvint/help.go` | `TestTCQV0071SameChangeTestsJoinTheirHunkOnly`, `TestTCQV0071SameChangeTestRefusals`, `TestTCQV0071SameChangeTestWitnessesByExecution` (`internal/cem/workflow`): a named test joins only its hunk and the selection digest; each refusal is `invalid-arguments`; on a real sandboxed run a test the change adds kills a mutant of its hunk and the witness detail names it, while the hunk the limit leaves unjudged stays `not-run`; measured PASS 2026-10-27 on the linux/WSL2 box (go1.27.3), 18.8 s |

The implementation and deterministic reference vectors are delivered as a candidate. The labelled
corpus, reporter compatibility measurements, independent implementation, and ten-change dogfood
gate remain `NOT_RUN`; no promotion or compatibility claim is made.
