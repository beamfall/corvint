# Obligation Closure Map V0 dogfood

Owner: Russell Lewis
Frozen: 2026-08-22
Intent status: proposed overall; accepted clauses/amendments: `OCM-V0-001` narrowing and `OCM-V0-013` (2026-09-01), `OCM-V0-009` dogfood-policy amendment (2026-09-05)
Delivery status: experimental
Authoritative inputs: `docs/SPEC-DRIVEN-DEVELOPMENT.md`, `docs/CHANGE-EVIDENCE-MAP.md`,
`docs/specs/cem-pilot-kit.md`, `docs/specs/cem-0.2-canonical-binding.md`

## Agent digest
- Claim: OCM maps scoped requirements to exact change-and-test witnesses or explicit unknowns; ordered multi-intent dogfood coordination is experimental.
- Status: proposed overall; accepted clauses/amendments: `OCM-V0-001` narrowing and `OCM-V0-013` (2026-09-01), `OCM-V0-009` dogfood-policy amendment (2026-09-05)/experimental; accepted amendments implemented without promotion
- Exists: an `ocm/0.1-experimental` structural traceability profile, verifier, and evidence mapping.
- Blocked on: ten-change dogfood and promotion gates; linked rows do not prove correctness or adequacy.
- Read next: Verified starting state; Requirements; Acceptance and dogfood.

## User and job

After an agent changes code against a specification, an author or reviewer must be able to see
whether every requirement in the declared task scope has an exact change and test-claim witness, or
an explicit unknown. CEM finds unsupported edits; OCM is its transpose and finds requirements that
produced no edit at all.

V0 tests structural traceability only. `linked` does not mean implemented correctly, adequately
tested, observed passing, or valuable.

## Verified starting state

- CEM binds every textual patch hunk to immutable evidence, an explicit unknown, or a mechanical
  disposition.
- Corvint capability specs use stable requirement IDs and traceability tables.
- The claim-ledger prototype can anchor test names and docstrings, but no OCM wire, verifier, or
  independent implementation exists.
- Corvint has not yet measured whether obligation closure finds omissions or saves review time.

## Requirements

For CEM 0.2 profile dispatch and inherited CEM validation precedence, this specification
normatively incorporates `cem-0.2-canonical-binding.md`; OCM adds only its OCM schema, obligation,
claim, and mutation ordering deltas below.

- `OCM-V0-001`: V0 MUST accept exactly one pinned Markdown intent scope containing one
  `## Requirements` section and enumerate every ID matching the frozen requirement syntax below,
  without aliases, inference, normalization, or ranges. The enumeration MUST contain at least one
  requirement; an empty enumeration is invalid with exact code `missing-requirements` (accepted
  2026-09-01 by decision 0012). Duplicate requirement IDs are invalid. Linked verification MUST
  re-extract the exact requirement statement using the same inline-code, bold-colon, or bold-period
  form without loosening the line anchor or requirement-ID boundary.
  Amendment (2026-09-12, span contract): a line inside a CommonMark fenced code block (a run of at
  least three backticks or tildes indented at most three spaces opens it; only a run of the same
  character at least as long, followed by nothing but spaces or tabs, closes it; an unclosed fence
  runs to the end of the document) is not a heading. A fenced `## Requirements` does not count
  toward the exactly-one rule, and a fenced `## ` line does not end the section. The section ends
  at the first unfenced line that is exactly `##` or starts with `## `, so a bare `##` (an empty
  ATX heading) ends it.
- `OCM-V0-002`: the map MUST bind the full target commit, intent path/blob/span/span digest, exact
  CEM map digest, and exact CEM patch digest. The intent path at the target tree MUST resolve to the
  declared blob; orphaned, historical, or stale intent blobs are invalid.
- `OCM-V0-003`: the map MUST contain exactly one obligation row for every enumerated requirement;
  missing, surplus, or duplicate rows fail verification.
- `OCM-V0-004`: an obligation has exactly one disposition: `linked`, with non-empty verified CEM
  hunk and test-claim IDs, or `unknown`, with empty references and one bounded reason.
- `OCM-V0-005`: a linked test claim MUST be content-addressed, re-extractable from a target-revision
  blob/span, and contain the exact obligation ID in its test name, docstring, or table-case anchor.
  Clarification (2026-09-07, no new rule): a producer that adds a linked claim MUST run the same
  exact-ID closure validation on the candidate map it is about to write and refuse with the
  verifier's `claim-obligation-mismatch` before publication; an anchor whose match is only
  delimiter-adjacent (for example `_TM-V0-008_`) is not the exact obligation ID. A `link` that
  would publish a map the verifier rejects is a producer defect, not a verifier one.
  Clarification (2026-09-12, no new rule): a commented-out test is never a claim; a Go raw string
  has no escapes, so a backslash before its closing backtick cannot extend it over later comments.
  Clarification (2026-09-12, no new rule): in JS/TS blobs a `/` that is not a comment opener divides
  when the previous significant token is an identifier, number, string/template/regex literal,
  `)`, `]`, `}`, or postfix `++`/`--`, and otherwise (including after `return`, `typeof`,
  `instanceof`, `in`, `of`, `new`, `delete`, `void`, `throw`, `case`, `do`, `else`, `yield`,
  `await`) opens a regex literal that skips escapes and `/` inside `[...]`, so a quote in it opens no
  string. This lexical rule is an approximation; where it misreads, masking fails toward fewer claims
  (invariant 2): a regex candidate that its line ends before closing masks the rest of that line.
  Clarification (2026-09-12, no new rule): a `${` inside a JS/TS template literal opens an
  expression that its matching `}` closes back into the template, so a quote or backtick inside the
  expression cannot end the template and expose a later comment as code.
  Clarification (2026-09-13, no new rule): every claim's `blobOid` MUST equal the target tree entry
  at its path, including when an earlier claim at the same path already verified; a claim naming any
  other blob fails `repository-object-unavailable`.
