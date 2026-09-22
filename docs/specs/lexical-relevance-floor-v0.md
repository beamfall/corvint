# Lexical Relevance Floor V0

Owner: Russell Lewis
Frozen: 2026-08-22
Amended: 2026-08-30 (30-case conformance plan alignment)
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `docs/CHANGE-EVIDENCE-MAP.md`, `docs/specs/ocm-v0-dogfood.md`,
`docs/specs/change-frontier-v0.md`

## Agent digest
- Claim: Lexical Relevance Floor qualifies bounded lexical proximity without treating it as semantic evidence or obligation closure.
- Status: proposed/experimental
- Exists: `internal/lrf`, `internal/lrfrepo`, and `conformance/lrf-v0` experimental surfaces.
- Blocked on: the frozen 41-case qualification plan (`conformance/lrf-v0/cases.json`) and independent evidence beyond structural self-use.
- Read next: User and job; Verified current state; Definitions.

### Status-marker classification (2026-09-12)

All 23 original no-run-status occurrences in this document are category (c): they define the evaluator's
closed status vocabulary and the fail-closed result required for missing, invalid, disclosed, empty,
or unrecomputable evaluation evidence. None claims that a local command was skipped. The
requirement-ID and named-test search finds the experimental `internal/lrf` conformance tests, but a
passing conformance test preserves these no-run results rather than replacing them with `PASS`.
The sealed held-out and prospective value experiments also require independent labels and
preregistration artifacts that are intentionally absent from this worktree.

## User and job

Before Corvint presents a structurally valid CEM or OCM link for later verification, a maintainer
needs a cheap deterministic reason to reject links that are obviously unrelated, self-referential,
oversized, or circular without adding an LLM judgment.

Lexical Relevance Floor V0 is a necessary candidate filter over already verified CEM and OCM
artifacts. Bare lexical overlap creates only a **lexically-proximate candidate**. For the narrow CEM
hunk-evidence obligation only, V0 composes that overlap with structural verification and an allowed
declared CEM basis relation as `cem-lexical-v0`. That composed relation is a qualifying CEM witness;
it says only that the hunk has a narrow, external, typed basis. It does not establish semantic
entailment, causality, correctness, claim adequacy, test execution, authorship, or product value.
WP4 may add only the caller-reported, non-closing `test-report-matched-v0` qualification relation;
it closes neither an OCM obligation nor the frontier. WP5 may later compose the bounded relations
while preserving that non-closing authority class.
Rejection or abstention preserves a visible unknown.

## Verified current state

- `internal/lrf`, `internal/lrfrepo`, and `conformance/lrf-v0` exist as experimental runtime and
  conformance surfaces. The legacy Python traceability rows below are historical and do not describe
  the current implementation.

- CEM 0.1 verifies Git identity, exact spans, complete hunk disposition, and drift, but `supported`
  currently means only that a producer attached structurally valid evidence.
- OCM V0 independently verifies CEM structure and exact obligation IDs in claim anchors, but an
  exact ID and any supported hunk can currently produce `linked` without checking requirement text
  against the hunk.
- Claim extraction and hygiene are producer behavior, not lexical relevance. Exact-ID anchors are
  useful structural candidates but cannot upgrade a result in this profile.
- The current Corvint dogfood OCM reports 12/12 obligations linked by twelve claims and two broadly
  reused hunks. That result is structural self-use evidence, not relevance evidence, and is the
  mandatory fabricated-fail regression for this slice.

## Definitions

- **term**: one normalized identifier produced by the frozen algorithm below;
- **proximate basis**: one structurally valid CEM basis whose bounded external evidence shares at
  least one term with the hunk's added bytes or final basename stem;
- **`cem-lexical-v0`**: the V0 qualifying relation for one CEM hunk-evidence edge. It requires a
  structurally valid `supported` hunk, a structurally valid basis carrying one of CEM 0.1's allowed
  declared relations, and that basis passing every applicable LRF narrowness, externality, and
  lexical rule. It witnesses only the existence of that typed proximate CEM basis;
- **lexically-proximate candidate**: an OCM obligation with at least one independently proximate
  permitted material hunk. It is not a witness;
- **material hunk**: a textual hunk already witnessed by `cem-lexical-v0` whose normalized path is
  neither the exact intent path nor any exact referenced claim path, and which is not a deletion;
- **abstained**: an edge belongs to a deletion V0 cannot evaluate or its own admitted lexical
  subject exceeds the retained-term cap; it stays unknown, is not treated as rejected evidence of
  irrelevance, and does not erase unrelated subjects;
- **rejected**: the bounded inputs were evaluable but did not meet the lexical floor;
- **witness**: in WP3, only a CEM hunk-evidence edge carrying `cem-lexical-v0`. This cannot witness
  or close an OCM obligation, claim, test, or frontier.
- **corpus subject**: an immutable CEM patch hunk or intent obligation enumerated before any
  evidence map is produced. It is an evaluation unit, not a new LRF wire row.

## Frozen identifier algorithm

The algorithm is byte-oriented and locale-independent:

1. Inspect only bytes admitted by the CEM or OCM bounds. Find maximal ASCII identifier runs
   matching `[A-Za-z][A-Za-z0-9_]*`. Non-ASCII bytes and punctuation are separators.
2. Split a run at `_`, at each letter/digit boundary, before an uppercase letter preceded by a
   lowercase letter, and before the final uppercase letter of an uppercase run followed by a
   lowercase letter. Thus `HTTPServer_v2` produces `HTTP`, `Server`, `v`, and `2` before filtering.
3. Lowercase with ASCII rules. Retain only segments containing a letter and at least four ASCII
   characters. Deduplicate into an unordered set before comparison.
4. Before tokenization, replace each complete byte sequence matching
   `(?:hunk|evidence|claim):sha256:[0-9a-f]{64}` with one ASCII space only when both ends are the
   input boundary or an ASCII byte outside `[A-Za-z0-9_]`; boundary bytes are retained. Then, in
   that result, scan left to right: at each byte that is the input start or follows a byte outside
   `[A-Za-z0-9_]` (non-ASCII included), take the longest sequence matching the requirement-ID
   grammar `[A-Z][A-Z0-9-]{2,31}-[0-9]{3}`; when the input ends after it or the next byte is outside
   `[A-Za-z0-9_]`, replace it with one ASCII space and resume after it, otherwise move one byte on
   without trying a shorter match (decision 0265). Thus `see WIDGET-001` supplies no term and
   `WIDGET-001-0023` and `xWIDGET-001` supply `widget`. For a
   requirement statement,
   parse the exact OCM line prefix `- \`ID\`: `, where `ID` is the already verified obligation ID,
   and tokenize only bytes after that prefix through but excluding its CRLF, LF, or EOF terminator.
   The prefix is not generic Markdown stripping; any other prefix is a structural OCM failure.
   Hex-only segments of eight or more characters never qualify.
5. For a path, take only the final basename bytes. Remove the final `.` and every following byte,
   then tokenize the remainder. Thus `widget.test.ts` uses `widget.test`, `.env` has an empty stem,
   `.config.json` uses `.config`, and `archive.tar.gz` uses `archive.tar`. A basename with no `.` is
   unchanged. Directory components and suffix bytes never contribute terms.
6. The following frozen language-neutral stop and boilerplate terms never qualify alone:

   ```text
   also been build change changed changes claim dist docs document documentation error evidence
   false from generated have hunk include into must only package packages print report reports
   requirement requirements return self shall should sidecar source sources string supported test
   testing tests that their then there these they this those true unknown value vendor vendored when
   where which while will with
   ```

There is no stemming, synonym expansion, fuzzy matching, Unicode normalization, project-specific
vocabulary, configurable stop list, or score. `corvint` is not a stop word. Reordering input arrays
cannot change candidate decisions or canonical output.

## Frozen subject text and path boundary

