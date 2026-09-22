# Authority trigger index V0

Owner: Russell Lewis
Date: 2026-09-07
Requirement prefix: `ATI-V0`
Intent status: accepted (owner instruction 2026-09-07)
Delivery status: implemented
Authoritative inputs: `../../AGENTS.md` invariants 1, 2, 3, 4 and 7,
`documentation-citation-gate-v0.md` DCG-V0-007..009 (the anchor this index reuses),
`go-production-kernel-migration-v0.md` GPK-V0-002 (the parity lock that keeps this off the harness
surface), and `../agent-memory/ideas.md` heading '2026-09-07 omp'.

## Agent digest
- Claim: A changed path names every project-owned authority document that already cites it, with a freshness verdict on each citation.
- Status: accepted (owner instruction 2026-09-07) / implemented
- Exists: `internal/contextindex/authority_trigger.go`, `corvint context authority <path>...`.
- Blocked on: emission through `harness event file-change`, which GPK-V0-002 pins byte-for-byte against the Python oracle (ATI-V0-012).
- Read next: Requirements; Non-goals; Unresolved.

## Human intent and scope

AGENTS.md invariant 3 puts project-owned authority above syntax, history, and learned traces, but
an agent only benefits from that ordering if the authority reaches it while it is editing. The two
available ways to make that happen are both bad: load the whole authority closure into every packet,
which spends budget on rules that do not apply, or rely on the agent to go looking, which is exactly
the failure invariant 3 exists to prevent.

`omp` (`omp.sh`) solves the same problem with Time-Traveling Stream Rules: a project rule stays
dormant until a declared trigger matches, then one rule is injected. Their trigger is authored by
hand in a rule file. Corvint does not need a new authoring surface, because its authority documents
already declare which paths they are about: they cite them. A citation is a statement, pinned to
immutable content, that a named path is the subject of a passage. That is the trigger.

Affected user: an agent or engineer changing a file in a repository whose rules live in prose.
Measurable job: for a set of changed paths, return every project-owned authority citation naming
them, ranked by invariant-3 authority, with a freshness verdict on each — and return nothing when
nothing cites them.

## Verified current state

At `02b654c` the index classifies 170 tracked Markdown files as `instructions`, `decision` or
`spec` — `AGENTS.md`, `CLAUDE.md`, `docs/decisions/*.md`, `docs/specs/*.md` and the few other paths
`documentKind` admits. `docs/agent-memory/*.md` is deliberately not among them: the citation gate
checks those files, but they are working notes, not authority. A scan of the 170 for backticked
full-path citations names 97 distinct repository paths, which is an upper bound because it does not
exclude fenced blocks or citations of untracked files. Against 2,657 tracked files that is at most
3.7%, so the index is silent for at least 96.3% of changes, and a path that does fire draws a small
table measured with the command itself: 7 triggers for
`internal/contextindex/git.go`, 6 for `internal/contextindex/impact.go`, 13 for
`cmd/corvint/main.go`, 4 for `script/check-line-citations.sh` — the last four all `pinned`, because
`documentation-citation-gate-v0.md` is the one document that has adopted content anchors.

One limit is inherited rather than introduced: `harness event` output is pinned byte-for-byte
against the Python oracle, which is why ATI-V0-012 keeps this index off the harness surface.

The ranking this spec requires was inert when the index first shipped, because `documentStatus` read
only a bare column-zero `status:` and Corvint writes the field four other ways, so exactly one document
of 165 was binding and every spec ranked `repository-spec`. Decision 0080 and `FPK-V0-029` repaired
the extraction on 2026-09-07; 113 documents of 165 now bind and the invariant-3 ordering is
exercised on Corvint's own corpus.

## Requirements

- `ATI-V0-001`: The index MUST compile its triggers from backticked full-path citations of the form
  `dir/file.ext:N`, `dir/file.ext:N-M` and `dir/file.ext:N,M`, optionally carrying the DCG-V0-006
  anchor, found in the body of every document the index classifies as `instructions`, `decision` or
  `spec`. It MUST NOT read a bare `:N` continuation or a bare basename, because both resolve through
  the surrounding prose and a trigger asserted from a reading of prose is the invented certainty
  AGENTS.md invariant 2 forbids.
