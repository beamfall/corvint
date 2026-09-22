# Test Claim Qualification V0

Owner: Russell Lewis
Frozen: 2026-08-23
Intent status: proposed
Delivery status: implementation-candidate; promotion evidence NOT_RUN
Freeze note: the 60-edge labelled corpus and reporter-compatibility runs are deferred until a WP6
authority root is specified; absent that root, the corpus would measure a relation nobody may act on.
Authoritative inputs: `docs/specs/ocm-v0-dogfood.md`,
`docs/specs/cem-0.2-canonical-binding.md`, `docs/specs/change-frontier-v0.md`,
`docs/PRODUCT.md`, `docs/BUILD-LOG.md`

## Agent digest
- Claim: TCQ deterministically qualifies selected OCM test anchors and caller-supplied JUnit matches without proving adequacy or correctness.
- Status: proposed/implementation-candidate; promotion evidence NOT_RUN
- Exists: deterministic reference implementation and vectors for selected OCM claim qualification.
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
  its target, command ID, command exit code, and deterministic rows;
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

  All fields are required and no others are allowed. `exitCode` is `0..2147483647`. Rows sort by
  `(executionKeySha256,id)` and are strictly unique. Observation ID is `test-observation:sha256:`
  plus `SHA-256(UTF8("corvint-test-observation/0.1-experimental") || 0x00 ||
  canonical-json-value(observation without id))`.
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

  No other conditional shape is valid.
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

  Association abstention uses exactly its one applicable association reason. Associated claims add
  all applicable hygiene reasons. Report projection adds exactly one of: no dynamic input or zero
  keyed matches gives `test-not-matched` (except that any unkeyed rows substitute
  `row-identity-unavailable`); repeated matches give `repeated-test-rows`; a unique non-passing row
  gives its exact status reason; a unique passing row gives neither. A unique passing row also adds
  `target-cleanliness-not-attested` and/or `command-failed` when applicable. Reasons are unique and
  sorted above.
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
     and self-ID checks for OCM, CEM, command, observation, and an optional TCQ result;
  4. inherited CEM 0.2/OCM dispatch and validation precedence, normatively defined by
     `cem-0.2-canonical-binding.md`, using the same CEM/OCM bytes and independent revisions;
  5. TCQ's local command target equality, target-tree cwd resolution, and remaining command
     validation;
  6. observation target, command ID, and declared report byte-count/digest equality against the
     copied report bytes;
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

V0 supports Python, Go, strict JUnit, one repository, one expected base, one target, omitted
environment, and one caller-reported relation. It adds no JavaScript/TypeScript classifier,
semantic floor, assertion detector, sibling extraction, retry deduplication, coverage ingestion,
path or stream raw-artifact input, artifact persistence, shell execution, daemon, database, network,
UI, signature, authenticated harness, policy service, or portable promotion claim. WP6 owns any
harness-controlled/authenticated upgrade after WP4 signal is measured.

If association precision, compatibility, privacy, boundary, or overhead gates fail, remove the TCQ
producer and consumer while retaining OCM, CEM, source fixtures, and labelled evaluation data. Do
not weaken abstention or relabel caller reports as proof.

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

The implementation and deterministic reference vectors are delivered as a candidate. The labelled
corpus, reporter compatibility measurements, independent implementation, and ten-change dogfood
gate remain `NOT_RUN`; no promotion or compatibility claim is made.