- Hunk terms come only from added payload bytes and the final basename stem of the post-image path.
  Removed bytes, old paths, directory components, context lines, hunk headers, and CEM/OCM metadata
  do not contribute. A modified rename therefore uses added bytes and the new basename stem only.
- Evidence terms come only from the exact pinned span and its final basename stem. Evidence
  directory components do not contribute.
- Requirement terms come only from the exact frozen requirement statement after removing its ID
  and Markdown prefix.
- Every textual path admitted by the inherited CEM patch profile uses the same lexical fallback.
  There is no suffix allowlist or inferred test, specification, generated, vendored, minified, or
  language class. Ruby, PHP, HTML, CSS, Rust, Java, Swift, extensionless, and unfamiliar textual
  paths are evaluated identically.
- Material-hunk filtering compares normalized paths byte-for-byte only with the exact intent path
  and exact referenced claim paths. Names or segments such as `test`, `spec`, `generated`, `vendor`,
  or `min` have no special meaning. Repository-owned class policy is deferred.
- The known `.corvint/change.cem.json`, `.corvint/change.ocm.json`, `.corvint/cem-review.md`, and
  `.corvint/ocm-review.md` names do not contribute path terms. A basis at either hunk endpoint remains
  `self-referential-basis`; WP2 separately excludes the declared CEM sidecar from canonical patches.
- Deletions abstain because removed bytes and old names cannot qualify and neither admitted CEM
  profile has an explicit deletion relation. A later profile may define one; V0 does not infer one.

## Requirements

### Boundary and CEM candidates

- `LRF-V0-001`: the profile MUST accept only a structurally valid CEM and, for obligation checks, a
  structurally valid OCM that binds a canonical `cem/0.2` CEM. CEM-only evaluation MAY use 0.1 or
  0.2. OCM plus 0.1 fails `unsupported-lrf-context`, exit `2`, with no LRF stdout, after CEM
  verification and before the OCM is read or verified; it is not a relevance miss. Structural
  failure retains the underlying verifier result, exit `2`, and no LRF stdout. This profile MUST NOT reinterpret invalid or unsupported input as a rejected edge.
- `LRF-V0-002`: every term set MUST use the frozen identifier algorithm and subject text. A
  requirement ID, content-derived ID, generic bookkeeping term, stop word, extension, directory,
  removed byte, context byte, or punctuation MUST NOT satisfy proximity alone.
- `LRF-V0-003`: an evaluated evidence span MUST be no larger than both 512 raw bytes and 20 logical
  LF-delimited lines. A non-empty span with no LF is one line; a terminal LF does not add an empty
  line. Either exceeded cap rejects that basis.
- `LRF-V0-004`: an evaluated basis MUST be external to both normalized hunk endpoints. For a create
  or delete, compare the one existing path. For a modified rename, evidence at either endpoint is
  self-reference.
- `LRF-V0-005`: each basis is evaluated independently. Its declared relation MUST be one of
  `specification`, `decision`, `test-claim`, `implementation`, `call-site`, `dependency`, or
  `incident`. A basis is proximate only when its evidence terms intersect the hunk terms. Each
  structurally valid allowed basis that passes every LRF rule qualifies as `cem-lexical-v0`; at
  least one such basis witnesses the narrow CEM hunk-evidence obligation. Rejected extra bases
  remain visible as issues but do not erase a qualifying edge.
- `LRF-V0-006`: overlap only through a CEM/OCM identifier, map path, generated report name, frozen
  boilerplate, directory, removed byte, or old rename basename MUST fail. Every evaluable
  nonqualifying basis edge is `rejected`; deletion and subject-term-bound basis edges are
  `abstained`; a hunk with no
  qualifying edge simply lacks `cem-lexical-v0`. Lexical overlap without the structural hunk, basis,
  declared-relation, narrowness, and externality checks MUST NOT emit
  `cem-lexical-v0`.

### OCM candidates and claim boundary

- `LRF-V0-007`: each hunk referenced by a structurally linked obligation MUST independently share
  at least one requirement term through its added bytes or final post-image basename stem. The
  exact obligation ID alone never satisfies this rule. If the hunk or obligation exceeds the
  retained-term cap, only that obligation-hunk edge abstains with `subject-term-bound-exceeded`.
- `LRF-V0-008`: an obligation is a `lexically-proximate-candidate` only when at least one referenced
  hunk already has `cem-lexical-v0`, passes `LRF-V0-007`, and is a permitted material hunk. Other
  referenced hunks remain visible as issues. Only exact intent and referenced claim paths are
  excluded from material hunks.
- `LRF-V0-009`: LRF MUST NOT inspect or classify claim content. Existing exact-ID claim anchors
  remain structural OCM candidates only and MUST NOT upgrade an LRF result. Claim hygiene and claim
  semantics are deferred to WP4.
- `LRF-V0-010`: claim-ID reuse is not a hard failure. An implementation MAY report reuse as a
  non-gating advisory outside the canonical LRF result, but it MUST NOT change candidate state or
  issue ordering.
- `LRF-V0-011`: a candidate MUST remain separate from execution observation. Existing OCM test
  execution remains `NOT_RUN`; LRF cannot change it to observed or passed.

### Determinism and failure

- `LRF-V0-012`: the profile MUST produce the canonical tuples and ordering below, network-free and
  source-body-free, with byte-identical canonical JSON across two fresh processes.
- `LRF-V0-013`: the profile MUST be an additive preflight. It MUST NOT add or silently rewrite
  CEM/OCM fields, declared basis relations, dispositions, structural coverage, claim anchors, or
  execution state. `cem-lexical-v0` is a derived verifier relation, not a new wire value. It may
  witness only the CEM hunk-evidence obligation; no WP3 result can close an OCM obligation or
  frontier.
- `LRF-V0-014`: input, deterministic work, and canonical output MUST remain within the exact frozen
  bounds below. Exceeding an aggregate lexical-byte, edge, result, issue, or output bound emits only
  the bound-failure document; it MUST NOT truncate, return prefix results, or vary with wall-clock
  speed or allocator behavior. Per-subject retained-term overflow is explicitly exempt: it emits the
  local abstention in `LRF-V0-006..008` and evaluation continues.
- `LRF-V0-015`: every corpus subject, selection, artifact, source-edge, raw-label, adjudication,
  partition, conformance plan/result, all-subject result, and metric commitment MUST use the
  canonical hash-only protocol below. A missing, noncanonical, mistimed, mismatched, or
  unrecomputable record makes the evaluation `NOT_RUN` and blocks promotion; it MUST NOT remove a
  subject or yield partial metrics.
- `LRF-V0-016`: the trusted independent custodian MUST run the frozen implementation over all sixty
  selected subjects and seal the complete canonical result bundle before publishing the open record
  or revealing the salt, held-out partition membership or identities, or held-out labels. The
  pre-sealed tuning-subject and tuning-label manifests MAY be disclosed before this run as specified
  below. Held-out scoring is then only deterministic selection from those sealed rows. Role,
  conformance, label, disagreement, adjudication, and fixture bundles MUST use their closed schemas;
  a missing row, mutation, identity ambiguity, or LRF execution after the open record is published
  or after the salt, held-out partition membership, held-out identities, or held-out labels are
  disclosed makes evaluation `NOT_RUN`.

## Canonical result contract

Canonical JSON is UTF-8, sorted-key, compact JSON with one terminal LF. Every LRF document has
exactly the four keys shown below; uppercase strings are metavariables, not literal output:

```json
{"inputs":["cem/0.1","CEM_MAP_SHA256","explicit-out-of-band","BASE_REVISION",null,"PATCH_SHA256",null,null,null,null,null,null,null,null],"issues":[],"profile":"lrf/0","results":[]}
```

