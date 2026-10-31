# Documentation corpus extraction, delta and rebuild V1

Owner: Russell Lewis
Date: 2026-10-24
Intent status: proposed
Delivery status: experimental

Authoritative inputs: native ticket V1-2525 and GitHub issues
[#1582](https://github.com/beamfall/corvint/issues/1582) to
[#1586](https://github.com/beamfall/corvint/issues/1586),
[#1599](https://github.com/beamfall/corvint/issues/1599),
[#1662](https://github.com/beamfall/corvint/issues/1662),
[#1664](https://github.com/beamfall/corvint/issues/1664) to
[#1667](https://github.com/beamfall/corvint/issues/1667) and
[#1700](https://github.com/beamfall/corvint/issues/1700). These requirements were split unchanged
from the [documentation corpus contract](documentation-corpus-v1.md), whose Requirements section was
over the 256-obligation OCM limit (`maxObligations`). Each keeps its former DCP-V1 number under the
DCX-V1 prefix, and the parent's stub at each former section carries the crosswalk. The split changes
no behavior, wire format, error code or receipt; the parent's input and authority boundary, failure
codes and acceptance gates still govern them.

## Agent digest
- Claim: Corpus extraction discovers tests, applies declared extractor profiles, computes revision deltas and claim patterns, and rebuilds from the embedded manifest.
- Status: proposed; experimental; DCX-V1-340..352, DCX-V1-360..376, DCX-V1-390..399 and DCX-V1-420..425 accepted under the owner's standing auto-accept (2026-10-11); the remaining blocks are implemented with focused tests.
- Exists: static test discovery, extractor profiles, whole-repository extraction, the cross-revision delta and carry-forward, claim patterns, the compact anchor table and provider, rebuild from the embedded manifest and corpus diagnostics, each with focused tests.
- Blocked on: owner acceptance and independent external utility qualification, as for the parent contract.
- Read next: Requirements; Failure modes; Traceability.

## User and current state

A maintainer builds and refreshes a corpus from a repository revision. These blocks discover tests
statically, apply repository-declared extractor profiles, extract a whole repository, compute the
worklist between two revisions and carry reviewed records forward, check claim mentions and
patterns, and rebuild an artifact from its own embedded manifest. Former IDs: DCP-V1-NNN is
DCX-V1-NNN for every ID listed here.

## Requirements

### Static test discovery (`docs corpus discover-tests`, GitHub #1582)

`corvint docs corpus discover-tests --runner playwright|rspec --revision FULL_COMMIT --path PATH`
reads the committed Playwright or RSpec test files under one repository-relative path at one full
commit through the TSA-V0 static readers and prints one `corvint-test-discovery/1` document on
standard output. It is a read command: it writes nothing to the repository, the index or a trace
store. The document is generated trust and sets no review verdict; it is test evidence an agent or
a provider can cite, not a corpus artifact. Status for `DCX-V1-370..376`: (accepted under the
owner's standing auto-accept (2026-10-11); GitHub #1582).

- `DCX-V1-370`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1582) The
  document names the runner, repository, full revision and path, lists each read file with its blob
  and SHA-256, and lists examples sorted by path, line and identity. Every title, tag, annotation,
  expectation, navigation, reach and hook carries an `external-provider` anchor whose blob, file
  digest and span digest are those of the committed lines, under the compiler's `maxSpanBytes`
  excerpt rule; a fact whose span cannot be anchored is not emitted with a zero-valued anchor but
  dropped with the example unknown `fact-span-unanchored` (DCX-V1-376). The bytes are identical
  across runs at the same revision. A runner other than
  `playwright` or `rspec`, an unsafe path or a missing argument is refused with `corpus-refused`, and
  a revision that is not a full commit with `corpus-invalid-revision`; the encoded document is
  bounded at 32 MiB. Acceptance: `TestDCXV1370DiscoverTestsDeterministicAnchored`,
  `TestDCXV1370DocsCorpusDiscoverTestsCLI`.
- `DCX-V1-371`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1582) Each
  example carries the identity `tests example-join` (`TSA-V0`) gives the same example: for RSpec the
  scoped id `spec/x_spec.rb[1:1]`, for Playwright the file and full title path, which form the
  `js-title-path` key once a run receipt supplies the project. An identity the source does not fix
  (a loop-generated, inline-included, shared-body or unplaced scoped id, or a non-literal title) is
  empty with its reason. An inline inclusion advances the scoped ids of its later siblings by the
  shared body's runtime slots, counted through nested inline inclusions; a loop in that body, or an
  inclusion that is unresolved or includes itself, leaves those later ids empty with
  `scoped-id-loop-generated` or `scoped-id-inline-include` rather than a guessed `[1:N]`. Acceptance:
  `TestDCXV1371DiscoverTestsIdentityJoins`.
- `DCX-V1-372`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1582) Each
  expectation names its matcher, negation, receiver and line span, including multi-line RSpec block
  expectations, whose matcher is read only from the `}` or `end` line that closes the expectation's
  own block (nested blocks are counted), never from an inner block's closing line. An expectation
  whose matcher or shape the reader cannot classify is kept with
  `matcher-not-read` or `assertion-shape-not-read`, never dropped. Acceptance:
  `TestDCXV1372DiscoverTestsExpectations`.
- `DCX-V1-373`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1582)
  Navigations and requests (Playwright `goto`, `route`, `waitForURL`, `waitForRequest`,
  `waitForResponse` and `request.<method>`; RSpec `visit` and request-spec verbs) in the example and its applicable hooks name the kind, method and literal target and say
  whether they come from the example or a hook; a computed target is `target-not-literal` with the
  rendered expression. Acceptance: `TestDCXV1373DiscoverTestsNavigations`.
- `DCX-V1-374`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1582)
  Statically reached modules are listed per example: page objects and helpers by static import,
  Playwright fixtures, RSpec shared examples and contexts by inclusion and files by `require` or
  `require_relative`, each with its declaration anchor and resolved repository path. A computed
  lookup, an unresolved import or an unregistered shared group carries the setup-select reason
  (`computed-lookup`, `import-unresolved`, `registry-unresolved`, `TSA-V0-003`). Acceptance:
  `TestDCXV1374DiscoverTestsReaches`.
- `DCX-V1-375`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1582) The
  hooks that apply to each example are listed outermost first with their kind, scope and whether the
  hook body asserts; tags and annotations come from the example's declaration, its enclosing groups
  and its runtime skip calls, and a conditional skip is marked conditional. Acceptance:
  `TestDCXV1375DiscoverTestsHooksTagsAnnotations`.
- `DCX-V1-376`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1582) What
  the readers cannot classify is an unknown with a reason and an anchor where one exists, never a
  guess or a silent drop: a non-literal metadata value, a project name that is not static, a file
  the reader cannot balance, a file over the reader's byte bound (`source-too-large`), a fact list
  over 512 entries (`fact-bound-exceeded`), a Playwright hook whose callback is not a literal
  function (`hook-body-not-literal`), an example span that cannot be excerpted
  (`example-span-unanchored`) and a fact span
  that cannot be anchored (`fact-span-unanchored`, for example an expectation line over 64 KiB),
  which is dropped rather than emitted without an anchor. Acceptance:
  `TestDCXV1376DiscoverTestsUnknowns`.

### Cross-revision delta worklist and carry-forward (`docs corpus delta`, `docs corpus carry-forward`)

`corvint docs corpus delta --previous OLD.json (--artifact NEW.json | --manifest NEW.json) [--limit N] [--offset N] [--uncovered changed|all] [--anchorless summary|list] [--format json|jsonl]`
compares a committed corpus artifact with the current revision and pages a worklist, and
`corvint docs corpus carry-forward --previous OLD.json (--artifact NEW.json | --manifest NEW.json) --provider ID`
emits the previous records of one provider whose span hashes still hold as a provider record for
the next build. Both are read commands: they read Git objects and the named local files, and write
only standard output. Neither changes a claim's text, trust or limitations. Status for
`DCX-V1-360..369`: accepted under the owner's standing auto-accept (2026-10-11); GitHub #1585.

- `DCX-V1-360`: `delta` prints one `corvint-corpus-delta/1` receipt: the previous and current
  sides (source `artifact` or `manifest`, artifact and manifest SHA-256, repository, tree,
  builder), `counts` for every class, `total`, `offset`, `limit`, `next_offset` when more entries
  follow, the page of `entries` and the `limitations`. `--previous` is read canonically
  (`ParseArtifact`). The current side is exactly one of `--artifact`, reopened and rederived
  (`Open`; amended by `DCR-V1-460` to the verified read), or `--manifest`, built in memory (`BuildAdmitted`) and never written. Both forms give the
  same entries and counts. Entries sort by class (`removed`, `changed`, `unresolved`, `uncovered`,
  `moved`, `carry-forward`), then record kind (region, subject, claim, relation), then ID. The CLI
  page is 1..256 entries, default 20; an offset past the end returns no entries. Acceptance:
  `TestDCXV1360DeltaWorklistClasses`, `TestDCXV1360DeltaCLI`.
- `DCX-V1-361`: The classified population is the previous corpus's subjects, claims and relations
  whose provider is a `records` provider. Native and compiler-derived records are rebuilt by the
  compiler: they are not classified, and their count is a limitation. Each entry names its class,
  record kind, ID, provider and reason, whether the current corpus already holds the ID, and each
  anchor's previous span and state. A record's class is its worst anchor's class. A record with no
  anchors is `unresolved`; by default it is counted in the `DCX-V1-422` summary instead of
  listed. Adoption-only subject kinds and relation types are classified like every other record
  (`DCX-V1-440`, amended by GitHub #1611). Acceptance: `TestDCXV1360DeltaWorklistClasses`.
- `DCX-V1-362`: Every anchor is re-validated against the current revision's blobs. A blob is read
  from Git and its SHA-256 checked, and the span is hashed with the same span digest and line
  numbering the build uses, so a span may end on the empty line after a final newline.
  - `carry-forward`: the span hash is confirmed at the same lines of the declared input. This
    covers both an unchanged blob and the same lines in a changed blob.
  - `moved`: the span hash is confirmed at exactly one other window of the same path, or at a span
    the symbol re-resolution of `DCP-V1-140..142` locates in another declared path. That span must
    hash to the same value, and a symbol anchor must find its symbol inside the re-pinned span.
  - `unresolved`: more than one window matches, or the hashing bound is reached.
  - `removed`: the path is no longer a regular file at the current revision, or the symbol is gone.
  A record whose span hash cannot be confirmed is never `carry-forward` or `moved`. Acceptance:
  `TestDCXV1360DeltaWorklistClasses`, `TestDCXV1366CarryForwardFeedsNextBuild`.
- `DCX-V1-363`: A `changed` anchor carries:
  - the previous span;
  - an `estimated` current span, mapped through a bounded Myers line diff of the two blobs;
  - up to 16 `changed_lines` hunks (1-based old and new starts and counts) that touch the span, with
    a truncation flag beyond 16.
  Lines the diff deleted with no replacement state that reason. A diff beyond 1024 edits is one
  `coarse` hunk, never a guessed alignment. A symbol anchor the symbol re-resolution reports
  changed keeps the resolver's estimated span and still carries the `changed_lines` of the previous
  blob against the current blob of the re-resolved path. Acceptance:
  `TestDCXV1360DeltaWorklistClasses`, `TestDCXV1363LineDiff`, `TestDCXV1360DeltaRepairs`.
- `DCX-V1-364`: For each current `source` input whose blob is new or changed, the inserted or
  replaced line ranges that no current records-provider claim covers are `uncovered` region entries.
  A previous claim's re-pinned current span counts as coverage; native compiler claims do not.
  Covered lines are subtracted, so each maximal uncovered sub-range of a changed range is its own
  region. Each region entry has:
  - the ID `PATH:START-END`;
  - the region's blob, SHA-256 and span hash;
  - up to 8 current subject IDs whose evidence overlaps it, plus the count left out.
  A new path, an unreadable previous blob or a coarse diff yields one whole-file range before
  subtraction. Inputs that cannot be read within the bounds are named in a limitation. In addition,
  every current subject that no current claim of any provider names as its subject is an
  `uncovered` entry of record kind `subject`, whether or not its lines changed, unless
  `--uncovered changed` counts them instead (`DCX-V1-421`); these entries are bounded by the corpus
  record capacity and paged with the rest. Acceptance:
  `TestDCXV1360DeltaWorklistClasses`, `TestDCXV1360DeltaCLI`, `TestDCXV1360DeltaRepairs`.
- `DCX-V1-365`: Carry-forward and moved records are demoted to `unresolved`, with a reason naming
  the missing ID, when a dependency is missing:
  - a claim whose subject is neither in the current corpus nor itself carried;
  - a relation with an endpoint in that state.
  Demotion repeats until stable across all providers. A dependency that only another provider
  carries therefore survives classification, and `carry-forward` for one provider refuses it
  (`DCX-V1-366`). Acceptance: `TestDCXV1360DeltaWorklistClasses`, `TestDCXV1360DeltaRepairs`.
- `DCX-V1-366`: `carry-forward --provider ID` emits a `corvint-corpus-provider/1` record. It names
  the previous provider's ID and version, and the current repository as its source. It holds that
  provider's `carry-forward` and `moved` records that the current corpus does not already hold.
  Rules:
  - each record is unchanged except that its anchors are re-pinned to the current revision (path,
    blob, SHA-256 and lines) with the same span hash;
  - a provider-reported `reviewed` trust is restored to its reported form, so the next build's
    import reproduces the previous trust, reported trust and limitations byte for byte;
  - each non-empty kind declares a `present` capability naming the previous corpus SHA-256;
  - the record must import on its own: when a carried claim's subject or relation endpoint is
    neither in the current corpus nor in the record, because another provider carries it, the
    operation is refused (`corpus-refused`). The refusal names up to 16 such dependencies and their
    carrying provider, so the caller carries and builds that provider first and then repeats.
    Records are never silently dropped or widened to another provider.
  Committed and declared as a records provider, the record builds without refusal. A second
  carry-forward against that build carries nothing. Acceptance:
  `TestDCXV1366CarryForwardFeedsNextBuild`, `TestDCXV1360DeltaRepairs`.
- `DCX-V1-367`: Reads are bounded and deterministic:
  - each blob is limited to 8 MiB and total admitted blob bytes to 64 MiB, both checked against
    the object header before the body is read;
  - every span digest the classification computes (unchanged-blob confirmation, previous span,
    placement windows, symbol re-resolution and estimated spans) is charged to a 64 MiB hashing
    bound before it is hashed; region digests are disjoint parts of read blobs and stay inside the
    read bound;
  - Git object reads are batched and tree lookups prefetched;
  - an exhausted Git budget is `corpus-capacity-exceeded`;
  - a blob past a bound, unreadable, or failing its recorded SHA-256 makes its anchor `unresolved`
    or `changed` with that reason, never confirmed;
  - the carry-forward record stays within 4096 records per kind and the provider encode bound, and
    is otherwise `corpus-capacity-exceeded`.
  The same inputs give byte-identical receipts and records. Acceptance:
  `TestDCXV1360DeltaWorklistClasses`, `TestDCXV1360DeltaRepairs`.
- `DCX-V1-368`: Neither operation writes the repository, the index, the working tree or any
  artifact. A `--manifest` current side is built in memory only. Acceptance:
  `TestDCXV1360DeltaWorklistClasses`, `TestDCXV1360DeltaCLI`.
- `DCX-V1-369`: The following are refused with exit 2 and no output; the option-form cases are
  argument errors and the others `corpus-refused`:
  - previous and current corpora that name different repositories;
  - a `--provider` that is not a `records` provider of the previous manifest;
  - both or neither of `--artifact` and `--manifest`;
  - a missing `--previous`, or a missing `--provider` for `carry-forward`;
  - an option the operation does not admit;
  - an invalid `--limit` or `--offset`.
  Local input files follow the `DCP-V1-054` path rule. An anchor under a non-`external-provider`
  authority is reported `unresolved` with its reason and never carried. An anchor in a declared
  additional repository is classified by `DCX-V1-420`, not excluded (amended by GitHub #1599).
  Acceptance: `TestDCXV1360DeltaWorklistClasses`, `TestDCXV1360DeltaCLI`,
  `TestDocsCorpusOperationFlagMatrix`, `TestDocsCorpusOperationHelpPrintsFlags`.

### Claim mention check and corpus patterns (`docs corpus patterns`)

A built corpus mechanically checks the code-like tokens its generated claims name, and
`corvint docs corpus patterns --artifact PATH [--format json|table]` reports deterministic
signals of generated-text failure modes. Both make no semantic judgment: a resolved mention is not
a correct claim, a flagged pattern is a sampling signal rather than a verdict, and neither changes
a claim, its evidence, its trust label or any other gap. Status for `DCX-V1-340..352`: accepted
under the owner's standing auto-accept (2026-10-11); GitHub #1584, #1586.

- `DCX-V1-340`: One documented ASCII extractor (`ExtractCodeTokens`) returns a claim's distinct
  code-like tokens in order of first appearance. A single-backtick span on one line is one
  candidate (its trimmed content). Outside backticks, text splits on whitespace; each chunk loses
  leading quotes and opening brackets, trailing quotes and sentence punctuation (`.,;:!?`), and a
  trailing `)`, `]`, `}` or `>` whose opener does not occur in the chunk, in time linear in the
  chunk. An uppercase HTTP method
  chunk (`GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, `OPTIONS`) followed by a chunk starting
  with `/` is one candidate. A candidate is then, in order: `route` (`METHOD /path`), `scope`
  (`A::B`), `member` (`A#b`, or dotted parts that are identifiers of two or more bytes and are
  backticked, called with `()`, contain a snake_case/camelCase/internally cased part, or have an
  uppercase later part, unless it is a file name), `path` (a slash path of `[A-Za-z0-9_./@-]`
  without `//` or a scheme whose last segment has an extension of 1 to 8 letters or digits, or a
  file name: a backticked, uncalled dotted name of identifier parts whose last part is one of the
  fixed extensions `bash c cc cfg conf cpp cs css csv go gradle h hpp htm html ini java js json jsx
  kt lock md mjs mod php proto py rb rs scss sh sql sum svg swift toml ts tsx txt vue xml yaml yml`,
  so `main.go` is a path and `os.stat` a member), `identifier` (snake_case, camelCase, PascalCase with an
  internal lower-to-upper transition, or `name()`), else `code` when backticked; anything else is
  not a mention. A candidate over 256 bytes (`MaxMentionBytes`) is not a mention; at most 64 tokens
  per claim (`MaxClaimMentions`) are checked and the rest are counted as `omitted`. The pattern
  report's `token-disjoint` pattern (`DCX-V1-348`) reuses this extractor. Acceptance:
  `TestDCXV1340ExtractCodeTokens`.
- `DCX-V1-341`: The build checks every claim of a non-native provider, except a supplied test-run
  receipt (`DCP-V1-211`), and stores one `corvint` artifact member `claim_mentions`, sorted by
  claim ID, holding each claim with at least one token: its subject, state, the counts
  `resolved_in_span`, `resolved_elsewhere`, `unresolved` and `omitted`, and every mention's token,
  kind, resolution and target. The member is part of the artifact digest; `Open` rederives it and
  refuses a stored artifact whose member differs. A corpus with no token-bearing checked claim has
  no member, so its bytes are unchanged. Acceptance: `TestDCXV1341ClaimMentionCheckIsBuiltAndPinned`.
- `DCX-V1-342`: A token is `resolved-in-span` when the text of the claim's own cited anchor spans
  at the pinned revision contains it (whole-word for identifier ends), its `()`-stripped form, for
  a route both its path and method (case-insensitive), for a path an anchor path equal to it or
  ending in `/` plus it, or for a scope or member every identifier component. Otherwise it is
  `resolved-elsewhere`, with `target` and `target_kind`, at the first match of: a corpus subject ID
  or name (`subject`, smallest subject ID); an indexed symbol name or its last component, or a
  subject anchor symbol (`symbol`, smallest `path:line`); for a path token, a tracked path or path
  suffix (`path`, smallest path). Otherwise it is `unresolved`. A claim is `consistent` with no
  unresolved mention, `unresolved` when all are unresolved, else `partially-unresolved`.
  Acceptance: `TestDCXV1341ClaimMentionCheckIsBuiltAndPinned`.
- `DCX-V1-343`: `get`, `trace`, `claims` and every receipt that returns a claim attach
  `claim_mentions` keyed by claim ID for each returned claim that has a stored check, on the v1
  and paginated v2 receipts and through the `corvint.docs_*` MCP tools and HTTP companion, which
  share those readers. The claim's text, evidence state and trust are unchanged. Acceptance:
  `TestDCXV1343ClaimMentionsAreServedWithTheClaim`, `TestDCXV1349CorpusPatternsCLI`.
- `DCX-V1-344`: `gaps` reports a `claim-mentions-unresolved` or `claim-mentions-partially-unresolved`
  gap on each such claim; the reason states the unresolved count out of the mention count and names
  up to 8 unresolved tokens, ending `, ...` when more exist. Acceptance:
  `TestDCXV1344ClaimMentionGapsAndCoverage`.
- `DCX-V1-345`: When a check is stored, coverage adds the rows `claim_mentions:consistent`,
  `claim_mentions:no-mentions`, `claim_mentions:partially-unresolved` and
  `claim_mentions:unresolved`, each a count over all checked claims at the corpus revision with a
  limitation that the check is mechanical and raises no trust. Acceptance:
  `TestDCXV1344ClaimMentionGapsAndCoverage`.
- `DCX-V1-346`: `docs corpus build --max-unresolved-mention-rate R` (R a decimal in [0, 1]) refuses
  a corpus whose checked claims with an unresolved mention, over checked claims with any mention,
  exceed R. The refusal is `corpus-refused` with `corpus build threshold exceeded:` naming the
  numerator, denominator, rate and threshold; nothing is written. A rate with no token-bearing claim
  is undefined and never refuses. Without the flag, the build is unchanged; a given flag with an
  empty value (`--max-unresolved-mention-rate=`) is an argument error (exit 2), never an absent
  threshold. Acceptance:
  `TestDCXV1346UnresolvedMentionThreshold`, `TestDCXV1349CorpusPatternsCLI`.
- `DCX-V1-347`: A claim's subject, a relationship's source or target, a journey's subject and an
  observation's test subject that names no corpus record refuse the build with `corpus-refused`;
  dangling references are never served. This pins existing build closure as the reference half of
  the consistency check. Acceptance: `TestDCXV1347DanglingReferencesRefuse`.
- `DCX-V1-348`: The pattern report (`corvint-corpus-patterns/1`) binds the artifact like an audit
  sample and examines the claims the mention check examines. It lists, in order, `token-disjoint`
  (at least one mention and none resolved in span), `templated-text` (a normalized template of at
  least 3 words, after lowercasing and replacing mentions, the subject name and digit runs, shared
  by claims on at least 3 distinct subjects), `anchor-cluster` (at least 5 claims cite an identical
  anchor), `broad-anchor` (a cited anchor of at least 200 lines, or a whole-file anchor of at least
  20 lines) and `uniform-shape` (within a subject kind of at least 10 claims, the most common shape
  of first three template words and a five-word length bucket covers at least 80%). Each pattern
  states its rule and thresholds, count, denominator, rate (abstaining at zero), group count and
  its sorted claim IDs, at most 256 (`MaxPatternClaims`) with the rest counted as `omitted`. The
  same artifact gives byte-identical output. Acceptance: `TestDCXV1348CorpusPatternReport`.
- `DCX-V1-349`: `docs corpus patterns --artifact PATH` prints the JSON report (default) or, with
  `--format table`, its text view. A missing artifact or an unknown format is an argument error
  (exit 2); `--artifact` follows the `DCP-V1-054` rule. Acceptance: `TestDCXV1349CorpusPatternsCLI`,
  `TestDocsCorpusOperationFlagMatrix`, `TestDocsCorpusOperationHelpPrintsFlags`.
- `DCX-V1-350`: `audit sample --stratify patterns` places each claim in its first flagged pattern
  in report order, else `unflagged`, and allocates the size by largest remainder in proportion to
  stratum population times a weight of 4 for a flagged stratum and 1 for `unflagged`, capped at
  each stratum's population, with ties by stratum name. Ranking, output and determinism follow
  `DCP-V1-265`; the sample's limitation states that flagged strata are over-sampled. Acceptance:
  `TestDCXV1350StratifyPatternsOverSamples`.
- `DCX-V1-351`: For a stratified sample, `audit report` adds `weighted`: the sum over populated
  strata of (stratum population / population) times stratum accuracy, with its population and
  definition and no interval. It abstains, naming the undefined strata, when any populated
  stratum's accuracy is undefined. `audit report` refuses a sample whose strata counts are not
  `0 <= sampled <= population` within the corpus record bound, whose strata names repeat, with a
  claim naming an unlisted stratum, whose stratum sampled count differs from its claims, or whose
  strata population and sampled totals differ from the design's. Acceptance:
  `TestDCXV1351WeightedAuditFigure`.
- `DCX-V1-352`: `docs corpus build --max-pattern-rate R` (every pattern) or
  `--max-pattern-rate NAME=R[,NAME=R]` refuses a corpus whose named pattern rates exceed their
  thresholds, as in `DCX-V1-346`, naming every exceeded threshold. An unknown or repeated pattern
  name, an empty value or a rate outside [0, 1] is an argument error (exit 2). Acceptance:
  `TestDCXV1352PatternRateThresholds`, `TestDCXV1349CorpusPatternsCLI`.

`DCX-V1-420..425`: accepted under the owner's standing auto-accept (2026-10-11); GitHub #1599 and
GitHub #1598 proposal 2; V1-2472. They amend `DCX-V1-361`, `DCX-V1-364` and `DCX-V1-369` so a delta
over a corpus whose sources live in `DCP-V1-175` additional repositories carries what still holds,
and a comparison of identical inputs is empty apart from carry-forward.

- `DCX-V1-420`: An anchor whose repository is a declared additional repository of the current
  manifest is re-validated exactly as a primary anchor (`DCX-V1-362`, `DCX-V1-363`), against the
  revision it was pinned to while the current manifest still pins that revision for the
  repository (`revision` or `revisions`, `DCP-V1-308`), and otherwise against the repository's
  current declared `revision`. Its blobs and tree entries are read only from the
  repository's declared checkout, as the build reads them (`DCP-V1-176`); its declared inputs are
  the current manifest's inputs at that revision; the checkout's root commit must equal the
  declared identity. Each such anchor names its `repository` in the delta entry; primary anchors
  omit the member, so their bytes are unchanged. A confirmed anchor is re-pinned to that
  repository's revision, path, blob, SHA-256 and lines with the same span hash, so `carry-forward`
  emits it and the next build imports it. A symbol anchor must find its symbol inside the re-pinned
  span in that repository's index; cross-path symbol re-resolution stays primary-only
  (`DCP-V1-179`). Every repository shares the `DCX-V1-367` read and hashing bounds, and blobs are
  batched per repository and pinned revision. The read bound counts the bytes actually read across
  all repositories together, batched prefetch included: each batch admits bodies only within what
  is left of the one shared 64 MiB bound, and a body a batch read is not charged again when the
  classification uses it. An anchor whose blob no longer fits is `unresolved` with the bound reason.
  A re-pinned additional-repository span never counts as coverage of a primary path. Acceptance:
  `TestDCXV1420DeltaAdditionalRepositoryAnchors`, `TestDCXV1420DeltaSharedReadBound`.
- `DCX-V1-421`: `--uncovered changed|all` selects the uncovered entries; the default is `all`, the
  `DCX-V1-364` worklist. `changed` lists only the changed-region entries and reports the number of
  subject-level entries it left out as `uncovered_subjects_omitted`. A comparison of identical
  inputs under `changed` lists no `uncovered` entry. Acceptance:
  `TestDCXV1420DeltaAdditionalRepositoryAnchors`, `TestDCXV1421DeltaViewsCLI`.
- `DCX-V1-422`: `--anchorless summary|list` selects how records with no anchors appear; the default
  is `summary`. `summary` lists none of them as entries. Instead `anchorless` reports one row per
  provider and record kind, sorted by provider then kind, with its count and the reason. `list`
  gives each its own `unresolved` entry, as before. Summarized records are still `unresolved` for
  the `DCX-V1-365` demotion and are never carried. `counts` and `total` count listed entries
  only. Acceptance: `TestDCXV1420DeltaAdditionalRepositoryAnchors`, `TestDCXV1421DeltaViewsCLI`.
- `DCX-V1-423`: `--format jsonl` classifies once and prints the whole worklist without paging.
  The first line is a `corvint-corpus-delta-stream/1` header: both sides, `counts`, `total`,
  `limitations`, `anchorless` and `uncovered_subjects_omitted`. Each later line is one entry, in
  worklist order, byte-identical to that entry in the paged receipt. The stream is bounded by the
  128 MiB corpus byte bound, and past it the operation is refused as `corpus-capacity-exceeded`.
  `--format json`, the default, keeps the paged receipt. The views of `DCX-V1-421` and
  `DCX-V1-422` apply to both formats. Acceptance: `TestDCXV1420DeltaAdditionalRepositoryAnchors`,
  `TestDCXV1421DeltaViewsCLI`.
- `DCX-V1-424`: The same inputs give the same worklist whichever builder built the previous corpus:
  only the anchors, records and current blobs decide a class, never the builder identity. A
  corpus rebuilt by a newer builder from identical pinned inputs is therefore all carry-forward.
  This holds for an additional repository pinned at more than one revision: an anchor at a
  further pinned revision is resolved at that revision (`DCX-V1-420`), so a comparison of such a
  corpus with itself is all carry-forward. Acceptance:
  `TestDCXV1420DeltaAdditionalRepositoryAnchors`, `TestDCXV1424DeltaHistoricalPins`.
- `DCX-V1-425`: These are refused with exit 2 and no output as argument errors:
  - `--uncovered` other than `changed` or `all`;
  - `--anchorless` other than `summary` or `list`;
  - `--format` other than `json` or `jsonl`;
  - `--format jsonl` together with `--limit` or `--offset`;
  - any of these options on `carry-forward`.

  These anchors are `unresolved` with a reason naming the repository and are never carried:
  - an anchor naming a repository the current manifest does not declare;
  - an anchor whose declared checkout cannot be opened or does not hold the declared identity and
    revision.

  A missing checkout does not stop `delta` before classification. When a declared additional
  checkout of a retained `--artifact` current side does not hold its identity at every pinned
  revision, the artifact cannot be rederived, so `delta` takes it from its canonical,
  digest-checked bytes instead. The primary checkout must still hold its declared identity, and
  every input of an available repository must name the regular blob its path holds at its pinned
  revision, or the operation is `corpus-refused`. The unavailable repository's anchors are then
  `unresolved` with the reason, other repositories classify as usual, and `limitations` states
  that the current side was not rederived. `carry-forward` and a `--manifest` current side still
  require the checkout, because they emit or build records for the next build. Acceptance:
  `TestDCXV1420DeltaAdditionalRepositoryAnchors`, `TestDCXV1421DeltaViewsCLI`,
  `TestDocsCorpusOperationFlagMatrix`, `TestDCXV1425DeltaUnavailableCheckoutCLI`,
  `TestDCXV1425OpenDeltaCurrentUnavailableCheckout`.

`DCX-V1-440..445`: accepted under the owner's standing auto-accept (2026-10-11); GitHub #1611.
They amend `DCX-V1-361`, `DCX-V1-365` and `DCX-V1-366`, so the documentation records an adoption
provider supplies (flows, paragraphs and their relations) carry across revisions like generic
records. Which records carry forward:

| Record | Carries forward |
|---|---|
| Records-provider subject, claim or relation of a generic kind | yes, as before (`DCX-V1-366`) |
| Adoption subject kind (`flow`, `paragraph`, `variation`, `coverage_definition`, `ticket`, `intent_comparison`, `inventory`, `manual_case`) or relation type (`asserts`, `navigates`, `retired_to`, `manual_case_for`, `migration_mapping`) | yes, with its typed details (`DCX-V1-440`, `DCX-V1-443`) |
| Record with no anchors | no: `unresolved`, summarized by default (`DCX-V1-422`) |
| Native or compiler-derived record | no: rebuilt by the compiler and counted in a limitation |
| Journey, observation or supplied gap | no: not classified, stated in `limitations` |

- `DCX-V1-440`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1611)
  A previous record of an adoption-only subject kind or relation type is classified
  exactly as a generic record (`DCX-V1-362`, `DCX-V1-363`, `DCX-V1-420`). When every anchor is
  confirmed it is `carry-forward` or `moved`. When only some anchors changed it is `changed`, and
  its entry lists every anchor with its own state, so the changed anchors carry their spans and
  `changed_lines`. Acceptance: `TestDCXV1440DeltaAdoptionRecords`.
- `DCX-V1-441`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1611)
  Missing evidence is never guessed. An adoption record whose kind requires typed
  details (every adoption subject kind except `variation`, `inventory` and `manual_case`, and
  `asserts`, `navigates` and `migration_mapping` relations) is `unresolved` with the reason
  `adoption record kind KIND requires typed details that the previous corpus does not hold` when
  the previous corpus holds none for it. A record whose kind is in neither the generic nor the
  adoption vocabulary is `unresolved` with that reason. A record with no anchors stays
  `unresolved` (`DCX-V1-422`).
  Acceptance: `TestDCXV1440DeltaAdoptionRecords`.
- `DCX-V1-442`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1611)
  The `DCX-V1-365` dependencies of a carried record include every record ID its
  typed details join to: a flow's section paragraphs, variations and entrypoints; a paragraph's
  flow and `retired_to` target; a coverage denominator; an intent comparison's lists; and a
  supplied status's supporting records. A carried record with such an ID neither in the current
  corpus nor carried is `unresolved`, with the reason `typed detail reference ID is neither in the
  current corpus nor carried forward`. A demotion re-checks the records that depend on the demoted
  ID until the carried set is stable. A carried `exact` test join whose file is not among the
  anchors its target test has at the current revision (the current subject's, or the carried
  subject's re-pinned anchors) is `unresolved` with that reason. `carry-forward` refuses a typed
  detail reference that only another provider carries, as it does a claim subject (`DCX-V1-366`).
  Only the referenced ID's presence is checked otherwise; the next build's own join validation
  (paragraph section and stable ID, referenced kinds) still governs a record that joins carried
  and current typed records. Acceptance: `TestDCXV1440DeltaAdoptionRecords`,
  `TestDCXV1442DeltaExactTestJoinNeedsItsFile`.
- `DCX-V1-443`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1611)
  When the provider imported as an adoption provider (its previous `import_parity`
  has a `typed_detail` row), or its carried records include an adoption kind or a record with
  typed details, `carry-forward` emits a `corvint-corpus-adoption-provider/1` record. It holds those
  records under the `DCX-V1-366` rules, plus each carried record's typed details unchanged under
  `details`. The bound is 100,000 records per kind instead of 4,096. Such a record imports only
  under a `corvint-corpus-input/2` manifest. Against a current `/1` manifest the operation is
  `corpus-refused` and names the required schema. Otherwise the record is the
  `corvint-corpus-provider/1` record of `DCX-V1-366`. Committed and declared, the adoption record
  builds with the same typed details, and a second carry-forward against that build carries
  nothing. Acceptance: `TestDCXV1440DeltaAdoptionRecords`.