- `OCM-V0-006`: the verifier MUST independently verify the referenced CEM and reject absent,
  invalid, drifted, or unknown hunk and claim references. It MUST dispatch on the exact CEM profile:
  `cem/0.1` derives with the historical fixed `.corvint/change.cem.json` convention, while `cem/0.2`
  requires and uses its validated exact `excludedPath`. Every OCM operation that verifies or links
  against a 0.2 CEM MUST receive independently supplied expected base and target arguments, require
  the CEM base and exact OCM target to match them, derive through the shared CEM implementation, and
  require the raw patch digest to match both maps. If the fixed sidecar exists at that target, its
  mode and raw bytes MUST pass the shared CEM artifact-binding rule. `link` performs all of this
  verification before changing an obligation, then mutates exactly the OCM bytes and resolves CEM
  hunk selectors against exactly the CEM bytes retained by that successful verification; it does not
  re-read either path. A semantic invalid verdict from the reader
  MUST refuse linking with its first issue code and optional message before selector, claim,
  or destination handling; a nil operational error does not mean that verification passed. A hunk
  selector that does not resolve to exactly one `supported` CEM hunk refuses with
  `unsupported-hunk-selector` (`internal/lrfrepo/ocm_write.go:784`).
- `OCM-V0-007`: `corvint ocm prepare|link|mark|status|verify|report` MUST provide a resumable,
  deterministic JSON-first workflow with numbered worklists and argv-array next actions.
  `prepare`, `link`, `status`, `verify`, and `report` MUST accept `--expected-base` and `--target` as
  defined below. After exact CEM profile dispatch both are independent authority for 0.2. Missing
  them fails `expected-base-required` then `target-required`; a resolved caller target different from
  exact OCM `targetRevision` fails `target-mismatch`. `mark` consumes no CEM, has no independent
  revision check, and accepts neither flag.
  Clarification (2026-09-13, no new rule): `prepare` fails `output-path-conflict` when its OCM path and
  CEM path name one file, whether equal after resolving an absolute path against the root or, when
  both exist, the same file through a case-folded alias, before any write.
  Amendment (2026-09-13, decision 0273): an explicit `link --output` that names the `--cem` file, or
  an explicit `mark --output` that names the fixed CEM path `.corvint/change.cem.json` (mark consumes
  no CEM), by the same resolved-path or file-identity test fails `output-path-conflict` before any
  write. `link` refuses it after the reader verdict check and before selector handling; `mark`
  refuses it after the map parses as OCM and before selector handling.
  Clarification (2026-09-13, no new rule): as in `CEM-PILOT-002`, only a file-system not-exist result
  makes the OCM map absent for `prepare`; an existing regular map it cannot read (for example one over
  the 1 MiB limit) fails `ocm-cli-error` and is left unchanged unless `--replace` is given.
  A readable same-binding map is resumed only after the same structural verification as a fresh read,
  using the already-read map bytes; without `--replace`, its first structural issue is returned before
  reuse. With `--replace`, a structurally invalid same-binding map is discarded for the fresh candidate.
- `OCM-V0-008`: status and reports MUST keep structural coverage separate from test execution.
  `linked` plus an unrun project gate MUST NOT become observed success.
- `OCM-V0-009`: same-change proposed intent MAY be an OCM scope but MUST NOT become historical CEM
  evidence. A bootstrap intent is an exact declared intent path absent at the CEM base; OCM
  bootstrap validation MUST require at least one matching CEM hunk and every matching hunk MUST
  remain explicitly `unknown`. Only after every final OCM map and the aggregate have independently
  verified against the same exact base, target, CEM map digest, and CEM patch digest MAY Corvint
  dogfood subtract those validated bootstrap unknowns before applying its configured CEM unknown
  ceiling. The raw CEM dispositions and counts remain unchanged. Missing, invalid, unavailable, or
  drifted OCM state grants no subtraction and fails closed. The OCM map remains a local reviewer
  artifact and is not part of the mapped patch. This dogfood-policy amendment changes no CEM or OCM
  wire profile and does not change general CEM CLI policy semantics. Accepted 2026-09-05 by
  decision 0055.
- `OCM-V0-010`: default solo use MUST allow visible unknown obligations; a repository MAY opt into
  `--max-unknown 0` without accounts, roles, a daemon, or a central policy service.
- `OCM-V0-011`: maps and reports MUST contain no source, diff, prompt, command-output, ticket-body,
  or test-result bodies. Paths, IDs, hashes, and selectors remain sensitive repository metadata.
- `OCM-V0-012`: parsing, Git access, counts, spans, paths, files, and subprocesses MUST be bounded,
  deterministic, network-free, and safe for untrusted repository content. OCM MUST use the shared
  CEM reciprocal-worktree validator: primary and real linked worktrees are valid; a symlinked `.git`
  marker, forged gitfile, or nonreciprocal administrative association fails
  `repository-object-unavailable`; an alternate object source fails
  `unsupported-object-alternates`. Implicit fetch is forbidden. Validated repository administrative
  metadata is authority and MUST remain stable for one invocation; hostile same-identity mutation is
  outside the V0 trust boundary.

## Wire profile

Frozen requirement-line syntax (inline-code, bold-colon, or bold-period):

```regex
^- (?:`([A-Z][A-Z0-9-]{2,31}-[0-9]{3})`:[ ]|\*\*([A-Z][A-Z0-9-]{2,31}-[0-9]{3})[.:]\*\*[ ])
```

The experimental profile is `ocm/0.1-experimental`. Its exact shape is:

