# Documentation corpus search ranking V1

Owner: Russell Lewis
Date: 2026-10-24
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
- Claim: Corpus docs_search ranks files and claims by served rank groups, query-word coverage, identifier tokens, named subjects and derived declarations.
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
| DCS-V1-540..546 | `internal/doccorpus/search_rank.go`, `search_tier.go`, `search_phrase.go`, `types.go`, `internal/mcp/corpusbridge/testdata/search-ranking-gh1665-leftovers/` | `TestDCSV1540QueryWordCountsOnce`, `TestDCSV1541IdentifierTokenTier`, `TestDCSV1542PathPhraseBeforeDeclarations`, `TestDCSV1543QuestionNamesASubject`, `TestDCSV1544ServedTiersAndTerms`, `TestDCSV1404SearchRankingEvalFixture`, `TestDCSV1546GH1665LeftoversRankingEvalFixture`, `TestDCSV1486GH1665RankingEvalFixture`; measured in docs/build-log/2026-10-24-gh1665-rank-leftovers.md |
| DCS-V1-547..550 | `internal/doccorpus/search_rank.go`, `search_tier.go`, `compile.go`, `encoding.go` | `TestDCSV1547TermsTierOrdersByCoverage`, `TestDCSV1548QuestionNamesAOneWordSubject`, `TestDCSV1549DerivedDeclarationsInAdditionalRepository`, `TestDCSV1485DerivationTwoRefusesDerivationOneArtifacts`, `TestDCXV1471DerivationVersionGuard`; measured in docs/build-log/2026-10-24-corpus-coverage-multirepo-derivation.md |
| DCS-V1-565..569 | `internal/doccorpus/search_rank.go`, `search_tier.go`, `compile.go`, `encoding.go`, `docs/RELEASE-NOTES.md` | `TestDCSV1565QuestionCoverageCountsContentWords`, `TestDCSV1566QuestionNamesAnIDSlug`, `TestDCSV1567NamedTierNeedsTheWholeName`, `TestDCSV1568DerivationStopsAtTheRecordBound`, `TestDCSV1569DerivationStopsAtTheOutputBound`, `TestDCSV1569TestDeclarationsStopAtTheOutputBound`, `TestDCSV1484DerivationNeverRefusesABuildableManifest`, `TestDCXV1471DerivationVersionGuard`, `TestDCXV1564ReleaseNotesListEveryDerivation`; measured in docs/build-log/2026-10-24-gh1665-coverage-slug-derivation-bound.md |
