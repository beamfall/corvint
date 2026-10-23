# docs_search file and claim ranking for GitHub #1597

Date: 2026-10-23. Lane: opencode/search-ranking-gh1597 (base dd0d09db5). Ticket: V1-2470.
Requirements `DCP-V1-400..404` in `docs/specs/documentation-corpus-v1.md` are accepted under the
owner's standing auto-accept (2026-10-11). They amend `DCP-V1-331`'s name field and replace its
shared exact bonus.

## Decisions

- Cause of the file-lookup regression: native file subjects already carry their path as their
  name, so their path segments were name terms. What changed in #1580 was that every term of a
  flow-docs paragraph subject's name, which is the paragraph body, also weighed 3. A paragraph
  that repeats the query words therefore outscored the module whose path names one of them.
  `DCP-V1-401` makes a name of more than 12 distinct terms prose, weighed as claim text.
  `DCP-V1-400` puts a `module` or `file` subject's anchor path terms in its name field, which
  covers supplied module subjects whose name is not their path. Path terms are read per query
  from the subjects and never added to the query index, so the index, its bounds and the
  companion encoding are unchanged. Native file subjects skip the extra read because their
  anchor path equals their name.
- Cause of the claim burial: raw scores were compared across families with weight 3 for subject
  names and 1 for claim text. `DCP-V1-402` divides each family's partial score by its largest
  possible partial score for the query. Exact matches add 1, and ties order subjects before
  claims. Without `include` the order is unchanged, because one divisor never reorders one
  family. The `DCP-V1-328` test assertion "a name match outranks a claim-text match" was the
  behaviour the issue reports as wrong. It now asserts that the claim holding every query term
  leads, and that the subject leads when both families match every term.
- Rejected: exposing field weights as `docs_search` arguments (a wider wire change that still
  needs the caller to tune them), and a separate claim list in the receipt (changes the result
  shape). Neither is needed for the two mechanisms.

## Evidence

- `DCP-V1-404` fixture: `internal/mcp/corpusbridge/testdata/search-ranking-gh1597/`, which holds
  six source files, six prose paragraph subjects, six short capability subjects, thirteen supplied
  claims and twelve `limit: 1` cases. Before the change (`search_rank.go` from dd0d09db5), 0 of 12
  cases passed: each expected file ranked 2nd and each expected claim ranked 2nd to 4th. After the
  change, 12 of 12 pass with rank 1, and two runs give equal ranks.
- The regression tests for the issue rows fail on the old ranking and pass after:
  `TestDCPV1400ModulePathSegmentsAreNameTerms` (paragraph ranked first),
  `TestDCPV1401ProseNameThreshold` and `TestDCPV1402ClaimFamilyNormalizedMerge` (six subjects
  ranked above the claim).
- Focused runs pass: the `internal/doccorpus`, `internal/mcp/corpusbridge`,
  `internal/corpusserve` and `internal/corpusindex` packages, plus `cmd/corvint` and
  `cmd/corvint-corpus-mcp` filtered to the search, flag-matrix and eval tests.

- An independent read-only review found no HIGH or MED issues. Two LOW findings were fixed: a
  repeated anchor-path walk, and the per-requirement status that `REQUIREMENTS.tsv` reads. The
  third LOW finding is kept as a limit below.

## Limits

- Each query walks every subject once. For each supplied `module` or `file` subject whose anchor
  path differs from its name, it also tokenizes that subject's ID, name and paths. Native file
  subjects skip the tokenizing. The cost grows with the number of such subjects.

- The reporter's 52-case labelled suite and corpus are not in the repository, so their before and
  after counts are `NOT_OBSERVED`. The fixture shows that the two mechanisms are fixed, not that
  the suite recovers.
- A short subject name that holds more query terms than a file's path still outranks the file.
  The 12-term prose threshold is a stated constant, not a tuned weight.