```json
{
  "spec": "ocm/0.1-experimental",
  "targetRevision": "full lowercase Git commit OID",
  "intentScope": {
    "path": "repository-relative path",
    "blobOid": "full lowercase Git blob OID",
    "span": {"start": 0, "end": 1},
    "spanSha256": "64 lowercase hex"
  },
  "cem": {"mapSha256": "64 lowercase hex", "patchSha256": "64 lowercase hex"},
  "claims": [{
    "id": "claim:sha256:64-lowercase-hex",
    "extractor": "corvint-test-claim/1",
    "path": "repository-relative path",
    "blobOid": "full lowercase Git blob OID",
    "selector": "extractor-owned stable selector",
    "span": {"start": 0, "end": 1},
    "spanSha256": "64 lowercase hex"
  }],
  "obligations": [{
    "id": "OCM-V0-001",
    "disposition": "linked",
    "reason": "change-and-test-linked",
    "hunkIds": ["hunk:sha256:64-lowercase-hex"],
    "claimIds": ["claim:sha256:64-lowercase-hex"]
  }]
}
```

All fields shown are required; no other fields are allowed. Duplicate JSON keys are invalid. Spans
are zero-based, half-open UTF-8 byte offsets into the named blob. A span digest hashes its exact raw
bytes. CEM map and patch digests hash their exact out-of-band raw bytes. A claim ID is
`claim:sha256:` plus the SHA-256 of the canonical claim object with its `id` field omitted.

Canonical JSON is UTF-8 with keys sorted lexicographically, no insignificant whitespace, and one
terminal LF. Claim IDs and obligation IDs are unique. Claims and all reference arrays sort by ID;
reference arrays are strictly unique, and obligations retain the requirement order extracted from
the intent scope. Paths use the same normalized repository-relative grammar as CEM.
The intent span starts at the exact `## Requirements` heading byte and ends immediately before the
next level-two heading or at EOF. Claim-ID preimages use the same canonical object encoding with no
terminal LF; complete OCM wire documents retain the one terminal LF above.
The verifier derives the CEM patch to `targetRevision` with the shared CEM profile. For `cem/0.1` it
applies the historical fixed `.corvint/change.cem.json` convention and checks an independently expected
base when one is supplied. For `cem/0.2` it requires that independent base and first requires
`"excludedPath":".corvint/change.cem.json"`; every consuming invocation also independently supplies
and exactly matches the OCM target. A present target sidecar must be a `100644` blob byte-identical to
the raw CEM input. The verifier rejects a patch digest that does not match both the OCM binding and
verified CEM. OCM defines no separate exclusion, artifact-binding, or Git diff implementation.

### CEM-profile CLI contract

| OCM command | `cem/0.2` revision authority | `cem/0.1` revision authority |
|---|---|---|
| `prepare` | `--expected-base` and `--target` required; caller target becomes exact OCM target | `--target` remains required; `--expected-base` is an optional independent CEM-base check |
| `link` | `--expected-base` and `--target` required before selectors or mutation; target must equal OCM | both optional independent checks; omitted target retains historical OCM-target behavior |
| `status`, `verify`, `report` | `--expected-base` and `--target` required; target must equal OCM | both optional independent checks; omitted target retains historical OCM-target behavior |
| `mark` | neither accepted; no CEM is consumed | neither accepted; no CEM is consumed |

For each CEM-consuming command, a supplied `--expected-base` resolves to a full commit OID and must
equal CEM `baseRevision` or fail `base-revision-mismatch`. A supplied `--target` MUST be a full
lowercase commit OID, resolve locally to itself, and equal exact OCM `targetRevision` or fail
`target-mismatch`; invalid target syntax retains `invalid-target-revision`, while a locally
unavailable target retains `repository-object-unavailable`. When the Git verification budget
elapses or is exhausted, or the caller cancels, while resolving the OCM target, the refusal is the
bounded-runner code (`git-timeout`, `git-budget-exceeded`, or `git-cancelled`), because that
outcome does not show the object is missing. Neither authority may be inferred from
CEM or OCM when required. `prepare --target` is itself the caller authority used to create or match
OCM `targetRevision`; it is validated before the OCM is created or resumed.

Only `prepare` emits `nextActions` in the OCM V0 envelope: exactly one `status` argv array. For 0.2 it
MUST contain the resolved `--expected-base` and `--target`. For 0.1 it contains resolved
`--expected-base` only when supplied and retains the historical omission of `--target`. `link`,
`mark`, `status`, `verify`, and `report` emit no `nextActions` field. No prior invocation state is
persisted or inferred; all other existing action fields and command envelopes remain unchanged.
As in `CEM-CB-016`, the action's first element is the literal contract command name `corvint`, never
the invoked program path or the `corvint` build name (decision 0122).

For CEM 0.2 dispatch, repository validation, alternates, revision resolution, sidecar binding, and
patch verification, use the normative precedence in `cem-0.2-canonical-binding.md`. OCM's only
local deltas are: `prepare` validates CEM before a resumable OCM output, while `link`, `status`,
`verify`, and `report` validate OCM before CEM; after inherited verification, OCM validates its
binding/obligation/hunk/claim relations, then command selectors, mutation policy, rendering, and
the requested local write. No `link` write occurs before those checks complete.

Limits: 1 MiB map, one intent scope, 256 obligations, 512 claims, 64 hunk references and 64 claim
references per obligation, and 512-byte UTF-8 paths. The verifier performs no implicit fetch.

Unknown reasons are exactly `unassessed`, `no-test-claim`, `insufficient-evidence`, or
`conflicting-evidence`. `linked` uses reason
`change-and-test-linked`.

### Failure codes

