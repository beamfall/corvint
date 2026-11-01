# Documentation corpus search ranking V1

Owner: Russell Lewis
Date: 2026-10-25
Intent status: proposed
Delivery status: experimental

Authoritative inputs: native ticket V1-2525 and GitHub issues
[#1597](https://github.com/beamfall/corvint/issues/1597) and
[#1665](https://github.com/beamfall/corvint/issues/1665). These requirements were split unchanged
from the [documentation corpus contract](documentation-corpus-v1.md), whose Requirements section was
over the 256-obligation OCM limit (`maxObligations`). Each keeps its former DCP-V1 number under the
DCS-V1 prefix, and the parent's stub at each former section carries the crosswalk. The split changes
no behavior, wire format, error code or receipt; the parent's input and authority boundary, failure
codes and acceptance gates still govern them.

## Agent digest
- Claim: Corpus docs_search ranks files and claims by rank groups, word coverage, identifier tokens, named subjects, match anchors and derived declarations.
- Status: proposed; experimental; DCS-V1-400..404 accepted under the owner's standing auto-accept (2026-10-11); the remaining blocks are implemented with focused tests.
- Exists: the field-weighted file and claim ranking, served rank groups, query-word and content-word coverage tiers and bounded declaration derivation in `internal/doccorpus`, each with focused tests.
- Blocked on: owner acceptance and independent external utility qualification, as for the parent contract.
- Read next: Requirements; Failure modes; Traceability.

## User and current state

An agent asking `docs_search` a question expects the file or claim that answers it near the top.
These blocks rank files and claims by served rank groups and query-word coverage, match identifier
tokens and named subjects, and derive bounded declarations from committed sources. Former IDs:
DCP-V1-NNN is DCS-V1-NNN for every ID listed here.

## Requirements

### File and claim ranking in `docs_search` (GitHub #1597)

The `DCP-V1-331` weights buried two answers a caller-labelled suite expected: a file (module
subject) lost to documentation paragraph subjects whose prose names repeat the query words, and an
included claim lost to every subject naming one query word, because claim text weighs 1 and a
name 3. These requirements amend `DCP-V1-331`'s name field and its cross-family merge; the IDF,
term order, kind filter and index/scan parity stay as stated there. Status for `DCS-V1-400..404`:
accepted under the owner's standing auto-accept (2026-10-11); GitHub #1597; V1-2470.

- `DCS-V1-400`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1597; V1-2470) The name field of a `module` or `file` subject also holds the evidence terms of
  each of its anchor paths, so a path segment weighs as a name term (3) and matches the subject
  even when its ID, name and claim text lack it. A native file subject, whose name is its path,
  ranks as before. Acceptance: `TestDCSV1400ModulePathSegmentsAreNameTerms`.
- `DCS-V1-401`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1597; V1-2470) A subject name holding more than 12 distinct evidence terms is prose, such as a
  documentation paragraph, unless the subject is a `module` or `file`: its name terms weigh as
  claim text (1) and its ID terms keep weight 3. A name of at most 12 distinct terms keeps
  `DCP-V1-331`'s name weight. Acceptance: `TestDCSV1401ProseNameThreshold`,
  `TestDCSV1400ModulePathSegmentsAreNameTerms`.
- `DCS-V1-402`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1597; V1-2470) Each family's score is normalized before the families merge: a subject's partial
  score is divided by the sum over the query terms of `3 × idf` over the subjects, and a claim's
  by the sum of `1 × idf` over the claims, so every partial hit of either family scores in
  `[0, 1]`. An exact case-insensitive name (or trimmed claim-text) match orders before every hit
  that is not exact, replacing `DCP-V1-331`'s shared bonus, even when tokenization leaves it no
  query term (a subject named `SignIn` for the query `signin`); exact hits order among themselves
  by score. Equal scores order subjects before claims, then by record ID (amended by
  `DCX-V1-475` and `DCX-V1-477`). A claim holding every
  query term therefore outranks a subject naming only some of them; without `include` the subject
  order is the one `DCS-V1-400` and `DCS-V1-401` give, since one divisor never reorders one family.
  Acceptance: `TestDCSV1402ClaimFamilyNormalizedMerge`, `TestDCSV1402ExactMatchPrecedesTermOverlap`,
  `TestDCPV1328SearchIncludesClaims`, `TestDCPV1331ExactBonusAcrossFamilies`.
- `DCS-V1-403`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1597; V1-2470) The ranking stays deterministic and bounded: terms are summed in sorted order, the
  path terms of `DCS-V1-400` are read per query from the artifact's subjects without modifying
  the query index or its encoding, and the indexed and scanning readers return the same order. A
  query keeps only its own terms of a subject's name field, so a miss retains no per-subject
  vocabulary. `docs_claims` with `query` (`DCP-V1-313`) is unchanged here (it is amended later by
  `DCX-V1-476` and `DCX-V1-477`). Acceptance:
  `TestDCSV1402ClaimFamilyNormalizedMerge`, `TestDCSV1400ModulePathSegmentsAreNameTerms`,
  `TestDCPV1331SearchRanking`, `TestDCSV1403MissRetainsNoSubjectNameTerms`.
- `DCS-V1-404`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1597; V1-2470) A committed synthetic fixture,
  `internal/mcp/corpusbridge/testdata/search-ranking-gh1597/` (a source tree, supplied records
  and a `corvint-corpus-eval-cases/1` case file of six file-lookup cases without `kind` and six
  claim-lookup cases with `include: ["claims"]`, each with `limit` 1), is built into a corpus and
  scored through `docs corpus eval`'s MCP tool path; every case passes, and twice in a row with
  equal ranks. Ranking changes to `docs_search` run it before they ship. Acceptance:
  `TestDCSV1404SearchRankingEvalFixture`.

### Served rank groups and derived declarations (GitHub #1665 rows 3 and 5)

After #1669 two rows of #1665 remained. Results carried no score, so a caller could not see that
the ID-ordered run of claims it received was one tie, or cut at a boundary (row 3). A method
declared in an indexed source input that a records provider owns (`def sync_with_pos` in a `.rb`
file) had no subject, so `docs_search --query sync_with_pos` and `--kind symbol` found nothing
(row 5). Status for `DCS-V1-479..486`: proposed, experimental; GitHub #1665.

- `DCS-V1-479`: Amending `DCS-V1-402` and `DCX-V1-477` only by adding a served member, a
  `docs_search` receipt carries `result_ranks`, an object keyed by each visible result's record
  ID. Each value holds `rank` (the 1-based position before paging), `group` (the 1-based tie
  group: results whose exact flag, phrase flag and normalized score are equal share one), `tier`
  (`exact`, `phrase` or `terms`) and `score` (the family-normalized score of `DCS-V1-402`, rounded
  to 6 decimals). The shorter-text and natural-ID keys of `DCX-V1-477` order results inside a
  group and never start one. Merged claim hits (`include: ["claims"]`) carry the same member. A
  page carries only its own results. Failure modes: two results of different groups never share a
  group number, and equal served scores in different tiers stay in different groups. Acceptance:
  `TestDCSV1479SearchServesRankGroups`.
- `DCS-V1-480`: A `docs_claims` receipt with `query` carries `result_ranks` with the same members
  plus `terms`, the count of distinct query terms the claim holds (an inflection counts its term
  once, `DCX-V1-476`). A group is the claims holding as many distinct terms with the same summed
  idf; `score` is the summed idf normalized by the sum over the query terms. An exact trimmed
  text match is tier `exact`. Acceptance: `TestDCSV1480ClaimsServeRankGroups`.
- `DCS-V1-481`: `result_ranks` is an additive optional receipt member under `DCP-V1-039`: the
  receipt schema stays `corvint-corpus-receipt/2`, the member is omitted (not empty) for every
  other operation, for `docs_claims` without `query` and for an empty result, and is derived per
  read from the ranking, never stored in the artifact or the query index. A reader that ignores
  it reads the receipt as before. Acceptance: `TestDCSV1479SearchServesRankGroups` (encoded bytes,
  decode and absence), `TestDCSV1486GH1665RankingEvalFixture` (served through the MCP tools).