- `DCX-V1-444`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1611)
  A delta of a corpus with itself (`--previous` equal to `--artifact`) lists no
  `unresolved` entry under the default views, in both the paged receipt and the jsonl stream.
  Every anchored records-provider record is `carry-forward`, including flows, paragraphs and their
  relations, with anchors in the primary repository, in a declared additional repository, or at an
  additional repository's further pinned revision. This holds however many records share one
  span. Each distinct verified span (repository, path, blob, lines) is charged to the `DCX-V1-367`
  hashing bound once, and its digest is reused from a cache of at most 65,536 spans. Only a digest
  actually computed is cached, and each anchor's own span hash must still equal it, so a span that
  was never hashed is never confirmed. Acceptance: `TestDCXV1440DeltaAdoptionRecords` over the
  additional-repository and multi-revision fixtures, and
  `TestDCXV1444SelfDeltaSharedSpanChargedOnce` (1,025 records sharing one 64 KiB span).
- `DCX-V1-445`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1611)
  The `docs corpus delta` and `carry-forward` help purposes, and so `help --json`
  and the capability catalog, state that adoption kinds such as flows and paragraphs carry forward
  with their typed details. Acceptance: `TestCAPV0019CommittedCatalogMatchesTheBinary`,
  `TestMCPV0100HelpToolMatchesTheCLI`, `TestDocsCorpusHelpListsOperations`.