The OCM parser, verifier and writers in `internal/lrfrepo` fail with the kebab-case codes below
(decision 0100), in addition to the codes named above. Each row cites the first emitting site and
quotes the message that site returns (`<value>` marks a substituted subject), which is the whole of
what the row asserts.

| Code | First emitting site | Message at the cited site |
|---|---|---|
| `ambiguous-claim-selector` | `internal/lrfrepo/ocm_write.go:821` | "claim selector does not identify exactly one claim" |
| `bootstrap-intent-not-unknown` | `internal/lrfrepo/ocm.go:692` | "bootstrap intent hunk must remain unknown" |
| `cem-map-digest-mismatch` | `internal/lrfrepo/ocm.go:411` | "CEM map digest does not match" |
| `cem-patch-digest-mismatch` | `internal/lrfrepo/ocm.go:437` | "CEM patch digest does not match" |
| `claim-conflict` | `internal/lrfrepo/ocm_write.go:937` | "claim ID has conflicting fields" |
| `claim-not-reextractable` | `internal/lrfrepo/ocm.go:712` | "claim cannot be re-extracted at target" |
| `duplicate-claim-reference` | `internal/lrfrepo/ocm_write.go:824` | "claim selectors must be unique" |
| `duplicate-hunk-reference` | `internal/lrfrepo/ocm_write.go:769` | "hunk selectors must be unique" |
| `duplicate-key` | `internal/lrfrepo/ocm_write.go:593` | "JSON contains a duplicate key" |
| `duplicate-obligation` | `internal/lrfrepo/ocm.go:293` | "obligation IDs must be unique" |
| `duplicate-or-unsorted-claims` | `internal/lrfrepo/ocm.go:247` | "claims must be sorted and unique" |
| `duplicate-or-unsorted-reference` | `internal/lrfrepo/ocm.go:358` | "<value> must be sorted and unique" |
| `fabricated-claim-id` | `internal/lrfrepo/ocm.go:245` | "claim ID is not content-derived" |
| `intent-scope-mismatch` | `internal/lrfrepo/ocm.go:453` | "intent scope is stale or invalid" |
| `invalid-cem` | `internal/lrfrepo/ocm_read.go:289` | "CEM verification failed" |
| `invalid-cem-digest` | `internal/lrfrepo/ocm.go:168` | "CEM binding digest is invalid" |
| `invalid-claim-blob` | `internal/lrfrepo/ocm.go:253` | "claim blob OID is invalid" |
| `invalid-claim-extractor` | `internal/lrfrepo/ocm.go:249` | "claim extractor is unsupported" |
| `invalid-claim-references` | `internal/lrfrepo/ocm_write.go:884` | "claim references must be non-empty and unique" |
| `invalid-claims` | `internal/lrfrepo/ocm.go:226` | "claims must be a bounded array" |
| `invalid-field` | `internal/lrfrepo/ocm.go:342` | "<value> must be a string" |
| `invalid-hunk-references` | `internal/lrfrepo/ocm_write.go:877` | "hunk references must be non-empty and unique" |
| `invalid-integer` | `internal/lrfrepo/ocm.go:312` | "<value> must contain non-negative integers" |
| `invalid-intent` | `internal/lrfrepo/ocm.go:544` | "intent scope must be UTF-8 Markdown" |
| `invalid-intent-blob` | `internal/lrfrepo/ocm.go:213` | "intent blob OID is invalid" |
| `invalid-intent-scope` | `internal/lrfrepo/ocm_write.go:541` | "intentScope must be an object" |
| `invalid-linked` | `internal/lrfrepo/ocm.go:756` | "linked obligation has invalid fields" |
| `invalid-map` | `internal/lrfrepo/ocm_read.go:249` | the code recorded for a verification failure that carries none, with that failure's message; `internal/lrfrepo/ocm_write.go:716` returns it as "OCM verification failed" |
| `invalid-object` | `internal/lrfrepo/ocm.go:322` | "<value> must be an object" |
| `invalid-obligation` | `internal/lrfrepo/ocm_write.go:67` | "selected obligation has no valid id" |
| `invalid-obligation-id` | `internal/lrfrepo/ocm.go:290` | "obligation ID is invalid" |
| `invalid-obligation-selector` | `internal/lrfrepo/ocm_write.go:53` | "decimal obligation selectors must be canonical and one-based" |
| `invalid-obligations` | `internal/lrfrepo/ocm.go:273` | "obligations must be a bounded array" |
| `invalid-ocm` | `internal/lrfrepo/ocm_write.go:162` | "<value> must be an object" |
| `invalid-path` | `internal/lrfrepo/ocm.go:210` | "path is invalid" |
| `invalid-reference-array` | `internal/lrfrepo/ocm.go:349` | "<value> must be a bounded array" |
| `invalid-reference-id` | `internal/lrfrepo/ocm.go:355` | "<value> contains an invalid ID" |
| `invalid-requirement-prefix` | `internal/lrfrepo/ocm.go:1335` | "requirement line is absent or ambiguous" |
| `invalid-requirements-section` | `internal/lrfrepo/ocm.go:556` | "intent must contain exactly one ## Requirements heading" |
| `invalid-selector` | `internal/lrfrepo/ocm.go:255` | "claim selector is invalid" |
| `invalid-span` | `internal/lrfrepo/ocm.go:315` | "<value> must be non-empty" |
| `invalid-span-digest` | `internal/lrfrepo/ocm.go:219` | "intent span digest is invalid" |
| `invalid-unknown` | `internal/lrfrepo/ocm.go:751` | "unknown obligation has invalid fields" |
| `invalid-unknown-reason` | `internal/lrfrepo/ocm_write.go:93` | "unknown reason is outside the allowlist" |
| `map-outdated` | `internal/lrfrepo/ocm_write.go:484` | "existing OCM map binds a different target, intent, or CEM" |
| `map-type` | `internal/lrfrepo/ocm_write.go:590` | "OCM root must be an object" |
| `missing-claim` | `internal/lrfrepo/ocm_write.go:808` | "at least one claim is required" |
| `missing-field` | `internal/lrfrepo/ocm.go:328` | "<value> is missing a required field" |
| `missing-hunk` | `internal/lrfrepo/ocm_write.go:759` | "at least one hunk is required" |
| `noncanonical-map` | `internal/lrfrepo/ocm.go:128` | "OCM bytes are not canonical" |
| `obligation-order-mismatch` | `internal/lrfrepo/ocm.go:746` | "obligations must retain requirement order" |
| `obligation-selector-out-of-range` | `internal/lrfrepo/ocm_write.go:59` | "obligation selector is outside the worklist" |
| `obligation-set-mismatch` | `internal/lrfrepo/ocm.go:735` | "obligations do not match requirements" |
| `ocm-cli-error` | `internal/lrfrepo/ocm_read.go:166` | one of "cannot read <label>", "<label> is not a regular file", "<label> exceeds the <limit>-byte limit", "<label> changed while being read", by the read failure kind |
| `output-path-conflict` | `internal/lrfrepo/ocm_write.go:211` | "OCM and CEM map paths must differ" |
| `unknown-claim-reference` | `internal/lrfrepo/ocm.go:770` | "linked claim is absent" |
| `unknown-field` | `internal/lrfrepo/ocm.go:333` | "<value> has an unknown field" |
| `unknown-hunk-reference` | `internal/lrfrepo/ocm.go:763` | "linked hunk is not verified and supported" |
| `unknown-obligation-id` | `internal/lrfrepo/ocm_write.go:120` | "obligation ID must select exactly one row" |
| `unsafe-output` | `internal/lrfrepo/ocm_read.go:426` | "output parent does not exist" |
| `unsupported-spec` | `internal/lrfrepo/ocm.go:154` | "OCM profile is unsupported" |