Canonical `lrf/0` authority is issued only by the raw repository adapter after CEM, optional OCM,
and Git object verification. The pure projection evaluator and its codec are private test seams;
caller-constructed projections and differently shaped, reordered, or mutated results cannot be
serialized through the public canonical codec. A byte-identical copy retaining its valid private
seal remains the same issued payload. Future internal composition consumes the same verified adapter
result and does not receive a skip-verification capability. Every repository-authority path always
uses the frozen default bounds and exposes no fault-injection behavior; tighter bounds and simulated
failures exist only in the pure private test seam and cannot produce public canonical variants.

`inputs` is the fixed tuple `[cemSpec, cemMapSha256, patchSource, baseRevision, targetRevision,
patchSha256, excludedPath, ocmSpec, ocmMapSha256, intentPath, intentBlobOid, intentStart,
intentEnd, intentSpanSha256]`. Map digests hash the exact bounded raw map bytes. The remaining values
are copied without normalization only from a successful WP2 structural command envelope. `patchSource`
MUST be exactly one of WP2's three values: `explicit-out-of-band` when a CEM 0.1 caller supplied
`--patch`, `default-out-of-band` when CEM 0.1 used its historical default, or `canonical-derived`
for CEM 0.2. No alias, including `supplied-out-of-band`, is accepted. For 0.1, `targetRevision` is
the independently resolved optional `--target` or JSON `null`; `excludedPath` is JSON `null`. For
0.2, the successful envelope supplies the independently resolved target and the exact exclusion
`.corvint/change.cem.json`. OCM supplies its exact map digest and complete intent
path/blob/half-open-span/digest identity; its target, CEM profile, map digest, patch digest, and
exclusion bindings MUST equal the successful CEM result before LRF runs. The seven OCM fields are
respectively JSON `null` when OCM is absent. An OCM may be supplied only with 0.2. This is the smallest V0
envelope that distinguishes explicit legacy, default legacy, and canonical patch authority and
binds every edge to the verified maps, patch, target, exclusion, and intent without source bodies.

The accepted contexts are frozen as follows; any other combination fails `unsupported-lrf-context`
after the CEM passes its structural verification, before any supplied OCM is read or verified, and
before lexical work:

| Context | `patchSource` | target | exclusion | OCM binding |
|---|---|---|---|---|
| CEM-only 0.1, explicit patch | `explicit-out-of-band` | resolved optional target or `null` | `null` | all seven OCM input slots `null` |
| CEM-only 0.1, default patch | `default-out-of-band` | resolved optional target or `null` | `null` | all seven OCM input slots `null` |
| CEM-only 0.2 | `canonical-derived` | required resolved target | `.corvint/change.cem.json` | all seven OCM input slots `null` |
| OCM plus CEM 0.2 | `canonical-derived` | required resolved target shared exactly by OCM | `.corvint/change.cem.json` shared exactly by OCM | exact OCM spec, digest, and intent identity |

Consequently an existing OCM bound to 0.1 cannot be evaluated by LRF V0. The 12/12 dogfood fixture
must preserve its original bytes and also freeze a semantically unchanged 0.2 rebound whose CEM
patch, hunk IDs, evidence IDs, obligation IDs, and obligation-hunk links are identical. Only that
0.2 rebound is the mandatory LRF fixture; inability to produce it blocks WP3 acceptance rather than
weakening the context rule.

Each result is the edge tuple
`[kind, subjectId, relatedId, relation, authorityClass, outcome]`:

- a CEM basis edge uses `kind=cem-basis`, hunk ID, evidence ID, the exact declared CEM basis relation,
  `authorityClass=producer-declared`, and `outcome=cem-lexical-v0|rejected|abstained`;
- an OCM obligation-hunk edge uses `kind=ocm-hunk`, obligation ID, hunk ID,
  `relation=requirement-hunk`, `authorityClass=producer-declared`, and
  `outcome=lexically-proximate-candidate|rejected|abstained`. An OCM edge abstains only when its
  hunk or obligation term set exceeds the subject cap; ordinary failed prerequisites are rejected.

Extracted terms never appear in canonical JSON or reports. They are source-derived verification
state; bound input identities, typed edge relations, authority classes, related immutable IDs, and
deterministic issues suffice for replay and review. `producer-declared` states that the producer
selected the relation or link; WP3 verifies structure and lexical conditions, not semantic truth.

Emit one result for each basis edge of every structurally `supported` hunk and one result for each
obligation-hunk edge of every structurally `linked` obligation. CEM results sort by verified patch
hunk order, then evidence ID, then declared relation. OCM results follow them in frozen intent order,
then hunk ID. Exact duplicate tuples collapse to one; input array order does not participate.
Aggregate rows are not emitted: a CEM hunk has the relation when any basis edge emits
`cem-lexical-v0`, and an OCM obligation is retained as a candidate when any of its hunk edges emits
`lexically-proximate-candidate`.

Each issue is `[kind, subjectId, relatedId, relation, code]`, using the corresponding result fields;
`no-material-hunk` instead uses `kind=ocm-obligation`, the obligation ID, empty `relatedId`, and
`relation=requirement-hunk`. All three identity fields are empty for a profile-wide issue. Exact
duplicate tuples collapse to one. CEM edge issues sort by their CEM result order and then code; OCM
edge issues follow in OCM result order and then code. Each obligation-wide issue immediately follows
the edge issues for that obligation. Profile-wide issues replace that ordering as specified below.

The frozen deterministic bounds are:

- at most 16,777,216 lexical input bytes: sum added payload length once per distinct hunk ID, exact
  span length once per distinct evidence ID, exact statement length once per obligation ID, and stem
  length once per distinct normalized basename (`WidgetController.py` and `WidgetController.go` are
  distinct basenames and therefore each charge the `WidgetController` stem);
- at most 256 retained terms in any one hunk, evidence, or obligation term set;
- at most 8,192 evaluated edge tuples, 8,192 result tuples, and 8,192 issue tuples;
- at most 4,194,304 canonical output bytes, including the terminal LF.

An internal or conformance invocation may tighten any bound, but no `lrf/0` invocation may raise a
bound above these frozen defaults.

Before adding an item, the implementation checks the applicable count and uses checked integer
addition for byte totals. A term-set overflow is local: the implementation caches one exceeded
sentinel per immutable subject, emits `subject-term-bound-exceeded` on every affected edge, and
continues unrelated subjects. Any exceeded aggregate lexical-byte, edge, result, issue, or output
bound yields no edge results and exactly one
issue `["profile","","","","relevance-bound-exceeded"]`; the bound document retains the verified
`inputs` tuple and therefore always fits the output bound.

Structural CEM verification runs first. Any OCM supplied with CEM 0.1 then fails the stable LRF
compatibility code `unsupported-lrf-context`, exit `2`, and emits no LRF stdout, without reading or
verifying that OCM, so an invalid OCM cannot outrank the more specific context diagnosis. With CEM
0.2, OCM verification runs next when supplied. Either structural failure preserves its existing error
code and exit `2`, emits no `lrf/0` bytes on stdout, and is not a relevance result. A complete normal LRF document exits
`0`, including when edges are rejected or abstained. An aggregate deterministic bound document exits `1`.
Wall-clock timeout, allocator failure, I/O failure, and process interruption are operational aborts:
exit `2`, no canonical LRF stdout, and no conformance claim about stderr bytes. They MUST NOT be
converted into a canonical bound issue.

Issue evaluation is first-failure, not accumulative. After structural/context checks, each applicable
bound is checked before consuming or appending its item, and each CEM basis edge is evaluated
independently in canonical result order:

1. A deletion emits `abstained` plus only `deletion-relation-required`; its span, path, and terms are
   not evaluated.
2. Otherwise an over-cap span emits `rejected` plus only `evidence-span-too-broad`.
3. Otherwise an endpoint match emits `rejected` plus only `self-referential-basis`.
4. Otherwise an over-cap hunk or evidence term set emits `abstained` plus only
   `subject-term-bound-exceeded`.
5. Otherwise absent qualifying term intersection emits `rejected` plus only
   `insufficient-lexical-support`.
6. Otherwise the edge emits `cem-lexical-v0` and no issue.

