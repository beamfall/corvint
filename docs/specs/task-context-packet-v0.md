# Task-Context Packet V0

Owner: Russell Lewis
Date: 2026-09-02
Requirement prefix: `TCP-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `docs/plans/BREAKTHROUGH-BET-2026-09-01.md` (the bet), `docs/decisions/0024-task-context-packet-2026-09-02.md`
(the owner's instruction and the measured basis), `docs/decisions/0025-cochange-slot-2026-09-02.md`
(the `cochange` slot), `docs/decisions/0026-pair-anywhere-medium-2026-09-02.md` (pair
confidence), `docs/decisions/0033-term-table-2026-09-02.md` (whole-token terms from the term table),
`docs/decisions/0035-reference-slot-2026-09-02.md` (the `reference` slot), `docs/specs/confidently-wrong-trial-v0.md` (the
consumer), `docs/decisions/0065-documentation-is-searchable-evidence-with-its-own-placement-2026-09-05.md`
(documentation admission and placement), `docs/decisions/0067-test-code-linking-in-the-context-packet-2026-09-05.md`
(the `test` relation), `docs/specs/go-production-kernel-migration-v0.md` (the `impact` reverse-import rules this
packet reuses), `AGENTS.md` invariants 1, 2, 4, and 8.

## Agent digest
- Claim: `corvint context` lists the files to read for one task from relations a term search cannot express and keeps the task's own path out of the results.
- Status: proposed/experimental
- Exists: `internal/contextindex/taskcontext.go` (slots incl. `cochange`, decision 0025; `reference`, decision 0035; `test`, decision 0067), `cmd/corvint/taskcontext.go`, help topic `context`, the trial's `corvint` arm; `internal/contextindex/lookup.go` and `cmd/corvint/context_lookup.go` (TCP-V0-017 lookups, proposed).
- Blocked on: a paired trial reading against `grep` on the held-out set; `prove` verdicts on these rows; owner review of the 2026-09-04 amendment TCP-V0-008..012, which is implemented and experimental (`internal/contextindex/taskcontext.go`, tests in `internal/contextindex/taskcontext_widening_test.go`) — it reserves governing instructions and task-named specs, narrows `definition` identifiers, and discloses unexamined scope and slot shortage in `coverage`, and the sentences marked (A) below belong to it.
- Read next: Requirements; Non-goals; Failure modes.

Wave 1: `TCP-V0-018` recipe is retired (0078); identifier terms (`019`) and named-test frames (`020`) failed promotion and remain proposed/off (0076/0077). `021` measures actual cold/hit state and refuses unequal paired results (0075). Decision 0079 repairs complete cold imports and deterministic test evidence.

## Intent and scope

The trial's first observation (`benchmarks/results/cw-trial-heldout-v1-first-run.json`) showed two
defects in what the `corvint` arm handed the agent. Its packets held the gold as a result row for 6
of 50 tasks against a term listing's 33, because `impact` walks reverse imports from a changed path
and most gold files sit in a different relation to it (its test or source counterpart, a file the
review comment names, a definition of an identifier the diff names, a sibling); and every `impact`
packet ranked the changed path first as an authoritative result, which is the row the agent cited
for the dominant confidently-wrong claim in every arm: naming the task's own file as the answer.

The packet compiled here is the answer to both. It is Go-only and experimental: no Python oracle
speaks it, `conformance/cli-parity-v0` does not cover it, and the `query` and `impact` wires are
unchanged. The measured basis (decision 0024) is a reimplementation of these sources over the
50-task corpus reaching gold in the top 20 for 42 tasks against grep's 33 with no task lost; that
number is a ceiling estimate on a small set, not a promise.

Affected user: an agent about to act on one task in one repository. Measurable job: name the files
it must read, each with the relation that admitted it, without naming the task's own file.

## Requirements

- `TCP-V0-001`: `context` MUST be read-only. It builds one index over the committed tree and
  writes no repository, trace, or ledger state on any path, including every error path; its wire
  carries `"mutates": false`.
- `TCP-V0-002`: The invocation is `corvint [--root PATH] context --task TEXT [--subject PATH]
  [--limit N]`. `--task` is required, non-empty, and at most 32,000 characters (a review comment
  with its diff hunk is the intended input, so the bound is wider than `query`'s); `--subject`
  MUST name a path in the Git tree at the revision, whether indexed, excluded, or of a kind the
  index does not read, else the command refuses;
  `--limit` defaults to 20 and is bounded as `impact`'s limit is. Any other argument is refused.
- `TCP-V0-003`: Results are files. Each result row carries `kind`, `id` (the path), `score`,
  `summary`, `action` (one imperative sentence saying what to do with the file for this task,
  fixed per relation and claiming only what the relation establishes: update the test
  counterpart, read the source counterpart, open a named file, reconcile a named definition,
  check an importer, open a file that names or is named by the subject, expect to touch a
  co-changed file, scan a sibling for the task's
  identifiers, read a code term match only if the terms are load-bearing, read a documentation
  term match on the same condition; decision 0028 and decision 0065), and exactly
  one evidence row (`path`, `line`, `blob_hash`, `reason`, `confidence`,
  `authority`) whose `reason` states the relation that admitted it; `blob_hash` is empty for a
  path the index did not read, and that row's `confidence` is downgraded to `low` and the row
  gains an `evidence_gap` field naming why -- the same disclosure TCP-V0-005 makes for the
  subject -- so an unpinned row never reports the relation's own higher confidence as if the
  index had backed it (AGENTS.md invariant 2). A path appears at most once;
  the first slot to admit it keeps it. (A) The reserved relations fix their own row fields so `rowAction` cannot fall through to the term-match
  sentence: `governing` takes the action "read this project's standing instructions before changing anything", `authority` `project-instructions`,
  and a fixed `score` of 1000; `spec-mentioned` takes "read the requirement clause this task names before changing its behavior", `authority`
  `repository-spec`, and 900. Both sit above every lexical row, and after corroboration, reserved rows are placed first and duplicate ordinary rows for the same path are removed, preserving each reservation's fixed fields and sole evidence row while appending the removed rows' actual relation names (`mentioned`, `lexical`, or any other) to its `summary` in fixed slot order. One path still appears once, and TCP-V0-011 counts the dropped row as `withheld` for its own relation.
- `TCP-V0-004`: The slots run in this order and each is capped at three rows before the lexical
  fill: `pair` (the subject's test or source counterpart by stem, the same directory first, then a
  mirrored test/source directory, then anywhere in the tree, or a member of its module directory;
  `test-convention`, high, except that a counterpart found anywhere in the tree by stem alone is
  medium, decision 0026), `mentioned`
  (a tracked path the task names by full path or by an unambiguous basename, and that path's
  counterparts; `task-text`, high), `definition` (a file whose indexed symbol defines an identifier
  the task names: a backticked name, weighted three, or an unbackticked word shaped like code,
  with an underscore, a digit, or camelCase, never a plain prose word; rarer definers first, names
  with more than 50 definers ignored; `syntax`, high), `reverse-import` (the `impact` reverse-import rules applied
  to the subject; `syntax`, high; plus, for a Swift subject under a SwiftPM `Sources/<module>/` or
  `Tests/<module>/` layout, a Swift source in another module that has an `import <module>` line and
  names a symbol the subject defines as a whole word, medium because the import names the module
  rather than the file, decision 0030), `reference` (a source that names, as a whole word in the
  term table, a symbol at least four bytes long that the subject defines (`index.Symbols` with
  `Path` equal to the subject), weighted by the symbol's rarity across sources; a name more than 50
  sources use is a common word and admits nothing; rows are ordered by summed weight then path,
  capped at three, and `score` is 600 plus ten times the weight, clamped to 799; `syntax`, medium,
  decision 0035; naming the subject's stem, in either direction, was probed and admits nothing on
  any set), `cochange` (a tracked path that changed in the same commits
  as the subject among the last 200 non-merge commits behind the revision, a commit touching more
  paths than the age-dependent cap ignored, and a grafted commit (a shallow clone's boundary or a
  grafts entry, whose path list is a diff against a parent Git does not have) never counted (50 paths for a history of at most 50 commits, 8 for
  one filling the 200-commit window, linear between), more shared commits first then path, capped
  at five rather than three because gold is multi-file and the fourth and fifth rows still carried
  gold in the measurement of decision 0025; `git-history`, medium at every count, since history
  is a reason to read a file and not proof that it changes, decision 0025 as amended), `sibling`
  (the subject's directory, then its parent subtree
  where the task's identifiers appear as whole words, a sibling whose basename shares a term with
  the subject's basename first (decision 0031), then by that identifier evidence; `directory`,
  medium), then `lexical` (task terms present as whole tokens in the file or its path, ordered by
  the BM25 score TCP-V0-014 defines, then distinct task terms, then path; `vocabulary`, low) to
  the limit. A term
  is a maximal ASCII alphanumeric run split where a lowercase letter meets an uppercase one,
  lowercased, of at least two bytes; an identifier word is a maximal ASCII word run of at least
  three bytes, as written. Both slots read the tree's term table (`TermTable`, built with the index
  and carried in its snapshot) rather than each source's body, so a term matches a token and not
  a substring (decision 0033). A path that a later slot
  would also have admitted is corroborated: the packet then ranks rows by the number of
  corroborating relations, slot order among equals, raises `score` by 50 per relation, and names
  the relations in `summary` after "; also"; the evidence row stays the admitting relation's
  (decision 0027). Without `--subject` the packet has the retrieval shape: `mentioned`,
  `definition`, `lexical`. (A) The rows reserved by
  TCP-V0-008 and TCP-V0-009 precede this order and each costs one row of `--limit`, and TCP-V0-010 narrows which
  identifiers the `definition` slot may use; the caps, weights and relative slot order above are otherwise unchanged.
- `TCP-V0-005`: The subject is never a result. It is carried under `subject` with its path and a
  `role` sentence saying it is the subject of the question and not one of its answers; every slot
  skips it. When the index pinned the subject, `subject.blob_hash` names the pinned blob. When it
  did not -- the subject was excluded (a forbidden path, a size bound, or a pin-time refusal) or
  its suffix is not admitted for indexing -- `subject.evidence_gap` names why instead; exactly one
  of `blob_hash` and `evidence_gap` is present, never neither and never both (AGENTS.md
  invariant 2: a subject the index could not read must say so, not look like one it could).
- `TCP-V0-006`: `state` is `READY` when at least one result exists and `NO_CANDIDATES` otherwise;
  `coverage` reports the candidates admitted across slots, the results included, and the results
  omitted by the limit. The packet carries no exclusion or unparsed-path samples: nothing in it
  can be mistaken for a result that is not one. (A) TCP-V0-011 adds sample-free `coverage` members; this ban and the two `state` values are
  unchanged. Its `critical` and `critical_missing` are exhaustive over a bounded reserved set (at most one `governing` plus three `spec-mentioned`
  rows), so they state completely what was reserved and what did not fit rather than sampling a larger unshown population.
  Accepted amendment (AT-07, decision 0052): `included_results`, `omitted_results`, `candidates` and
  the top-level `state` are computed before possession suppression and frozen; a new coverage
  member `suppressed_results` carries the count removed by possession; when suppression removes
  every result, `coverage.uncertainty` gains the line "N results suppressed by caller possession"
  and `state` stays as computed (READY/BUDGETED), never NO_CANDIDATES. End of accepted amendment.
- `TCP-V0-007`: Output is canonical JSON (`gokernel.CanonicalJSON`), byte-identical over an
  identical tree, recent history (the 200 commits the `cochange` slot reads), task, subject, and
  limit.
- `TCP-V0-008`: (A) V0 reserves at most one `governing` row. The implementation MUST build a governing-row generator, since `documentKind` classifies
  without establishing applicability. It ranks every tracked path classified `instructions` by one precedence — the literal order `AGENTS.md`,
  `CLAUDE.md`, `GEMINI.md`, `copilot-instructions.md`, `.github/copilot-instructions.md`, then `.github/instructions/*.instructions.md` by ascending
  path. No other nested path is eligible. Project-operation query admission MUST apply this same eligibility check before treating a path classified
  `instructions` as operational; it MAY remain a narrower instruction subset. It skips the subject (TCP-V0-005) and takes the highest-precedence remainder. If that
  candidate exceeds `maxSourceBytes` or is listed in `Index.Exclusions` it is unread: nothing is reserved, no lower-precedence file substitutes, and
  TCP-V0-011's `unexamined` records `governing` with state `capped`. Otherwise it becomes the first result, with TCP-V0-003's one evidence row,
  costing one row of `--limit`. The reservation never consults the task's vocabulary; it is applied after TCP-V0-004's corroboration sort and before
  the final truncation, so neither can reorder or drop it.
- `TCP-V0-009`: (A) The implementation MUST build a definition-owner resolver; no existing helper carries ownership. A spec defines an id when its
  body carries that id's clause line in the grammar `script/check-requirement-definitions.sh` uses: a list-leading, optionally bolded or backticked id
  followed by a colon or a period. The pass reads exactly the tracked `docs/specs/*.md` paths at the captured tree, non-recursive, excluding
  `README.md`, through the bounded reader `sourceTextBounded`, so a spec over `maxSourceBytes` defines nothing and is reported `unexamined`
  `spec-mentioned` state `capped`. A task-side id token has the shape `[A-Z][A-Z0-9-]*-[0-9]{3}` bounded on both sides by a character outside
  `[A-Za-z0-9_-]` or by the text boundary; the subject's own content is scanned for such tokens only when the subject is tracked, read, and under
  `maxSourceBytes`. The pass runs only when at least one candidate id token or tracked `docs/specs/*.md` path appears in the task or subject;
  otherwise it is skipped and its `unexamined` state is `not-applicable`. A match is admitted ONLY when a defining clause exists, so prose such as
  `ISO-123` admits nothing. When several specs define the id, all are candidates in ascending path order, the first that fits the cap is admitted, and
  its evidence `reason` names the matched id then `ambiguous-definition (N specs)`. A tracked `docs/specs/*.md` path the task names is admitted the
  same way, its `reason` naming the path. Paths are deduplicated before the cap of three `spec-mentioned` rows. Title-term overlap alone MUST NOT
  admit a `spec-mentioned` row — discovery relevance is not governing authority — scoped to this relation only: lexical admission of specs is
  unchanged. Spec-mentioned rows are placed immediately after the governing row and before every slot row; corroboration cannot outrank them.
  `coverage` carries the receipt member `governance`: `reserved` when a governing row exists, else `spec-mentioned` when any spec-mentioned row
  exists, else `unresolved`, which means no instruction file and no defining spec was found for this task, and never that none exists (invariant 2).
- `TCP-V0-010`: (A) The implementation MUST build a definition-eligibility predicate applied inside the `definition` row generator only, as a filter
  over the identifiers that generator consumes; it MUST NOT narrow the shared `taskIdentifiers` set, so `pair` ordering and the `sibling` slot's
  identifier evidence are unchanged. An identifier is eligible when it is backticked in the task (the existing tests' `Other` and `Split` stay
  eligible on that ground), camelCase, snake_case, or otherwise carries a digit or an underscore. Operationally: an identifier qualifies when backticked in the task or when it
  contains an uppercase letter after its first character, an underscore, or a digit; an unbackticked lowercase-only identifier does not qualify.
- `TCP-V0-011`: (A) `coverage` gains four members with fixed shapes. `critical` (what TCP-V0-008/009 reserved) and `critical_missing` (reserved
  selectors present at the revision that did not fit) are arrays of `{"relation", "path"}` selectors sorted by relation then path, each exhaustive
  over the bounded reserved set, so neither is a TCP-V0-006 sample. `unexamined` is an array of `{"relation", "state", "withheld"}` in TCP-V0-004's
  fixed relation order with the two reserved relations first: `governing`, `spec-mentioned`, `pair`, `mentioned`, `definition`, `reverse-import`,
  `reference`, `cochange`, `sibling`, `test`, `lexical`, `documentation`; `state` is `examined`, `capped`, `empty-history`, `subject-absent` (the relation needs a
  subject and the retrieval shape has none), `subject-symbols-incomplete` (amended 2026-09-12, invariant 2: `reference` only, when
  `Index.Unparsed` records the subject with facts other than `imports` or `Index.ExtractionNotes` records it, so the slot read a
  missing or partial symbol table for the subject; `withheld` is still counted), or `not-applicable`; `withheld` is an integer, or `null` when not counted. A task-named path runs the
  `pair` generator even without `--subject`, so that relation is `examined`, including when no counterpart is found. Its count includes counterparts
  materialised through the mention slot under the same admission accounting; without a subject or a named path it remains `subject-absent` / `null`.
  `withheld` is the candidates a
  relation's existing generator materialised, minus every candidate admitted under any relation (a corroborated path is not withheld), minus the
  subject, except a row dropped by TCP-V0-003's promote-in-place rule, which is withheld for its own relation. It MUST be measured from existing
  generator output and MUST NOT widen generation. `budget_shortage` is `slots` for slot-cap or final-limit omissions; otherwise `work` when the
  history window fills or a subject-containing commit exceeds the cochange path cap; otherwise `none`. `work` reports unexamined scope without
  asserting that additional candidates exist, and history-read errors retain the existing failure behavior. A `--subject` packet at `--limit` 1
  therefore carries the governing row alone and names every other reserved selector in `critical_missing`. `governance` is as TCP-V0-009 defines it.
  `NO_CANDIDATES` MUST NOT be read as absence of evidence or as an answerability claim (invariant 2).
- `TCP-V0-012`: (A) Before TCP-V0-008..011 leaves experimental these tests MUST exist and pass, over fixtures with these constraints: (a) a task whose
  body **and path** vocabulary are disjoint from the instruction file still reserves it; (b) an equal-byte lexical control over a fixture retaining a
  title-overlap decoy spec that MUST NOT be admitted as `spec-mentioned` (it may still appear through lexical), the decoy and the substituted id
  sharing no token with any other fixture file, and where, with a limit admitting every generated row, removing the id or path mention loses exactly
  the `spec-mentioned` rows under this projection: compare the remaining result paths and relations after projecting the defining-spec paths out
  of both packets, and assert independently calculated coverage counts, reserved selectors and per-relation `withheld` values for each packet; (c) a `--limit` below a reserved set of at least two selectors names the exact missing selectors in
  `critical_missing`, in TCP-V0-011's fixed order, with `budget_shortage` `slots`; (d) a prose-identifier control loses the three prose `definition`
  rows and gains no new `definition` row, lexical refill of the freed slots being permitted; (e) two runs over identical pinned inputs (tree, history
  window, task, subject, limit) are byte-identical (TCP-V0-007); (f) the subject is absent under both new relations in a fixture where it would itself
  qualify as governing and as a defining spec (TCP-V0-005). This remains an AT-05 slice: whole-task cost and correctness under
  `BRAIN-DOG-012/013/015/016` is a separate held-out obligation no test here discharges.
- `TCP-V0-013`: Documentation-suffix hits (`.md`, `.mdx`, `.rst`, `.txt`) produced by the lexical
  generator carry the `documentation` relation and name that class in their evidence reason. The
  lexical order of TCP-V0-014 is preserved independently inside the code and documentation
  classes. The lexical fill places code rows until the packet holds five code rows (rows the
  earlier slots took count toward those five), then at most two documentation rows, then every
  remaining code row, then the remaining documentation rows. Both classes retain the lexical
  slot's score, confidence, authority, token rules, and final-limit behavior.
- `TCP-V0-014`: (accepted 2026-09-05 by decision 0066; experimental) The lexical slot scores every
  source with BM25 (k1 1.2, b 0.3) over three fields and orders by that score descending, then
  distinct task terms descending, then path: (a) body terms, with the inverse document frequency
  `ln(1 + (N - n + 0.5) / (n + 0.5))` from the term's posting length `n` over `N` indexed sources,
  the term frequency from the counted body postings, and the length normalisation from the
  source's body token count against the mean; (b) path terms at term frequency one without length
  normalisation; (c) whole identifiers the task names (an ASCII word with an inner camel-case
  boundary or an underscore) matched as written against the identifier vocabulary, at term
  frequency one. Task terms are TCP-V0-004's terms plus each such whole identifier lowercased,
  minus a fixed list of English function and retrieval-phrasing words that has no per-repository
  parameter. Document lengths are derived on first use from the counted postings; the snapshot
  format is unchanged. The evidence reason names the distinct-term and occurrence counts, the
  rarest matched term with its inverse document frequency, and the score. Falsifier (a decision
  rule read from the `paired` section of a `tools/retrieval-bench` report over the development,
  cross-fitted folds of `benchmarks/README.md`; a point estimate at or above a baseline with an
  interval spanning zero is unresolved, never a pass): (1) on each Agent Retrieval Bench positive
  subset the paired difference of the `context` arm minus the strongest of `grep`, `grep-ident`,
  and `bm25:ident` in that cell must have a bootstrap 95% interval whose lower bound is above 0
  on at least three subsets and at or above −0.02 on the fourth for recall@10, and at or above
  −0.02 on every subset for recall@5 and MRR@20; (2) no repository with at least five samples in
  a subset may show a mean recall@10 difference below −0.10 against that strongest baseline;
  (3) on `v2_abstention` the arm abstains on at least 0.50 of the 50 natural no-gold samples with
  Wilson lower bound at or above 0.35 while positive hit@20 stays within 0.05 of the accepted
  value; (4) the run is registered (samples, `corvint`, and fold-map digests) before it
  executes, and a rerun after a code change is a new registration; and (5) the blind-v3 query
  cases' file-level critical misses must not exceed the previous order's. Any failed clause rolls
  the order back to distinct terms, occurrences, then path.

- `TCP-V0-015`: (proposed 2026-09-05 by decision 0067; experimental) In the retrieval shape (no
  `--subject`) the packet carries a `test` relation: when it admits a code file, that file's test
  counterpart becomes admissible, and when it admits a test, the source it exercises. The anchors
  are every row the earlier slots admitted plus the lexical hits that would fill the packet to one
  row short of the limit. A counterpart is bound by four deterministic signals derived at query
  time from the existing tables, never from a new snapshot table: (a) the mirrored path/stem
  `pair` reads (`pairRelation`, without the module-directory case); (b) an import edge from the
  test to the source through `impact`'s reverse-import rules over `Imports`; (c) a whole-word
  mention in the test of a name the source declares (at least four bytes, at most fifty
  definers), weighted by the name's rarity over the identifier vocabulary `ln((N + 1) / n)` as
  `reference` weights it, over names with at most five hundred postings; (d) a test name `Test<Name>`
  or `test_<name>` among the test's indexed symbols, or `describe('<name>')` in its text, whose
  camel-split tokens start with a declared name's tokens. Candidates order by the anchor's
  packet position (the packet's own evidence order), then summed signal weight (a high `pair`
  relation, an import edge and a test name weigh one each, a stem match elsewhere in the tree
  one half, and each mention its inverse document frequency), then fired signals, then path;
  at most one `test` row is
  admitted (`test-convention`; high with two or more signals, else medium; score 650), placed
  after the `definition` slot and before the lexical fill. The evidence reason is
  `tests <anchor>: <signals>` or `is tested by <anchor>: <signals>` with the fired signals in the
  order (a)-(d), the mention signal naming the rarest name and its inverse document frequency.
  Equal-IDF witness names use lexical order, and mention weights accumulate in sorted
  identifier order. Cold builds must retain the supported import edges the relation consumes,
  including when no subject exists; cached and cold packet bytes must agree (decision 0079).
  For a test anchor, signal (b) corroborates only its ten best source candidates; for a source
  anchor, test import edges also generate candidates. Signal (d)'s `describe` form is read
  only for the ten best candidates. TCP-V0-011's `withheld` counts the relation's candidates; with a subject the relation
  is `not-applicable`. The `definition` slot (TCP-V0-010) additionally refuses a backticked
  identifier that is on TCP-V0-014's English stop list, which gains `all`. Falsifier: on the
  Agent Retrieval Bench the `context` arm's code2test hit@k and recall@20 must rise over the
  TCP-V0-014 tree and edit2ripple recall@20 must stay at or above the grep arm's; either
  failure removes the slot.
- `TCP-V0-017`: (proposed 2026-09-05, not accepted; experimental) `context` offers three read-only
  structural lookups over the index the packet reads, `corvint [--root PATH] context defs
  IDENTIFIER [--limit N]`, `context refs IDENTIFIER [--limit N]`, and `context grep TERM...
  [--limit N]`, so an agent iterates against the snapshot instead of a text search. Each answers
  from the tree's snapshot when `corvint index` wrote one, else from one index build, writes
  nothing on any path, and carries the packet's identity and evidence shape: `revision`,
  `"mutates": false`, one evidence row per result naming path, line, blob hash, reason,
  confidence, and authority, and a `coverage` count of candidates against the limit. (a) `defs`
  lists the symbols whose name equals IDENTIFIER, then those equal ignoring case, each class
  ordered by fewest definers of that name, then name, path, line; a row carries the symbol kind,
  the declaration line, and the end line when the extractor reports one (0 otherwise, never
  inferred). (b) `refs` lists the files that name IDENTIFIER as a whole word of the identifier
  vocabulary or that import a file defining it under the reverse-import rules of `impact`,
  excluding the defining files, with the whole-word count and first line; files that both name
  and import come first, then name only, then import only, then count descending, then path.
  (c) `grep` tokenises the terms as the body postings were tokenised, scores every source holding
  a token with TCP-V0-014's BM25 over the body and path fields without the whole-identifier field
  or the stop list, orders by score, distinct tokens, then path, and quotes at most five matching
  lines per listed file with their line numbers; a file whose tokens occur only in its path
  quotes no line and claims none (amended 2026-09-12, AGENTS.md invariant 2): its evidence row's
  `line` is 0, never an inferred line 1, and its `reason` ends `; path-only match, no line`;
  substring and regular-expression matching are not offered and belong to the n-gram posting lane. `--limit` defaults to 20 and is bounded as
  TCP-V0-002's. An identifier or term that is empty or longer than 256 bytes is refused with
  `unsupported-context-lookup-identifier` and exit status 2. Excluded paths never appear: the
  modes read only `Symbols`, the term table, the import graph, and `Sources`, none of which
  carries an excluded path. Falsifier: on a snapshot hit over this repository each mode completes
  under 50 ms wall at n >= 10, output is byte-identical across runs, and the default
  `context --task` output is byte-identical to the previous build. Measured 2026-09-05
  (`docs/plans/context-lookup-verbs-2026-09-05.md`): the lookup work is 0.06-5.4 ms but the gob
  snapshot decode is 67-75 ms, so the wall-time falsifier fails until a sectioned snapshot
  (`IDX-SNAP-V0-014`) lands; the clause cannot be accepted before that.
  rarest matched term with its inverse document frequency, and the score. Falsifier: on the
  Agent Retrieval Bench positive subsets the `context` arm's recall@20 must be at or above the
  grep arm's on every subset it answers, and the blind-v3 query cases' file-level critical misses
  must not exceed the previous order's; either regression rolls the order back to distinct terms,
  occurrences, then path.
- `TCP-V0-016`: (accepted 2026-09-12 by decision 0068's amendment, amended by decision 0146;
  experimental) The packet carries an
  answerability verdict under `coverage.answerability` computed after the slots. (a) The task's
  specific terms are its names: each backticked token (split on `.`, three bytes or more) and
  each identifier whose form with outer underscores trimmed has an inner camel-case boundary or
  an underscore, taken from the task with JSON escape sequences and JSON object keys treated as
  whitespace; a name is specific when fewer sources carry it than the cut, one sixteenth of the
  indexed sources and never below two. A source carries a name when it writes it as an
  identifier, carries it lowercased as one body term, or has it in its path. Plain words are not
  terms of the conjunction. (b) A name is known when at least one indexed source carries it or
  at least one tracked path contains it (a source the index does not read, such as `.java`,
  still answers by path); the
  conjunction is over the known names, a source's support is the count of them it carries, and
  the required support is the smaller of the known count and two (every known name for two or
  fewer, two for three or more); both are reported. (c) The slot rows are withheld in exactly one case: the task
  writes two or more specific names, none is known (zero support anywhere), and no row was
  admitted by one of the seven rescue relations `mentioned`, `pair`, `definition`,
  `reverse-import`, `reference`, `cochange`, `sibling`: such a row is evidence of a locus that a
  vocabulary absence cannot overrule (invariant 2), and the verdict is then `relations-answer`
  with nothing withheld. The lexical fill, `documentation`, the reservations, a `test` row
  (TCP-V0-015, which can derive from lexical anchors alone) and any unlisted kind rescue nothing.
  A second case, a refuted name beside three or more known names no source carries two of
  together, was measured on 2026-09-05 and withdrawn: it abstained on four `v2_trace2code`
  positives whose joining source (`tests/test_options.py` in pallets/click) is over the index's
  source size bound and so has no postings, an index gap rather than an absence. Reserved rows (TCP-V0-008,
  TCP-V0-009) stay; `state` follows TCP-V0-006 over the rows that remain, so a withheld packet
  with no reserved row is `NO_CANDIDATES` and one with a governing row is `READY`; the admitted
  rows stay counted under `coverage.candidates` and `omitted_results`. (d) `coverage.answerability`
  carries `verdict` (`supported`, `not-withheld`, `no-specific-terms`, `relations-answer`, or
  `unsupported-conjunction`). `supported` means at least one indexed source reaches `required`
  support and carries `nearest_claims`: at most the first three best-supporting sources ordered by
  path, each with `path`, `blob_hash`, and the names it `supports` and `lacks`. `not-withheld`
  means the withholding rule did not fire but no indexed source reaches `required` support, as for
  one unknown name or scattered known names that no source supports together; it carries
  `nearest_claims: []` and makes no support claim. The other evidence is
  `specific_terms` (each `term` with its indexed `sources` count and
  its `tracked_paths` count), `known`, `required`, `support`,
  `supporting_sources`, a `reason` sentence naming every name no source carries, and on
  `unsupported-conjunction` `nearest_claims`: the first three withheld slot rows in packet
  order, each with `path`, `blob_hash`, the names it `supports` (none, by construction) and the
  names it `lacks`; `blob_hash` is empty for a path the index did not read, and that entry gains
  the same `evidence_gap` field TCP-V0-003 defines for a result row (AGENTS.md invariant 2). A
  withheld packet is an answerability claim about the index only through this member; the
  `state` alone still is not one (TCP-V0-011). No number here is calibrated on a benchmark: the
  cut is a fixed fraction and the pair is the smallest conjunction. Falsifier: on the Agent
  Retrieval Bench `v2_abstention` release the `context` arm's `selective_success` on the
  `natural_no_gold` stratum must exceed the arm's value without this verdict (0.0), while on
  `v2_trace2code` and `v2_code2test` its `hit@k` and `recall@20` fall by no more than 0.01 and
  its `abstained` rate on positives stays below 0.05; a miss on any of these withdraws the
  withholding (the member may stay as disclosure). Status (decision 0068 amendment, 2026-09-12):
  the whole-subset rule passed on development data only (natural 4/50, counterfactual 10/32, one
  trace2code positive withheld); a post-run subgroup HOLD on that loss is uncleared, so the
  verdict is not advertised as delivered abstention until a non-development qualification under
  decision 0070 clears it. Decision 0078's 0/50 was measured on a tree that counted `test` rows as
  a rescue, the defect the seven-kind list in (c) excludes.
- `TCP-V0-018`: (retired 2026-09-05 by decision 0078) The experimental recipe R and every
  extension are removed. `CORVINT_CONTEXT_RECIPE`, including former values `r`, `r+anchor`,
  `r+doctail`, and their combinations, MUST NOT change the default context packet. The existing
  default golden remains the byte-identity oracle. This stable ID is retained for the retirement
  record and MUST NOT be reused for a successor. Historical positive development falsifiers
  passed for `r+doctail`; the registered wave-1 no-gold comparison tied at zero success. Retirement
  is administrative after failed promotion under decision 0070, not a new numerical falsification.
  Evidence and blind-v4 `NOT_RUN` reasons are preserved in
  `docs/plans/nextgen-r3-code-first-order-2026-09-05.md`. Reintroduction requires a new accepted
  experiment contract.


- `TCP-V0-019`: (proposed 2026-09-05, not accepted; experimental) With
  `CORVINT_CONTEXT_TERMS=ident`, the lexical fields select identifiers from the task's string
  values: valid JSON is decoded recursively, with object keys omitted and string values
  traversed in sorted-key order; malformed JSON is plain text. Selected names are ASCII
  identifiers of at least four characters with an inner underscore or a lowercase-to-uppercase
  boundary, plus stems of code filenames with suffixes `go`, `py`, `rs`, `ts`, `tsx`, `js`,
  `java`, `kt`, `rb`, `cs`, `cpp`, `c`, `h`. Lowercased exact names multiply body/path BM25
  contributions by 3, their camel/snake-split terms by 1; duplicate terms keep their maximum
  multiplier. Case-sensitive whole identifiers multiply the existing whole-word contribution
  by 3, so an exact spelling is stronger evidence than its split parts or wrong-case form.
  With no selected identifier, use TCP-V0-014's ordinary terms over the decoded values.
  No field name, repository label, task-type label, or gold path is special-cased; identifier-shaped
  scalar label values remain possible artifacts, disclosed in the plan. Structural slots,
  reservations, explanation shape, snapshot and query/eval behavior are unchanged. Unset or
  unknown values preserve the existing packet bytes. First-target falsifier: trace2code
  recall@5 >= 0.502 and MRR >= 0.372, with code2test recall@5 >= 0.225; failure on any endpoint
  falsifies this candidate. The plan `docs/plans/context-identifier-terms-2026-09-05.md` freezes
  constants before fold A and B, requires preregistered sample/binary/source/patch/fold digests,
  and labels all v2 results development. Promotion additionally requires decision 0070's full
  paired ladder and clean-partition rule; passing these point targets alone cannot promote.
  Measured 2026-09-05: the pooled first-target endpoints pass, but trace2code fold B loses
  to `bm25:ident` with recall@5 difference -0.094595 (95% interval [-0.189189, -0.013514])
  and MRR difference -0.095146 ([-0.174558, -0.018054]); both signs reverse fold A.
  The frozen candidate is therefore falsified for promotion under decision 0070.
  Rollback removes the flag and selector; the default order already stays unchanged.


### Accepted bounded measurement amendment (decision 0075, 2026-09-05)

- `TCP-V0-021`: `tools/retrieval-bench --snapshot-latency` MUST materialize a private copy once
  per repository and base commit, explicitly index that copy once, and time matched `context`
  calls with the snapshot absent and present for every selected sample. The cold call temporarily
  moves an existing scratch snapshot outside the copy and restores it even on cancellation.
  The bench MUST refuse dirty copies before or after a sample, differing cold/hit rankings,
  failed commands, and unknown cache observations. Under `CORVINT_BENCH_SNAPSHOT_TRACE=1` only,
  successful Go `context` calls disclose the actual loader hit boolean on stderr as
  `corvint-bench-snapshot: hit=true` or `corvint-bench-snapshot: hit=false`; packet bytes and every
  frozen wire remain unchanged. No diagnostic or malformed diagnostics mean `UNKNOWN`, never
  an inferred hit. The bench records the hit `wall_ms`, matched `cold_wall_ms`, and separate
  `OBSERVED_HIT` / `OBSERVED_MISS` distributions; indexing and copying are outside both spans.
  Before the first retriever, it MUST exclusively create and sync a registration file containing
  samples, target binary, bench binary, and frozen fold-map digests, selected sample IDs, arm
  selection, limit, mode and relevant ranking/storage environment. Reusing a registration path
  refuses the run. Registration and report paths MUST name distinct files; normalized paths,
  symbolic-link aliases (including parent directories and dangling final links), and existing
  same-file aliases MUST be refused before retrieval or writes and checked again before writing
  the report. Changing either binary or the recorded environment during the run refuses the
  final report. Input corpus/cache paths remain read-only; only scratch copies and explicit
  registration/report paths are written. Child cancellation MUST reap the process group before
  scratch cleanup. Falsifier: on an idle host (load average below 20), the registered full
  `v2_code2test` run's observed-hit `context` wall p50 MUST be below 100 ms, with every matched
  ranking unchanged. A missing observation, dirty copy, mismatched pair, or unmet latency target
  prevents promotion; v2 ranking figures are development evidence only. Plan and remaining
  evidence: `docs/plans/retrieval-bench-snapshot-hits-2026-09-05.md`.

- `TCP-V0-020`: (proposed 2026-09-05, not accepted; experimental) Under
  `CORVINT_CONTEXT_FRAME_RELATION=1`, in retrieval shape only, the first three distinct
  sorted tracked test paths named by the task's existing mentioned-path rule anchor
  an implementation-directed `test` pass. Indexed non-test, non-documentation sources
  qualify through the existing mirrored-stem relation, the existing import resolution
  rules restricted to the anchor's import map, or a whole identifier the test names
  that the source declares (at least four bytes, at most fifty definers, at most five
  hundred identifier postings). The existing weights apply: high stem one, distant
  stem one half, import one, each identifier its `ln((N+1)/postings)` rarity.
  Identifier contributions accumulate in sorted order. Candidates order by sorted
  anchor position, summed signal weight, signal count, then path, with one candidate
  per path. At most three rows enter before the mentioned and definition slots, with
  the existing `test` row fields and evidence naming the anchor and actual signals.
  The later one-row test pass is skipped on activated tasks; the single candidate set
  feeds existing withheld accounting. More than three named tests marks the pass
  `capped` and reports slot shortage. Final corroboration and governing/spec reservations
  retain precedence; early admission is not an unconditional final-rank reservation.
  Unset or unknown flags, explicit subjects, and tasks with no resolved named test
  retain the previous behavior. There are no new persistent tables or read-side writes.
  Falsifier: registered A-then-B development runs with frozen mechanism must reach
  trace2code recall@5 at least 0.55 and code2test recall@5 at least 0.215 (baseline
  0.225, loss at most 0.01); report recall@5/10/20 and MRR on both folds of all four
  positive subsets against the full ladder. Promotion additionally requires decision
  0070; v2_abstention is development because its earlier results changed withholding.
  Blind-v4's separate decision-0037 run conditions cannot be bypassed. Plan and
  preregistration: `docs/plans/context-frame-relation-2026-09-05.md`. Rollback: unset
  the flag or remove the frame pass; default and frozen wires remain unchanged.

- `TCP-V0-022`: (proposed 2026-09-22, not accepted; experimental; decision 0333) With
  `CORVINT_CONTEXT_ANCHORS=on`, the task's repository anchors are a fourth lexical field matched
  verbatim. An anchor is a literal of 4 to 256 bytes carrying at least one ASCII word run of
  three bytes, taken from the task text (TCP-V0-019's decoded string values when the task is
  valid JSON) by five classes tried in order, each consuming its spans before the next: `error`
  (a double-quoted string, quotes stripped), `url` (`scheme://` up to whitespace or a closing
  delimiter), `frame` (`path.ext:line`), `enum` (`Scope::Value`, one or more `::`), `config`
  (a lowercase dotted key that is not a bare file name with a code, Markdown or data suffix).
  Anchors keep text order, drop duplicate literals and stop at sixteen. A source carries an
  anchor when the bytes occur in its bounded body (TCP-V0-014's size bound) with neither
  word-shaped edge extended by a word byte; the candidates are the intersection of the `Words`
  postings of the anchor's word runs, and an anchor with more than 512 candidates contributes
  nothing. A carrying source is credited like a body term: idf from the number of carrying
  sources, tf the occurrence count, the same k1 and b, tripled under TCP-V0-019's exact weight.
  The credit lives inside the lexical slot: the row keeps kind `lexical` (or `documentation`),
  score 300 and authority `vocabulary`, its reason is prefixed `anchor: ` + "`literal` xN" +
  ` verbatim; `, and it can never precede a reserved TCP-V0-008/TCP-V0-009 row. No index,
  snapshot or pack change; an unset or other value preserves the existing packet bytes.
  Falsifier (not yet run: the frozen `agent_retrieval_bench` corpus is not on the build host):
  a registered `tools/retrieval-bench` run on `v2_code2test` and `v2_trace2code` with the flag
  unset and `on` must lose at most 0.01 recall@5 on every fold and must report the
  anchor-bearing samples as their own subset; promotion additionally requires decision 0070's
  paired ladder. Rollback: unset the flag or remove the anchor field; the default wire never
  changed.

## Non-goals and authority

Forward imports of the subject, cross-directory definition-to-reference edges, and re-export
following are measured gains (decision 0024) that this slice does not ship; multi-file co-change
queries, confidence-pruned re-ranking of the syntax slots by history, and a commit-size cap tuned
on mature repositories are the co-change follow-ups decision 0025 leaves open; they are the next
slots, not implied ones. No falsifier or verdict is attached here: that is `prove`'s job, and
embedding this packet in `prove` is a separate slice. This `context` profile does not govern or
alter the `query` and `impact` wires; their independent `GPK-V0-055`/`GPK-V0-056` ranking
corrections and registered Python divergences do not change these slots. The slot caps and weights
are the measured values on one 50-task corpus; retuning them is a decision, not a drift. (A) TCP-V0-008..012 add no runtime model, no learned ranker, and no
cross-request possession or memory of what a caller already holds: deduplication is of paths within one packet at one revision. They add no
option to TCP-V0-002 and no byte budget; `budget_shortage` reports slot and work limits only.
Markdown stays the only documentation kind `documentKind` classifies as `instructions`, `decision`,
or `spec`, so only a Markdown file can own a `TCP-V0-008` governing row or a `TCP-V0-009`
definition-owner row; widening to another documentation suffix is decision
`0087-non-markdown-authority-2026-09-11.md`'s to make, not this packet's. TCP-V0-022 adds no
anchor table to the index, snapshot or pack and no packet member; its five anchor shapes are fixed,
not a grammar per language, and the bench's separate anchor-bearing subset is
`tools/retrieval-bench`'s to add, not this packet's.

Operational note (V1-0051): `corvint context` and the generic harness-event dispatch it shares
(`cmd/corvint/taskcontext.go`, `cmd/corvint/main.go`) write a local pprof CPU profile when the
operator sets `CPUPROFILE=PATH` in the process environment. It is off by default, diagnostic only,
and does not widen what either read-only path reads, returns, or mutates (AGENTS.md invariant 4).

## Failure modes

- An untracked or absent subject: refused with an error naming the revision (TCP-V0-002). An
  excluded or unread-kind subject is admitted without a blob hash and contributes no identifier
  evidence; its pairing and sibling slots still work from the path alone.
- A task naming an ambiguous basename: no `mentioned` row for it; the lexical fill may still
  list the files.
- A repository whose grammar the index does not parse: `definition` and `reverse-import` are
  empty for it (Swift excepted: its symbols are extracted and its module edge is the rule above,
  so a Swift subject outside the SwiftPM layout has no `reverse-import` row) and the packet says
  nothing about why; the consumer must not read absence as
  evidence of absence (AGENTS.md invariant 2).
- A subject with no commit among the last 200 (a new file, or a shallow clone, whose boundary
  commit lists every path it holds and is not counted), or whose every
  commit is a bulk sweep over the cap: the `cochange` slot is empty and the packet does
  not say why; absence is not evidence that nothing co-changes (invariant 2).
- A subject in a flat root directory: the `sibling` slot admits its root siblings; the parent
  subtree rule does not apply.
- A subject with no extracted symbol of four bytes or more (a grammar the index does not parse, a
  data file): the `reference` slot is empty and the packet does not say why (invariant 2). A
  subject a grammar refused or whose symbol walk stopped short is the exception: the index records
  it, and TCP-V0-011 reports `reference` as `subject-symbols-incomplete`.
- A task that pastes a trace naming symbols the tree does not yet define (an undefined-symbol
  build failure, a test the fix adds): TCP-V0-016 counts them as refuted names; the packet still
  answers while one known name remains, and is withheld when the trace names nothing the tree
  carries and no rescue relation is admitted (one `v2_trace2code` positive, gin-gonic/gin, whose
  trace names only the fix's new symbols). A task that is a
  JSON document with label values (`source_type`) shaped like identifiers gains refuted names
  the rule cannot tell from the task's own; only keys and escapes are stripped.
- (A) An instruction file over the index's source size bound (`maxSourceBytes`), or listed in `Index.Exclusions`, is not read, so TCP-V0-008 reserves
  nothing at all: the packet reports the `governing` relation's `unexamined` state as `capped` rather than omitting it silently. A tree with no
  tracked instruction file and no defining spec for any id the task names reserves nothing and reports `governance` `unresolved`, which is not
  evidence that it has none.
- (TCP-V0-017) An identifier no grammar extracted (a language without a symbol walker, a
  definition in an excluded or unread path): `defs` reports `NO_CANDIDATES`, `refs` lists no
  importers, and neither says why; absence is not evidence of absence (invariant 2). A Go
  reference inside the definer's own package has no import edge and ranks by whole-word count
  only.
- (TCP-V0-022) An anchor whose word runs are all under three bytes, or whose candidate
  intersection exceeds 512 sources, is not verified and contributes nothing, and the packet does
  not say so: the anchor field is a credit inside the lexical slot, not a relation, so a missing
  anchor row is not evidence that the literal is absent (invariant 2). A quoted prose fragment is
  an `error` anchor by shape; it rarely occurs verbatim and then only adds lexical credit.

## Acceptance evidence

`internal/contextindex/taskcontext_test.go` (subject kept out, pair first, reverse importer and
mentioned path admitted, retrieval shape, `NO_CANDIDATES`, untracked subject refused; TCP-V0-016's
fixtures: names no source carries withhold the rows and name the nearest claims, a lexically
derived `test` row does not rescue them, only the seven named relations count, a mentioned path
overrules the absence, one source carrying the conjunction answers, all-but-one of three known
names answers); `internal/contextindex/taskcontext_support_test.go`
(`TestTaskContextReportsDeclaredRequiredSupport`: `required` is `min(known, 2)` from zero to four
known names, and one unknown name answers `supported` with zero support and the no-conjunction
reason);
`TestTaskContextAdmitsFilesNamingASubjectSymbol` (a symbol reference corroborates an importer and
admits a non-importer; the subject and a short symbol admit nothing);
`internal/contextindex/termtable_test.go` (the tokeniser against the regex definition, whole-token
lexical rows and whole-word identifier evidence, the table's layout and its snapshot encoding);
`cmd/corvint/taskcontext_test.go` (invocation parsing, read-only run, subject absent from
results); the trial's development rerun reading beside the first observation in `docs/BUILD-LOG.md`;
`internal/contextindex/lookup_test.go` and `cmd/corvint/context_lookup_test.go` (TCP-V0-017,
proposed: the order of each mode, excluded paths absent, the typed refusal, a read-only and
byte-identical run).
`internal/contextindex/ranking_regression_test.go` (recipe R, TCP-V0-018: the default path against
its 2a76e40 golden and one ordering fixture per mechanism).
`internal/contextindex/context_anchors_test.go` (TCP-V0-022, proposed: one table row per anchor
class carried verbatim and not by its split tokens, the extraction bounds and whole-anchor rule,
the `anchor:` reason behind the governing row, default bytes unchanged).

## Rollback

Delete the two source files, their tests, the help topic, and the dispatch line in
`cmd/corvint/main.go`; point the trial's `corvint` arm back at `impact`/`query`. No persisted state
depends on this packet. TCP-V0-017's lookups roll back alone: delete
`internal/contextindex/lookup.go`, `cmd/corvint/context_lookup.go`, their tests, the lookup
dispatch block in `cmd/corvint/main.go`, and the help suffix in `cmd/corvint/help.go`.
TCP-V0-022 rolls back alone: delete `internal/contextindex/context_anchors.go` and its test, the
`anchors` fields and the anchor loop in `lexicalHits`, and the `field >= 2` widening in
`queryTermGain`; the default wire never changed.

## Traceability

| Requirement | Implementation | Test |
|---|---|---|
| TCP-V0-001 | `runTaskContext` (the tree's snapshot when `corvint index` wrote one, else one `contextindex.Build`; no writer, index-snapshot-v0) | `TestRunTaskContextIsReadOnlyAndKeepsTheSubjectOut` |
| TCP-V0-002 | `parseTaskContextInvocation`, `TaskContext` (limit, task, subject checks) | `TestParseTaskContextInvocation`, `TestTaskContextRetrievalShapeAndNoCandidates` |
| TCP-V0-003 | `taskContextCompiler.packet`, `rowAction`, `contextRow` | `TestTaskContextKeepsTheSubjectOutOfTheResults`, `TestTaskContextRowsCarryAnAction` |
| TCP-V0-004 | `compile`, `corroborate`, `pairRows`, `pairConfidence`, `mentionRows`, `symbolRows`, `importerRows`, `referenceRows`, `subjectSymbols`, `readCoChangeHistory`, `dropGraftedCommits`, `cochangeCommitCap`, `cochangeRows`, `siblingRows`, `identifierEvidence`, `lexicalRows`, `buildTermTable`, `countTerms`, `scanWords` | `TestTaskContextKeepsTheSubjectOutOfTheResults`, `TestTaskContextAdmitsReverseImportersAndMentionedPaths`, `TestTaskContextAdmitsFilesNamingASubjectSymbol`, `TestTaskContextAdmitsCoChangedPathsAndIgnoresBulkCommits`, `TestTaskContextCochangeSkipsTheShallowBoundaryCommit`, `TestCochangeCommitCapTightensWithRepositoryAge`, `TestTaskContextRanksCorroboratedRowsFirst`, `TestTaskContextRetrievalShapeAndNoCandidates`, `TestCountTermsMatchesTheRegexTokeniser`, `TestLexicalRowsMatchWholeTokensFromTheTable`, `TestIdentifierEvidenceCountsWholeWordsByWeight`, `TestContextEqualIDFTestEvidenceIsStable` |
| TCP-V0-005 | `take` (subject skipped), `packet` (`subject` member), `subjectEvidenceGap` | `TestTaskContextKeepsTheSubjectOutOfTheResults`, `TestRunTaskContextIsReadOnlyAndKeepsTheSubjectOut`, `TestTaskContextRetrievalShapeAndNoCandidates` |
| TCP-V0-006 | `packet` (`state`, `coverage`) | `TestTaskContextRetrievalShapeAndNoCandidates` |
| TCP-V0-007 | `runTaskContext` (`gokernel.CanonicalJSON`) | `TestRunTaskContextIsReadOnlyAndKeepsTheSubjectOut` |
| TCP-V0-008 | `taskContextCompiler.governingRow`, `instructionCandidates`, `instructionRank`, `projectOperationPath`, `readable`, `reserve`, `reservedRows` | `TestTaskContextReservesInstructionsForUnrelatedVocabulary` (falsifier a), `TestTaskContextKeepsTheSubjectOutUnderReservedRelations` (falsifier f), `TestTCPV0008NestedInstructionEligibilityIsShared` |
| TCP-V0-009 | `specMentionedRows`, `specPaths`, `specDefinitions`, `requirementIDs`, `requirementIDTokens`, `idBoundary`, `namedSpecPaths`, `governance` | `TestTaskContextAdmitsSpecsNamedByIdOrPath`, `TestTaskContextEqualByteLexicalControlLosesOnlySpecMentionedRows` (falsifier b) |
| TCP-V0-010 | `definitionEligible` (applied in `symbolRows` only) | `TestTaskContextProseIdentifierControlAdmitsNoDefinitions` (falsifier d), `TestSymbolRowsDefinerCountsExact` |
| TCP-V0-011 | `criticalSelectors`, `unexamined`, `withheld`, `budgetShortage`, `markRan`, `markState`, `markSubjectSymbols`, `subjectSymbolsIncomplete`, `pairRows`, `takeSlot`, `anyEligible`, `packet` (`coverage`) | `TestTaskContextReportsCriticalMissingAndSlotShortage` (falsifier c), `TestTaskContextReportsUnexaminedScopePerRelation`, `TestTaskContextReportsNamedPathPairScope`, `TestTaskContextReportsNamedPathPairSlotOmissions`, `TestTaskContextDisclosesAnUnparsedSubjectsSymbols` |
| TCP-V0-012 | `internal/contextindex/taskcontext_widening_test.go` | the six cases above plus `TestTaskContextAmendedPacketIsByteIdenticalAcrossRuns` (falsifier e) |
| TCP-V0-013 | `lexicalRows`, `isDocumentationSuffix`, `contextRelationOrder` | `TestTaskContextPlacesDocumentationAfterFiveCodeRows` |
| TCP-V0-014 | `lexicalRows`, `taskLexicalTerms`, `TermTable.documentLengths` | `TestLexicalRowsMatchWholeTokensFromTheTable`, `TestLexicalRowsOrderByBM25AndAnswerWholeIdentifiers` |
| TCP-V0-015 | `testRows`, `testAnchors`, `testLinker`, `testCandidate`, `testNameRemainder`, `nameTokens`, `definitionEligible` (stop-list rule) | `TestTaskContextLinksTestsByEachSignal`, `TestTaskContextReservesOneTestSlot`, `TestTaskContextDefinitionSlotSkipsBacktickedProse`, `TestContextColdBuildRetainsTestImportWinner`, `TestNameTokensMatchesRegexOracle`, `TestCreditMirroredMatchesFullScan` |
| TCP-V0-017 | `LookupDefinitions`, `LookupReferences`, `countWholeWord`, `LookupGrep`, `grepScores`, `matchingLines`, `parseContextLookupInvocation`, `runContextLookup` | `TestLookupDefinitionsOrdersExactThenRarestCaseInsensitive`, `TestLookupReferencesExcludesDefinersAndRanksImportersFirst`, `TestLookupGrepRanksByBM25AndQuotesMatchingLines`, `TestLookupGrepCountsATokenInBodyAndPathOnce`, `TestLookupGrepPathOnlyHitClaimsNoLine`, `TestLookupNeverListsExcludedPathsAndRefusesBadIdentifiers`, `TestParseContextLookupInvocation`, `TestRunContextLookupIsReadOnlyAndDeterministic`, `TestRunContextLookupRefusesAnEmptyIdentifier` |
| TCP-V0-016 | `taskNames`, `nameShaped`, `specificTerms`, `pathSources`, `trackedNaming`, `answer`, `relationRows`, `nearestClaims`, `withheldClaims`, `answerability.unsupported`, `answerability.packet`, `answerability.claimPackets`, `reservedOnly` | `TestTaskContextAbstainsOnAnUnsupportedConjunction`, `TestTaskContextReportsDeclaredRequiredSupport`, `TestTaskContextSupportedVerdictNamesItsSupport`, `TestTaskContextNotWithheldVerdictHasNoClaims` |
| TCP-V0-018 | retired recipe; default `compile` and `lexicalRows` only; decision 0078 | `TestContextRecipeDefaultPathIsByteIdentical`, `TestContextRecipeRetiredFlagsAreIgnored` |
| TCP-V0-019 | `configureContextTerms`, `contextQueryValues`, `selectContextTerms`, `queryTermGain`, `lexicalHits` | `TestContextIdentifierTermsValuesAndWeights`, `TestContextIdentifierTermsProseAndMalformedFallback`, `TestContextIdentifierTermsExactOutranksParts`, `TestContextIdentifierTermsDefaultBytes` |
| TCP-V0-021 (accepted measurement) | `registerBeforeRun`, `prepareSnapshotSample`, `validateSnapshotPair`, `runTaskContext`, `ownProcessGroup` | `TestRegistrationOutputAliasesRefuseBeforeRetrievalOrWrites`, `TestRegistrationOutputAllowsDistinctFiles`, `TestSnapshotLatencyReusesCopiesAndRegistersBeforeRetrieval`, `TestSnapshotLatencyRestoresHiddenSnapshotOnCancellation`, `TestSnapshotLatencyRefusesUnknownOrDifferentRanking`, `TestBenchRejectsDirtySampleBeforeNextRetrieval`, `TestSnapshotIndexCancellationLeavesNoDescendant`, `TestBenchSnapshotTraceObservesHitWithoutChangingPacket` |
| TCP-V0-020 | `frameRelationRows`, `namedTestFrames`, `frameCandidates`, `creditFrameIdentifiers` | `TestFrameRelationSignals`, `TestFrameRelationOrderingAndCoverage`, `TestFrameRelationScopeAndDeterminism` |
| TCP-V0-022 | `configureContextAnchors`, `taskAnchors`, `anchorCandidates`, `countAnchor`, `anchorOccurrences`, `anchorReason`, `lexicalHits`, `queryTermGain` | `TestContextAnchorClassesMatchVerbatim`, `TestContextAnchorsExtractionBounds`, `TestContextAnchorsExplainAndNeverOutrankAuthority`, `TestContextAnchorsDefaultBytes` |