#### Dogfood aggregate failure codes

The `OCM-V0-013` aggregate in `internal/dogfoodocm` also fails with the kebab-case codes below
(decision 0100). Each row cites the first emitting site and quotes the message returned there,
adding in parentheses the limit checked; the bound checks run after the duplicate-requirement
check, in the order shown.

| Code | First emitting site | At the cited site |
|---|---|---|
| `invalid-json` | `internal/dogfoodocm/aggregate.go:168` | "verified OCM map could not be decoded" |
| `invalid-map` | `internal/dogfoodocm/aggregate.go:338` | "<value>: one OCM map failed verification", or the map path followed by the verification cause, when the failed map's first issue carries no code |
| `map-too-large` | `internal/dogfoodocm/aggregate.go:242` | "aggregate OCM map bytes exceed the limit" (summed map bytes over 1 MiB) |
| `too-many-claims` | `internal/dogfoodocm/aggregate.go:245` | "aggregate OCM claims exceed the limit" (summed claims over 512) |
| `too-many-obligations` | `internal/dogfoodocm/aggregate.go:248` | "aggregate OCM obligations exceed the limit" (summed obligations over 256) |

## Simpler baseline and non-goals

The baseline is the spec traceability table, CEM, project tests, and an agent-authored completion
summary. V0 does not add multiple specs, multi-repository evidence, automatic semantic linking,
test-execution attestations, signatures, external tickets, hosted coordination, UI, MCP/LSP, or a
new specification language. OCM does not define a second exclusion policy or Git-layout contract;
it consumes the profile dispatch and validated repository authority defined by CEM WP2.

## Acceptance and dogfood

Run OCM on the next ten substantive Corvint changes whose owning spec has at most 32 requirements.
Each advertised rejection needs a genuine-pass, fabricated-fail, and no-input fixture. Required
fixtures cover duplicate intent IDs, missing/surplus/duplicate obligations or references, stale
intent, wrong-target CEM, invalid CEM,
missing/ambiguous/moved test claims, proposed same-change intent, hostile fields/paths/symlinks,
limits, both CEM profiles, independent expected-base mismatch, a real linked worktree, forged and
symlinked Git metadata, and byte-identical reports from two fresh processes. Denied-path and
denied-repository fixtures must emit no forbidden path, OID, digest, selector, count, or derived
claim.

The 0.2 CLI matrix MUST exercise `prepare`, `link`, `status`, `verify`, and `report` with matching,
missing, and mismatched `--expected-base` and `--target`. Missing values fail
`expected-base-required` then `target-required`; mismatches fail `base-revision-mismatch` then
`target-mismatch` under the precedence above. A `link` mismatch MUST leave the map byte-identical.
Companion 0.1 cases preserve required prepare target and optional checks elsewhere. Fresh and resumed
`prepare` fixtures assert its sole exact `status` next action contains the profile-appropriate resolved
revision arguments; every other OCM command fixture asserts `nextActions` is absent. Adjacent
multi-defect fixtures cover every precedence stage, including invalid CEM plus missing authority,
both missing revision arguments, target mismatch plus forged target sidecar, and forged worktree plus
configured alternate. Integration fixtures accept target-side sidecar absence and an exact `100644`
raw-byte match, while forged bytes or mode `100755` fail `excluded-artifact-mismatch` through OCM.

The global `--root` names the sole requested worktree boundary. After reciprocal validation, its
per-worktree administrative directory and common local object store are authorized repository
metadata even when a supported linked-worktree layout places them outside the worktree directory.
Unrelated worktrees, siblings, alternates, worktree source paths, and implicit fetch remain denied.
An escaping path fails normalized parsing before access. An unavailable root, forged association,
or object emits only `repository-object-unavailable`; it must not echo an unverified path, OID,
digest, selector, count, or derived claim. Fixtures place canary values in a sibling repository and
permission-denied object store and assert that none occurs in stdout, stderr, or the report.