### Repository-declared extractor profiles (issue 1583)

- `DCX-V1-390`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1583) A
  repository declares extractors in a closed `corvint-extractor-profile/1` JSON file committed at
  the requested revision: `provider` and `version`, and 1 to 64 extractors, each with an `id`, a
  generic `subject` (`endpoint`, `job`, `event`, `data_entity`, `ui_surface` or `symbol`), 1 to 64
  repository path or glob `files` (`*`, `?` and `**`; `node_modules` is never walked), an RE2
  line `pattern` of at most 4,096 bytes and 16 capture groups with the named groups its subject
  needs, and optionally a
  `candidate` pattern, a `field_pattern`/`end_pattern` pair (data entities only), a `role`
  (`declares`, `produces` or `consumes`, required for and only for `job` and `event`) and a
  `tool_output` pin. Profiles are configuration: they add no subject kind, authority or Core
  taxonomy, and an unknown member, duplicate id, missing or extra group, or invalid pattern is
  refused before any source file is read. A `**` file glob matches in time linear in the pattern and
  path segment counts, however many `**` segments it has. Acceptance:
  `TestDCXV1390ExtractorProfileIsClosed`, `TestDCXV1390ExtractorGlobMatchIsLinear`.
- `DCX-V1-391`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1583) Each
  matched line becomes one generic subject `PROVIDER:KIND:NAME` (an endpoint is named `METHOD
  path`, a data-entity field `entity.field` with kind `data_entity`, and a produce or consume site
  an `interaction` subject), carrying one exact anchor per site: repository, revision, path, Git
  blob, file SHA-256, the matched line span and its span SHA-256, authority `external-provider`
  and evidence kind `source`. Acceptance: `TestDCXV1391ExtractorEmitsGenericSubjectsWithExactAnchors`.