- `ATI-V0-002`: A trigger MUST fire only for a path the caller named. When no citation names any
  requested path the result MUST be an explicit empty table in state `NO_CANDIDATES`; dormancy is
  the expected case and MUST NOT be reported as an error.
- `ATI-V0-003`: Every row MUST report its relation as `cites` and MUST NOT assert that the cited
  authority governs, applies to, or is violated by the change. Whether a citing passage constrains
  the path or merely describes it is a reading this index does not make.
- `ATI-V0-004`: Every row MUST carry an evidence handle naming the citing document's path, the line
  the citation sits on, that document's blob hash, and a reason that states the citation verbatim,
  so the row explains its own inclusion (invariant 1).
- `ATI-V0-005`: Every row MUST carry the invariant-3 authority label and confidence for its citing
  document, computed by the single function `impact` uses, so that one document cannot rank one way
  inside a packet and another way inside a trigger.
- `ATI-V0-006`: Every row MUST report one anchor state. `pinned` and `drifted` are reserved for a
  citation carrying a DCG-V0-006 anchor, and the digest MUST be computed exactly as DCG-V0-007,
  DCG-V0-008 and DCG-V0-009 define it, because the gate's `--hash` is the only supported way to mint
  a pin. A citation with no anchor MUST be `unpinned`, which is the corpus default and not a defect.
  A citation whose cited bytes cannot be read at the index revision MUST be `unreadable` and MUST
  NOT be reported as `drifted`.
- `ATI-V0-007`: A citation inside a fenced code block MUST NOT produce a trigger.
- `ATI-V0-008`: The table MUST be ordered deterministically by cited path, then authority rank, then
  anchor state (`pinned`, `unpinned`, `drifted`, `unreadable`), then citing document, then citing
  line. Order MUST NOT depend on the order the caller wrote its paths.
- `ATI-V0-009`: The table MUST be bounded by a caller limit of 1 to 50, defaulting to 20, and MUST
  report candidates, included and omitted counts so a truncated table is visible as truncated.
- `ATI-V0-010`: The result MUST separately name its two blind spots: every requested path the index
  does not track, and every authority document whose bytes did not load. Both would otherwise be
  indistinguishable from an absence of authority, which is the failure invariant 2 names.
- `ATI-V0-011`: The command MUST be read-only over repository and trace state, MUST refuse a path
  that is empty, absolute, or not repository-relative and normalized, and MUST refuse an invocation
  naming no path at all rather than answering it as a dormant one.
- `ATI-V0-012`: The index MUST NOT be emitted through `harness event`. `GPK-V0-002` pins that
  command's stdout byte-for-byte against the Python oracle and `harness-file-change` is one of the
  pinned cases, so adding a field there is a divergence whether or not the field is empty in the
  parity fixture. Harness emission requires the `corvint-harness-event/1` profile flag day already
  deferred by decision 0007 D8.

## Non-goals and simpler baseline

The simpler baseline is what exists without this: the agent reads the authority closure itself, or
the packet carries it. This index does not replace either; it adds a retrieval path that costs
nothing until a cited path is touched.

It does not decide relevance, applicability, or violation. It does not key triggers on symbols:
authority documents do cite backticked identifiers, but resolving one to a definition is ambiguous
in a way a path is not, and a wrong symbol trigger is worse than no trigger. It does not accept
authored glob rules, which would be a new authoring surface and a second source of truth. It does
not read prose path boundaries such as AGENTS.md's `protocol/**`, because extracting a rule from a
sentence is inference, not evidence. It does not mutate, cache, or persist anything, and it reads no
revision other than the index's own.

## Trust boundary, limits, and failure modes

Document bodies and cited file bodies are untrusted input; the index reads only what the built
context index already pinned, spawns no subprocess, and writes nothing. A citation is evidence that
a document mentions a path at a revision, never evidence that the document is correct.