Each OCM obligation-hunk edge is then evaluated independently in frozen intent/hunk order. An
over-cap hunk or obligation term set emits `abstained` plus only `subject-term-bound-exceeded`.
Otherwise it emits
`lexically-proximate-candidate` with no issue only if its referenced hunk already has at least one
`cem-lexical-v0` edge, is a permitted material hunk, and shares a requirement term. At the first
failed prerequisite it emits `rejected` plus exactly one `obligation-hunk-mismatch`; later
prerequisites are not evaluated for that edge. After all links for one obligation, emit exactly one
`no-material-hunk` when none became a candidate. Every rejected or abstained edge has one exact edge
issue, and an obligation-wide issue does not replace its edge issues. Only a global
bound failure discards all edge and obligation issues and emits the single profile issue.

| Order | Code | Condition |
|---:|---|---|
| 1 | `relevance-bound-exceeded` | a frozen aggregate lexical-byte, edge, result, issue, or output bound is exceeded |
| 2 | `deletion-relation-required` | deletion abstains because admitted CEM profiles have no explicit deletion relation |
| 3 | `evidence-span-too-broad` | basis exceeds either evidence-span cap |
| 4 | `self-referential-basis` | evidence path equals a hunk endpoint |
| 5 | `subject-term-bound-exceeded` | this edge's hunk, evidence, or obligation term set exceeds 256 retained terms; only the edge abstains |
| 6 | `insufficient-lexical-support` | evaluated CEM basis has no qualifying shared term |
| 7 | `obligation-hunk-mismatch` | first failed OCM prerequisite: no CEM witness, nonmaterial hunk, or no qualifying requirement term |
| 8 | `no-material-hunk` | obligation has no permitted proximate material hunk |

Codes are profile results, not additions to CEM or OCM reason enumerations. Independent basis and
hunk failures remain visible even when another edge retains the aggregate candidate.

### Named failure codes

The LRF evaluator and its `corvint lrf` command emits the kebab-case codes below (decision 0100).
Each row cites the first emitting site and quotes the message returned there or states the
condition checked there, which is the whole of what the row asserts.

| Code | First emitting site | At the cited site |
|---|---|---|
| `invalid-lrf-request` | `internal/lrf/types.go:194` | the `invalid(message)` constructor, returned by request and limit validation in `validate.go` and `evaluate.go` with the failing check as its message (for example "LRF limits must be positive and cannot exceed profile defaults", "obligation identities must be unique", "target revision is invalid") |
| `invalid-requirement-prefix` | `internal/lrf/terms.go:221` | "requirement line prefix is invalid" when an obligation statement does not begin with ``- `ID`: `` for its own id; validation returns it, while term extraction at `internal/lrf/evaluate.go:260` treats it as term overflow |
| `lrf-operational-failure` | `cmd/corvint/lrf.go:61` | "LRF evaluation was interrupted" when the context is done after `lrfrepo.Evaluate` returns; `emitLRFError` also uses this code for an error that carries none, writing `{"code", "error", "ok": false}` to stderr with exit 2 |

## Simpler baseline and YAGNI cuts

The baseline is current structural CEM/OCM verification followed by human review. V0 adds only the
derived `cem-lexical-v0` hunk-evidence relation and the deterministic OCM candidate filter above. It
does not add semantic entailment, OCM or frontier closure, embeddings, an LLM verifier, claim
analysis, test adequacy, stemming, synonyms, learned or configurable vocabularies, suffix or
repository-class policy, generator or deletion relations, authenticated identity, signatures,
authenticated product roles or authorization, a daemon, a database, network access, execution
attestation, a new claim language, or a new CEM/OCM wire version.

## Acceptance matrix

Each evaluable family requires a genuine-candidate, fabricated-rejection, and no-input/abstention
fixture where applicable. The minimum suite is:

| Fixture | Required result |
|---|---|
| snake/camel/acronym/digit and reserved-term tokenizer | exact frozen term sets and byte-identical ordering; `corvint` retained |
| exact ID removal, malformed Markdown prefix, CRLF/LF/EOF requirement lines | complete IDs removed; malformed prefix is structural failure with exit 2 and no LRF stdout; line endings yield identical terms |
| `widget.test.ts`, `.env`, `.config.json`, `archive.tar.gz`, and extensionless basenames | exact frozen stems; directories and final suffix bytes never contribute |
| source hunk plus narrow external specification | `cem-lexical-v0` only with a shared stable identifier and allowed declared relation |
| test and documentation hunks plus narrow external bases | same CEM relation and material-hunk rules as every other textual path |
| Ruby, PHP, HTML, CSS, Rust, Java, Swift, and unfamiliar textual paths | identical lexical fallback; no suffix-based abstention |
| unrelated pinned rule or license | `insufficient-lexical-support` |
| self-citation at old or new rename endpoint | `self-referential-basis` |
| rename sharing only an old basename, directory, context, or removed byte | rejected; matching added byte or final new basename retains candidate |
| 513-byte span and 21-line span | `evidence-span-too-broad` |
| one matching and one unrelated basis on one hunk | candidate retained and rejected extra basis visible |
| created ordinary source with matching external basis | candidate |
| deleted source | `deletion-relation-required` and abstained |
| exact obligation ID with unrelated hunk or ID-only claim anchor | no upgrade; obligation mismatch remains visible |
| exact intent-path or referenced-claim-path-only change | `no-material-hunk`; names containing test/spec/vendor/generated/min do not exclude material hunks |
| known sidecar citing itself | `self-referential-basis`; sidecar name supplies no term |
| one claim reused across two otherwise matching obligations | no canonical hard failure; each obligation evaluated independently |
| no evidence or structurally unknown obligation | no fabricated result; original structural unknown remains inspectable |
| CEM/OCM structural failure | underlying code, exit 2, and no canonical LRF stdout |
| CEM-only 0.1 explicit/default patch, CEM-only 0.2, and OCM plus 0.2 | exact context-table input tuple, including `null`; OCM plus 0.1 is `unsupported-lrf-context`, exit 2, no LRF stdout |
| one CEM edge failing span, self-reference, and lexical checks | only `evidence-span-too-broad`; deletion variant emits only `deletion-relation-required` |
| one OCM edge failing every ordinary prerequisite | one `obligation-hunk-mismatch`, then one obligation-wide `no-material-hunk` |
| one over-cap CEM subject and one over-cap OCM subject beside unrelated in-cap subjects | affected edges abstain with `subject-term-bound-exceeded`; in-cap results remain; no candidate/witness for affected edges |
| each aggregate lexical-byte, edge, result, issue, and output boundary at limit and limit+1 | at-limit canonical result; limit+1 bound document, exit 1, and no partial results |
| timeout, allocator failure, I/O failure, and interruption injection | operational exit 2 and no canonical LRF stdout; never `relevance-bound-exceeded` |
| same edge under two declared relations | distinct typed tuples; input digests and relations change canonical bytes |
| added producer `authorityClass` wire field | structural unknown-field rejection, exit 2, and no LRF stdout; canonical authority is verifier-derived |
| commitment codec scalar/escape/key-order vectors and every record example | exact canonical bytes and domain hashes reproduced by two independent implementations |
| missing reveal, swapped hash domain, altered subject/artifact/label, early salt, broken record link, or LRF execution after the open record is published or after the salt, held-out partition membership, held-out identities, or held-out labels are disclosed | whole evaluation `NOT_RUN`, no subject removal and no partial metrics |
| empty/duplicate role, absent/altered closed conformance input, missing all-subject result, open record published or salt, held-out partition membership, held-out identities, or held-out labels disclosed before result seal, or output substitution | whole evaluation `NOT_RUN`; held-out metrics are never recomputed from a new run |
| duplicate reviewer ID, wrong subject, oversized/wrong-typed label field, unsorted evidence, incomplete disagreement, or invented adjudication | label manifest invalid and whole evaluation `NOT_RUN` |
| 12/12 dogfood bundle | only `roles`, `subjects`, `source-edges`, and `fixture-labels` manifests; subject labels bind every fabricated source edge without an edge-label identity |