A scratch-repository dogfood fixture MUST prove both `OCM-V0-009` policy branches: a declared intent
absent at the base, with its matching CEM hunk unknown, passes OCM bootstrap validation and the
adjusted zero-unknown dogfood policy; adding any unknown hunk outside an OCM-validated bootstrap
scope fails `max-unknown-exceeded`. Both branches retain the raw CEM unknown disposition and count.

Promote only with 100% deterministic requirement enumeration, zero accepted dropped/surplus rows,
at most 5% reviewer-invalid links, median authoring overhead below three minutes and 10% of task
latency, zero access-boundary leaks, and either two baseline-missed omissions found or at least 20%
lower reviewer reconstruction time.

Subject to every hard and quality gate in the preceding paragraph, after ten changes promote if OCM
finds at least two baseline-missed omissions or saves at least 20% review reconstruction time. Kill
if it finds none and saves less than 10%. For the remaining cases, extend exactly once for ten more
changes, then promote only under the same complete gate set and otherwise kill. Median overhead above
five minutes, invalid links above 5%, fewer than half of changed requirements honestly linkable, or
any access leak kills immediately.

## Traceability

The `src/context_corvint_ocm*`/`src/corvint_cli.py` citations below are historical implementation
references, not live authority: decision 0012 R0
(`docs/decisions/0012-expert-panel-ratifications-2026-09-01.md`) rules the Python implementation
non-authoritative and slated for separate removal. The native OCM status/verify/report surface is
`internal/lrfrepo` plus `cmd/corvint` (`docs/specs/go-production-kernel-migration-v0.md`,
`GPK-V0-037`).

| Requirement | Implementation | Evidence |
|---|---|---|
| OCM-V0-001..006, 008, 010..012 | `src/context_corvint_ocm.py` | `tests/test_context_corvint_ocm.py` |
| OCM-V0-001 (native exact statement re-extraction) | `internal/lrfrepo/ocm.go` | `internal/lrfrepo/ocm_test.go:TestOCMLinkedVerificationAcceptsRequirementLineForms` |
| OCM-V0-001 (fenced headings are not headings; bare `##` ends the section) | `internal/lrfrepo/ocm.go` (`requirementsFromBlob`, `fencedLines`) | `internal/lrfrepo/ocm_test.go:TestRequirementEnumerationIgnoresHeadingsInsideFences`, `internal/lrfrepo/ocm_test.go:TestRequirementEnumerationBareATXHeadingEndsSection` |
| OCM-V0-005 (JS regex-literal versus division masking) | `internal/lrfrepo/ocm.go` (`maskComments`, `jsSlashOpensRegex`, `jsRegexEnd`) | `internal/lrfrepo/ocm_jsindex_test.go:TestJSRegexLiteralKeepsCommentsMasked` |
| OCM-V0-005 (JS template-expression masking) | `internal/lrfrepo/ocm.go` (`maskComments`) | `internal/lrfrepo/ocm_jsindex_test.go:TestJSTemplateExpressionKeepsCommentsMasked` |
| OCM-V0-005 (each claim blob OID matches its target tree entry when the path repeats) | `internal/lrfrepo/ocm.go` (`ocmBlobReader.blob`) | `internal/lrfrepo/ocm_anchor_test.go:TestOCMClaimBlobOIDVerifiedWhenPathRepeats` |
| OCM-V0-005, OCM-V0-006 | `internal/lrfrepo/ocm_write.go`, `internal/lrfrepo/ocm.go` | `internal/lrfrepo/ocm_anchor_test.go:TestOCMLinkRejectsFalseAnchorBeforePublication`, `internal/lrfrepo/ocm_anchor_test.go:TestOCMExistingFalseLinkRejected`, `internal/lrfrepo/ocm_selector_test.go:TestOCMFullHunkSelectors`, `internal/lrfrepo/ocm_selector_test.go:TestOCMRequirementIDSelectorsProduceClaims` |
| OCM-V0-006 (link mutates the exact OCM and CEM bytes retained by verification) | `internal/lrfrepo/ocm_read.go` (`OCMReadResult`), `internal/lrfrepo/ocm_write.go` (`LinkOCM`) | `internal/lrfrepo/ocm_anchor_test.go:TestOCMLinkMutatesTheVerifiedMapBytes` |
| OCM-V0-007 | `internal/lrfrepo/ocm_write.go` | `internal/lrfrepo/ocm_selector_test.go:TestOCMClaimSelectorMiss` |
| OCM-V0-007 (prepare refuses an absolute or case-folded alias of the CEM path as its output) | `internal/lrfrepo/ocm_write.go` (`PrepareOCM`), `internal/lrfrepo/ocm_read.go` (`sameInputFile`) | `internal/lrfrepo/ocm_prepare_test.go:TestPrepareOCMRefusesAliasOfCEMPath` |
| OCM-V0-007 (mark and link refuse an explicit output naming the CEM file; decision 0273) | `internal/lrfrepo/ocm_write.go` (`MarkOCM`, `LinkOCM`), `internal/lrfrepo/ocm_read.go` (`sameInputFile`) | `internal/lrfrepo/ocm_anchor_test.go:TestOCMMarkAndLinkRefuseOutputAliasOfCEMPath` |
| OCM-V0-007 (prepare refuses an existing unreadable map without `--replace`) | `internal/lrfrepo/ocm_write.go` (`resumeOrReplace`), `internal/lrfrepo/ocm_read.go` (`unreadableExistingMap`) | `internal/lrfrepo/ocm_prepare_test.go:TestPrepareOCMRefusesUnreadableExistingMap` |
| OCM-V0-007 (prepare structurally verifies already-read same-binding bytes before resume) | `internal/lrfrepo/ocm_write.go` (`resumeOrReplace`), `internal/lrfrepo/ocm_read.go` (`readOCMBytes`) | `internal/lrfrepo/ocm_prepare_test.go:TestPrepareOCMRefusesStructurallyInvalidSameBindingMap` |
| OCM-V0-005 (producer pre-publication exact-ID validation) | `internal/lrfrepo/ocm_write.go` (`LinkOCM`), `internal/lrfrepo/ocm.go` | `cmd/corvint/ocm_test.go` `TestOCMLinkRejectsClaimWithoutExactObligationIDBeforePublication`; `conformance/divergence-register.md` `DR-0031` |
| OCM-V0-009 | `internal/lrfrepo/ocm.go`, `script/dogfood-change.sh`, `script/dogfood-check.sh` | `internal/lrfrepo/ocm_prepare_test.go`, `script/dogfood-change_test.sh` |
| OCM-V0-006, OCM-V0-007; GPK-V0-008 | `internal/lrfrepo/ocm_write.go` | `cmd/corvint/ocm_test.go:TestOCMLinkRejectsInvalidReaderVerdict` (binding refusal, first-issue precedence, exact frozen-Python streams, repository/Git entries and modes, retained noncanonical refusal) |
| OCM-V0-006..012 | `src/context_corvint_ocm_workflow.py`, `src/corvint_cli.py` | `tests/test_context_corvint_ocm_workflow.py`, `tests/test_cli.py` |
| OCM-V0-012 (Git bound reached while resolving the OCM target keeps its runner code) | `internal/lrfrepo/ocm.go` (`verifyOCMBinding`, `gitBoundReached`) | `internal/lrfrepo/ocm_read_test.go:TestOCMBindingNamesGitBudgetExpiryNotMissingObject` |
| OCM-V0-013 | `script/dogfood-change.sh`, `script/dogfood-check.sh`, `internal/dogfoodocm`, `cmd/corvint/dogfood_ocm.go` | `internal/dogfoodocm/aggregate_test.go`, `script/dogfood-change_test.sh`, `cmd/corvint/ocm_test.go:TestDogfoodOCMArgumentErrorsAreInvalidArguments` |