- `DCS-V1-482`: Amending `DCP-V1-003`, the first native provider derives a declaration subject
  (`<native>:symbol:<path>:<line>:<name>`, kind `symbol`, or `test` on a test path) and its
  `:declaration` claim for each symbol the native source index holds in a primary-revision
  `source` input that only non-native providers claim. IDs, anchors (`syntax`, `source`, the
  symbol name) and evidence (`source-derived`, `generated`) are the native provider's own
  declaration records; no file subject or excerpt is added, and the owning provider's records are
  unchanged. Acceptance: `TestDCSV1482DeclarationsOfRecordsOwnedSource`.
- `DCS-V1-483`: Derivation runs after every provider, once per path, and skips an input a native
  provider claims (it already declares its symbols), an unparsed or natively excluded input, an
  input of another revision (amended by `DCS-V1-549`) and a non-`source` input. A manifest without a native provider
  derives nothing. Acceptance: `TestDCSV1483DerivationScope`.
- `DCS-V1-484`: Derivation never refuses a manifest the compiler built before it. When a compile
  that derived declarations is refused by a corpus bound or check (a record count, the output
  byte bound), it is compiled once more without them, restoring the incremental reuse state the
  refused compile recorded, and the artifact carries one `not-collected` gap
  (`derived declaration bound: ...`) on the first derivable input's native file ID. A refusal of
  the retry is the refusal the manifest had before derivation. Failure modes: an error that is
  not a corpus refusal (Git, capacity, input availability, cancellation) is returned without a
  retry; a refusal of the first attempted declaration (an oversized declaration line) is retried
  like a later one; the retry leaves one cached shard per compiled shard. Acceptance:
  `TestDCSV1484DerivationNeverRefusesABuildableManifest`,
  `TestDCSV1484RefusedFirstDeclarationRestoresShardCache`,
  `TestDCSV1484RetryAnswersOnlyCorpusRefusals`.
- `DCS-V1-485`: Derived declarations change the records the same inputs build, so
  `DerivationVersion` is `2` (`DCX-V1-471`): the verified read refuses an artifact recorded by
  derivation 1 with `derivation version differs (artifact 1, running builder 2); rebuild the
  corpus`, and the rebuilt artifact serves the declarations. Acceptance:
  `TestDCSV1485DerivationTwoRefusesDerivationOneArtifacts`, `TestDCXV1471DerivationVersionGuard`.
