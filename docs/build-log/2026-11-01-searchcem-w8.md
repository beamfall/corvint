# Lane searchcem-w8: answerability, CEM interop envelope, context abstention and ranking, CEM base retention (2026-11-01)

Base `8daec98ce4349c1583186692726d2c3a782ac647`. Tickets in priority order: V1-3708, V1-3756, V1-3709,
V1-3710, V1-3711, V1-3758. Requirement IDs used: `RDS-V0-960..963`, `CEM-V1-961..963`,
`TCP-V0-960..961` (`CEM-V1-960` was already taken by V1-2519, so the CEM prefix starts at 961).

## V1-3708: answerability no longer proposes `unanswerable` for a named symbol

Delivered (`RDS-V0-960..963`, `internal/disagree/disagree.go`).

Decisions:
- Channel S admits **structural references**: a file naming a seed as a whole word where the name
  resolves without prose matching, that is a Go file in a definer's own package or an importer of a
  definer. `contextindex.LookupReferences` was not reused: it caps at 50 rows and orders cross-package
  importers before same-package files, so it starved the same-package tests the lexical channel ranks.
  The scan is bounded to the same-package and importer scope.
- A Go package import resolves **once**: a forward hop links one representative file (`doc.go`, else
  the file named after its directory, else the first non-test file by path), and a reverse hop links
  each importer of a seed's package once (`imports PACKAGE_PATH`). `_test.go` files are never
  resolution targets. The representative is a deterministic choice, not a claim about use.
- **Identifier-shaped seeds only**: backticked, or holding an underscore, or holding both cases. Lower-
  case prose words (`signal`, `where`) and unquoted acronyms never vote, so pure prose reports
  `structural_vote: absent` / `uncertain`.
- **Tiered ranking**: declares, then references, then import hops, then link count, occurrences, path.

Evidence: the audit scratchpad (`audit-search.md`) is not on this host, so the "audit's four tasks"
were reconstructed. Retained runs at the branch head over this repository:

| Task | Before | After |
|---|---|---|
| `where is EvalQuery defined` | unanswerable, Jaccard 0 | answerable, high, 0.4286 |
| `PythonImportCandidates misses a dotted module` | (symbol task) | answerable, high, 0.4286 |
| `retriever disagreement answerability signal` | unanswerable, Jaccard 0 | uncertain, structural vote absent |
| `kubernetes helm stripe webhook` | uncertain, absent | uncertain, absent |

2 answerable / 2 not, and none `unanswerable`. Tests: `TestStructuralChannelSeedsDeclarerAheadOfPackageFanOut`,
`TestStructuralChannelReferencesResolveStructurally`, `TestIdentifierTokensDropProse`,
`TestAnswerabilityNamedSymbolUnderImportFanOutIsNotUnanswerable`. The cmd-level low-agreement fixture
was rebuilt with forward import hops, because the old fixture's partial overlap became a real in-package
reference under RDS-V0-960.

## V1-3756: the portable consumer verifies every committed CEM

Delivered (`CEM-V1-961`, `CEM-V1-962`). Decision: the audit read the legacy `cem01-go verify-stable`
route's 4096-entry refusal (`CEM-V1-352`, already declared) as the consumer refusing every real CEM.
The repository-scale route is `verify-stable-canonical`; the fork taken was to measure it over the whole
corpus and state each route's envelope, not to raise the legacy bound.

Corpus measurement (Linux amd64, `corvint cem verify-stable` vs `cem01-go verify-stable-canonical`, both
built from this branch): all 111 maps give the same outcome, stage, code and `verdictSha256`.

| Checkout | ACCEPT | REJECT | UNSUPPORTED | Agreement |
|---|---|---|---|---|
| linked worktree | 108 | 1 (`8668f77b`, `binding`) | 2 (`repository-object-unavailable`) | 111/111 |
| ordinary `git clone --no-local` (`[remote]`, `[branch]` config) | 104 | 1 | 6 (`repository-object-unavailable`) | 111/111 |

Each canonical run took under a second. The ordinary-config refusal is the legacy route's declared
profile (`CEM-V1-962`); a linked worktree is refused there as `unsupported-object-alternates`, because
the alternates probe fails first under a `.git` file. That mislabel is recorded, not changed: changing a
portable code would move the shared tamper matrix. Test: `TestCEMV1961CanonicalRouteHasNoLegacyTreeOrConfigBound`
(5000 extra entries and an ordinary config: canonical ACCEPT, legacy snapshot `unsupported-resource-limit`,
legacy repository `unsupported-repository-envelope`).

## V1-3709: context reports query's abstention on an out-of-domain task

Delivered (`TCP-V0-960`). Decisions:
- `EvalQuery`'s ranking pass and relevance floor were split out unchanged (`evalRank`,
  `evalRanked.relevanceFloor`). `contextindex.QueryRelevanceAbstention` calls only those, so it reads no
  traces, makes no Git observation and cannot fail on worktree drift the way a full `EvalQuery` would.
- The check runs only when the packet's own verdict is `no-specific-terms`, the signal-less case.
- Only `below-relevance-floor` is adopted. Query's `no-relevant-candidates` means its record, document
  and symbol classes ranked nothing, which is no judgement on context's lexical file rows. Adopting it
  added false abstentions to five documentation and record-data tests.
- Rows and `state` stay as compiled. The packet gains a top-level `abstention` block and an uncertainty
  line, so nothing that keys on `READY` breaks.