Native selector diagnostic amendment to `OCM-V0-007` (2026-09-08): the owner's Task 2
follow-up explicitly requests printing the normalized fragment on a miss. A missing `/case:` selector retains
`claim-selector-out-of-range` and appends the extractor's bounded normalized case fragment only
when it differs from the supplied suffix. The hint neither selects a claim nor asserts that a
matching or unique claim exists. Exact selection and ambiguity refusal remain unchanged. Full
CEM hunk IDs already resolve; absent, stale, unknown, and mechanical hunks remain unsupported.
`prepare` initializes `claims: []`; `link --test-path` explicitly extracts claims from the pinned
test blob. An empty prepared claim list alone does not establish an extraction failure; Go `t.Run`
case anchors and JavaScript `test()`/`it()` titles naming a requirement ID extract at `link`.
These follow-ups preserve the frozen Python oracle. The new hint changes observable stderr;
DR-0032 records that difference as OPEN, with no discriminating frozen corpus row and no promotion.
The separate lane L DR-0031 false-anchor promotion hold also remains.

Delivery remains experimental until the ten-change dogfood and promotion gates above complete.

Rollback removes experimental OCM files and policy only. CEM, specifications, tests, and accepted
intent remain unchanged.

## Accepted amendment: ordered multi-intent dogfood closure

**Status: accepted 2026-09-01 by explicit repository-owner decision selecting design (b).** This
amendment does not change this document's proposed intent status, alter the
`ocm/0.1-experimental` wire profile, or change a conformance fixture or manifest. Its implementation
and evidence must be added to the traceability table before the widened dogfood scope may be
reported as delivered. No other proposed intent is accepted.

At revision `3b9397d`, `ARTIFACT-GO-V0-001..007` are not inside
`docs/specs/release-artifact-integrity-v0.md`'s sole `## Requirements` span. They are under the later
`## Accepted amendment: Go binary archive profile` heading. An unchanged OCM 0.1 map for that path
therefore enumerates `ARTIFACT-V0-001..010`, not the requested Go-archive obligations. Design (b)
requires a separate owner-reviewed, docs-only normalization that moves the already
accepted `ARTIFACT-GO-V0-001..007` clauses verbatim into that sole `## Requirements` section while preserving
decision 0010's authority and all traceability. Until that prerequisite lands, the widened dogfood
scope MUST fail closed rather than claim those seven obligations. That normalization makes one map
enumerate both `ARTIFACT-V0-001..010` and `ARTIFACT-GO-V0-001..007`; selecting only the seven Go
archive IDs would instead require a separately accepted owning intent document or a selector/wire
change and is not proposed here.