| Failure | Behavior |
|---|---|
| No citation names the requested path | empty table, `NO_CANDIDATES` (ATI-V0-002) |
| Requested path is untracked | empty or partial table plus the path in `untracked_paths` (ATI-V0-010) |
| Authority document body did not load | document in `unread_documents`, its citations absent (ATI-V0-010) |
| Cited path unreadable at the index revision | row present, anchor `unreadable` (ATI-V0-006) |
| Pinned citation whose content changed | row present, anchor `drifted`, ranked last (ATI-V0-006, ATI-V0-008) |
| More triggers than the limit | table truncated, `omitted_results` non-zero (ATI-V0-009) |
| Path empty, absolute, or unnormalized | typed refusal, no partial answer (ATI-V0-011) |

A trigger fires on the citing document's own bytes at the index revision, so a citation added in an
uncommitted edit does not fire until the index sees it, and an anchor compares against the indexed
revision rather than the worktree. That is deliberate: an anchor states what the author read at a
committed revision, and comparing it against unsaved bytes would report drift that nobody caused.

## Acceptance criteria and testing matrix

| Requirement | Evidence |
|---|---|
| ATI-V0-001, ATI-V0-003, ATI-V0-004, ATI-V0-007 | `TestAuthorityTriggersFireOnlyForCitedRequestedPaths` |
| ATI-V0-002 | the same test's `pkg/quiet.go` half, whose only citation is fenced |
| ATI-V0-005, ATI-V0-008 | `TestAuthorityTriggersOrderByAuthorityThenAnchor` |
| ATI-V0-006 | `TestAuthorityTriggerAnchorStatesMatchTheCitationGate`, covering a trailing-whitespace line, a written range, drift, and no anchor |
| ATI-V0-009, ATI-V0-010 | `TestAuthorityTriggersBoundResultsAndReportBlindSpots` |
| ATI-V0-011 | `TestAuthorityTriggersRefuseUnusablePaths`, `TestContextAuthorityModeAcceptsSeveralPathsAndStaysReadOnly` |
| ATI-V0-012 | no `harness` source references the index; `conformance/cli-parity-v0/manifest.json` `harness-file-change` stdout digest unchanged |

Measured evidence at `02b654c`: at most 97 cited paths of 2,657 tracked; 7, 6, 13 and 4 triggers for
the four paths named in Verified current state.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| ATI-V0-001, ATI-V0-007 | `documentCitations`, `authorityCitation` | fixture citations, fenced and unfenced |
| ATI-V0-002, ATI-V0-009 | `LookupAuthorityTriggers` through `lookupEnvelope` | bounded-table test |
| ATI-V0-003, ATI-V0-004 | `authorityTriggerRow.wire` | evidence-handle assertions |
| ATI-V0-005 | `documentAuthority` in `internal/contextindex/impact.go`, shared with `documentResult` | authority-order test |
| ATI-V0-006 | `anchorState`, `anchorOf`, `citedNumbers` | anchor-state test |
| ATI-V0-008 | `sortAuthorityTriggers` | authority-order test |
| ATI-V0-010 | `untrackedAuthorityPaths` and the unread-document list | blind-spot test |
| ATI-V0-011 | `authorityTriggerPaths`, `cleanImpactPath`, `validateLookupLimit` | refusal test |
| ATI-V0-012 | absence: no reference from `internal/gokernel` | parity manifest digest |

## Rollout, rollback, and drift

The index is additive and opt-in: nothing calls it unless a caller runs `context authority`, no
existing output changes, and reverting the slice removes one file, one `context` mode, and one
factored helper. `analyzerSchemaID` advanced to `corvint-analyzer/16` because
`TestAnalyzerSchemaInputs` audits every production source in the package; the review found no change
to extracted facts or pack encoding, and the bump is the conservative behavior that audit
deliberately requires.

Drift rule: this index and `script/check-line-citations.sh` must compute the same anchor over the
same span. If either changes, the other changes in the same commit, or `pinned` and `drifted` stop
meaning what DCG-V0-010 says they mean.

## Unresolved

Two things are open. Harness emission is blocked on the profile flag day (ATI-V0-012), so today the
index is a command an agent must choose to run rather than a rule that arrives unbidden — which is
the half of the omp mechanism that actually removes the failure. And adoption of content anchors
remains at seven citations of 438, so `pinned` versus `unpinned` still discriminates little.