- `DCX-V1-392`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1583) Relations
  are derived only from exact names: `implements` from a `handler` symbol to its endpoint or UI
  surface when exactly one `symbol` subject with one site carries that name, `calls` from a
  produce interaction to its declared job or event, `depends_on` from a consume interaction to its
  declared job or event, and `related_to` from a data-entity field to its entity. A relation
  carries both ends' anchors: the site that derives it and the other end's subject anchors (the
  handler definition, the job or event declaration, or the entity declaration), at most 64.
  Acceptance: `TestDCXV1392ExtractorDerivesImplementsAndCallsRelations`,
  `TestDCXV1392ExtractorRelationsCiteBothEnds`.
- `DCX-V1-393`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1583) The
  inventory's `provider` member is a `corvint-corpus-provider/1` record with `generated` trust whose
  source is the extracted revision. Committed unchanged and declared as a provider input, it builds
  through the existing provider admission with every anchor resolving to a declared input;
  claims, journeys and observations are `unsupported` and gaps `not-collected`. Acceptance:
  `TestDCXV1393ExtractorProviderRecordBuildsUnchanged`.
- `DCX-V1-394`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1583) A
  `tool_output` extractor names its `tool` and the SHA-256 of one committed output file; it never
  runs the tool. A file whose bytes match the pin yields subjects with evidence kind `imported`
  and derivation `imported`; a mismatch yields a `tool-output-digest-mismatch` gap and no subject.
  Acceptance: `TestDCXV1394ExtractorReadsOnlyPinnedToolOutput`.
- `DCX-V1-395`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1583) What an
  extractor cannot read or recognize is an explicit inventory gap with extractor, path, line and
  reason, never a guess: `missing-file`, `not-regular-blob` (a symlink or submodule), `file-over-bound`,
  `not-utf8`, `no-files-matched`, `line-over-bound`, `unsupported-shape` (a `candidate` line the
  pattern does not match), `multiple-matches`, `invalid-capture`, `field-outside-entity`,
  `unresolved-handler`, `ambiguous-handler`, `undeclared-reference` (a produce or consume of a name
  no extractor declares) and `anchor-bound`. A `multiple-matches` or `line-over-bound` entity
  declaration ends the open entity, so later fields are `field-outside-entity` gaps rather than
  fields of the previous entity. At most two matches per line are evaluated, and the record bound is
  enforced as each line is scanned, so a dense file is refused without anchoring all of it.
  Acceptance: `TestDCXV1395ExtractorReportsUnreadableFilesAndShapesAsGaps`,
  `TestDCXV1395AmbiguousEntityDeclarationClosesTheEntity`,
  `TestDCXV1395ExtractorDenseFileIsRefusedWhileScanning`.
- `DCX-V1-396`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1583) `job` and
  `event` join the generic provider subject kinds; a record using them is otherwise admitted
  unchanged. Acceptance: `TestDCXV1396JobAndEventAreGenericSubjectKinds`.
- `DCX-V1-397`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1583) When a
  corpus declares any `job` or `event` subject, a flow relation of kind `produces` or `consumes`
  that otherwise joins also checks its `name` against those subjects: a match records the sorted
  `name_subjects`, and no match makes the join `unresolved` with the reason naming the undeclared
  name. A corpus without job or event subjects joins byte-for-byte as before. Acceptance:
  `TestDCXV1397FlowRelationNamesCheckDeclaredJobAndEventSubjects`.
- `DCX-V1-398`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1583)
  Extraction reads only Git objects at the full revision through the verified object session,
  within the existing per-file and walk bounds, writes nothing, and returns identical bytes for
  identical inputs. Acceptance: `TestDCXV1398ExtractorIsDeterministicAndReadOnly`.
- `DCX-V1-399`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1583)
  `corvint docs corpus extract --profile PATH --revision FULL_COMMIT` prints exactly the producer's
  `corvint-extractor-inventory/1` bytes; a missing profile or a non-full revision is refused with
  no output. Acceptance: `TestDCXV1399DocsCorpusExtractCLI`, `TestDocsCorpusOperationFlagMatrix`,
  `TestDocsCorpusHelpListsOperations`.

### Whole-repository extraction, delta argument presence and derivation notes (GitHub #1700, #1667)