Before implementation, freeze the original Corvint 12/12 OCM dogfood bytes, then freeze the required
edge-identical canonical 0.2 rebound as `current-corvint-ocm-12-of-12-vacuous`. Also freeze a valid
0.2 control artifact with all twelve obligations structurally unknown and human-reviewed mutation
evidence recording that only OCM claim objects, obligation dispositions/reasons, and claim/hunk reference arrays changed
between control and fixture; CEM, patch, code, intent, and subject identities remain byte-identical.

There is no separate or edge-labelled dogfood-manifest format. The fixture bundle is exactly one
canonical `roles` manifest, one `subjects` manifest containing the twelve obligation subjects in
intent order, one `source-edges` manifest containing every immutable
`[contextSha256,ocm-hunk,obligationId,hunkId,requirement-hunk,producer-declared]` edge in canonical
order, and one `fixture-labels` manifest using the same two-reviewer/disagreement/adjudication schema
as the corpus. Each subject label—not each edge label—has `expected=not-proximate`,
`fabricated=true`, and mutation evidence binding the control and fixture artifacts. Every listed
source edge MUST emit `outcome=rejected`; one retained lexical candidate fails the fixture. Each
obligation MUST also produce `no-material-hunk`, and the fixture cannot report an OCM witness or
closed frontier. The broad or self-referential CEM dogfood fixture uses the same four-manifest bundle
with CEM hunk subjects and basis source edges; every fabricated subject remains non-candidate until
rebuilt with narrow external bases or explicit structural unknowns.

Acceptance requires all genuine CEM relations and OCM candidates to be retained, every fabricated
link to be rejected, deletions to abstain, stable complete issue ordering, and byte-identical results
from two fresh processes. One accepted self-reference, ID-only upgrade, removed-byte/directory-only
overlap, inferred path-class decision, partial bound result, bare-LRF witness, or WP3 OCM/frontier
closure is an immediate release blocker.

## Evaluation and kill gates

### Superseded-pending-redesign: custodial seven-role evaluation

The custodial seven-role gate below is superseded-pending-redesign. Its required distinct custodian,
producer, implementer, two reviewers, adjudicator, and result operator make the shipped single-owner
runtime permanently ineligible, so it can never certify that runtime. The retained text records the
failed gate and does not design its replacement.

The labelled unit is a subject fixed before evidence production:
`(contextSha256, subjectKind, subjectId)`. `contextSha256` is `H("context", bytes)` over the
commitment-codec encoding of the tuple
`[repositoryCorpusId,prIdentity,baseCommit,targetCommit,patchSha256,excludedPath,intentPath,
intentBlobOid,intentStart,intentEnd,intentSpanSha256]`, with offsets encoded as decimal strings and
all five intent values JSON `null` when no scope was preregistered. The two external IDs are exact
preregistered UTF-8 strings; revisions and digests are lowercase full hex.
`canonicalSubjectIdentity` is the commitment-codec tuple
`[contextSha256,subjectKind,subjectId]`. `subjectKind` is
`cem-hunk` or `ocm-obligation`. A CEM subject is every textual canonical-patch hunk, including a deletion or hunk
that will have no basis, except the exact pinned intent path. Claim paths are not excluded because
they do not exist as authority before artifact production. An OCM subject is every obligation in the
exact preregistered target intent scope, including one later recorded as structurally unknown.

For a valid evaluated context, a CEM subject is `candidate` when any of its canonical basis-edge rows
emits `cem-lexical-v0`; an OCM subject is `candidate` when any of its canonical obligation-hunk rows
emits `lexically-proximate-candidate`. A subject with no producer-declared edge, an explicit unknown,
or no LRF result row is a non-candidate and remains in the denominator. Exact duplicate subject
identities within one context collapse to their first canonical occurrence; the same subject ID in a
different context remains distinct. Differing context payloads with the same `contextSha256`, or
unequal source subjects producing the same canonical subject identity, make the corpus `NOT_RUN`.
Ordering and partition digest collisions use the explicit canonical-byte tie-breaks below.
Structural, compatibility, or operational failure makes
the affected evaluation `NOT_RUN`, never a non-candidate, and no subject may be dropped or replaced.
Duplicate edge tuples retain the canonical result contract above and cannot add sample weight.

### Canonical evaluation commitments