- `OCM-V0-013`: one dogfood change MAY declare an ordered set of pinned Markdown intent
  scopes and MUST prepare and independently verify exactly one unchanged `ocm/0.1-experimental`
  map per scope against the same exact target revision, expected base, CEM map digest, and CEM patch
  digest. `OCM-V0-001` and `OCM-V0-002` remain the per-map contract: each map contains exactly one
  intent path/blob/span/span digest and one ordered obligation sequence. The dogfood coordinator
  MUST sort scopes by normalized path bytes, then retain each map's extracted requirement order,
  when rendering its aggregate worklist and coverage. It MUST reject an absent or empty scope set,
  an absent map, any scope or map that is invalid, unavailable, stale, drifted, or bound to different
  revision or CEM bytes, and any requirement ID repeated within or across scopes. It MUST NOT
  deduplicate, merge,
  partially accept, or report aggregate closure after any such failure. Per-map failures retain the
  existing OCM taxonomy and MUST identify the failing map path and the underlying failed check's
  code and message, including repository and CEM failures whose standalone envelope omits or
  normalizes that detail. The standalone wire envelope remains unchanged. Given identical inputs
  and repository state, per-map acceptance MUST agree with standalone `ocm status`, including in
  reciprocal linked worktrees with a `.git` file and objects in the common Git directory.
  Aggregate-only failures are `missing-intent-scope`,
  `intent-scope-drift`, or the existing `duplicate-requirement`, with that precedence.

  The aggregate is dogfood coordination state, not an OCM map, consumable OCM bundle, or new source
  of intent. Ordinary OCM, LRF, TCQ, and Frontier consumers continue to receive exactly one map.
  Shared claim or hunk IDs may recur across maps; requirement IDs may not. The aggregate binds the
  sorted scope paths and verified map digests in `scopeSetSha256`: SHA-256 over the ASCII domain
  `corvint-dogfood-ocm-scope-set/0`, one NUL byte, then a no-terminal-LF canonical JSON array of exact
  `{path,mapSha256}` objects in scope order. It exposes each per-scope verdict plus one fail-closed
  aggregate verdict and contains no source or requirement bodies. Preparation and final
  verification MUST use fresh reads and keep the scope list, intent bindings, revision, and CEM
  bindings stable. Maps remain resumable and may change only through the existing verified `link`
  and `mark` mutations. The coordinator computes `scopeSetSha256` only from the final verified map
  bytes; `dogfood-check` freshly verifies every final map, recomputes that digest, and rejects any
  later list, binding, or map-byte drift. The coordinator accepts at most 16 scopes, 256 obligations,
  512 claims, and 1 MiB of aggregate raw map bytes; every existing per-map claim, reference, path,
  Git, repository, and network bound remains unchanged.

  The native aggregate reader is `corvint dogfood-ocm status --expected-base REV --target REV`.
  Missing or malformed command and flag arguments MUST report `invalid-arguments` on stderr and
  exit 2; they never become `internal-error` or enter aggregate verification.

  Under this design, `script/dogfood-change.sh`'s scalar `DOGFOOD_INTENT` input is superseded by
  `DOGFOOD_INTENTS_FILE`, which names a bounded LF-terminated manifest containing one normalized
  repository-relative intent path per line in strict bytewise order, with no blank lines, comments,
  or duplicate paths. The script reads those lines without shell word splitting, snapshots the
  validated list once, invokes the existing OCM prepare and status surfaces once per scope with
  distinct deterministic private map paths, records every per-scope
  step, and emits success only after the aggregate duplicate, binding, drift, and policy checks
  pass. After the source-normalization prerequisite above,
  `docs/specs/go-production-kernel-migration-v0.md` followed by
  `docs/specs/release-artifact-integrity-v0.md` therefore closes GPK obligations followed by the
  artifact spec's Python-wheel and `ARTIFACT-GO-V0-001..007` obligations; it does not concatenate,
  renumber, or reinterpret either source.

  This is the primary design because `GPK-V0-002`, `GPK-V0-003`, and `GPK-V0-037` make exact OCM
  0.1 bytes part of the Python-oracle parity surface, while decision 0007 and
  `script/agent-preflight.sh` keep `src/**` frozen until W15. The generic frozen requirement syntax
  already accepts `ARTIFACT-GO-V0-001`, as proved by
  `internal/lrfrepo/ocm_test.go:TestRequirementEnumerationAcceptsArtifactGoPrefix` in the
  `4b63e32` tree; this amendment changes scope composition, not requirement grammar.

  The rejected multiple-intents-in-one-map alternative would replace `intentScope` with an ordered
  array and therefore requires a separately versioned profile such as `ocm/0.2-experimental`, new
  canonical bytes and digests, and a Go-only compatibility state until the frozen Python oracle can
  mirror it or W15 retires that obligation. Silently widening OCM 0.1 is forbidden, and creating a
  blocked wire profile is unnecessary for this dogfood outcome. The rejected generated combined
  intent alternative would have to materialize a derived Markdown blob in the exact target tree to
  satisfy `OCM-V0-002`, freeze concatenation and provenance semantics, detect source drift and
  cross-source duplicates before generation, and keep a redundant intent document synchronized.
  That adds derived authority and failure modes while the original two specs already provide the
  immutable intent blobs.

  This amendment overlaps but does not answer the open CEM policy-exception question in
  `docs/agent-memory/questions.md`: every per-scope map consumes the same CEM policy outcome, and
  aggregate closure grants no exception authority. It also does not answer the open `lrfrepo`
  Python-claim question: multi-scope coordination does not change claim extraction, grammar,
  Frontier, or TCQ behavior.

  Decision 0055 later answered the dogfood-policy question without changing aggregate closure or
  either wire profile: only OCM-validated base-absent bootstrap unknowns are subtracted before the
  repository dogfood ceiling.

**Accepted owner decision:** “I accept proposed `OCM-V0-013` and select design (b): the dogfood
obligation scope is an ordered set of independently pinned `ocm/0.1-experimental` maps, with
fail-closed cross-scope duplicate and drift checks; the OCM 0.1 wire remains unchanged. I also
authorize a separate docs-only normalization moving the accepted `ARTIFACT-GO-V0-001..007` clauses
verbatim into `docs/specs/release-artifact-integrity-v0.md`'s sole `## Requirements` section while
preserving decision 0010 authority and traceability. I understand and accept that the widened scope
thereby also includes `ARTIFACT-V0-001..010`. Nothing else is newly accepted.”