`docs corpus extract` emitted only a `corvint-corpus-provider/1` record, bounded at 4096 records
and 4 MiB, so a repository with 13,051 definition sites was refused (V1-2574, GitHub #1700), and
the refusal named neither the count nor the extractor. `docs corpus delta` and `carry-forward`
chose the current side by value, so `--manifest= --artifact=corpus.json` was read as an artifact
instead of refused (V1-2543). And no release note named the merges that changed the derivation
(V1-2545). Status for `DCX-V1-560..564`: proposed, experimental; GitHub #1700 and #1667.

- `DCX-V1-560`: Amending `DCX-V1-393` and `DCX-V1-399`, `docs corpus extract --schema 2` (and
  `ExtractorInput.Schema` `2`) emits the inventory's `provider` member as a
  `corvint-corpus-adoption-provider/1` record, which a manifest `/2` admits: the subject, relation,
  matched-file and gap bounds are `MaxCorpusRecords` (100,000) and the record is bounded by
  `MaxProviderBytes` (64 MiB); the inventory schema and every other member are unchanged. Without
  `--schema`, or with `--schema 1`, the output is byte-identical to before. Any other value is
  refused with `--schema must be 1 or 2` and no output. Failure modes: a manifest `/1` still
  refuses an adoption record; the output is deterministic for identical inputs; nothing is written.
  Acceptance: `TestDCXV1560ExtractorSchemaTwoAdmitsMoreThanTheRecordBound` (14,000 sites in 14
  files, past #1700's 13,051, build through a manifest `/2`; refused before the change),
  `TestDCXV1560DocsCorpusExtractSchemaCLI`, `TestDocsCorpusOperationFlagMatrix`.
- `DCX-V1-561`: Amending `DCX-V1-395`, an extraction over its record bound is refused with
  `extractor inventory exceeds the record bound: N KIND at extractor "ID" path P, over the bound of
  B`, where `N` is the count reached (bound plus one), `KIND` is `sites`, `gaps`, `matched files`,
  `subjects` or `relations`, the extractor and path are named where one crossed it, and `B` is the
  schema's bound. A `/1` refusal adds that `--schema 2` admits 100,000. Failure modes: the refusal
  is still raised while scanning, so a dense file is not anchored whole (`DCX-V1-395`). Acceptance:
  `TestDCXV1560ExtractorSchemaTwoAdmitsMoreThanTheRecordBound`,
  `TestDCXV1395ExtractorDenseFileIsRefusedWhileScanning`.
- `DCX-V1-562`: A provider record within the record bound but over its schema's input byte bound
  (4 MiB for `/1`, 64 MiB for the adoption schema) is refused at extraction with `extractor provider
  record is N bytes, over the SCHEMA input bound of B bytes`, a `/1` refusal adding that `--schema 2`
  admits 67,108,864 bytes, instead of printing a record that build refuses. Acceptance:
  `TestDCXV1562ExtractorRefusesARecordOverTheInputBound`.
- `DCX-V1-563`: Amending `DCX-V1-360` for `delta` and `carry-forward`, the current side is chosen
  by flag presence, as `build` does (`DCX-V1-495`): exactly one of `--artifact` and `--manifest`
  must be given, and the given one must be non-empty; `--manifest= --artifact=PATH` in either order,
  an empty value alone, or both flags exit 2 with `requires exactly one of --artifact or --manifest`
  on stderr, no stdout and no write. Acceptance: `TestDCXV1563DeltaCurrentSideIsDecidedByPresence`
  (`--manifest= --artifact=corpus.json` exits 0 before the change).
- `DCX-V1-564`: `docs/RELEASE-NOTES.md` has a `Corpus derivation versions` section naming, for each
  derivation version from 1 to the running `DerivationVersion`, the merge (`#N`) that introduced it,
  what changed, and the rebuild command `corvint docs corpus build --artifact`. A merge that raises
  `DerivationVersion` without an entry fails the check. The owner may move or reformat the section;
  the check moves with it. Acceptance: `TestDCXV1564ReleaseNotesListEveryDerivation`.

Non-goals: no sharded extractor output (a record over 64 MiB stays refused, naming its bytes), no
change to the default `/1` output, no new manifest or provider schema, and no automatic release-note
generation. Rollback: drop `--schema` from `extract` and the `ExtractorInventory` encode case,
restore the plain record-bound refusals, restore the value-based delta check and remove the
release-note section and its test. No stored state or artifact changes, so nothing needs migration.

### Experimental compact per-file anchor table (GitHub #1664)

A `corvint-corpus-adoption-provider/1` anchor repeats its file's repository, revision, path, blob
and whole-file hash, and its authority, evidence kind and reason: about 530 bytes on #1664's
corpus, where about 28,500 exact one-line anchors added about 15 MB of provider shards. An optional
compact record encoding states each file once. Status for `DCX-V1-487..489`: proposed,
experimental; GitHub #1664. `DCX-V1-472` already charges provider shards apart; neither 128 MiB
bound changes.

- `DCX-V1-487`: Amending `DCP-V1-033`, an adoption provider record or shard may instead use
  `corvint-corpus-adoption-provider/2`: the `/1` members plus `files`, a table of
  `{repository?, revision?, path, blob, sha256}` (at most 100,000 entries), and optional
  `anchor_defaults` `{authority?, evidence_kind?, reason?}`. Every anchor of a `/2` record
  (evidence `anchors` and observation `step_evidence`) is compact: a required zero-based `file`
  index, `start_line`, `end_line`, `span_sha256`, and optional `symbol`, `region`, `authority`,
  `evidence_kind` and `reason`. Decoding expands each compact anchor to the `/1` anchor it stands
  for: an absent file repository or revision is the record's `source` one, an absent
  authority, kind or reason is the default's, and a present member, even empty, is kept. The record is then admitted exactly as the `/1`
  record with those anchors, under the same manifest `/2` requirement, anchor checks, per-shard
  bound and charges on the compact pinned bytes, and builds the same artifact records; `/1`
  records and shards are read unchanged. Acceptance: `TestDCXV1487CompactAnchorTableEquivalent`.
- `DCX-V1-488`: A compact record is closed. A missing or out-of-range `file` index, a `/1` file
  member inside a compact anchor, an unknown table or default member, a file table over 100,000
  entries, or a `files` or
  `anchor_defaults` member in any other provider schema is refused as a located closed-JSON
  refusal (`DCX-V1-470`) naming the input and JSON pointer, whatever the member order; an expanded anchor that fails a `/1`
  anchor check is refused as that check refuses it. Acceptance: `TestDCXV1488CompactAnchorRefusals`.
- `DCX-V1-489`: The compact encoding is measured on a synthetic adoption record of #1664's shape,
  28,500 anchored inventory subjects over 17,400 files (and over 2,000 files), against the same
  record in `/1`. Its anchor bytes are at most 50% (35% over 2,000 files) of the `/1` anchor
  bytes, it expands to exactly the `/1` record, and both aggregate bounds stay 128 MiB.
  Acceptance: `TestDCXV1489CompactAnchorSizeMeasured`; measured in
  https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-corpus-compact-anchor-table.md.

Failure modes: a compact record a producer writes against the wrong source revision expands to
anchors that fail the `/1` revision check, never to anchors at another revision; a `/2` record
under a `/1` manifest is refused as an adoption record is. Non-goals: the builder writes no `/2`
record (only the explicit `compact-provider` producer of `DCX-V1-531` does); the artifact, its anchors and receipts keep their
format, so no derivation version changes (`DCX-V1-471`); the span hash stays per anchor. A `/2`
record decodes in about twice the time of its `/1` form. Rollback removes the `/2` decode path;
`/1` records are unaffected, and corpora pinning `/2` records are refused until rewritten as `/1`.

### Corpus rebuild from the embedded manifest (GitHub #1667)

`DCX-V1-471` admits an artifact from another attested builder revision of the same derivation,
but an artifact whose derivation, builder schema or Go toolchain changed could only be refreshed
by re-supplying the manifest it was built from. Status for `DCX-V1-495..498`: proposed,
experimental; GitHub #1667. This delivers the rebuild that the next section lists as not
delivered.

- `DCX-V1-495`: `corvint docs corpus build` takes exactly one of `--manifest INPUT.json` or
  `--artifact CORPUS.json`. With `--artifact` it builds the manifest that artifact embeds, under
  the same thresholds, admission and pin verification as `--manifest`, and prints the new
  artifact to stdout; it writes no file, the artifact read included. The manifest is read without
  the closed record decode or any builder, derivation or pin check, so an artifact whose records
  this builder no longer decodes still yields it. Its output bytes equal `build --manifest` of the
  same manifest, so rebuilding a current artifact reproduces it. Neither or both options, or
  an empty value for the one given, are an argument error with no output. Acceptance: `TestDCXV1495RebuildFromEmbeddedManifest`,
  `TestDCXV1495BuildRebuildsFromEmbeddedManifest`, `TestDocsCorpusOperationFlagMatrix`,
  `TestDocsCorpusOperationHelpPrintsFlags`.
- `DCX-V1-496`: Before any Git read or build, the rebuild refuses (`corpus-refused`, no output)
  bytes that are not JSON, whose last `sha256` member is not the canonical tail or does not match
  the content digest, whose schema is not a `corvint-evidence-corpus/` schema, that exceed the
  artifact size bound of their schema (4 MiB for `/1`, 128 MiB for `/2`), that embed no
  manifest or a manifest that does not decode closed, or whose `manifest_sha256` does not match
  the embedded manifest. Every pinned input is then reread from Git and refused unless its blob
  and SHA-256 match the manifest (`input changed after pinning`). Acceptance:
  `TestDCXV1496RebuildRefusesUnverifiedManifest`, `TestDCXV1495BuildRebuildsFromEmbeddedManifest`.
- `DCX-V1-497`: Amending `DCX-V1-471`, every `builder differs:` refusal of the verified read ends
  with the explicit rebuild (`docs corpus build --artifact <artifact>` into a new file). A read
  never rebuilds or writes: a refused read runs no build and leaves the artifact file's bytes
  unchanged. `docs corpus build --help` names what changes the derivation version and that a
  rebuild goes to a new file renamed over the old one. Acceptance:
  `TestDCXV1497ReadNeverRebuilds`, `TestDCXV1471VerifiedReadAcrossBuilderRevisions`,
  `TestDCXV1495BuildRebuildsFromEmbeddedManifest`.
- `DCX-V1-498`: A corpus MCP server answers its first call from an artifact recorded by another
  attested builder revision of the same derivation (`DCX-V1-471`). One whose derivation differs
  is refused as `corpus-refused`, naming both versions and the rebuild. Once the artifact is
  rebuilt into a new file renamed over the served one, the running server refuses with its
  restart message (`DCP-V1-143`) and a restarted server answers. Acceptance:
  `TestDCXV1498MCPServesArtifactsAcrossBuilderUpgrades`.

### Compact provider producer and rebuild reuse (GitHub #1664, #1667)

The compact `/2` encoding of `DCX-V1-487` had only a reference encoder inside a test, and
`build --artifact` rederived every record even when the running builder's derivation was the
artifact's own: about 11 s and 1.6 GB on #1667's corpus. Status for `DCX-V1-530..534`: proposed,
experimental; GitHub #1664, #1667.

- `DCX-V1-530`: `doccorpus.EncodeCompactProvider` writes a `/1` adoption provider record as the
  `/2` record that expands to it: its anchors index a file table in first-appearance order, a file
  omits its repository or revision when it is the record's `source` one, and with defaults an
  anchor omits each authority, kind or reason equal to the default's; a member that differs, even
  an empty one, is written, so it expands to itself and not to the default. Its bytes equal the
  `DCX-V1-487` reference encoder's for the same record and defaults. A record of any other schema
  is refused, as is a table over the 100,000-entry bound. Acceptance:
  `TestDCXV1530CompactEncoderMatchesReference`.
- `DCX-V1-531`: `corvint docs corpus compact-provider --input PROVIDER.json` reads one `/1`
  adoption provider record or shard within its per-shard bound (128 MiB) and prints its `/2`
  encoding, with the most frequent anchor authority, kind and reason as `anchor_defaults` (each
  tie going to the byte-smallest value; none when the record has no anchor). It refuses with no
  output: unreadable or malformed input, a `/2` or non-adoption schema, an output over the
  per-shard bound, and a result that does not decode back to exactly the input record. The output
  is deterministic, and the build admits the same records from it as from its input. It writes no
  file, and the caller repins the manifest to the new bytes. Acceptance:
  `TestDCXV1531CompactProviderProducer`, `TestDCXV1531CompactProviderCommand`,
  `TestDocsCorpusOperationFlagMatrix`, `TestDocsCorpusOperationHelpPrintsFlags`.
- `DCX-V1-532`: Amending `DCX-V1-495`, `build --artifact` reuses the artifact's records instead
  of rederiving them when all of these hold: the running builder is attested; the artifact
  decodes closed and canonical; the verified read admits its recorded builder (`DCX-V1-471`: same
  builder schema, derivation version and engine, Go toolchain included); its embedded manifest,
  manifest digest, profile digest and schema pairing match the manifest being built; and, after
  `DCX-V1-496`'s full pin rereads every pinned input from Git, the primary revision resolves to
  its recorded tree. Only the builder and the content digest are then rewritten, and the query
  index is admitted as `build` admits it (`DCP-V1-196`). The printed bytes equal a full build of
  the same manifest. Because `--artifact` always builds the embedded manifest, every shard digest
  matches, so reuse is of the whole artifact. Acceptance:
  `TestDCXV1532RebuildReusesSameDerivation`, `TestDCXV1532BuildArtifactReusesRecords`.
- `DCX-V1-533`: Any other artifact is rebuilt in full, to the same bytes. This covers another or an
  unrecorded derivation, another engine, digests or a tree a full build would not reproduce, and
  any artifact under an unattested running builder. The verified read still refuses an
  unrecorded-derivation artifact (`DCX-V1-471`). A pin refusal is the full build's own refusal.
  Acceptance: `TestDCXV1533RebuildRederivesOtherwise`, `TestDCXV1532BuildArtifactReusesRecords`.
- `DCX-V1-534`: The rebuild is measured on a synthetic inventory of 9,500 files (38,010
  subjects, 28,511 claims, 28,500 provider anchors, 62 MB artifact). Each mode runs in its own process,
  and both print the bytes of a full build. Acceptance: `TestDCXV1534RebuildReuseMeasured` (env-gated);
  measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-corpus-compact-producer-and-rebuild-reuse.md.
- `DCX-V1-813`: (proposed, experimental under the standing lane auto-accept; V1-2565) A pin that
  only verifies, the reuse pin of `DCX-V1-532` and the post-compile re-pin of a full build
  (`DCP-V1-040`), MUST pin each revision through `contextindex.BuildRevisionPin`: the identity,
  tree listing, admission and verified source pins of `BuildRevisionContext`, and so its refusals,
  without compiling the symbol, import, vocabulary and term tables nothing downstream of such a pin
  reads. The opening pin of a full build keeps `BuildRevisionContext`. The printed bytes and every
  refusal are unchanged. Acceptance: `TestDCXV1532RebuildReusesSameDerivation`,
  `TestDCXV1533RebuildRederivesOtherwise` unchanged; `TestDCXV1534RebuildReuseMeasured` at 9,500
  files on linux/WSL2 2026-10-31, base then head test binaries alternated five times on one
  prepared corpus at load 1.4 to 4.6: reuse median 2.14 s / 443 MiB before, 1.87 s / 411 MiB
  after (the ticket's bar was 2.52 s / 421 MiB); full rebuild 4.23 to 4.46 s before, 3.87 to
  4.02 s after. Rollback: drop `verifyOnly`, so every pin compiles its index.

Failure modes: a hand-edited artifact whose records were re-signed, and whose recorded builder
the verified read admits, is reused as is. The verified read already serves such an artifact
without rederiving it, so the rebuild extends it no further trust. A full rebuild is the
remedy, by changing the derivation or rebuilding with `--manifest`. A Go toolchain change still
forces a full rebuild, because the engine is part of the admitted derivation. Non-goals: no
per-shard record cache, no persisted state, and no change to the artifact, manifest, provider or
receipt encodings. Rollback removes `compact-provider`, `EncodeCompactProvider` and the reuse
path. `build --artifact` then rederives every artifact as `DCX-V1-495` did, and `/2` records
already written stay readable.

### Corpus diagnostics, builder upgrades, eval cost and ranking residuals (GitHub #1662, #1664 to #1667)

A caller-labelled adoption on main-730c6e0 reported five residuals. A malformed provider shard
was refused as `invalid closed JSON input` with no file, record or position (#1666). The 128 MiB
aggregate input bound counted provider record shards with the source inputs and named no
contributor (#1664). `docs corpus eval` gave no returned rows for a failing case and paid a
checkout stamp and freshness lookup per case (#1666). Every builder revision made every existing
artifact unreadable (`builder differs`), although the derivation was unchanged (#1667). Five
`docs_search` and `docs_claims` ranking misses remained (#1665). A caller-owned extractor profile
(#1662) could not be used at all. Status for `DCX-V1-470..477`:
proposed, experimental; GitHub #1664 to #1667. Status for `DCX-V1-478` and `DCX-V1-503..506`:
proposed, experimental; GitHub #1662, implemented under the orchestrator's 2026-10-24 decision,
owner acceptance open.

- `DCX-V1-470`: A closed JSON refusal of a named local input (a manifest, a provider record or
  shard, an extractor profile) names its location: `invalid closed JSON input: <input>: <JSON
  pointer>[: record <id>]: <reason> (offset <byte>)`. The pointer is the decoder's JSON pointer of
  the first invalid value, or `/` for a top-level error. The record ID is the `id` string of the
  top-level array element the pointer enters, when it is printable and at most 256 bytes. The
  reason is `unknown member`, `expected <kind>, got <kind>` or the syntax error, and the whole
  message is bounded. An artifact refusal keeps the bare message, which the streamed and
  in-memory decoders must report identically (`DCR-V1-431`); the decision is unchanged and every
  refusal stays `corpus-refused`. Acceptance: `TestDCXV1470LocatedDecodeRefusal`.
- `DCX-V1-471`: The builder engine names a derivation version (`native-contextindex/<schema>/
  derivation-<N>/<go version>`), raised by any change that alters a derived record, index or
  encoding byte for the same inputs. The verified read (`DCR-V1-410`) admits an artifact whose
  builder differs from the running one only in an attested revision: the builder schema, the
  derivation version and the rest of the engine must be equal, and the recorded revision must be
  attested: a full lowercase Git object ID of 40 or 64 hex digits, so an unrecorded, `+dirty`,
  empty or malformed revision is not. Otherwise it refuses with `builder differs:` and the
  member that differs. A changed derivation version names both versions and asks for a rebuild;
  an artifact from before this requirement has no recorded version and is refused as
  `unrecorded`. `--verify-rederive` still compares the rederived bytes, builder included.
  `TestDCXV1471DerivationVersionGuard` pins the digest of a fixture artifact with the builder
  blanked to the derivation version, so a derived-byte change without a version raise fails the
  test. Acceptance: `TestDCXV1471VerifiedReadAcrossBuilderRevisions`,
  `TestDCXV1471DerivationVersionGuard`, `TestDCRV1410VerifiedReadRefusals`.
- `DCX-V1-472`: Amending `DCP-V1-033` and `DCP-V1-157`, the pinned bytes of a corpus's distinct
  inputs are charged to two bounds: the provider record shards (`purpose: provider`) to the
  aggregate provider record bytes (128 MiB), and every other input to the aggregate pinned input
  bytes (128 MiB). Every distinct input is charged before the refusal, which names the bound that
  overflowed, both classes' totals and limits, and the three largest inputs of that class by size,
  then path. The per-shard bound (64 MiB) and the 128 MiB artifact bound are unchanged, so a corpus
  with a large provider record and large sources can still be refused at the artifact.
  Acceptance: `TestDCXV1472ProviderRecordBytesChargedApart`, `TestDCPV1157CapacityRefusalsAreTypedAndEarly`.
- `DCX-V1-473`: Amending `DCP-V1-252`, a failing eval case also carries `returned_rows`: its
  served top-k rows by rank with record ID, kind, name (or claim text) and the first path they
  cite, each bounded. A missed answer case whose `k` is below the query maximum also carries
  `expected_rank`: the first rank an expected item is ranked at when the same call is repeated at
  the maximum limit, or nothing when none is. The repeat is never served, so it skips the MCP
  result frame bound, and it changes no score, latency, byte count or baseline comparison. A passing case carries neither. Acceptance:
  `TestDCXV1473FailingCasesListReturnedRows`.
- `DCX-V1-474`: An eval suite opens the verified artifact and observes the checkout's freshness
  once before its first case, and answers every case through `CallTimed` from that artifact and
  overlay. No per-case checkout stamp or freshness lookup runs. After the last case it checks once
  that the checkout did not change: an equal checkout stamp passes without Git; otherwise every
  pin must still resolve and the observation key must equal the overlay's, or the evaluation is
  refused (`checkout changed while the cases ran`). The session is carried only by `Evaluate`'s
  context, so a served MCP call never takes it. Acceptance: `TestDCXV1474EvalSuiteObservesOnce`,
  `TestDCPV1251EvalScoresThroughTheMCPToolPath`; per-case cost in
  https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-corpus-diagnostics-and-ranking.md.
- `DCX-V1-475`: Amending `DCS-V1-402`, a `docs_search` subject whose name field (`DCS-V1-400`,
  `DCS-V1-401`) holds the query's words as a contiguous run of whole words orders after the
  exact matches and before every other hit. Words split at every rune that is not a letter or
  digit and at case changes, and keep stop words, so `logout_redirect_url` and
  `LogoutRedirectURL` match each other and the phrase `on deck` matches the path segment
  `on_deck`. A query of one identifier also matches inside a prose name. A query of fewer than two
  words has no phrase. A phrase match is a hit on its own, even when no query term scores the
  subject (the phrase `as is`, both stop words, finds `app/as_is.rb`); every subject the kind
  filter keeps is checked once, after an allocation-free test for the phrase's longest word. The
  word split is separate from the frozen evidence tokenizer and changes
  no indexed term. Acceptance: `TestDCXV1475PhraseAndIdentifierMatchesRankFirst`,
  `TestDCSV1404SearchRankingEvalFixture`.
- `DCX-V1-476`: Amending `DCP-V1-313` and `DCP-V1-328`, a claim's ID and text also match the
  regular inflections of a query term of three or more lowercase letters: `-s`, `-es`, `-ed`,
  `-ing`, a doubled final consonant (`set`, `setting`) and a dropped final `e` (`revoke`,
  `revoking`, `revoked`). A matched inflection counts its query term once. The forms are looked up
  in the existing claim-term postings, so the index and its encoding are unchanged. Acceptance:
  `TestDCXV1476ClaimTextMatchesInflections`.
- `DCX-V1-477`: Amending `DCP-V1-313` and `DCS-V1-402`, ties no longer order by ID string.
  `docs_claims` with `query` orders claims holding as many distinct query terms by the summed idf
  of the terms they hold, then the shorter text, then natural ID order, in which each run of
  digits compares by value (`claim:209` before `claim:2004`). `docs_search` orders equal scores
  subjects before claims, then the shorter name or text, then natural ID order. Index and scan
  readers keep the same order. Acceptance: `TestDCXV1477MeaningfulTieBreaks`,
  `TestDCSV1404SearchRankingEvalFixture`.
- `DCX-V1-478`: (proposed, experimental; GitHub #1662; owner acceptance open) `docs corpus
  extract --profile PATH --profile-sha256 HEX --revision FULL_COMMIT` admits a caller-owned
  `corvint-extractor-profile/1` file outside the extracted repository. With `--profile-sha256`,
  `PATH` must be a clean absolute path whose directory, with symlinks resolved, lies outside the
  repository root, compared by filesystem identity of every ancestor so a case alias of the root is
  still inside. The file is read along that resolved path with every directory component pinned
  and no symlink followed, so a component replaced by a symlink after the check is refused; it
  must be a regular file within the input bound, and its exact bytes must hash to `HEX` (64
  lowercase hex digits). This needs the darwin/linux no-follow reader; on other platforms a
  caller-owned profile is refused before it is read. It is then
  admitted exactly as a committed profile (`DCX-V1-390`), and every source file is still read only
  from Git at the revision (`DCX-V1-398`), so the same profile bytes yield the same subjects,
  relations and gaps as when committed. Without `--profile-sha256` an absolute or otherwise
  non-repository path keeps the `DCX-V1-399` refusal, which for an absolute path adds that a
  caller-owned profile also needs the digest. A malformed digest, a relative or unclean path, a
  path inside the repository, a missing, non-regular or over-bound file, or a digest mismatch
  (which names the file's digest) is refused with no output before any source file is read. A
  caller-owned profile is usable whether or not the repository commits one, and its records carry
  the same `generated` trust and `external-provider` authority: a profile is configuration and
  grants no authority (`DCX-V1-390`). Acceptance:
  `TestDCXV1478CallerProfileExtractsLikeTheCommittedProfile`,
  `TestDCXV1478CallerProfileRequiresAMatchingDigest`,
  `TestDCXV1478CallerProfileDirectorySwapIsRefused`,
  `TestDCXV1478CallerProfileUnsupportedPlatformIsRefused`.
- `DCX-V1-503`: (proposed, experimental; GitHub #1662; owner acceptance open) A caller-profile
  inventory's `profile` member records `origin: caller`, `sha256` (the digest of the file's exact
  bytes, as pinned), `document` (the admitted profile) and `canonical_sha256` (the digest of that
  document's canonical encoding), and never the caller's local path; `path` and `blob` are
  omitted. Its limitations add that the profile is caller-owned and not declared by the
  repository, and the provider record's `subjects` capability reason names
  `sha256:HEX`, also when no pattern matched and the capability is `absent`. A
  repository-declared inventory omits `origin`, `document` and `canonical_sha256` and keeps its
  bytes. Acceptance: `TestDCXV1503CallerProfileProvenanceIsRecorded`,
  `TestDCXV1503CallerProfileWithNoSubjectsKeepsItsDigest`,
  `TestDCXV1478CallerProfileExtractsLikeTheCommittedProfile`.
- `DCX-V1-504`: (proposed, experimental; GitHub #1662; owner acceptance open) The provider record
  of a caller-profile inventory, committed unchanged and declared as a provider input, builds
  through the existing provider admission (`DCX-V1-393`), and the built artifact still names the
  profile digest through that capability reason. No artifact, manifest or provider record schema
  changes. Acceptance: `TestDCXV1504CallerProfileProviderRecordBuildsUnchanged`.
- `DCX-V1-505`: (proposed, experimental; GitHub #1662; owner acceptance open) A caller-profile
  extraction writes neither the repository nor the profile or its directory, reads no working-tree
  source, and returns identical bytes for identical inputs. Acceptance:
  `TestDCXV1505CallerProfileExtractionIsReadOnly`.
- `DCX-V1-506`: (proposed, experimental; GitHub #1662; owner acceptance open) `extract` takes
  `--profile-sha256` as its one optional option, in `help --json` and the capability catalog; the
  CLI prints exactly the producer's caller-profile inventory bytes and a refusal prints nothing.
  A present but empty `--profile-sha256` is an argument error, never the repository-profile
  route.
  Acceptance: `TestDCXV1506DocsCorpusExtractCallerProfileCLI`, `TestDocsCorpusOperationFlagMatrix`,
  `TestDocsCorpusHelpListsOperations`, `TestCAPV0019CommittedCatalogMatchesTheBinary`.

Caller-owned profiles (`DCX-V1-478`, `DCX-V1-503..506`) add one CLI option and optional
inventory members; they run no tool, open no network, add no cache or persisted state, and do not
change the artifact, manifest or provider record formats. They are not a way to read sources from
outside Git, to pin a profile by path, or to raise an extractor's authority. Failure modes: a
caller who edits the profile without updating the digest is refused, never silently re-pinned; a
profile placed inside the working tree is refused rather than treated as committed. Rollback
removes the option and the optional members: caller-profile inventories already emitted stay
readable to every lenient reader, and their provider records stay admissible.
Caller-owned profiles stay refused on platforms without the no-follow reader, so Windows support
is not met. Not delivered here: a `score` or rank group on served rows and symbol subjects
derived from indexed inputs (#1665 rows 3 and 5). The compact per-file anchor table of #1664
(`DCX-V1-487..489`) and the rebuild from an embedded manifest of #1667 (`DCX-V1-495..498`) are
specified in their own sections, as are the compact `/2` producer and rebuild reuse (`DCX-V1-530..534`), and
release notes naming the merges that change the derivation version remain owner work (V1-2545).

### Extraction enumeration of large revisions (GitHub #1662 residual)

`docs corpus extract` walked the revision one Git operation per tree, so a revision with more
than about 1,000 directories exhausted the 1,024-operation budget (`git-budget-exceeded`) before
any file was read. Status for `DCX-V1-520..522`: proposed, experimental; GitHub #1662; owner
acceptance open.

- `DCX-V1-520`: (proposed, experimental; GitHub #1662; owner acceptance open) A glob in an
  extractor's `files` resolves against one verified walk of the revision that costs one Git
  operation to pin the commit and its self-hashed root tree plus one `cat-file --batch` per
  directory level, whose records must self-hash and carry the OID their verified parent names.
  The walk lists every non-tree entry (symlinks and gitlinks stay listed and become gaps when
  read, `DCX-V1-395`), exactly as `ls-tree -r` would, independent of how many directories the
  revision holds. A name Git's tree grammar admits, such as one containing a backslash, is
  listed rather than refused, as the per-tree walk did. Acceptance:
  `TestDCXV1520RevisionPathsMatchesLsTreeWithinOneOpPerLevel`,
  `TestDCXV1520RevisionPathsOpsIndependentOfDirectoryCount`,
  `TestDCXV1520RevisionPathsListsGitSafeNamesUnfiltered`,
  `TestDCXV1520ExtractionEnumeratesMoreThan1024Directories`.
- `DCX-V1-521`: (proposed, experimental; GitHub #1662; owner acceptance open) The walk descends
  only into a directory below which some glob can match: until a glob's first `**` segment each
  glob segment must match exactly one directory segment, and the glob's last segment names a file
  below the directory. Directories named `node_modules` stay pruned. Pruning never removes a path
  a glob matches. Acceptance: `TestDCXV1521ExtractorGlobPruningIsSound`,
  `TestDCXV1521RevisionPathsPrunesRejectedSubtrees`,
  `TestDCXV1520ExtractionEnumeratesMoreThan1024Directories`.
- `DCX-V1-522`: (proposed, experimental; GitHub #1662; owner acceptance open) The walk keeps
  explicit bounds and refuses before it reads the next level: 400,000 listed tree entries
  (`affected.MaxWalkEntries`), `MaxPathSetTreeBytes` of tree output (`git-output-exceeded`) and
  `MaxTreeDepth` levels, and an unpinned revision is refused before any Git operation. Extraction
  reports every refusal as `corpus-refused` with `extraction cannot enumerate the revision`.
  Acceptance: `TestDCXV1522RevisionPathsRefusesPastItsBounds`.

These rows change no inventory, profile, provider record or artifact format and add no cache,
option or persisted state; a revision that extracted before yields the same inventory bytes.
Failure modes: a tree object that does not self-hash or is not the one its parent names refuses
the extraction; a revision past a bound is refused rather than listed in part. The per-extractor
4,096-file record bound (`MaxRecords`) still applies to the matched files (`DCX-V1-812`). Rollback restores the
per-tree `RevisionFS` walk in `extractorTree`. Timing on linux/WSL2 is retained in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-corpus-extract-enumeration.md`.

### Legacy discovery enumeration of large revisions (V1-2560)

`docs corpus discover-tests` legacy discovery (`DCP-V1-060..068`) still walked its `--path` one
Git operation per tree through `RevisionFS`, the root cause `DCX-V1-520` removed from extraction,
so a root holding more than about 1,000 directories exhausted the 1,024-operation budget. Status
for `DCX-V1-810..811`: proposed, experimental under the standing lane auto-accept; V1-2560.

- `DCX-V1-810`: (proposed, experimental; V1-2560) Legacy discovery of a directory resolves its
  test files through the verified walk of `DCX-V1-520`, descending only the discovery root's
  ancestors and the root's own subtree, never a `node_modules` directory at or below the root, so
  its Git operations grow with directory depth, not directory count. The walk's entry, tree-byte
  and depth bounds of `DCX-V1-522` refuse before the next level is read, reported as
  `corpus-refused` with `legacy discovery cannot enumerate the revision`. A test-named symlink or
  other non-tree entry stays listed, so the read refuses it as before. Acceptance:
  `TestDCXV1810LegacyDiscoveryEnumeratesMoreThan1024Directories` (1,100 directories below the
  root, under the default budget and under `boundCorpusGitOps(phaseGitOps)`; fails first on the
  per-tree walk with `git-budget-exceeded`).
- `DCX-V1-811`: (proposed, experimental; V1-2560) The discovered test files keep the per-tree
  walk's order, each directory's entries in name order, so a directory sorts before a sibling file
  its name prefixes, and every inventory existing fixtures produce is byte-identical. Acceptance:
  `TestDCXV1811LegacyPathsMatchPerTreeWalk` (the listing equals an `fs.WalkDir` over `RevisionFS`
  for four roots with `a.spec.js` beside `a/b.spec.js` and nested `node_modules`), the existing
  `TestLegacy*` tests and `TestDocsCorpusDiscoverLegacyCLI` unchanged.

No inventory format, option, cache or persisted state changes. Rollback restores the
`RevisionFS` walk in `legacyPaths`.

### Extractor matched-file bound on large repositories (V1-2561)

A caller-owned `app/**/*.rb` profile over a large Rails repository (GitHub #1662's reporter has
13,806 such files) passes enumeration (`DCX-V1-520`) and then meets the per-extractor matched-file
bound. Reproduced on linux/WSL2 at 2026-10-31: 4,200 matching files refuse under provider/1 at the
4,096 bound; `--schema 2` (`DCX-V1-560`) extracts them. Status for `DCX-V1-812`: proposed,
experimental under the standing lane auto-accept; V1-2561.

- `DCX-V1-812`: (proposed, experimental; V1-2561) The matched-file bound is operator-declared by
  the output schema: `MaxRecords` (4,096) for provider/1 and `MaxCorpusRecords` (100,000) under
  `--schema 2`. Its refusal MUST name the matched-file count, the bound and the extractor, under
  provider/1 the `--schema 2` bound, and how to scope the profile:
  `narrow extractor "<id>" files globs to at most <bound> files`. Acceptance:
  `TestDCXV1812MatchedFilesBoundNamesScopeAndSchemaTwo` (4,200 matched files refuse with both
  levers named under the default and `--schema 1`, and extract under `--schema 2`; fails first on
  the missing scope hint).

Rollback drops the scope sentence from the refusal; no format, option or state changes.

## Input and authority boundary

### Experimental extractor profile limits (issue 1583)

`docs corpus extract` (`DCX-V1-390..399`) is line-oriented: one RE2 match per line, no
multi-line constructs except the bounded `field_pattern`/`end_pattern` entity block, no language
parser, and no framework convention is assumed beyond what a profile declares. A handler join is an
exact name match within the profile; overloaded, namespaced or dynamically dispatched handlers are
`unresolved-handler` or `ambiguous-handler` gaps. Framework tool output is read only as a committed
file whose digest the profile pins; Corvint never runs the tool, so a stale committed output is
the producer's responsibility. Extracted subjects and relations stay `generated` trust and carry no
authority; inventory gaps are reported in the inventory, not as supplied corpus gaps, because a
supplied gap must name a subject.

## Non-goals

The claim mention check and pattern report (`DCX-V1-340..352`) make no semantic or correctness
judgment, do not parse non-ASCII identifiers, compare templates only exactly after
normalization, use fixed thresholds, examine no native claim or supplied test-run receipt, give
the weighted audit figure no interval, and never raise or lower trust. Rollback removes the
`claim_mentions` member (omitted when empty), its gaps and coverage rows, the `patterns`
operation and stratification and the two build flags; corpora without token-bearing checked
claims build the same bytes as before.

Static test discovery (`DCX-V1-370..376`) does not run tests, evaluate Ruby or JavaScript, expand
loops, resolve dynamic imports, judge assertion adequacy or write a corpus provider record; it is
generated evidence for a caller to cite. Rollback removes the `discover-tests` subcommand and the
`internal/testsource/discover_tests*.go` producer; no stored state depends on it.

Extractor profiles (`DCX-V1-390..399`) are not a framework catalog, language parser or Core
taxonomy: they add no subject kind beyond the generic `job` and `event`, run no tool, infer no
handler by convention and promote no extracted subject past `generated` trust.

## Failure modes

- Rebuild from the embedded manifest (`DCX-V1-495..498`): a tampered, noncanonical or
  non-corpus artifact, or one whose embedded manifest or manifest digest does not hold, is refused
  before any Git read; a pinned input Git no longer holds refuses the build; neither path prints a
  partial artifact. Shell redirection onto the artifact being read truncates it before the build
  reads it, so the guide and help write to a new file and rename it.
- Diagnostics and builder upgrades (`DCX-V1-470..474`): a located refusal never reports a record
  ID that is not printable or is over 256 bytes, and is bounded however long the input. An
  artifact whose builder schema, derivation version, Go toolchain or engine differs, or whose
  revision is not an attested Git object ID, is still refused; a derived-byte change merged without a
  derivation version raise fails `TestDCXV1471DerivationVersionGuard` rather than serving
  artifacts the running builder would derive differently. A capacity refusal charges every
  distinct input before it reports. An eval whose checkout changes while its cases run is
  refused, never reported against a mix of two states; the `expected_rank` repeat failing leaves
  the field absent and the case's score unchanged.
- Ranking (`DCX-V1-475..477`): a query of one word, or only of punctuation, has no phrase and
  ranks as before; inflections widen only claim-term lookups and never add an index key; the
  natural ID order falls back to the string order when digit runs are equal in value.
A delta anchor whose span hash cannot be confirmed at the current revision (unreadable or
over-bound blob, digest mismatch, ambiguous window, hashing bound) is `changed`, `removed` or
`unresolved` with its reason and is never carried (`DCX-V1-362`, `DCX-V1-367`); a carried record
with an unconfirmed dependency is demoted (`DCX-V1-365`); a carry-forward whose record depends on
another provider's carried record is refused with the missing dependencies named (`DCX-V1-366`); a
cross-repository comparison, an undeclared carry-forward provider or an ambiguous current side is
refused (`DCX-V1-369`). An anchor in an additional repository that the current manifest does not
declare, or whose checkout is unavailable or holds another identity, is `unresolved` with that
reason, and a retained current artifact that cannot be rederived for that reason is classified
from its canonical bytes with the limitation stated, unless an available repository's input does
not name the blob its path holds, which is refused (`DCX-V1-425`); an invalid delta view or a
paged jsonl stream is refused (`DCX-V1-425`); a stream past 128 MiB is
`corpus-capacity-exceeded` (`DCX-V1-423`).
A current artifact the verified read refuses (`DCR-V1-410`), or that `--verify-rederive` cannot
rederive, refuses the delta with no output; the delta never classifies against it
(`DCR-V1-460`). A verified current artifact with an input that does not name the blob its path
holds at its pinned revision is refused before classification (`DCR-V1-464`).
An adoption record missing the typed details its kind requires is `unresolved` with that reason
(`DCX-V1-441`); a carried record whose typed details join to an ID neither current nor carried is
demoted (`DCX-V1-442`); an adoption carry-forward against a `/1` current manifest is refused
(`DCX-V1-443`).

- `DCX-V1-370..376` discovery failure modes: a test file the reader cannot balance, a file over the
  reader's byte bound, a non-literal title, tag, target or project, an unresolved import and a fact
  list over its bound each become a typed unknown with a reason; an unsafe path, an unknown runner or
  a short or unknown revision is refused before any read; an encoded document over 32 MiB is refused
  rather than truncated.

## Acceptance evidence

Each block names its acceptance tests, and the Traceability table maps every block to its
implementation and evidence. The parent contract's acceptance gates apply.

## Rollback

Revert the implementing change a block names. The split itself reverts by moving these blocks back
into the parent under their unchanged DCP-V1 numbers and renaming the DCXV1 test prefixes back to
DCPV1.

## Traceability

| Requirements | Implementation boundary | Required evidence |
| --- | --- | --- |
| DCX-V1-495..498 | `internal/doccorpus/rebuild.go`, `encoding.go`, `cmd/corvint/docs_corpus.go`, `docs_corpus_help.go`, `internal/capcatalog/catalog.json` | `TestDCXV1495RebuildFromEmbeddedManifest`, `TestDCXV1495BuildRebuildsFromEmbeddedManifest`, `TestDCXV1496RebuildRefusesUnverifiedManifest`, `TestDCXV1497ReadNeverRebuilds`, `TestDCXV1498MCPServesArtifactsAcrossBuilderUpgrades`, `TestDocsCorpusOperationFlagMatrix`, `TestDocsCorpusOperationHelpPrintsFlags` |
| DCX-V1-530..534 | `internal/doccorpus/compact_anchor.go`, `rebuild.go`, `encoding.go`, `cmd/corvint/docs_corpus.go`, `docs_corpus_help.go`, `docs.go`, `internal/capcatalog/catalog.json` | `TestDCXV1530CompactEncoderMatchesReference`, `TestDCXV1531CompactProviderProducer`, `TestDCXV1531CompactProviderCommand`, `TestDCXV1532RebuildReusesSameDerivation`, `TestDCXV1532BuildArtifactReusesRecords`, `TestDCXV1533RebuildRederivesOtherwise`, `TestDCXV1534RebuildReuseMeasured`, `TestDocsCorpusOperationFlagMatrix`, `TestDocsCorpusOperationHelpPrintsFlags` |
| DCX-V1-470 | `internal/doccorpus/encoding.go`, `shards.go`, `extractor.go` | `TestDCXV1470LocatedDecodeRefusal` |
| DCX-V1-471 | `internal/doccorpus/encoding.go`, `read.go` | `TestDCXV1471VerifiedReadAcrossBuilderRevisions`, `TestDCXV1471DerivationVersionGuard`, `TestDCRV1410VerifiedReadRefusals` |
| DCX-V1-472 | `internal/doccorpus/capacity.go`, `compile.go` | `TestDCXV1472ProviderRecordBytesChargedApart`, `TestDCPV1157CapacityRefusalsAreTypedAndEarly` |
| DCX-V1-473 | `internal/mcp/corpusbridge/eval.go` | `TestDCXV1473FailingCasesListReturnedRows` |
| DCX-V1-474 | `internal/mcp/corpusbridge/eval.go`, `corpusbridge.go` | `TestDCXV1474EvalSuiteObservesOnce`, `TestDCPV1251EvalScoresThroughTheMCPToolPath`; measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-corpus-diagnostics-and-ranking.md |
| DCX-V1-475..477 | `internal/doccorpus/search_rank.go`, `search_phrase.go`, `typed_query.go` | `TestDCXV1475PhraseAndIdentifierMatchesRankFirst`, `TestDCXV1476ClaimTextMatchesInflections`, `TestDCXV1477MeaningfulTieBreaks`, `TestDCSV1404SearchRankingEvalFixture` |
| DCX-V1-478, DCX-V1-503..506 | `internal/doccorpus/extractor.go`, `cmd/corvint/docs_corpus.go`, `docs_corpus_help.go`, `docs.go` | `TestDCXV1478CallerProfileExtractsLikeTheCommittedProfile`, `TestDCXV1478CallerProfileRequiresAMatchingDigest`, `TestDCXV1478CallerProfileDirectorySwapIsRefused`, `TestDCXV1478CallerProfileUnsupportedPlatformIsRefused`, `TestDCXV1503CallerProfileProvenanceIsRecorded`, `TestDCXV1503CallerProfileWithNoSubjectsKeepsItsDigest`, `TestDCXV1504CallerProfileProviderRecordBuildsUnchanged`, `TestDCXV1505CallerProfileExtractionIsReadOnly`, `TestDCXV1506DocsCorpusExtractCallerProfileCLI`, `TestDocsCorpusOperationFlagMatrix`, `TestDocsCorpusHelpListsOperations`, `TestCAPV0019CommittedCatalogMatchesTheBinary` |
| DCX-V1-813 | `verifyOnly` and `pin` in `internal/doccorpus/compile.go`, `rebuild.go`; `BuildRevisionPin` in `internal/contextindex/corpus.go` | `TestDCXV1532RebuildReusesSameDerivation`, `TestDCXV1533RebuildRederivesOtherwise`, `TestDCXV1534RebuildReuseMeasured` |
| DCX-V1-812 | `bound` in `internal/doccorpus/extractor.go` | `TestDCXV1812MatchedFilesBoundNamesScopeAndSchemaTwo` |
| DCX-V1-810..811 | `legacyPaths` in `internal/doccorpus/behavior_legacy_discovery.go` | `TestDCXV1810LegacyDiscoveryEnumeratesMoreThan1024Directories`, `TestDCXV1811LegacyPathsMatchPerTreeWalk` |
| DCX-V1-520..522 | `internal/cem/gitauth/revision_paths.go`, `internal/doccorpus/extractor.go` | `TestDCXV1520RevisionPathsMatchesLsTreeWithinOneOpPerLevel`, `TestDCXV1520RevisionPathsOpsIndependentOfDirectoryCount`, `TestDCXV1520RevisionPathsListsGitSafeNamesUnfiltered`, `TestDCXV1520ExtractionEnumeratesMoreThan1024Directories`, `TestDCXV1521ExtractorGlobPruningIsSound`, `TestDCXV1521RevisionPathsPrunesRejectedSubtrees`, `TestDCXV1522RevisionPathsRefusesPastItsBounds`; measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-corpus-extract-enumeration.md |
| DCX-V1-560..564 | `internal/doccorpus/extractor.go`, `capacity.go`, `cmd/corvint/docs_corpus.go`, `docs_corpus_help.go`, `docs_corpus_delta.go`, `docs/RELEASE-NOTES.md` | `TestDCXV1560ExtractorSchemaTwoAdmitsMoreThanTheRecordBound`, `TestDCXV1560DocsCorpusExtractSchemaCLI`, `TestDCXV1562ExtractorRefusesARecordOverTheInputBound`, `TestDCXV1563DeltaCurrentSideIsDecidedByPresence`, `TestDCXV1564ReleaseNotesListEveryDerivation`, `TestDocsCorpusOperationFlagMatrix`; measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-corpus-coverage-multirepo-derivation.md |
| DCX-V1-360..369 | `internal/doccorpus/delta.go`, `delta_diff.go`, `cmd/corvint/docs_corpus_delta.go`, `docs_corpus.go`, `docs_corpus_help.go` | `TestDCXV1360DeltaWorklistClasses`, `TestDCXV1366CarryForwardFeedsNextBuild`, `TestDCXV1363LineDiff`, `TestDCXV1360DeltaRepairs`, `TestDCXV1360DeltaCLI`, `TestDocsCorpusOperationFlagMatrix`, `TestDocsCorpusOperationHelpPrintsFlags` |
| DCX-V1-487..489 | `internal/doccorpus/compact_anchor.go`, `shards.go`, `encoding.go` | `TestDCXV1487CompactAnchorTableEquivalent`, `TestDCXV1488CompactAnchorRefusals`, `TestDCXV1489CompactAnchorSizeMeasured`; measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-corpus-compact-anchor-table.md |
| DCX-V1-420..425 | `internal/doccorpus/delta.go`, `delta_open.go`, `cmd/corvint/docs_corpus_delta.go`, `docs_corpus.go`, `docs_corpus_help.go` | `TestDCXV1420DeltaAdditionalRepositoryAnchors`, `TestDCXV1420DeltaSharedReadBound`, `TestDCXV1424DeltaHistoricalPins`, `TestDCXV1425OpenDeltaCurrentUnavailableCheckout`, `TestDCXV1425DeltaUnavailableCheckoutCLI`, `TestDCXV1421DeltaViewsCLI`, `TestDocsCorpusOperationFlagMatrix`, `TestDocsCorpusOperationHelpPrintsFlags` |
| DCX-V1-440..445 | `internal/doccorpus/delta.go`, `adoption.go`, `cmd/corvint/docs_corpus_help.go`, `internal/capcatalog/catalog.json` | `TestDCXV1440DeltaAdoptionRecords`, `TestDCXV1442DeltaExactTestJoinNeedsItsFile`, `TestDCXV1444SelfDeltaSharedSpanChargedOnce`, `TestCAPV0019CommittedCatalogMatchesTheBinary`, `TestMCPV0100HelpToolMatchesTheCLI`, `TestDocsCorpusHelpListsOperations` |
| DCX-V1-340..352 | `internal/doccorpus/claim_mentions.go`, `claim_patterns.go`, `claim_audit_report.go`, `compile.go`, `query.go`, `pagination.go`, `typed_query.go`, `types.go`, `cmd/corvint/docs_corpus.go`, `docs_corpus_audit.go`, `docs_corpus_help.go` | `TestDCXV1340ExtractCodeTokens`, `TestDCXV1341ClaimMentionCheckIsBuiltAndPinned`, `TestDCXV1343ClaimMentionsAreServedWithTheClaim`, `TestDCXV1344ClaimMentionGapsAndCoverage`, `TestDCXV1346UnresolvedMentionThreshold`, `TestDCXV1347DanglingReferencesRefuse`, `TestDCXV1348CorpusPatternReport`, `TestDCXV1349CorpusPatternsCLI`, `TestDCXV1350StratifyPatternsOverSamples`, `TestDCXV1351WeightedAuditFigure`, `TestDCXV1352PatternRateThresholds`, `TestDocsCorpusOperationFlagMatrix`, `TestDocsCorpusOperationHelpPrintsFlags` |
| DCX-V1-370..376 | `internal/testsource/discover_tests.go`, `discover_tests_playwright.go`, `discover_tests_rspec.go`, `internal/doccorpus/line_anchor.go`, `cmd/corvint/docs_corpus_discover_tests.go` | `TestDCXV1370DiscoverTestsDeterministicAnchored`, `TestDCXV1370DocsCorpusDiscoverTestsCLI`, `TestDCXV1371DiscoverTestsIdentityJoins`, `TestDCXV1372DiscoverTestsExpectations`, `TestDCXV1373DiscoverTestsNavigations`, `TestDCXV1374DiscoverTestsReaches`, `TestDCXV1375DiscoverTestsHooksTagsAnnotations`, `TestDCXV1376DiscoverTestsUnknowns` |
| DCX-V1-390..392 | `internal/doccorpus/extractor.go` | `TestDCXV1390ExtractorProfileIsClosed`, `TestDCXV1390ExtractorGlobMatchIsLinear`, `TestDCXV1391ExtractorEmitsGenericSubjectsWithExactAnchors`, `TestDCXV1392ExtractorDerivesImplementsAndCallsRelations`, `TestDCXV1392ExtractorRelationsCiteBothEnds` |
| DCX-V1-393..395 | `internal/doccorpus/extractor.go`, `providers.go` | `TestDCXV1393ExtractorProviderRecordBuildsUnchanged`, `TestDCXV1394ExtractorReadsOnlyPinnedToolOutput`, `TestDCXV1395ExtractorReportsUnreadableFilesAndShapesAsGaps`, `TestDCXV1395AmbiguousEntityDeclarationClosesTheEntity`, `TestDCXV1395ExtractorDenseFileIsRefusedWhileScanning` |
| DCX-V1-396 | `internal/doccorpus/providers.go` | `TestDCXV1396JobAndEventAreGenericSubjectKinds` |
| DCX-V1-397 | `internal/doccorpus/flow_relations.go` | `TestDCXV1397FlowRelationNamesCheckDeclaredJobAndEventSubjects` |
| DCX-V1-398 | `internal/doccorpus/extractor.go` | `TestDCXV1398ExtractorIsDeterministicAndReadOnly` |
| DCX-V1-399 | `cmd/corvint/docs_corpus.go`, `docs_corpus_help.go`, `docs.go` | `TestDCXV1399DocsCorpusExtractCLI`, `TestDocsCorpusOperationFlagMatrix`, `TestDocsCorpusHelpListsOperations` |