The commitment codec is a dependency-free canonical JSON subset. Values are only JSON `null`,
booleans, Unicode-scalar strings, arrays, and objects; JSON numbers are forbidden, and every count,
ordinal, or offset is instead a base-10 string with no sign or leading zero except `"0"`. Input with
a duplicate object key, invalid UTF-8, a BOM, an unpaired surrogate, or a forbidden value is invalid.
Object keys sort by their raw UTF-8 bytes. Arrays preserve declared order. Serialization emits no
whitespace or terminal LF; it writes `true`, `false`, and `null` literally, escapes `"` as `\"` and
`\` as `\\`, escapes every U+0000..U+001F scalar as lowercase `\u00xx`, and emits every other scalar
as its original UTF-8 bytes without Unicode normalization or optional escaping. A verifier MUST
parse, reserialize, and require byte equality before hashing.

Every manifest has exactly this envelope, where `KIND` and `ENTRIES` are replaced before canonical
serialization:

```json
{"entries":[],"kind":"KIND","spec":"lrf-eval-manifest/0"}
```

The admitted manifest kinds and exact entry arrays are:

| Kind | One entry | Order |
|---|---|---|
| `roles` | `[role,memberId]` | exact role order `custodian`, `producer`, `wp3-implementer`, `reviewer-a`, `reviewer-b`, `adjudicator`, `result-operator`; exactly one non-empty entry per role |
| `preregistration` | `[repositoryCorpusId,prIdentity,baseCommit,targetCommit,mergeOrdinal,intentPath,intentBlobOid,intentStart,intentEnd,intentSpanSha256]` | preregistered PR order; five intent values are `null` together when absent |
| `subjects` | `[canonicalSubjectIdentity,contextPayload]` | PR order, then CEM patch order, then OCM intent order |
| `selected-subjects` | `canonicalSubjectIdentity` | PR order after secret per-PR cap |
| `source-edges` | `[contextSha256,kind,subjectId,relatedId,relation,authorityClass]` | PR order, then canonical CEM/OCM source-edge order |
| `artifacts` | `[contextSha256,cemRawSha256,ocmRawSha256]` | PR order; `ocmRawSha256` is `null` when OCM is absent |
| `tuning-subjects` or `heldout-subjects` | `canonicalSubjectIdentity` | raw partition-digest order |
| `tuning-labels`, `heldout-labels`, or `fixture-labels` | `[canonicalSubjectIdentity,rawLabelA,rawLabelB,disagreement,adjudicatedLabel]` | corresponding subject order; label rules below are total |
| `conformance-plan` | `[coverageId,inputSha256,expectedOutputSha256]` | exact spec-owned coverage-ID order below; no missing or extra entry |
| `conformance-results` | `[coverageId,resultSha256,"PASS"]` | exact conformance-plan order |
| `all-results` | `[canonicalSubjectIdentity,candidate,lrfResultSha256]` | all sixty selected subjects in selected-subject manifest order |
| `metrics` | `[metric,numerator,denominator,status]` | exact metric order `precision`, `recall`, `critical-recall`, `gaming-rate`, `fabricated-catch`; counts are canonical decimals and `status` is `PASS`, `FAIL`, `REPORTED`, or `NOT_RUN` as defined below |

`contextPayload` is the exact context tuple defined above. All seven role `memberId` values are
pairwise distinct ASCII strings matching `[A-Za-z0-9][A-Za-z0-9._-]{0,63}`; no role is absent,
repeated, aliased, or represented by an empty manifest. There is no source/build role in V0: the
frozen implementation revision and sealed results are evaluation inputs, not proof of runtime
behavior. All evaluation-record offsets and lengths
use decimal strings even when the corresponding LRF wire field is an integer. These seven identities
are evaluation-only procedural memberships, not authenticated Corvint users, product roles, or
authority classes. Digests inside manifests are lowercase 64-hex strings; revisions and object IDs
retain their frozen representation.

Below, `codec(value)` means the exact canonical bytes above and `hex(digest)` means lowercase
64-character hexadecimal. Define `H(kind, bytes)` as SHA-256 over the UTF-8 bytes `corvint-lrf-eval/0`, one NUL byte, the exact
ASCII `kind`, one NUL byte, then `bytes`. A manifest digest is `H(manifest.kind,
canonicalManifestBytes)`. Raw producer-packet, CEM, OCM, conformance input, conformance expected
output, conformance observed result, critical-evidence, and LRF-result bytes use respectively
`producer-packet`, `cem-artifact`, `ocm-artifact`, `conformance-input`, `conformance-output`,
`conformance-result`, `critical-evidence`, and `lrf-result`.
Record bytes use
their exact record `spec` as `kind`. Thus every `*Sha256` field has one type fixed by its referenced
manifest, artifact, or record; a digest from another domain cannot substitute. The salt is exactly
32 bytes and its published commitment is `H("partition-salt", salt)`.

The following records are themselves canonical codec objects with exactly the shown fields. The
uppercase values are metavariables. Their one-line examples also show canonical key order:

```json
{"conformancePlanSha256":"HASH","corpusId":"CORPUS","phase":"pre-artifact","preregistrationSha256":"HASH","producerPacketSha256":"HASH","rolesSha256":"HASH","saltCommitmentSha256":"HASH","selectedSubjectsSha256":"HASH","spec":"lrf-eval-commit/0","subjectsSha256":"HASH"}
{"artifactsSha256":"HASH","commitmentRecordSha256":"HASH","heldoutLabelsSha256":"HASH","heldoutSubjectsSha256":"HASH","phase":"pre-tuning","sourceEdgesSha256":"HASH","spec":"lrf-eval-seal/0","tuningLabelsSha256":"HASH","tuningSubjectsSha256":"HASH"}
{"allResultsSha256":"HASH","conformanceResultsSha256":"HASH","implementationRevision":"FULL_COMMIT_HEX","phase":"pre-partition-reveal","sealRecordSha256":"HASH","spec":"lrf-eval-results-seal/0"}
{"phase":"partition-open","resultsSealRecordSha256":"HASH","saltHex":"64-LOWERCASE-HEX","sealRecordSha256":"HASH","spec":"lrf-eval-open/0"}
{"metricsSha256":"HASH","openRecordSha256":"HASH","phase":"post-selection","spec":"lrf-eval-metrics/0"}
```

The spec-owned conformance plan has exactly these coverage IDs in this order:

```text
token.identifier-split
token.id-redaction
token.id-redaction-hunk
token.basename
token.stop-terms
cem.candidate
cem.deletion
cem.span-cap
cem.span-cap-at-limit
cem.self-reference
cem.self-reference-create
cem.no-overlap
cem.path-only-overlap
cem.extra-basis
cem.allowed-relations
cem.relation-per-basis
ocm.candidate
ocm.no-cem-witness
ocm.nonmaterial
ocm.intent-path
ocm.requirement-prefix
ocm.no-overlap
ocm.id-only
ocm.no-links
context.01-explicit
context.01-default
context.02-cem
context.02-ocm
context.unsupported
failure.bound
failure.operational
failure.structural
determinism.repeat
determinism.duplicate-collapse
dogfood.ocm-12
dogfood.cem-broad
commitment.mutation
claim.no-upgrade
claim.reuse-inert
claim.reuse-issue-order
execution.not-observed
```

Before implementation work, the custodian MUST publish the raw canonical `conformance-plan`
manifest, every input and expected-output byte string, and bind its digest in the commit record. Each
exact ID occurs once with one nonempty frozen input and expected-output byte string; each is at most 4,194,304 bytes and all pairs total at most
67,108,864 bytes. Their digests use the conformance domains above. No implementer-selected ID,
replacement, omission, or extra is allowed. After implementation freeze, `conformance-results`
contains the same IDs and one observed-result digest each; `PASS` is valid only when the revealed
observed bytes equal the corresponding frozen expected bytes. A missing preimplementation plan or
any non-`PASS` row makes the experiment `NOT_RUN`.

The manifest and payload bundle MUST be an accepted spec-owned companion fixture, not generated or
chosen by the implementer, producer, or custodian. Its registered exact manifest digest is
`8509abd6780bb220449f25a85db3d170f4dc6f5a90217e6129995420772d8c6b`. The coverage-ID list alone is
not a fixture and cannot satisfy the gate. The companion is registered but remains conformance
evidence only. It applies only to a future implementation and evaluation begun after this digest was
published. The already-started WP3 runtime can never satisfy this preregistration, so its experiment
remains `NOT_RUN` regardless of later conformance success.

After tuning, the implementation is frozen to one independently resolved full lowercase Git commit
OID. The member in role `result-operator`, under the trusted custodian, evaluates every one of the
sixty selected subjects from the frozen artifacts without disclosing results to the implementer.
`all-results` contains exactly sixty unique rows in selected-subject order. Each row binds the exact
canonical LRF result bytes under `H("lrf-result", bytes)` and the candidate bit rederived from those
bytes. The result document's `inputs` MUST equal the tuple independently rederived by successful
structural verification of that subject's exact committed context, canonical patch, and CEM/OCM raw
artifacts; its result/issue subject identity MUST be valid for the row. A wrong-context document is
`NOT_RUN`, not a non-candidate. A no-edge subject remains a row with `candidate:false`. Before
publishing the open record or revealing the salt, held-out partition membership or identities, or
held-out labels, the custodian publishes the results-seal record binding the implementation
revision, complete conformance results, complete all-results manifest, and prior seal record.

Publication order is normative and each record digest uses `H(record.spec, canonicalRecordBytes)`:

1. After closed role membership, immutable subjects, and the secret selection are derived, but
   before any CEM/OCM evidence artifact or implementation work, publish the commit record plus raw
   roles and conformance-plan manifests plus all conformance payloads, require their digests to
   match, and retain their exact bytes.
2. After all artifacts, source edges, two raw labels, disagreements, adjudications, and both secret
   partitions are frozen, but before any tuning identity or label is disclosed, publish the seal
   record binding the commit-record digest and retain every referenced manifest.
3. Disclose only the raw tuning-subject and tuning-label manifests. Their recomputed hashes MUST
   equal the seal fields; the salt and held-out manifests remain secret.
4. After tuning ends, freeze the implementation revision. The result operator runs the closed
   conformance plan and LRF over all sixty selected subjects. While the open record remains
   unpublished and the salt, held-out partition membership and identities, held-out labels, and all
   result rows remain undisclosed, publish the results-seal record and retain exact
   conformance/result bytes.
5. Publish the open record, then reveal every retained raw manifest, record, artifact, packet,
   conformance payload/result, LRF result, label, and critical/mutation evidence byte string needed
   to recompute every prior commitment. Verify them before scoring. No LRF execution is permitted
   after open-record publication or after any earlier disclosure of the salt, held-out partition
   membership or identities, or held-out labels. Compute each held-out metric only by selecting the
   twenty revealed held-out identities from the already sealed sixty rows and labels.
6. Publish the canonical metrics manifest and metrics record.

An audit verifier starts from the preregistered revisions and intent bytes, re-derives the canonical
patch, context payloads, subjects, secret cap, and partition; reserializes every disclosed manifest
and record; recomputes every domain-separated digest and record link; verifies role, raw artifact,
label, conformance, and result schemas; and recomputes metrics only from sealed rows. It performs no
LRF execution after the open record is published or after the salt, held-out partition membership,
held-out identities, or held-out labels are disclosed. Every byte, digest, subject, ordering
decision, candidate bit, and record
reference MUST equal the reveal. A missing reveal input, noncanonical byte,
hash or link mismatch, wrong phase/order, subject change, salt mismatch, artifact repair, label
replacement, or forbidden late execution makes the whole corpus `NOT_RUN` and blocks promotion without
reporting partial metrics. This protocol authenticates no person and adds no signatures, encryption,
Merkle tree, timestamp service, or new runtime dependency; its trust boundary is the independent
custodian's honest protocol execution. Commitments prevent accidental or post-hoc mutation; they do
not defend against a malicious custodian, result operator, operating system, process, clock, or
runtime and are not execution attestation.

Before any evidence artifact, tuning, or Corvint result exists, an independent corpus custodian MUST
preregister an ordered list of consecutive real pull requests by repository corpus ID, immutable PR
identity, base commit, target commit, and merge order. Eligibility is exactly: merged, non-fixture,
and containing at least one textual hunk admitted by the frozen CEM patch profile. For each PR the
custodian also preregisters zero or one intent path and exact target span selected from PR metadata by
one frozen rule; absence yields no OCM subjects. The custodian derives and freezes every subject from
the canonical patch and intent before CEM/OCM production. A PR does not need pre-adoption artifacts.

The custodian generates one secret 32-byte salt and, before artifacts are produced, publishes
the exact commit record binding `H("partition-salt", salt)`, the preregistration and complete subject
manifests, the roles manifest, the selected 60-subject manifest, and the producer packet. Within each PR, subjects sort
by `H("subject-order", salt || NUL || canonicalSubjectIdentityBytes)`, then canonical identity bytes
on a digest tie; retain at most 12. The custodian continues in preregistered PR order until exactly
60 distinct subjects are selected. Fewer than 60 is `NOT_RUN`. Selection therefore depends only on
immutable subjects and the precommitted secret, never on a producer edge or Corvint output.

After subject commitments, a producer who is neither the WP3 implementer nor a label reviewer MAY
backfill one structurally valid canonical CEM 0.2 per registered PR and, when an intent was
preregistered, one structurally valid OCM. The producer uses one preregistered instruction packet,
may inspect the registered repository and PR record, and MUST NOT run or inspect LRF. CEM maps cover
every patch hunk and OCM maps retain every obligation; missing support is encoded as the existing
explicit unknown rather than omission. Candidate edges may enrich a subject but cannot create,
remove, reorder, or crowd out a subject. Freeze the raw artifacts, their digests, and complete source
edge manifests before any LRF execution; a later invalid artifact makes evaluation `NOT_RUN` and is
not repaired after an LRF result.

The same secret partitions the selected subjects by
`H("partition-order", salt || NUL || canonicalSubjectIdentityBytes)`, then canonical identity bytes
on a digest tie: first 40 tuning, remaining 20 held out. After artifact and label records are sealed,
the custodian reveals only the tuning identities and labels. It withholds the salt, held-out
identities, held-out labels, and every LRF result until the implementation revision, closed
conformance results, and all sixty result rows are frozen in the results-seal record. It then reveals
the salt and held-out manifest under the published open record and verifies all prior commitments;
no LRF execution is permitted after the open record is published or after the salt, held-out
partition membership, held-out identities, or held-out labels are disclosed. Failure cannot be
repaired by changing code, artifacts, subjects, salt, partitioning, labels, results, thresholds, or
PR sequence.

The members in roles `reviewer-a` and `reviewer-b` independently label every selected subject while
blind to Corvint output. Their IDs are therefore exact, distinct, bounded by the role schema, and
different from the producer, custodian, implementer, adjudicator, and result operator. A raw label
is an object with exactly the ten keys
`critical`, `criticalEvidence`, `expected`, `fabricated`, `fabricationEvidence`, `gameable`,
`gamingEvidence`, `rationale`, `reviewerId`, and `subject`; no field is optional. `critical`,
`fabricated`, and `gameable` are booleans;
`expected` is exactly `proximate` or `not-proximate`; `reviewerId` equals the applicable role member;
`subject` is byte-identical under the codec to its enclosing `canonicalSubjectIdentity`; and
`rationale` is a nonempty Unicode-scalar string of at most 2,048 UTF-8 bytes. `expected=proximate`
means at least one typed producer relation for that subject is genuinely supported by a narrow
external basis (CEM) or genuinely links a permitted material hunk sharing a requirement term (OCM);
it does not judge claim hygiene or test execution.

Uppercase strings in this example are metavariables:

```json
{"critical":false,"criticalEvidence":[],"expected":"not-proximate","fabricated":false,"fabricationEvidence":null,"gameable":false,"gamingEvidence":null,"rationale":"RATIONALE","reviewerId":"REVIEWER_A_MEMBER_ID","subject":["CONTEXT_SHA256","cem-hunk","HUNK_ID"]}
```

`criticalEvidence` is a sorted unique array of zero through eight exact tuples
`[sourceKind,sourceIdentity,rawSha256,byteLength,start,end,impactClass]`. `sourceKind` is exactly one of
`pr-review`, `incident`, `policy-contract`, `test-failure`, or `change-record`; `sourceIdentity` is a
nonempty Unicode-scalar string of at most 256 UTF-8 bytes; `rawSha256` is
`H("critical-evidence", rawBytes)`; `byteLength` is the exact decimal raw length from 1 through
16,777,216; `start` and `end` are canonical decimal strings with
`0 <= start < end <= byteLength`; and `impactClass` is exactly one of `security-boundary`, `privacy`,
`irreversible-data-loss`, `external-contract`, or `production-availability`. Tuples sort by their
complete codec bytes, which is also the final tie-break. `critical=true` requires one through eight
records whose cited bytes state the affected boundary; `critical=false` requires an empty array.
Every exact raw record is sealed by the label manifest and revealed after results are sealed;
missing, length-mismatched, or digest-mismatched bytes invalidate the label.

`fabricated=true` requires `expected=not-proximate` and `fabricationEvidence` equal to one exact
mutation tuple `[mutationId,beforeArtifactSha256,afterArtifactSha256,affectedFields,humanRationale]`.
`gameable=true` requires `expected=not-proximate` and `gamingEvidence` equal to the same tuple shape.
`mutationId` matches `[A-Za-z0-9][A-Za-z0-9._:-]{0,127}`. Each digest MUST equal either
`H("cem-artifact", rawBytes)` or `H("ocm-artifact", rawBytes)` for both members of the pair; its
artifact domain is unambiguous from the permitted pointer root, and mixed domains are invalid. The
exact raw bytes are revealed after the result seal; `humanRationale` is nonempty and
at most 2,048 UTF-8 bytes. `affectedFields` is a raw-UTF-8-sorted unique array of one through 64
pointers, each matching exactly one of `/evidence/N`, `/claims/N`, `/hunks/N/basis`, or
`/obligations/N/disposition`, `/obligations/N/reason`, `/obligations/N/hunkIds`, or
`/obligations/N/claimIds`, where `N` is canonical decimal without a leading zero except `0`. No
other pointer qualifies, and all pointers in one tuple MUST belong to the same CEM or OCM domain.

The subject kind fixes that domain: `cem-hunk` uses CEM and `ocm-obligation` uses OCM. For
fabrication, `afterArtifactSha256` MUST equal that subject context's frozen artifact digest in the
`artifacts` manifest; for gaming, `beforeArtifactSha256` MUST equal it. Both raw artifacts MUST have
the same supported spec and immutable context bindings. Each listed pointer MUST resolve in both
canonical JSON documents to unequal codec values. Replacing each listed value with JSON `null` MUST
make the documents byte-identical after canonical serialization; otherwise the evidence is invalid.
This pointer-diff validation emits no report or digest and proves only the declared byte locations,
not the reviewers' semantic fabrication or behavior-preservation judgment.

Fabrication means the reviewer judges every candidate-capable declared edge was deliberately planted
for evaluation. Gaming means the reviewer judges the recorded evidence/link-only mutation preserves
the intended behavior and typed relation while being able to flip the subject's lexical outcome.
These are qualitative independent reviewer findings, not mechanically proved mutation semantics.
There is no checker, checker digest, generated report, or automation claim. The before/after digests,
permitted pointers, reviewer identity, and human rationale are the complete evidence. A false value
requires the corresponding evidence field to be `null`.

Each label bundle contains exactly two raw labels ordered by raw UTF-8 `reviewerId`; equality is
invalid rather than a tie. The allowed disagreement fields are exactly `critical`,
`criticalEvidence`, `expected`, `fabricated`, `fabricationEvidence`, `gameable`, `gamingEvidence`,
and `rationale`. The
disagreement object's `fields` is the raw-UTF-8-sorted exact set of those fields whose codec values
differ, and `reviewerIds` is the two sorted reviewer IDs; no other field may occur. When that set is
empty, both disagreement and adjudication are `null`. Otherwise both are present, the adjudicated
label's `reviewerId` equals the `adjudicator` role member, its subject is identical, and its nine
non-ID fields exactly copy one complete raw label rather than mixing or inventing values. Both raw
labels, the disagreement, the adjudication, evidence, and rationales remain immutable. Metrics use
the common raw values when there is no disagreement and only the adjudicated label otherwise.

- `TP`: candidate and labelled `proximate`; `FP`: candidate and labelled `not-proximate`; `FN`:
  non-candidate and labelled `proximate`; `TN`: non-candidate and labelled `not-proximate`.
- Precision is `TP/(TP+FP)` and recall is `TP/(TP+FN)` on the held-out partition. A zero denominator
  is `NOT_RUN`, never zero or pass. Critical recall uses only `critical=true, expected=proximate`
  units; with none it is `NOT_RUN`. Promotion requires precision at least 0.85 and critical recall
  at least 0.90, no more than 0.10 below the structural-candidate baseline of 1.00.
- CEM relation and evidence selection are producer declarations. `cem-lexical-v0` verifies their
  structure, bounded externality, and lexical proximity, not semantic truth. A held-out FP is
  `gameable` only when its already sealed final label has `gameable=true` and valid gaming evidence;
  no post-result relabelling is allowed. The gaming rate is `gameableFP/(TP+FP)` over all held-out
  candidates. Zero candidates is `NOT_RUN`; above 0.20 kills.
- A fabricated miss is a preregistered subject with `fabricated=true, expected=not-proximate`.
  `fabricatedCatch = fabricated non-candidates / all fabricated misses`; a zero denominator is
  `NOT_RUN`. Promotion requires at least 0.25. The 12/12 dogfood fixture is mandatory conformance
  evidence but is not added to the independent 60-subject denominator.
- Preregister a separate feasibility cohort of the first five consecutive eligible real pull
  requests after implementation freeze. Enumerate every subject before artifact production and
  apply the same blind independent labels. Stop WP3 rather than add semantic machinery when more
  than 0.60 of subjects labelled
  `expected=proximate` are rejected or abstained; a zero positive denominator is `NOT_RUN`.
- Bare lexical overlap becoming a witness, an OCM candidate closing an obligation/frontier, any
  critical or denominator gate remaining `NOT_RUN`, or any held-out reuse for tuning is a release
  blocker.

The metrics manifest stores the exact integer numerators and denominators from these formulas as
canonical decimal strings. A zero denominator stores numerator `"0"`, denominator `"0"`, and
`NOT_RUN`. Otherwise recall is `REPORTED`; precision and critical recall are `PASS` exactly when
`100*numerator >= 85*denominator` and `100*numerator >= 90*denominator`; gaming rate is `PASS`
exactly when `100*numerator <= 20*denominator`; fabricated catch is `PASS` exactly when
`100*numerator >= 25*denominator`. The complementary state is `FAIL`. No floating-point rendering
or recomputation run participates.

Corvint-authored fixtures establish deterministic conformance only. Promotion still requires the
sealed held-out and prospective gates above and the Verified Absence Frontier gates. WP5 cannot
begin until WP2 canonical binding, WP3 `cem-lexical-v0`, and WP4's caller-reported, non-closing
`test-report-matched-v0` qualification are delivered; delivery does not make that relation eligible
for default closure.

## Traceability and delivery

The `src/context_corvint_lrf.py`/`src/corvint_cli.py` citations below are historical implementation
references, not live authority: decision 0012 R0
(`docs/decisions/0012-expert-panel-ratifications-2026-09-01.md`) rules the Python implementation
non-authoritative and slated for separate removal.

| Requirement | Expected implementation surface | Required evidence |
|---|---|---|
| `LRF-V0-001..006`, `012..016` | `src/context_corvint_lrf.py`, `src/corvint_cli.py`, `docs/lrf-0.schema.json` | CEM unit/conformance mutations, local and aggregate bound probes, operational-abort probes, current-dogfood observation |
| `LRF-V0-007..011`, `012..016` | `src/context_corvint_lrf.py`, `src/corvint_cli.py`, `docs/lrf-0.schema.json` | OCM adversarial fixtures and current 12/12 failure |
| Evaluation and kill gates | frozen labelled corpus and hash-only evaluation verifier | independent raw labels, canonical commitment vectors, and recorded `PASS`, `FAIL`, or `NOT_RUN` |

Three clause parts have dedicated conformance cases, each shown to fail under a scratch mutation of
`internal/lrf` that violates only that part (`docs/reviews/corpus-mutation-audit-2026-09-13.md`):
`LRF-V0-005` one hunk/evidence pair under two relations yields one tuple per basis
(`cem.relation-per-basis`, `conformance/lrf-v0/cases.json:2067@1496c01c`); `LRF-V0-012` exact
duplicate basis tuples collapse to the single-tuple bytes (`determinism.duplicate-collapse`,
`conformance/lrf-v0/cases.json:4929@219f067c`); and `LRF-V0-010` claim-path reuse leaves a
four-row issue list byte-identical (`claim.reuse-issue-order`,
`conformance/lrf-v0/cases.json:6496@6422be9c`).

`LRF-V0-001` context precedence: an OCM supplied with CEM 0.1 refuses `unsupported-lrf-context`
before OCM verification, whether that OCM is valid or not
(`cmd/corvint/lrf_test.go:TestLRFOCMPlusCEM01IsUnsupportedBeforeOCMVerification`).

`LRF-V0-002` frozen algorithm step 4 admits only an ASCII byte outside `[A-Za-z0-9_]` as a
content-ID boundary: a content ID beside a non-ASCII byte is tokenized, not redacted
(`TestContentIDBesideNonASCIIByteIsNotRedacted`). The same step redacts a bounded requirement ID in any
term source, so its letters cannot satisfy proximity alone (`TestRequirementIDInAddedBytesSuppliesNoTerm`).

Delivery remains `not-started` until implementation, deterministic fixtures, and the mandatory
current-dogfood failures exist. Claim hygiene is a WP4 concern. This spec commit is intent preflight
only and is not delivery evidence.

## Compatibility

This proposed, pre-release revision intentionally breaks prior draft `lrf/0` bytes: OCM results add
`abstained`, both edge families add `subject-term-bound-exceeded`, and formerly profile-wide term
overflow becomes local. No backward compatibility is claimed for the superseded draft bytes or
their prior companion digest. Regenerating and re-registering the closed companion records the new
contract; it is conformance evidence only and does not make the already-started value experiment
preregistered or promote delivery.

## Rollback

Before promotion, rollback removes the relevance preflight and restores structural CEM/OCM results
as candidate links. It must preserve original maps, explicit unknowns, failed evaluations, and this
specification as decision history. A failed floor does not justify silently reporting an OCM witness
or closed frontier, or a CEM witness without `cem-lexical-v0`.