Repro at the head: `kubernetes helm stripe webhook` and `quarterly marketing budget spreadsheet` both
give context `abstention.active` `below-relevance-floor`, matching query `OUT_OF_SCOPE`. For the other 5
probes, including `how do I bake sourdough bread`, neither surface abstains. Query itself does not floor
that one, which is outside this ticket. Tests: `TestTaskContextAgreesWithQueryAbstention`,
`TestTaskContextIgnoresQueryEmptyUniverse`. Limit: trace-learned query candidates can lift query's floor,
and this check does not read them.

## V1-3710: an acronym no longer takes the definition slot

Delivered in part, with the rest BLOCKED (`TCP-V0-961`). Criterion 2 is delivered. An unquoted
all-capitals token named by at least `max(sources/16, 2)` sources does not take the definition slot.
On this repository `CEM canonical binding` no longer ranks `internal/revieworder/order.go` (it was #2,
by `type CEM`), and `internal/cem/wire/canonical.go` moves from #6 to #5. The test fails with the rule
disabled.

Criterion 1 is **BLOCKED**: slot bonuses additive on the lexical score, measured on the frozen
retrieval eval. The frozen `tools/retrieval-bench` corpora (`corpus/<release>/*.chunks.jsonl`) are not
on this host, and re-ranking every slot without the TCP-V0-994 no-regression run would ship an
unmeasured ranking change. The measurement needs a host with the corpora.

## V1-3711: NOT DELIVERED

Not delivered. A typo/trigram fallback disclosed as fuzzy, and a config/shell path boost: the remaining
lane budget went to V1-3758 and to the closeout. Its criterion 2 (the three probes in the top 5 on the
frozen eval) is also blocked on the absent corpora.

## V1-3758: committed CEM bases are retained or labelled

Delivered, with two sub-criteria BLOCKED (decision 0588, `CEM-V1-963`). Measured: 17 of 111 bases are
not reachable from `origin/main` and 14 are not ancestors of their target. Decision: sealed maps are not
re-anchored, because rewriting `baseRevision` breaks the target's exact sidecar. Instead each of the 17
is labelled in `.corvint/cem-lineage.tsv`:

- 11 archive-lineage, retained ref `beamfall/corvint-history@1c4d8793`;
- 4 rebased-lane-base;
- 2 base-unavailable.

`make cem-base-retention-check` (a gate step, with fixture test `cem-base-retention-test`) refuses any
other unretained base. BLOCKED:

- Archive reachability was not verified, because `beamfall/corvint-history` is not readable from this
  host (`git ls-remote` needs credentials).
- The 4 rebased-lane bases need a pushed archive ref, and this lane may not push.

## Tests run (branch head)

- `internal/disagree`: whole package, pass.
- `internal/contextindex`: whole package, pass. The analyzer input pin was repinned with
  `script/analyzer-schema.sh bump`; the schema stays `corvint-analyzer/150` because the output digest
  is unchanged.
- `interop/cem01-go`: whole module, pass.
- `cmd/corvint -run 'Context|Answerability|Query|CoreFreeze|SlotWeight|Capabilit'`: pass. The
  package is too large to run whole inside 600 s.
- `script/check-cem-base-retained_test.sh`, `script/runtime-environment_test.sh` and
  `script/release-env-compat_test.sh`: pass.
- `make shell-test-registration-check`: pass.
- `make cem-base-retention-check` over the corpus: pass (111 maps, 17 labelled).
- Not run: `tools/retrieval-bench` (no corpora); `cmd/corvint` whole package.

## Friction

1. `~/lanes/heavy.sh` refuses a whole-package run even for a one-file package. Repro:
   `~/lanes/heavy.sh env GOTOOLCHAIN=local go test ./internal/disagree/` gives `REFUSED
   whole-reason-missing`. `corvint affected` itself selects the package whole.
2. The installed `corvint` is stale. Repro: `corvint affected --base 8daec98c` prints
   `DEGRADED self-binary-stale: built from f1680433b107`, so every probe needed a scratch build.
3. `corvint affected --base 8daec98c` gives `scope: UNKNOWN`. It selects
   `internal/analyzerhtmlcss` (UNBOUNDED_READER) because of the new `.corvint/cem-lineage.tsv`, and
   `internal/dashboard/roadmap` because of `REQUIREMENTS.tsv`, so it cannot narrow.
4. The lane's requirement block was not free per prefix. Repro: `git grep CEM-V1-960 origin/main`
   matches V1-2519 (`docs/specs/cem-stable-v1.md:729`).
5. The audit scratchpads (`audit-search.md`, `cem-audit-format.md`) are not on this host, so the
   "audit's four tasks" in V1-3708 could not be rerun as written.
6. The frozen retrieval-bench corpora are absent. Repro: `find ~ -name '*.chunks.jsonl'` finds
   nothing, which blocks V1-3710 criterion 1 and V1-3711 criterion 2.
7. The `beamfall/corvint-history` archive is unreadable here. Repro:
   `git ls-remote https://github.com/beamfall/corvint-history` asks for a username.
8. The legacy `cem01-go verify-stable` route mislabels a linked worktree. Repro:
   `cem01-go verify-stable --repository <linked worktree> ...` returns `unsupported-object-alternates`
   with no alternates file present.
9. `corvint query 'TEXT'` refuses positional text (`unrecognized arguments`); only `--task` works.
10. `TestAnalyzerSchemaInputs` fails on any contextindex edit, but only a whole-package run shows it.
    Parallel contextindex lanes will conflict on that pin.
11. `TestHostAdapterJavaScriptHarnessInterruption` fails on a host without Node instead of skipping.
    Repro: `go test -run TestHostAdapterJavaScriptHarnessInterruption ./cmd/corvint` with no `node` on
    PATH.
12. The Corvint MCP server failed to connect in this session (`CONNECTION_CLOSED`).
13. Decision number 0588 may collide with another wave-8 lane; the orchestrator renumbers at merge.