- `DCS-V1-486`: A committed synthetic fixture, `internal/mcp/corpusbridge/testdata/search-ranking-gh1665/`
  (a source tree whose Ruby service and model a records provider owns, supplied records
  reproducing the issue's examples, and a `corvint-corpus-eval-cases/1` file tagged by issue row),
  is scored through `docs corpus eval`'s MCP tool path: every case passes, twice with equal ranks,
  and `docs_search` and `docs_claims` serve a rank per result with a visible tie group. Measured
  on the base `bcbd5ded`: rows 1, 2 and 4 pass (4 of 4), row 5 passes 0 of 4, and no result carries
  a rank; after this change all 8 cases pass and every served row is ranked. Acceptance:
  `TestDCSV1486GH1665RankingEvalFixture`.

Non-goals: no score threshold or cut-off parameter, no change to any ranking order, no new index
key or artifact member, no symbols the native index does not already extract (class members of
TypeScript classes stay unextracted), and no derivation for inputs of other revisions. Rollback:
drop `result_ranks` (readers that ignore it are unaffected), remove the derivation and the retry,
and return `DerivationVersion` to `1`; artifacts built in between are then refused as another
derivation and rebuilt. No stored state needs migration.

### Query words, identifier tokens and named subjects (GitHub #1665 re-test leftovers)

The re-test of #1665 after #1683 left three shapes open. An exact identifier or file name
(`waitlist_notifier`, `on_deck_controller.rb`) returned a shorter feature name or the file's own
declarations, all phrase matches, ahead of the file. A question ("What breaks in Reservation
Cancellation, and under what conditions?") was one tie group with `terms: 0` on every result, and a
plural question word yielding two evidence terms (`breaks`, `break`) outweighed the two words of the
flow's name, so a short name holding that word ranked first. Status for `DCS-V1-540..546`:
proposed, experimental; GitHub #1665.

- `DCS-V1-540`: Amending `DCS-V1-400` and `DCS-V1-402`, `docs_search` groups the query's evidence
  terms by the query word (`DCX-V1-475` words) that yields them, merging groups that share a term
  so word order never changes the grouping; a term no word yields is a group of its own. A subject or merged claim scores, per group, the best weighted idf among the
  group's terms it holds, and the family maximum adds the group's largest idf once, so a word and
  its singular count once. Groups and their terms are summed in sorted order. Failure modes: a
  plural word never counts twice; "condition conditions" and "conditions condition" rank alike;
  the score stays in [0, 1]. Acceptance:
  `TestDCSV1540QueryWordCountsOnce`.
- `DCS-V1-541`: A query that is one identifier of at least two words ranks, in tier `identifier`
  after `exact`, a subject one of whose name tokens, or of a module or file subject's anchor-path
  tokens, spells it: the token's words, or its words without its last `.` suffix, equal the
  query's. A token is a run of letters, digits, `_`, `-` and `.`, trimmed of `.`; the ID is not
  read. So `pos_sync`, `PosSync` and `pos_sync.rb` name `src/app/services/pos_sync.rb`, and the
  name "POS sync dashboard" names none of them. A spaced query is never an identifier. Acceptance:
  `TestDCSV1541IdentifierTokenTier`.
- `DCS-V1-542`: Amending `DCX-V1-475`, a phrase match orders by where the phrase is held: tier
  `path-phrase` (a module or file subject's name or anchor path) before `phrase` (another
  subject's non-prose name, or a prose name for a one-identifier query) before `id-phrase` (only
  the subject's ID). The query "on deck" ranks `on_deck_controller.rb` before the
  `OnDeckController` declaration, and both before the `index` declaration whose ID embeds the path
  (V1-2549). Acceptance: `TestDCSV1542PathPhraseBeforeDeclarations`.
- `DCS-V1-543`: A question-shaped query (it ends with `?`, or its first whitespace field is a
  question word, ignoring case) ranks in tier `named`, after the phrase tiers and before `terms`,
  a subject whose non-prose name of at least two words, at least one an evidence term, appears as
  a contiguous run of the query's words. An allocation-free prefilter skips names whose letter
  pieces the lower-cased query does not hold. Failure modes: a statement never gets `named`; the
  identifier `is_admin` is not a question; a single-word name is never `named` (amended by
  `DCS-V1-548`). Acceptance:
  `TestDCSV1543QuestionNamesASubject`.
- `DCS-V1-544`: Amending `DCS-V1-479`, `tier` is one of `exact`, `identifier`, `path-phrase`,
  `phrase`, `id-phrase`, `named` and `terms`, in rank order; a group is the results of one tier
  with one served score; and `docs_search` serves `terms`, the number of query words a result
  holds (omitted when zero). `docs_claims` ranking (`DCS-V1-480`) is unchanged. Acceptance:
  `TestDCSV1544ServedTiersAndTerms`, `TestDCSV1479SearchServesRankGroups`.
- `DCS-V1-545`: The tiers and groups are computed per read from the artifact and the query, with
  the same order from the query index and the subject scan; no index key, artifact member,
  encoding or the frozen evidence tokenizer changes, and no stored state is written. Acceptance:
  every test above compares the indexed and scanned orders; `TestDCSV1404SearchRankingEvalFixture`.
- `DCS-V1-546`: A committed synthetic fixture,
  `internal/mcp/corpusbridge/testdata/search-ranking-gh1665-leftovers/` (three question cases and
  five identifier cases reproducing the re-test shapes, including an unowned `.yml` file the
  native analyzer does not parse and a records-owned Ruby service), is scored through the MCP tool
  path: every case passes, twice with equal ranks. Frozen outcomes, measured on linux/WSL2: on the
  base `0949876e` the fixture passes 5 of 8 (question 2 of 3, identifier 3 of 5); after this change
  8 of 8. The #1665 fixture's row 4 now names the controller file as its one strict subject (it
  previously accepted any result anchored to the path, which the `index` declaration satisfied):
  0 of 1 on the base, 1 of 1 after; its other 7 cases and all 12 #1597 cases pass before and after.
  `TestDCSV1479SearchServesRankGroups` expected `terms` absent on `docs_search`; it now expects
  `1`. Acceptance: `TestDCSV1546GH1665LeftoversRankingEvalFixture`,
  `TestDCSV1486GH1665RankingEvalFixture`.

Non-goals: no document-length normalization (short names already break score ties,
`DCX-V1-477`), no stop-word or tokenizer change, no `docs_claims` change, no learned or tunable
weights, and no `named` tier for single-word subject names (amended by `DCS-V1-548`). Rollback: restore the per-term sum and
the exact, phrase and terms tiers of `DCX-V1-475` and `DCS-V1-479`, and the fixtures' previous
expectations. No stored state needs migration.

### Query-word coverage, one-word named subjects and multi-repository declarations (GitHub #1665 re-test of a681846)

The re-test of #1665 on `a681846` left three shapes open. In tier `terms` a result holding a few
rare query words in a short name outscored a prose name holding every query word, so a file
holding 5 of 5 words ranked 7th and one holding 10 of 11 ranked 15th (V1-2575). A question naming
a one-word flow ("How does Waitlist work?") got no `named` tier (V1-2568, the `DCS-V1-543` limit).
And a records-owned source of an additional repository got no derived declaration, so its
identifiers stayed unsearchable in a multi-repository corpus (V1-2567, row 5). Status for
`DCS-V1-547..550`: proposed, experimental; GitHub #1665.

- `DCS-V1-547`: Amending `DCS-V1-544`, inside tier `terms` a result holding more query words
  (`DCS-V1-540` groups; the served `terms`) orders before one holding fewer, whatever their scores;
  among equal counts the score orders as before, and a group is the results of one tier with one
  held-word count and one served score. Other tiers are unchanged. Failure modes: a higher-scoring
  result holding fewer words never orders first; two results with equal score but different counts
  are never one group; index and scan orders stay equal (`DCS-V1-545`). Acceptance:
  `TestDCSV1547TermsTierOrdersByCoverage` (5-word and 11-word queries of the re-test; it ranks the
  expected result 4th before the change).
- `DCS-V1-548`: Amending `DCS-V1-543`, a question-shaped query also names, in tier `named`, a
  subject whose non-prose name is one word that is an evidence term, when the query holds that word
  as a capitalized letter-and-digit run, other than the query's first run, that is one identifier
  word and not a question word. So "How
  does Waitlist work?" names the `Waitlist` flow; "how does waitlist work?", a first-word
  "Waitlist: how does it work?" and a statement name nothing. Failure modes: a lower-case word or
  the first word never names a one-word subject; an identifier such as `WaitlistNotifier` is not
  the one word `waitlist`. Acceptance: `TestDCSV1548QuestionNamesAOneWordSubject`.
- `DCS-V1-549`: Amending `DCS-V1-482` and `DCS-V1-483`, derived declarations also cover `source`
  inputs at each additional repository's declared revision (`DCP-V1-175`); other revisions are
  still skipped. A declaration of an additional repository is
  `<native>:symbol:<repository id>:<path>:<line>:<name>`, anchored in that repository; primary
  revision IDs are unchanged, and the `DCS-V1-484` gap names `<native>:file:<repository id>:<path>`.
  Failure modes: a manifest without a native provider derives nothing; a single-repository
  artifact's records are byte-identical; an additional repository's path never collides with a
  primary path of the same name. Acceptance:
  `TestDCSV1549DerivedDeclarationsInAdditionalRepository` (no declaration before the change),
  `TestDCXV1471DerivationVersionGuard`.
- `DCS-V1-550`: Amending `DCS-V1-485`, `DCS-V1-549` changes the records the same multi-repository
  inputs build, so `DerivationVersion` is `3`: the verified read refuses a derivation 2 artifact
  with `derivation version differs (artifact 2, running builder 3); rebuild the corpus`. The
  single-repository fixture digest of derivation 3 equals derivation 2's. Acceptance:
  `TestDCSV1549DerivedDeclarationsInAdditionalRepository`,
  `TestDCSV1485DerivationTwoRefusesDerivationOneArtifacts`, `TestDCXV1471DerivationVersionGuard`.

Non-goals: no coverage multiplier or score threshold, no document-length normalization, no
`docs_claims` change, no `named` tier for a one-word name in a statement, and no derivation for
revisions other than the primary and each additional repository's declared one. Rollback: restore
the score-only `terms` order and the two-word `named` rule, drop the additional-repository
derivation and return `DerivationVersion` to `2`; artifacts built in between are refused as
another derivation and rebuilt. No stored state needs migration.

### Content-word coverage, ID-slug names and bounded declaration derivation (GitHub #1665 re-test of ec41e4e)

The re-test of #1665 on `ec41e4e` left three shapes open (V1-2598). The `DCS-V1-547` coverage
count held the frame words of a question, so in "Where in the source is Player Types
implemented?" a paragraph holding `source` and `implemented` held as many query words as the
subject holding `player` and `types`, and general search fell from 63 to 59 of 64 cases. A flow
whose ID slug spells the question's name (`flows/reservation-cancellation`) but whose name does not
reached no `named` or path-phrase tier, while a paragraph named `Cancellation`, or `In the source`,
reached tier `named` by one word. And a multi-repository corpus over a bound derived 0
declarations: the `DCS-V1-484` retry dropped all of them, and its gap, `derived declaration bound`,
named no bound, value or count and sat on the first derivable file. Status for `DCS-V1-565..569`:
proposed, experimental; GitHub #1665.

- `DCS-V1-565`: Amending `DCS-V1-547` and `DCS-V1-544`, in a question-shaped query
  (`DCS-V1-543`) a query word counts as held (the served `terms` of a subject or claim) only when
  it is a content word: it yields an evidence term and is neither a question word nor a frame word
  (`source`, `sources`, `codebase`, `repo`, `repository`, `implemented`, `implement`, `implements`,
  `implementation`, `implementations`, `defined`, `define`, `defines`, `definition`, `declared`,
  `declare`, `declares`, `located`, `location`, `live`, `lives`, `handled`, `handle`, `handles`,
  `work`, `works`, `happen`, `happens`). A term group only frame and question words yield is not
  held; one any content word, or no word (an identifier compound), yields is. Scores are unchanged,
  and a statement still counts every word. Failure modes: a frame word never lifts a result over one
  holding more content words inside tier `terms`; a statement's served `terms` are unchanged; claims
  follow the same rule. Acceptance: `TestDCSV1565QuestionCoverageCountsContentWords` (the frame
  paragraph holds 3 words before the change).
- `DCS-V1-566`: Amending `DCS-V1-543`, a question-shaped query also names, in tier `named`, a
  subject whose ID slug (the ID after its last `/` or `:`, without its last dot suffix) has at
  least two identifier words, at least two of them content words, that appear as a contiguous run
  of the question's words. So "Where in the source is Reservation Cancellation implemented?" names
  `flow:flows/reservation-cancellation` and `doc:docs/reservation-cancellation.md`. Failure modes:
  a one-word slug (`flows/reservation`) and a statement name nothing; the slug prefilter keeps the
  named check allocation-free for subjects whose slug the query cannot hold. Acceptance:
  `TestDCSV1566QuestionNamesAnIDSlug` (the flow is in tier `terms` before the change).
- `DCS-V1-567`: Amending `DCS-V1-543` and `DCS-V1-548`, a multi-word name reaches tier `named`
  only when at least two of its words are content words, and a one-word name only by a capitalized
  field that no other capitalized field, other than the first, adjoins. So `Cancellation` is not
  named by "Reservation Cancellation", nor `In the source` by its frame words; "How does Waitlist
  work?" still names `Waitlist`. Failure modes: a one-word match inside a longer capitalized name
  never reaches tier `named`; `DCS-V1-548`'s accepted cases still hold. Acceptance:
  `TestDCSV1567NamedTierNeedsTheWholeName` (`Cancellation` is named before the change),
  `TestDCSV1548QuestionNamesAOneWordSubject`.
- `DCS-V1-568`: Amending `DCS-V1-484`, derivation stops at the corpus record bound in the first
  compile: it derives, in manifest input order and symbol order, the declarations the record bound
  leaves room for (the bound less the larger of the subject and claim counts the providers built)
  and drops the rest, with no retry. A build that drops any derived declaration carries exactly one
  `not-collected` gap on `<native>:derived-declarations`, no file ID, whose reason is
  `derived declaration bound: B; derived K of N declarations of source inputs other providers
  claim, D dropped from [<repository id>:]<path>:<line> onward in manifest input order`, where `B`
  names each bound that dropped any: `record bound R records (SCHEMA) reached`, `output byte bound
  L bytes (SCHEMA) exceeded at S bytes with A derived` (`DCS-V1-569`), or `the compile with A
  derived was refused (REASON)` (any other corpus refusal, which derives none as before). Failure
  modes: a build that drops nothing has no such gap; a non-corpus error is returned without a
  retry, as before; the gap never names an arbitrary file. Acceptance:
  `TestDCSV1568DerivationStopsAtTheRecordBound` (2 compiles before the change),
  `TestDCSV1484RefusedFirstDeclarationRestoresShardCache`,
  `TestDCSV1484RetryAnswersOnlyCorpusRefusals`.
- `DCS-V1-814`: (proposed, experimental under the standing lane auto-accept; V1-2748, GitHub
  #1665) The per-declaration encoding is recorded, not reduced, in this version. Measured
  2026-10-31 on linux/WSL2 over 401 derived declarations of one records-owned Ruby controller: a
  derived declaration adds 2,032 bytes (subject 962, declaration claim 1,070), of which its two
  anchors take 1,085; the rest is member names, the evidence states and the one limitation
  string. The anchors are the cost: each carries the repository, revision, blob, file and span
  digests that let every reader, the verified read, `delta` and carry-forward verify that record
  alone (product invariant 1), and a provider's compact `/2` record (`DCX-V1-487`) is already
  expanded to those anchors at admission. Halving it needs an artifact-level file table, a new
  artifact schema that every reader, the streamed decode (`DCR-V1-431`), `delta` and the MCP
  bridge would decode, with a `DerivationVersion` bump; that is its own change with owner review
  and is not taken here. Until then `DCS-V1-568` and `DCS-V1-569` keep the build within the
  128 MiB bound by deriving the prefix that fits and naming the rest in one gap, and a derive
  filter scopes the declarations a caller wants. Acceptance:
  `TestDCSV1814DerivedDeclarationEncodedSize` (logs the size and its parts; fails above 2,048
  bytes, so an encoding change that grows a declaration is caught), the ranking and declaration
  tests unchanged.
- `DCS-V1-569`: Amending `DCS-V1-484`, a compile the output byte bound refused is retried once
  deriving the prefix of the same declarations that fits under the bound with a 4096-byte reserve,
  not none. It drops declarations until a lower bound on their encoded bytes (subject, claim, their
  quoted capability IDs and, for a test declaration, its unobserved-test gap and shortest gap
  offset) reaches the overshoot, since an overestimate would keep too many. If that retry is
  refused too, the next derives none. The byte bound is known only after the whole artifact is
  encoded, so an over-bound corpus compiles twice (three times at most). `DerivationVersion` is `4` (`DCX-V1-471`, amending `DCS-V1-550`): the
  same over-bound inputs now build declarations and a different gap, so the verified read refuses a
  derivation 3 artifact with `derivation version differs (artifact 3, running builder 4); rebuild
  the corpus`; the single-repository fixture digest of derivation 4 equals derivation 3's.
  Failure modes: the retry never exceeds the bound it answers; an artifact of an earlier derivation
  is refused, never read with a stale gap. Acceptance:
  `TestDCSV1569DerivationStopsAtTheOutputBound` and `TestDCSV1569TestDeclarationsStopAtTheOutputBound`
  (before the change the retry derived 0, which
  `TestDCSV1484DerivationNeverRefusesABuildableManifest` then asserted),
  `TestDCSV1484DerivationNeverRefusesABuildableManifest`, `TestDCXV1471DerivationVersionGuard`,
  `TestDCXV1564ReleaseNotesListEveryDerivation`.

Non-goals: no caller choice of which declarations to keep (by path or kind), no output-size
estimate before the first encode, no new frame-word configuration, no score change, and no change
to the `exact`, `identifier`, `path-phrase`, `phrase` or `id-phrase` tiers. Rollback: drop the
content-word coverage, the slug and whole-name `named` rules, and the bounded derivation (restore
the derive-none retry and its file gap), and return `DerivationVersion` to `3`; artifacts built in
between are refused as another derivation and rebuilt. No stored state needs migration.

### Declaration claims rank by their symbol (GitHub #1665 re-test of b63dd0d)

The re-test of #1665 on `b63dd0d` (V1-2628) added a `kind: native` provider and 8,326 bounded
declarations, and the claims suite fell from 45 to 39 of 52 cases. All six misses had one shape:
`claims "validate PLI lightspeed payments"` returned `.../lightspeed_payments/validate_pli.rb:43:pli:declaration`
(`def pli`), then `:10:call:declaration` and `:39:club:declaration`, above the descriptive claims
that hold the query words. A claim's own ID terms rank it (`DCP-V1-313`), and a declaration
claim's ID is its symbol subject's ID, which spells the declaring file's path, so `def call`
held every word of the path. Status for `DCS-V1-570`: proposed, experimental; GitHub #1665.

- `DCS-V1-570`: Amending `DCP-V1-313` and `DCS-V1-402`, a native declaration claim (one from a
  `kind: native` provider of the manifest whose ID is its subject's ID plus `:declaration` and
  whose one anchor names a symbol) is ranked, in the
  `claims` operation and in `docs_search` with `include:["claims"]`, by the terms of its text and
  of its anchor's symbol name in place of its ID's terms; every other claim, including a records
  provider's claim of the same shape, keeps its ID and text terms. So a declaration claim matching a query only through its file path is not a hit, and
  `claims "club"` or `claims "ValidatePli"` still ranks the declaration of `club` or
  `ValidatePli` first. Both query-index builders derive the same claim-term postings; the
  postings are not encoded, so no artifact or derivation version changes. Failure modes: a
  declaration claim never outranks a claim whose text holds more query words because of its
  path; a query naming the symbol still finds its declaration claim; a query naming only a path
  finds the declaration claims by `claims --path`, not by text search; claim-term blocks a read
  derives before it has the manifest are used only when every declaration-shaped claim they
  ranked by its symbol came from a native provider, and are otherwise derived again. Acceptance:
  `TestDCSV1570DeclarationClaimsRankBySymbolNotPath` (the `pli`, `call` and `club` declaration
  claims order first before the change),
  `TestDCSV1570DeclarationClaimRanksWhenTheQueryNamesTheSymbol`,
  `TestDCSV1570RecordsDeclarationShapedClaimKeepsItsIDTerms`,
  `TestDCSV1482DeclarationsOfRecordsOwnedSource`.

Non-goals: no change to subject ranking, which still holds a declaration subject's ID in its name
field. Restriction of derivation by path or kind (V1-2614) and the cost of an over-bound compile
(V1-2613) are `DCS-V1-573` and `DCS-V1-572`. Rollback: rank declaration claims by their whole ID
again by reverting the change; no stored state needs migration.

### Class members, the over-bound retry and a derivation filter (GitHub #1665 re-test of 39f5c02)

The re-test of #1665 reported three gaps. A TypeScript method such as `syncWithPos` in
`export class Cart { syncWithPos(): void {} }` had no declaration, because the native web index
records only top-level declarations, so `docs_search --query syncWithPos` could not find it
(V1-2548). A 114 MB, four-repository corpus over the 128 MiB output bound built in 15.1 s against
10.9 s for its first compile, since the retry repeats the whole compile (V1-2613). And a bounded
derivation keeps a prefix chosen by manifest input order, not by what the caller needs (V1-2614).
Status for `DCS-V1-571..573`: proposed, experimental; GitHub #1665.

- `DCS-V1-571`: Amending `DCS-V1-482` and `DCP-V1-003`, the methods of every class body in an
  indexed TypeScript or JavaScript source are declarations: a native-owned source's members are
  native symbol subjects with the same ID shape, anchor and `:declaration` claim as its top-level
  declarations, and a records-owned source's members are derived declarations under the
  `DCS-V1-568` and `DCS-V1-569` bounds. A member is a name followed by `(`, `<` or `?(` at the
  start of a class-body member, after any decorators, `*`, and modifiers (`public`, `private`,
  `protected`, `static`, `readonly`, `abstract`, `async`, `override`, `declare`, `accessor`,
  `get`, `set`), and `#name` keeps its `#`; constructors, fields and index signatures are not
  members. A class body is the first `{` at the bracket depth of the `class` keyword outside its
  heritage clause's type arguments, so `extends Base<{ a(): void }>` opens none, and a line break
  after a name, literal, number or closing bracket ends a member. The class-body walk is a
  separate pass over the lexer's tokens; the native index's
  top-level symbols, imports and import-parity records are unchanged. Members are ordered with the
  top-level declarations by line, and a member on the same line and name as one is not repeated.
  `DerivationVersion` is `5`: an artifact of derivation 4 is refused with `derivation version
  differs (artifact 4, running builder 5); rebuild the corpus`; the single-repository fixture
  digest of derivation 5 equals derivation 4's, since it holds no web source. Failure modes: a
  source the lexer cannot finish (a quoted string the line ends inside, an unterminated template
  or comment) or whose brackets do not balance or match yields no members, never a guessed one; a
  `class` property access (`x.class`) is not a class; a getter and setter of one name on one line
  are one declaration; interface members, object-literal methods and arrow-function properties
  are not walked; the symbol
  resolver does not re-resolve a member anchor, so a member anchor in a later revision is
  unresolved, never re-anchored to another declaration. Acceptance:
  `TestDCSV1571TypeScriptClassMembersAreDeclarations` (the subjects hold no `syncWithPos` before
  the change), `TestWebClassMembersNamesMethodsOfEveryClassBody`,
  `TestDCSV1571AccessorPairOnOneLineIsOneDeclaration`,
  `TestDCXV1471DerivationVersionGuard`, `TestDCXV1564ReleaseNotesListEveryDerivation`.
- `DCS-V1-572`: Amending `DCS-V1-484` and `DCS-V1-569`, an over-bound corpus still compiles twice,
  and the retry reuses what the refused compile pinned: the index of each revision, every input's
  bytes as verified against its blob and digest, and each input's line table. The retry pins its
  inputs only to close, so a build that once pinned three times pins twice; its artifact is
  byte-identical to a retry that pins again. Anchoring a declaration reads one shared line table
  per input and copies only its own line, where it split and digested the whole input per anchor.
  One compile cannot replace the two: the output byte bound applies to the final canonical
  encoding, and the stages after derivation (capability counts, claim-mention, flow and test
  joins, readable-record validation) read the derived records, so a single compile that guessed
  the kept prefix before encoding would build different bytes from the retry, or exceed the bound.
  Failure modes: an input that changes during the build is refused by the closing pin as before;
  a refusal by the closing pin hands its retry nothing pinned; the line table refuses exactly the
  spans the whole-input split refused. Acceptance: `TestDCSV1569DerivationStopsAtTheOutputBound`
  (3 pins before the change), `TestDCSV1572RetryFromPinnedInputsIsByteIdentical`,
  `TestDCSV1572AnchorLinesSelectWhatSpanSelects`; measured in
  https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-25-gh1665-class-members-retry-filter.md.
- `DCS-V1-573`: Amending `DCS-V1-482` and `DCS-V1-568`, the manifest's first `kind: native`
  provider may carry `derive: {paths?, kinds?}`, which restricts the declarations derived from
  source inputs other providers claim. An input is derived when `paths` is empty or one entry
  names it, and `kinds` is empty or holds its declaration kind (`symbol` or `test`, by the
  `DCS-V1-482` test-path rule). A path entry is `[<repository id>:]<path>`, exact or, ending in
  `/`, a directory prefix; the qualifier names an additional repository, and no qualifier the
  primary one. A colon always ends a qualifier, so a primary path holding a colon is selected
  only through a directory entry. Both lists are sorted and duplicate-free, and `paths` holds at most 64 entries.
  Excluded inputs are not counted against the record or byte bounds, and the build carries one
  `not-collected` gap on `<native>:derivation-filter` whose reason is `derivation filter (paths
  [P]; kinds [K]) excluded N declarations of M source inputs: <first 16 inputs>[ and X more]`.
  The field is omitted when absent, so a manifest without it keeps its bytes, every manifest read
  before stays readable, and `DerivationVersion` does not change; native-owned inputs keep every
  declaration unless the filter sets `native` (`DCS-V1-590`). Failure modes: a filter on a records provider or on a native provider that derives
  nothing, an empty filter, an unknown kind, a qualifier naming no declared additional
  repository, a non-canonical path, or unsorted or duplicate
  entries refuse the manifest with `corpus-refused` before any input is read; a builder from
  before this change refuses a manifest with the field as an unknown field, never ignores it; a
  filter that selects nothing derives nothing and names every excluded input in the gap.
  Acceptance: `TestDCSV1573DerivationFilterKeepsTheSelectedDeclarations`,
  `TestDCSV1573DerivationFilterRefusesMalformedFilters`,
  `TestGH1665ResidualTruncatedDeclarationRecoveredByFilter` (the GitHub #1665 exact-file
  residual shape: a bulk file earlier in input order exhausts the bound and drops the queried
  method's declaration; a `paths` filter keeps it and it ranks first).

Non-goals: no interface, object-literal or arrow-function members; no symbol re-resolution of
member anchors; no output-size estimate before the first encode; no filter on native-owned inputs
(added opt-in by `DCS-V1-590`) or by symbol name. Rollback: revert the change and return `DerivationVersion` to `4`; artifacts of
derivation 5 are then refused as another derivation and rebuilt, and a manifest that uses
`derive` is refused as an unknown field until the filter is removed. No stored state needs
migration.

### Declaration subjects rank by their symbol; anchored inputs derive first (GitHub #1665 re-test of 6edb100)

Status: experimental under the standing lane auto-accept (V1-2792, V1-2793). The reporter's
re-test of 6edb100 searched with `include: ["claims"]` for "validate PLI lightspeed payments" and
"set admin user". Native declaration subjects such as `call`, `club`, `user` and `Admin` held the
query words only through the file path in their IDs, and ranked above the descriptive claims.
`DCS-V1-570` had fixed this for declaration claims only. Separately, the derived declaration
bound dropped declarations in manifest input order, so a bulk file early in the manifest could
crowd out the file that the caller's records point at.

- `DCS-V1-574`: Amending `DCS-V1-400`, `DCS-V1-570` and the parent's `DCP-V1-328`. `include:
  ["claims"]` on `search` stays additive: it adds claim hits to the subject hits in one ranking,
  and is not a claims-only filter. A caller that wants claims alone uses the `claims` operation,
  and the `--include` help and the agent guide say so. When a search includes claims, a native
  declaration subject is ranked by the terms of its symbol name (name weight) and of its claims'
  text (claim weight), never by the path its ID spells. A native declaration subject is one that a
  `kind: native` provider of the manifest built, whose ID is `<provider>:symbol:<path>:<line>:<name>`,
  and whose one anchor names its symbol. It also does not take the id-phrase tier, because only its
  ID holds that phrase. A query that names the symbol still finds the declaration, ranked by name
  as before. The terms come from the query index's claim postings, so the slow and fast read paths
  rank identically. A search without `include` ranks subjects exactly as before, ID terms included.
  Failure modes: a subject that is not a native declaration (records subjects, modules, inventory
  rows, a native subject with a different ID shape or anchor) keeps its ID terms; a declaration
  subject with no claim ranks by its name alone; a query matching nothing but a declaration's path
  answers without that declaration, as `claims --query` does. Acceptance:
  `TestDCSV1574DeclarationSubjectsRankBySymbolNotPath` (reporter case "validate PLI lightspeed
  payments"), `TestDCSV1574AdminDeclarationsDoNotCrowdOutTheClaim` (reporter case "set admin
  user"), `TestDCSV1574DeclarationSubjectRanksWhenTheQueryNamesTheSymbol`,
  `TestDCSV1574SubjectOnlySearchKeepsIDTerms`. The first two failed before the change.
- `DCS-V1-575`: Amending `DCS-V1-568`, `DCS-V1-569` and `DCS-V1-573`. Derived declarations are
  counted against the record and output byte bounds in derivation order. That order puts first
  every candidate source input that a subject or claim of a non-native provider anchors (by
  revision and path), then every other candidate input. Each group follows manifest input order,
  and symbol order within an input. A caller steers the kept set by anchoring the files it cares
  about in its records, and narrows it further with the `DCS-V1-573` filter. The kept declarations
  are added to the artifact in manifest input order and symbol order, whatever order derived them.
  The bound gap still counts every dropped declaration and names the first one dropped, now in
  derivation order: `... N dropped from <repo:path:line> onward in derivation order (inputs
  records anchor first, then manifest input order)`. A build that reaches no bound, including
  every existing under-bound fixture, is byte-identical to derivation 5, so the
  `TestDCXV1471DerivationVersionGuard` digest is unchanged. Builds over the bound can keep
  different declarations, so `DerivationVersion` becomes `6`, and a verified read refuses a
  derivation-5 artifact until it is rebuilt. Failure modes: when anchored inputs alone exceed the
  bound, the later anchored inputs in manifest order are dropped and named; when no input is
  anchored, the order is manifest input order, as in derivation 5; the output-byte retry keeps
  the derivation-order prefix that fits. Acceptance: `TestDCSV1575AnchoredInputsDeriveFirst`,
  which failed before the change; `TestGH1665ResidualTruncatedDeclarationRecoveredByFilter`,
  `TestDCSV1568DerivationStopsAtTheRecordBound`, `TestDCSV1569DerivationStopsAtTheOutputBound`,
  `TestDCXV1471DerivationVersionGuard` and `TestDCXV1564ReleaseNotesListEveryDerivation` (which
  needs the merge number in the derivation-6 release-notes entry).

Non-goals: no claims-only mode for `search`; no change to plain `search` subject ranking or to
records, module or inventory subjects; no caller-supplied priority list or ranking of
declarations by query; no derivation without a native provider (V1-2747 owns that). Rollback:
revert the change and return `DerivationVersion` to `5`. Derivation-6 artifacts are then refused
as another derivation and rebuilt. No stored state needs migration, and no wire field changes.

### Mixed-query identifier tokens, match anchors, unmatched terms and exported object keys (GitHub #1970, #1971, #1975)

Status: experimental under the standing lane auto-accept (V1-3068, V1-3069, V1-3073). Three
reports against the corpus search: a mixed query such as "parseRequest corpus request reader" ranked
a prose document above the `parseRequest` declaration it named, and the document's result pointed at
its first line rather than the matching paragraph (#1970); a partly matching query gave no sign which
of its words matched nothing (#1971); and the keys of an exported configuration object, such as
`optimizeDeps` in `export default defineConfig({ ... })`, had no declaration (#1975).

- `DCS-V1-580`: Amending `DCS-V1-541`. In a query of two or more words, a word that is an
  identifier token (after trimming punctuation such as backquotes, it splits into two or more
  identifier words, as `parseRequest` or `parse_request` does) and that equals a subject's name,
  ignoring case, places that subject in the identifier tier unless an earlier tier already holds it.
  A plain word is never an identifier token, and a one-word query keeps the `DCS-V1-541` rule.
  Failure modes: a token that names no subject changes nothing; several subjects with the token's
  name share the tier and are ordered by the existing within-tier score. Acceptance:
  `TestDCSV1580MixedQueryRanksExactSymbolToken`, which failed before the change.
- `DCS-V1-581`: A served `result_ranks` entry for a subject gains `match_claim`, the ID of that
  subject's claim covering the most query word groups, and `match_anchor`, that claim's first
  anchor. Both are omitted when no claim covers a query word, or when the best claim's anchor names
  the subject's own path and start line (a declaration's claim, which adds nothing). The claim is
  chosen from the artifact's claims whether or not the search includes claims, and ties keep the
  first claim in artifact order. The fields are additive and optional; existing readers ignore them.
  Acceptance: `TestDCSV1581DocumentResultCarriesMatchingParagraphAnchor`.
- `DCS-V1-582`: Wire contract. A `search` receipt gains `unmatched_terms`: the query's
  whitespace-separated words, in query order and without duplicates, whose evidence terms no
  subject (and, when the search includes claims, no claim) holds. A word that is an identifier with
  a compound term (`ReaderParse` → `reader-parse`) is unmatched when its compound is absent, even if
  its parts occur. A word with no evidence terms (a stop word or punctuation) is never reported. The
  field is omitted when every word matched. `miss` is unchanged: it stays empty whenever any result
  is served and remains `no-match` only when nothing matched, in which case `unmatched_terms` lists
  every content word. Acceptance: `TestDCSV1582SearchReportsUnmatchedTerms`.
- `DCS-V1-583`: Amending `DCS-V1-482` and `DCS-V1-571`. The top-level keys of an exported object
  literal in an indexed TypeScript or JavaScript source are declarations of kind `property`, with
  the same subject, anchor and `:declaration` claim shape and the same native-owned and
  records-owned handling as class members. The recognized forms, at the file's top level, are
  `export default {…}`, `export default f({…})`, `export default NAME` where `NAME` is a top-level
  `const`, `let` or `var` bound to such an object and the statement ends after `NAME` (at `;`, end
  of file, or by ASI at a later line that starts with a word that does not continue the expression,
  a quoted string, `{`, `!` other than `!=` or `!==`, or a `++` or `--` punctuation pair (not `+`
  before a string), but not a template literal; V1-3075, V1-3076, V1-3089),
  `export const|let|var NAME[: type] = {…}` or a call wrapping one, and `module.exports = {…}`. A key is an identifier or a quoted string of
  identifier characters, `-` and `.`, of at most 128 bytes, followed by `:`, `,`, `}`, `(` or `?`,
  after any `async`, `get`, `set` or `*` modifier. Nested keys, spreads, computed keys, a `NAME` that only starts the
  exported expression (`export default NAME ? a : b`, `NAME || b`, `NAME.inner`), objects not
  exported, and objects returned from functions are not declarations. At most 128 keys per object
  and 256 per file are kept, in source order. Failure modes: mismatched or unclosed brackets yield
  no keys for that file and do not refuse the build; an over-bound object keeps its first keys.
  Acceptance: `TestDCSV1583WebExportedObjectKeys`, `TestDCSV1583WebExportedObjectKeysAreBounded`,
  `TestDCSV1583ExportedObjectKeysAreDeclarations` (a key query ranks the key above an unrelated
  const), `TestDCSV1571TypeScriptClassMembersAreDeclarations`.
- `DCS-V1-584`: Because `DCS-V1-583` adds subjects to builds of sources with exported objects,
  `DerivationVersion` becomes `7` and a verified read refuses a derivation-6 artifact until it is
  rebuilt. A source with no exported object is byte-identical to derivation 6, so the
  `TestDCXV1471DerivationVersionGuard` digest is unchanged. `docs/RELEASE-NOTES.md` lists
  derivation 7 with its merge number. Acceptance: `TestDCXV1471DerivationVersionGuard`,
  `TestDCXV1564ReleaseNotesListEveryDerivation`.

Non-goals: no change to `miss` semantics or to the MCP outcome classification; no per-word match
positions; no nested or computed object keys, no evaluation of spreads or imports, and no keys of
non-exported objects. Rollback: revert the change and return `DerivationVersion` to `6`.
Derivation-7 artifacts are then refused and rebuilt; the additive `match_claim`, `match_anchor`
and `unmatched_terms` fields disappear, and no stored state needs migration.

### Native derivation filter for native-owned inputs (GitHub #2000)

Intent: an ownership assessment over a large production scope needs the native module subjects,
not every symbol and test declaration, and those declarations alone can fill the output bound
before any provider adds flows. `DCS-V1-573` restricted only inputs other providers claim.

- `DCS-V1-590`: (proposed; GitHub #2000; V1-3103; experimental under the standing lane
  auto-accept) Amending `DCS-V1-573`. The `derive` filter gains an optional `native: true`,
  which applies its `paths` and `kinds` also to the symbol and test declarations of the inputs
  the filtering provider itself owns, and `kinds` admits `module`, which names the file subject
  every input keeps, so `kinds: ["module"]` derives no declaration. An unselected native-owned
  input keeps its file subject, excerpt and gaps and loses only its declarations, and the build
  carries one `not-collected` gap on `<native>:native-derivation-filter` whose reason is `native
  derivation filter (paths [P]; kinds [K]) excluded N declarations of M native-owned inputs:
  <first 16 inputs>[ and X more]`. Without `native` the filter keeps its `DCS-V1-573` meaning,
  and an absent field encodes nothing, so every existing manifest keeps its bytes and its
  artifact, and `DerivationVersion` does not change. Module ownership (`DCP-V1-720..724`) reads
  only module subjects and their anchors and relations, so a module-only corpus keeps it.
  Non-goals: a per-symbol filter, a filter on a second native provider, and an automatic
  module-only profile. Failure modes: `native: true` with neither `paths` nor `kinds` is refused
  as a vacuous filter; a builder from before this change refuses `native` or the `module` kind as
  unknown, never ignores it; search and impact reads over a module-only corpus find no
  declaration subjects, which the gap states. Rollback: drop the field and the `module` kind;
  manifests without them are unaffected. Acceptance:
  `TestDCSV1590NativeDerivationFilterRestrictsNativeOwnedInputs` (`internal/doccorpus`), which
  failed before the change.

### Claim kinds, path owners and navigation queries (GitHub #2012, #2013, #2014)

Intent: an agent asking where a path is documented, which flows a test file drives, or what a
spec promises must reach the owning flow, the flow named by its test, and the behavioural claim
rather than the run record that repeats the spec name. Each answer is read from typed provider
evidence; where that evidence is absent, the answer is absent or labelled `unspecified`.
Status for `DCS-V1-591..597`: proposed; experimental under the standing lane auto-accept;
V1-3125, V1-3126 and V1-3127.

- `DCS-V1-591`: (GitHub #2014; V1-3127) A `claims` receipt carries `claim_kinds`, which maps
  each visible claim to its kind as `claimParityKind` reads it from the claim's typed details:
  `status`, `test_run`, `coverage_ledger`, `flow_completeness` or `unresolved_concept` for a
  supplied record of that type, else the provider's `claim_kind` (`behavior`, `constraint`,
  `side_effect`, `integration`, `permission`), else `unspecified`. The kind is never inferred
  from a claim's ID or text, so a claim shaped like a run record without a typed test-run detail
  is `unspecified`. Other operations keep their bytes.
- `DCS-V1-592`: (GitHub #2014; V1-3127) `claims` takes an optional kind filter (`Kinds`; MCP
  `kind`; CLI `--kind`) and exclusion (`ExcludeKinds`; MCP `exclude_kind`; CLI `--exclude-kind`),
  each one kind or a list of distinct kinds from `DCS-V1-591`'s closed vocabulary. They apply to
  every claims form (path, query, state), before paging, and keep only the claims whose kind is
  named and not excluded. An unknown, repeated or empty kind list (a CLI flag given with an empty
  value included), a kind both kept and excluded, or an exclusion on any operation other than
  `claims` is refused as invalid arguments (CLI exit 2). `docs_claims` advertises both properties; no other tool advertises `exclude_kind`.
- `DCS-V1-593`: (GitHub #2014; V1-3127) A `claims` query ranks every claim whose kind is
  `test_run` after every other claim, whatever its word coverage, and starts a new rank group
  there. Each claim keeps its served tier, terms and score, and the order within each part is
  unchanged. A claim with no typed kind keeps its place.
- `DCS-V1-594`: (GitHub #2012; V1-3125) `locate` on a path also returns, for each subject anchored
  at that path, every relation of type `documents` whose `To` is that subject, and that
  relation's `From` subject when the corpus holds it: exactly one incoming `documents` hop, each
  owner justified by its relation. Outgoing `documents` relations, other relation types and a
  second hop add nothing; a dangling `From` keeps only its relation. A record already in the
  receipt is not repeated, and the indexed and scanned forms return the same records.
- `DCS-V1-595`: (GitHub #2012; V1-3125) `recommend-tests` on a path returns the same owners and
  relations, and also the test-link relations (`asserts`, `navigates` with a TestLink detail) of
  those owners, as it does for the path's own subjects. The recommendation stays
  `full-relevant-suite-required` with narrowing not allowed: an owner's test link is evidence,
  never a selection. A `docs_locate` or `docs_recommend_tests` batch keeps one receipt per path.
- `DCS-V1-596`: (GitHub #2013; V1-3126) `navigation` takes a `query` instead of an `id`, never
  both. It selects each `flow` subject whose ID, name, or the file or title of its own `asserts`
  or `navigates` relations' TestLink details hold every query evidence term (an inflection of a
  term counts; compound terms are not required). For each selected flow it returns the flow, those
  TestLink relations with their details, and the records the `id` form returns for that flow,
  including the selectors of every behavior-contract test that names it, so two selected flows
  sharing a test each keep that test's selectors. A
  query with no evidence term, or one whose terms no single flow holds, returns no result; a
  subject that is not a flow is never selected by itself. Paging and receipt bounds are the `id`
  form's, and the `id` form is unchanged.
- `DCS-V1-597`: (GitHub #2013; V1-3126) `docs_navigation` advertises an optional `query` (1 to
  1024 bytes) and no longer requires `id`; exactly one of them is accepted, and `id` with `query`,
  neither, or an empty query is refused as invalid arguments. The CLI `docs corpus navigation`
  takes exactly one of `--id` or `--query`, decided by which flags are given and not by their
  values, and the one given must not be empty (exit 2 otherwise); its receipt is byte-identical
  to the MCP tool's.

Non-goals: a guessed claim kind, a kind taxonomy beyond `DCS-V1-591`, more than one `documents`
hop, a change to test-selection authority, and navigation query support in the companion
`corvint-corpus-mcp` request parser (`corpusindex.ParseRequest`), which refuses `query` for
`navigation` as it does for `locate`. Failure modes: a provider that omits typed kinds leaves its
claims `unspecified`, so the filter can drop them and the demotion cannot reach them; an owner
whose subject is absent is returned only through its relation; a navigation query that matches
many flows is bounded by paging and the receipt bound. Rollback: revert the implementing change;
`claim_kinds` and `exclude_kinds` are omitted when empty, so receipts of other operations and
requests without the new fields keep their bytes. Acceptance:
`TestDCSV1591ClaimsReceiptLabelsClaimKinds`, `TestDCSV1592ClaimsKindFilter`,
`TestDCSV1593ClaimsQueryRanksTestRunsAfterOtherClaims`,
`TestDCSV1594LocateReturnsDocumentingOwners`, `TestDCSV1595RecommendTestsFollowsOwnerTestLinks`,
`TestDCSV1596NavigationQuery` (`internal/doccorpus`) and
`TestDCSV1594CorpusPathOperationsReturnDocumentingOwners`, `TestDCSV1592CorpusClaimKindsCLIAndMCP`,
`TestDCSV1597CorpusNavigationQueryCLIAndMCP` (`cmd/corvint`), each of which failed before the
change.

## Non-goals

The parent contract's non-goals and simpler baseline apply; these blocks add none of their own.

## Failure modes

`docs_search` ranking (`DCS-V1-400..403`) has no new refusal. A query without evidence terms has no
partial hit in either family (its normalizing sum is zero) and answers only exact matches or the
existing no-match miss. A module or file subject with no anchor path keeps its ID and name terms.
A short subject name that holds more query terms than a file's path still outranks the file: the
fixture of `DCS-V1-404` shows the two issue mechanisms are fixed, not that every caller suite
recovers.

## Acceptance evidence

Each block names its acceptance tests, and the Traceability table maps every block to its
implementation and evidence. The parent contract's acceptance gates apply.

## Rollback

Revert the implementing change a block names. The split itself reverts by moving these blocks back
into the parent under their unchanged DCP-V1 numbers and renaming the DCSV1 test prefixes back to
DCPV1.

## Traceability

| Requirements | Implementation boundary | Required evidence |
| --- | --- | --- |
| DCS-V1-400..403 | `internal/doccorpus/search_rank.go` | `TestDCSV1400ModulePathSegmentsAreNameTerms`, `TestDCSV1401ProseNameThreshold`, `TestDCSV1402ClaimFamilyNormalizedMerge`, `TestDCSV1402ExactMatchPrecedesTermOverlap`, `TestDCSV1403MissRetainsNoSubjectNameTerms`, `TestDCPV1328SearchIncludesClaims`, `TestDCPV1331SearchRanking`, `TestDCPV1331ExactBonusAcrossFamilies` |
| DCS-V1-404 | `internal/mcp/corpusbridge/testdata/search-ranking-gh1597/` | `TestDCSV1404SearchRankingEvalFixture` |
| DCS-V1-479..481 | `internal/doccorpus/types.go`, `search_rank.go`, `typed_query.go`, `receipt_annotations.go` | `TestDCSV1479SearchServesRankGroups`, `TestDCSV1480ClaimsServeRankGroups`, `TestDCSV1486GH1665RankingEvalFixture` |
| DCS-V1-482..484 | `internal/doccorpus/compile.go` | `TestDCSV1482DeclarationsOfRecordsOwnedSource`, `TestDCSV1483DerivationScope`, `TestDCSV1484DerivationNeverRefusesABuildableManifest`, `TestDCSV1484RefusedFirstDeclarationRestoresShardCache`, `TestDCSV1484RetryAnswersOnlyCorpusRefusals` |
| DCS-V1-485 | `internal/doccorpus/encoding.go` | `TestDCSV1485DerivationTwoRefusesDerivationOneArtifacts`, `TestDCXV1471DerivationVersionGuard` |
| DCS-V1-486 | `internal/mcp/corpusbridge/testdata/search-ranking-gh1665/` | `TestDCSV1486GH1665RankingEvalFixture` |
| DCS-V1-540..546 | `internal/doccorpus/search_rank.go`, `search_tier.go`, `search_phrase.go`, `types.go`, `internal/mcp/corpusbridge/testdata/search-ranking-gh1665-leftovers/` | `TestDCSV1540QueryWordCountsOnce`, `TestDCSV1541IdentifierTokenTier`, `TestDCSV1542PathPhraseBeforeDeclarations`, `TestDCSV1543QuestionNamesASubject`, `TestDCSV1544ServedTiersAndTerms`, `TestDCSV1404SearchRankingEvalFixture`, `TestDCSV1546GH1665LeftoversRankingEvalFixture`, `TestDCSV1486GH1665RankingEvalFixture`; measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-gh1665-rank-leftovers.md |
| DCS-V1-547..550 | `internal/doccorpus/search_rank.go`, `search_tier.go`, `compile.go`, `encoding.go` | `TestDCSV1547TermsTierOrdersByCoverage`, `TestDCSV1548QuestionNamesAOneWordSubject`, `TestDCSV1549DerivedDeclarationsInAdditionalRepository`, `TestDCSV1485DerivationTwoRefusesDerivationOneArtifacts`, `TestDCXV1471DerivationVersionGuard`; measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-corpus-coverage-multirepo-derivation.md |
| DCS-V1-565..569 | `internal/doccorpus/search_rank.go`, `search_tier.go`, `compile.go`, `encoding.go`, `docs/RELEASE-NOTES.md` | `TestDCSV1565QuestionCoverageCountsContentWords`, `TestDCSV1566QuestionNamesAnIDSlug`, `TestDCSV1567NamedTierNeedsTheWholeName`, `TestDCSV1568DerivationStopsAtTheRecordBound`, `TestDCSV1569DerivationStopsAtTheOutputBound`, `TestDCSV1569TestDeclarationsStopAtTheOutputBound`, `TestDCSV1484DerivationNeverRefusesABuildableManifest`, `TestDCXV1471DerivationVersionGuard`, `TestDCXV1564ReleaseNotesListEveryDerivation`; measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-gh1665-coverage-slug-derivation-bound.md |
| DCS-V1-570 | `internal/doccorpus/query_index.go`, `query_index_fast.go` | `TestDCSV1570DeclarationClaimsRankBySymbolNotPath`, `TestDCSV1570DeclarationClaimRanksWhenTheQueryNamesTheSymbol`, `TestDCSV1570RecordsDeclarationShapedClaimKeepsItsIDTerms`, `TestDCSV1482DeclarationsOfRecordsOwnedSource`; recorded in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-declaration-claims-ranking.md |
| DCS-V1-571..573 | `internal/contextindex/webimports.go`, `internal/doccorpus/compile.go`, `types.go`, `read.go`, `encoding.go`, `docs/RELEASE-NOTES.md` | `TestDCSV1571TypeScriptClassMembersAreDeclarations`, `TestWebClassMembersNamesMethodsOfEveryClassBody`, `TestDCSV1571AccessorPairOnOneLineIsOneDeclaration`, `TestDCSV1569DerivationStopsAtTheOutputBound`, `TestDCSV1572RetryFromPinnedInputsIsByteIdentical`, `TestDCSV1572AnchorLinesSelectWhatSpanSelects`, `TestDCSV1573DerivationFilterKeepsTheSelectedDeclarations`, `TestDCSV1573DerivationFilterRefusesMalformedFilters`, `TestDCXV1471DerivationVersionGuard`, `TestDCXV1564ReleaseNotesListEveryDerivation`; measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-25-gh1665-class-members-retry-filter.md |
| DCS-V1-574 | `internal/doccorpus/search_rank.go`, `cmd/corvint/docs_corpus_help.go`, `internal/capcatalog/AGENT-GUIDE.md` | `TestDCSV1574DeclarationSubjectsRankBySymbolNotPath`, `TestDCSV1574AdminDeclarationsDoNotCrowdOutTheClaim`, `TestDCSV1574DeclarationSubjectRanksWhenTheQueryNamesTheSymbol`, `TestDCSV1574SubjectOnlySearchKeepsIDTerms`, `TestDCPV1328SearchIncludesClaims`, `TestDCSV1570DeclarationClaimsRankBySymbolNotPath` |
| DCS-V1-575 | `internal/doccorpus/compile.go`, `encoding.go`, `docs/RELEASE-NOTES.md` | `TestDCSV1575AnchoredInputsDeriveFirst`, `TestGH1665ResidualTruncatedDeclarationRecoveredByFilter`, `TestDCSV1568DerivationStopsAtTheRecordBound`, `TestDCSV1569DerivationStopsAtTheOutputBound`, `TestDCXV1471DerivationVersionGuard`, `TestDCXV1564ReleaseNotesListEveryDerivation` |
| DCS-V1-580..582 | `internal/doccorpus/search_rank.go`, `types.go`, `receipt_annotations.go`, `internal/capcatalog/AGENT-GUIDE.md` | `TestDCSV1580MixedQueryRanksExactSymbolToken`, `TestDCSV1581DocumentResultCarriesMatchingParagraphAnchor`, `TestDCSV1582SearchReportsUnmatchedTerms`, `TestDCSV1541IdentifierTokenTier`; recorded in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-28-corpus-mixed-query-unmatched-terms-config-keys.md |
| DCS-V1-583..584 | `internal/contextindex/webimports.go`, `internal/doccorpus/compile.go`, `encoding.go`, `docs/RELEASE-NOTES.md` | `TestDCSV1583WebExportedObjectKeys`, `TestDCSV1583WebExportedObjectKeysAreBounded`, `TestDCSV1583ExportedObjectKeysAreDeclarations`, `TestDCSV1571TypeScriptClassMembersAreDeclarations`, `TestDCXV1471DerivationVersionGuard`, `TestDCXV1564ReleaseNotesListEveryDerivation` |
| DCS-V1-814 | `internal/doccorpus/declaration_size_v12748_test.go` (measurement only; no encoding change) | `TestDCSV1814DerivedDeclarationEncodedSize` |
| DCS-V1-590 | `DerivationFilter.Native`, `native`, `validDerivationFilter`, `repositoryQualifier` in `internal/doccorpus/compile.go`, `types.go`; `docs/DOCUMENTATION-CORPUS.md` | `TestDCSV1590NativeDerivationFilterRestrictsNativeOwnedInputs`, `TestDCSV1573DerivationFilterKeepsTheSelectedDeclarations`, `TestDCSV1573DerivationFilterRefusesMalformedFilters` |
| DCS-V1-591..597 | `ClaimKinds`, `ValidClaimKinds` in `internal/doccorpus/adoption.go`; `types.go`, `query.go`, `typed_query.go`, `path_owners.go`, `receipt_annotations.go`; `internal/mcp/corpusbridge/corpusbridge.go`; `cmd/corvint/docs_corpus.go`, `docs_corpus_help.go` | `TestDCSV1591ClaimsReceiptLabelsClaimKinds`, `TestDCSV1592ClaimsKindFilter`, `TestDCSV1593ClaimsQueryRanksTestRunsAfterOtherClaims`, `TestDCSV1594LocateReturnsDocumentingOwners`, `TestDCSV1595RecommendTestsFollowsOwnerTestLinks`, `TestDCSV1596NavigationQuery`, `TestDCSV1594CorpusPathOperationsReturnDocumentingOwners`, `TestDCSV1592CorpusClaimKindsCLIAndMCP`, `TestDCSV1597CorpusNavigationQueryCLIAndMCP`, `TestDCPV1313ClaimTextQueryReachesClaimRecall`, `TestDocsCorpusOperationFlagMatrix`, `TestDocsCorpusOperationHelpPrintsFlags`; recorded in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-29-corpus-claim-kinds-path-owners-navigation-query.md |
