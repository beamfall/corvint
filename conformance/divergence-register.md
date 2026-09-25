# Divergence register

Authority: `docs/specs/go-production-kernel-migration-v0.md` `GPK-V0-033` / `GPK-V0-034`.

Every case where the Go candidate and the Python oracle produce different observable results is
recorded here and adjudicated **against the frozen spec**, never by preferring a runtime. Expected
bytes are authored from spec text; they are never emitted by candidate Go code.

Outcomes are exactly one of:

| Outcome | Meaning | Required action |
|---|---|---|
| `python-defect` | the oracle contradicts the spec, candidate matches it | author the expectation from spec text; candidate passes unchanged; mark the case **known-divergent**. **Do NOT repair `src/`** — it is scheduled for deletion and is no longer the expectation (`GPK-V0-033`) |
| `go-defect` | the candidate contradicts the spec | repair Go; Python unchanged and MUST keep cross-checking — it is the only independent signal against a candidate defect |
| `spec-gap` | the spec does not determine the observable | park the case; amend the owning spec first; **neither runtime wins by default** |

An **open** entry blocks `PASS` for its command and blocks that command's contribution to the
`GPK-V0-025` retirement window. A `python-defect` marked **known-divergent** does not block: the
expectation comes from the spec, the candidate satisfies it, and the oracle's disagreement is
recorded rather than repaired.

Under `GPK-V0-036`, a command's Python source may be deleted once its rows are `PASS` on
spec-authored expectations, every register entry for it is adjudicated with no `go-defect` or
`spec-gap` outstanding, and its clause-coverage table has no `UNVERIFIED` clause — regardless of the
global retirement window.

Two failure modes this register exists to prevent:

1. **Silent divergence behind a green suite.** A corpus that cannot discriminate a known divergence
   is not evidence. The discriminating case must exist before the command is promoted.
2. **Adjudication drift toward the candidate.** Reading Go's behavior first and calling it correct
   is how "the spec is the authority" degrades into "Go is the authority." Every entry cites spec
   lines, not runtime output.

## Known limitation: agreed-wrong cases

This register catches **disagreements**. It does not catch the more dangerous class: where Go was
written by reading the Python implementation, faithfully reproduced a defect, and both runtimes now
agree against the spec. Nothing in `GPK-V0-033` surfaces that — the case passes, and the spec is
never consulted.

DR-0001 was found only because Go independently got it right. That was luck, not process. DR-0007,
DR-0008 and DR-0009 are all of this class and were all found by reading, not by running the corpus:
DR-0007 by red-team review, DR-0008 by dogfood, DR-0009 by a code audit of the receipt shaper. Three
of ten entries reached the register through a path the register itself does not provide.

Closing this needs a **spec-coverage audit**, not more parity cases: for each normative clause in an
owning spec, does a corpus case exist that would FAIL if that clause were violated? Clauses with no
such case are unverified regardless of how green the suite is. Until that audit exists, a `PASS` row
means "the two runtimes agree and no divergence is known," which is weaker than "the spec is
satisfied." Reports MUST NOT state the stronger claim.

Tracked in `docs/agent-memory/ideas.md`.

---

## Open

### DR-0001 — `lrf`: deletion old-path basename stem charged against the lexical byte bound

- **Status:** LANDED — adjudicated `python-defect` / known-divergent. Discriminating case
  `failure.bound / lexical-bytes.deletion-stem` is in the corpus (commit `5d61722`); Go passes the
  spec-authored expectation; `src/` deliberately unrepaired.
- **Command:** `lrf`. **Discovered:** 2026-08-28.

**Divergence.** For a hunk with `new_path == null` (a pure deletion), the two runtimes disagree on
whether the *old* path's basename stem counts toward the aggregate `lexical_bytes` bound.

- Python charges it. `LRFHunk.path` (`src/context_corvint_lrf.py:130`) falls back to `old_path` when
  `new_path is None`, and `_account_request` (`src/context_corvint_lrf.py:551-563`) calls
  `account_basename(hunk.path)` for every hunk unconditionally.
- Go does not. `Hunk.lexicalPath()` (`internal/lrf/types.go:124`) returns `""` when `NewPath == nil`,
  and `exceedsLexicalBytes` (`internal/lrf/validate.go:213`) accounts via `lexicalPath()` while
  relation checks continue to use `path()`.

**Adjudication: `python-defect`.** The frozen spec excludes old paths twice, and neither line admits
an exception:

- `docs/specs/lexical-relevance-floor-v0.md:102` — "Hunk terms come only from added payload bytes and
  the final basename stem of the post-image path. Removed bytes, **old paths**, directory components,
  context lines, hunk headers, and CEM/OCM metadata do not contribute."
- `docs/specs/lexical-relevance-floor-v0.md:315` — "A deletion emits `abstained` plus only
  `deletion-relation-required`; its span, path, and terms are **not evaluated**."

A deletion has no post-image path, so it contributes no stem. Go is correct, so the case expectation
is authored from the spec and Go passes it unchanged. The case is marked **known-divergent**:
Python's disagreement here is expected and is not a failure. `src/context_corvint_lrf.py` is NOT
repaired — it is scheduled for deletion and no longer supplies the expectation. Go MUST NOT be
regressed to reproduce the oracle's behavior.

**Why the suite is green anyway.** Corrected 2026-08-28 — an earlier draft of this entry claimed no
case comes near the bound. That was wrong. `failure.bound` is a matrix case whose sub-cases
`lexical-bytes.at-limit` and `lexical-bytes.plus-one` use a **lowered** `limits.lexical_bytes` of
`14` and probe the boundary exactly. The accounting is verifiable by hand: added `widget` (6) +
evidence span `widget` (6) + hunk basename stem `a` (1) + evidence basename stem `b` (1) = 14.

The real gap is narrower and more specific: every hunk in that matrix is a **modification**
(`old_path: "a", new_path: "a"`). The corpus never combines a lowered bound with a deletion, so the
one behavior the two runtimes disagree about is never reached. The corpus is well built; it simply
lacks one cell.

**Discriminating case required before `lrf` may be promoted** (`GPK-V0-034`). Add a sub-case
`lexical-bytes.deletion-stem` to the existing `failure.bound` matrix:

- one hunk with `old_path: "a"`, `new_path: null`, `added: ""` (a deletion contributes no added bytes)
- one evidence entry with `path: "b"`, `span: "widget"`
- `limits.lexical_bytes: "7"`

Expected bytes are authored from the spec, not from either runtime. Under
`lexical-relevance-floor-v0.md:102` the deleted old path contributes nothing, so the total is
span 6 + evidence stem `b` 1 = **7**, which is at the limit and MUST evaluate, with the deletion
abstaining under `deletion-relation-required` (`:315`). Python currently computes 8 by adding the old
path's stem `a` and raises bound-exceeded; Go computes 7 and proceeds. `7` is the unique limit value
that separates them — at `8` both pass, at `6` both fail.

Go passes that sub-case against the spec-authored expectation today. Python does not, and is left as
is; the runner records the disagreement as known-divergent rather than red.

**Confirmed by execution (2026-08-28).** Both accounting functions were driven directly —
Python `_account_request` via `src/`, Go `exceedsLexicalBytes` via a temporary in-package probe
(removed after the run). The runtimes agree on every cell except the single predicted one:

| hunk | `lexical_bytes` | Python | Go |
|---|---|---|---|
| modification (`a` -> `a`) | 14 | fits | fits |
| deletion (`a` -> null) | 6 | bound-exceeded | bound-exceeded |
| **deletion (`a` -> null)** | **7** | **bound-exceeded** | **fits** |
| deletion (`a` -> null) | 8 | fits | fits |

Python's deletion total is 8 (span 6 + stem `a` 1 + stem `b` 1); Go's is 7 (span 6 + stem `b` 1).
`7` is the unique separating value, as predicted from spec text before either runtime was run. The
modification row fitting at exactly 14 reproduces the corpus's own `lexical-bytes.at-limit`
sub-case, which independently validates the accounting model used here.

**Case is authored, verified, and staged.** The expectation was written from spec text
(`:102` and `:315`), then checked against Go's real output, which matches byte-for-byte:
`issues: [[cem-basis, <hunk>, <evidence>, specification, deletion-relation-required]]` and
`results: [[..., producer-declared, abstained]]`, exit 0. Python rejects the same request with
bound-exceeded — that is the recorded divergence, left unrepaired per `GPK-V0-033`.

**Blocked on:** adding the case moves the `cases.json` oracle digest, which the in-flight `ocm` lane
is explicitly guarded against. Land after that lane clears. The applier is staged in the session
scratchpad as `apply-dr0001-case.py`; it was dry-run against a copy and produces 93 added lines with
zero removals (it preserves the file's `indent=2` serialization, which round-trips exactly).

### DR-0002 — `lrf`: deletion old-path terms cross the subject-term bound

- **Status:** LANDED — adjudicated `python-defect` / known-divergent. Discriminating case
  `failure.bound / terms.deletion-old-path` is in the corpus (commit `2f66d22`); Go passes the
  spec-authored expectation; `src/` deliberately unrepaired.
- **Command:** `lrf`. **Raised:** 2026-08-28 while authoring the DR-0001 repair.

**Structural observation.** The same old-path-versus-post-image-path split as DR-0001 appears a
second time, in **term derivation** rather than byte accounting:

- Python `terms_for_hunk` (`src/context_corvint_lrf.py:692-700`) builds hunk terms from `item.path`,
  which falls back to `old_path` for a deletion. It is reached from two call sites, `:766` and
  `:800`.
- Go derives the same terms from `hunk.lexicalPath()` in both `evaluateCEMEdge`
  (`internal/lrf/evaluate.go:203`) and `evaluateObligationEdge` (`:223`), so a deletion contributes
  no path terms.

Spec `lexical-relevance-floor-v0.md:102` is the same authority as DR-0001: hunk terms come from added
bytes and "the final basename stem of the **post-image** path" only. On that reading Go is again
correct.

**Confirmed by execution (2026-08-28).** The masks described below were checked and one of them does
not hold. Term derivation itself diverges outright:

| runtime | deletion hunk terms (`old_path=zebra.py`, `new_path=null`, `added=""`) |
|---|---|
| Python | `{zebra}` — from the deleted file's old basename |
| Go | `{}` |

Go produces `{zebra}` if forced through `path()` instead of `lexicalPath()`, so the method split is
the sole cause.

Two masks were then tested:

1. The CEM abstain at `:315` does hold — a deletion abstains before its terms are compared.
2. The subject-term bound does **not** mask it. For a pure deletion Python's count is 1 and the
   minimum legal `limits.terms` is 1 (`_validate_limits` rejects `value < 1`), so 1 never exceeds 1.
   But a deletion carrying non-empty `added` bytes is **accepted by validation** (verified), and that
   reopens the gap:

| `limits.terms` | Python | Go |
|---|---|---|
| 2 | `subject-term-bound-exceeded` — 3 terms (`alpha`, `beta`, `zebra`) | no overflow — 2 terms (`alpha`, `beta`) |
| 3 | no overflow | no overflow |

At `limits.terms = 2` the runtimes return different dispositions and different issue codes:
Python abstains, Go proceeds to the witness check and rejects with `obligation-hunk-mismatch`.

**Adjudication: `python-defect` (known-divergent, no repair),** on the same authority as DR-0001.
`lexical-relevance-floor-v0.md:102` restricts hunk terms to added bytes and the **post-image**
basename stem. A deletion has no post-image path, so `zebra` must not be a term. Go is correct.

**Confirmed end-to-end (2026-08-28).** Both full evaluators were run on the identical request
(Go `EvaluateWithLimits`, Python `_evaluate_lrf_for_tests`). The `ocm-hunk` edge diverges exactly at
the predicted limit:

| `limits.terms` | Python `ocm-hunk` issue | Go `ocm-hunk` issue |
|---|---|---|
| **2** | `subject-term-bound-exceeded` | `obligation-hunk-mismatch` |
| 3 | `obligation-hunk-mismatch` | `obligation-hunk-mismatch` |

The `cem-basis` edge emits `deletion-relation-required` and the `ocm-obligation` edge emits
`no-material-hunk` identically in both runtimes at both limits, so the divergence is isolated to the
one edge and the one bound. The spec-authored expectation is `obligation-hunk-mismatch`, because
`:102` excludes the old path from hunk terms and therefore no overflow occurs — which is Go's
behavior, unchanged.

Full OCM context is required for this case to validate: `ocm_spec`, `ocm_map_sha256`, `intent_path`,
`intent_blob_oid`, `intent_start`, `intent_end`, and `intent_span_sha256` must ALL be present or the
request is rejected with `OCM context identity is invalid` (`internal/lrf/validate.go:164-168`).

**Discriminating case required** (`GPK-V0-034`): one hunk with `old_path: "zebra.py"`,
`new_path: null`, `added: "alpha beta"`, and `limits.terms: 2`. Expected result is authored from the
spec — two terms, no overflow.

**Note.** DR-0001 and DR-0002 are separate Python call sites (`_account_request` and
`terms_for_hunk`) and stay separate register entries with separate discriminating cases, even though
neither is repaired. Two recorded divergences with two cases is the evidence; one merged note is not.

### DR-0003 — `cem prepare`: a rejected invocation leaves its private parent directory behind

- **Status:** LANDED — opened as `spec-gap`, closed by amending the owning spec, then re-adjudicated
  `python-defect` / known-divergent. The case was parked while the gap was open, because the
  register's rule for a spec-gap is "park the case; amend the owning spec first; neither runtime wins
  by default." `GPK-V0-008` now decides the observable, so the case ships (2026-08-28).
- **Command:** `cem prepare`. **Discovered:** 2026-08-28, while authoring the mutating-action corpus.

**Divergence.** `cem prepare --base nope --target <resolvable>` exits 2 in both runtimes with
identical bytes (`git-read-failed`, "Git could not resolve pinned evidence"), but leaves the
repository in different states.

- Python creates the patch cache's private parent `.git/corvint` (mode `0700`) **before** it resolves
  the revisions it was handed, so the directory survives the rejection.
  `_ensure_private_parent` runs at `src/context_corvint_cem_workflow.py:543-546`, ahead of the
  `_resolve_commit` call that fails.
- Go resolves first and creates nothing, leaving the worktree and Git directory byte-identical.

Measured on the `cem-unprepared` fixture: the only difference between the two after-states is one
entry, the directory `.git/corvint`. No file is created by either runtime; no artifact exists on either
side.

**Why `spec-gap` and not `python-defect`.** `GPK-V0-008` is the clause that governs mutator failure,
and it constrains the **artifact**: "Failure or interruption MUST leave either the exact old artifact
or exact new artifact, never a hybrid." Both runtimes satisfy it — the patch cache is absent either
way, so there is no hybrid artifact. An empty parent directory is not an artifact, and no clause in
`go-production-kernel-migration-v0.md` or `cem-external-interop-v0.md` says whether a rejected
mutator may leave one. The spec does not determine the observable, so neither behavior can be called
the defect without first amending the spec.

**How it was closed** (operator decision, 2026-08-28). `GPK-V0-008` was amended to state the missing
invariant: a mutator that exits non-zero without promoting an artifact MUST leave the worktree and
the Git directory byte-identical, including directory entries and their modes, so argument validation
and revision resolution MUST precede the creation of any scaffolding — an owner-private parent
directory or a lock included. The clause decides the observable the artifact rule did not reach.

Go already satisfies it, unchanged. Python does not, so this became `python-defect` / known-divergent
under `GPK-V0-033` and `src/` is deliberately NOT repaired. The discriminating case
`cem-prepare-unresolvable-base` (`cem prepare --base nope --target 3bd8f2c2…` on `cem-unprepared`)
now ships as an operator-accepted oracle-only divergence declaring the exact created path `.git/corvint`
— the same mechanism already carrying the two trace-operation-lock divergences, which are instances
of the identical pattern (the oracle scaffolds before it validates).

**Why the amendment went this way rather than excluding the directory from the snapshot.** Both
readings close the gap. Declaring private parent directories unobservable would have made the case
pass by blinding the mutation snapshot to directory entries — but that snapshot noticing an empty
directory is precisely what made this divergence visible at all, and a crude hash-only probe had
already missed it once during authoring. The corpus keeps the detector and takes the stronger
invariant.

**Retirement-window note.** `GPK-V0-025` forbids the window advancing while any command in the
supported slice holds an open register entry. DR-0003 was the only open entry; with it landed, no
register entry blocks the window.

### DR-0004 — `impact`: test-marker evidence array ordering differs between runtimes

- **Status:** LANDED — opened as `spec-gap`, closed by amending `GPK-V0-027`, then re-adjudicated
  `python-defect` / known-divergent. Go already satisfied the amended clause and did NOT change, so
  no recorded expectation moved; `src/` is deliberately unrepaired. The discriminating case needs a
  spec-authored `impact` corpus, which does not yet exist — see **Outstanding** below (2026-08-28).
- **Command:** `impact`. **Discovered:** 2026-08-28, by `conformance/perf-v0` while measuring the
  `impact-path` task; the harness compares stdout on every sample and typed it `unequal-stdout`
  rather than reporting a timing anomaly.

**Divergence.** `impact cmd/corvint/main.go --limit 10` emits the same multiset of `test-marker`
evidence entries in a different order at `context.results[1].evidence`. Reproduced independently
2026-08-28, and on 100 of 100 sample pairs by the perf harness — deterministic, not intermittent.

| | entry `[0]` | entry `[1]` |
|---|---|---|
| Go | `same-package test carries scenario:z-last` | `same-package test carries feature:a-first` |
| Python | `same-package test carries feature:a-first` | `same-package test carries scenario:z-last` |

Every other field of both entries is identical.

**Mechanism — corrected 2026-08-28.** An earlier draft of this entry said neither runtime sorts. That
is wrong about Go, and the error mattered, because it inverted which runtime carries the defect.

- **Go sorts explicitly.** `markerRelationLess` (`internal/contextindex/impact.go:234`) is a total
  order over the referenced relation's first-occurrence path, line, and column, then the relation
  key, then the occurrence's own line and column; `impact.go:139-144` sorts before truncating. Both
  markers here sit on line 22 of one file, so the tiebreak is column — and the source text reads
  `// scenario:z-last feature:a-first`, so `scenario` sorts first.
- **Python does not sort.** `src/context_corvint.py:902-913` appends in `corvint.markers.items()`
  dict-insertion order and then applies `marked[:MAX_EVIDENCE]` directly.

**Why `spec-gap` and not `go-defect`.** `GPK-V0-002` classifies a reorder as a wire change "owned by
its existing spec", so ordering is not outside the spec's concern — it routes to the owning clause.
`GPK-V0-003` binds Go to every **accepted** ordering rule. There is no accepted ordering rule here:
no clause states one, and no conformance fixture pins it. Decisively, `GPK-V0-027` — the clause that
owns `impact` — affirmatively declines to make the oracle's bytes authoritative: "Observed byte
parity on named Corvint and Beamfall fixtures is dogfood evidence, **not universal Python parity**."
Contrast `GPK-V0-028`, which for `query` requires an exact byte match "including instruction-line
selection, referenced-tool ordering, ... uncertainty ordering". The spec knows how to mandate
ordering and did so for `query`, not for `impact`.

The counter-argument — that `GPK-V0-031` names "tuple ordering, evidence, and result limiting" as
compatibility surface — fails on that clause's own text: "The slice MUST NOT inherit impact's
slash-qualified-module restriction". `GPK-V0-031` explicitly distinguishes itself from `impact` and
cannot bind it.

**Why it is not cosmetic.** `MAX_EVIDENCE = 10` (`src/context_corvint_index.py:24`; Go truncates at
`internal/contextindex/impact.go:143-144` and `:294`). Past ten markers on one file the two orders
retain **different subsets**, so this becomes a content divergence, not a presentation one — and it
quietly undercuts `GPK-V0-027`'s own requirement to "use immutable committed evidence for clean
paths", because which evidence survives truncation depends on incidental iteration order.

**How it was closed** (operator decision, 2026-08-28). `GPK-V0-027` was amended to require that
every evidence array be emitted in a deterministic total order derived from stable content — the
referenced relation's first-occurrence path, line, and column, then the relation key, then the
occurrence's own line and column — so that `MAX_EVIDENCE` truncation retains the same entries on
every run and in every runtime. The clause states explicitly that an order derived from index
insertion or map iteration does not satisfy it even where stable within one runtime.

Go's `markerRelationLess` already implements exactly that order, so **Go did not change and no
recorded expectation moved**. Python's insertion order does not satisfy the clause, so this is a
`python-defect` under `GPK-V0-033` and `src/` stays unrepaired.

The projected cost of this amendment did not materialize. It was expected to force Go to sort,
break `impact` byte parity, and require an order-normalized comparison; because Go was already
correct, none of that was needed. That projection rested on the mechanism error corrected above.

**Mechanism — refined 2026-08-30. The oracle has TWO orders, and the divergent one is the
*sorted* one.** The account above is right that Go sorts and that Python emits `corvint.markers`
insertion order, but it presents the oracle as having a single order. It has two, selected by
whether the bounded index was built in-process or restored from `CORVINT_CACHE_DIR`:

- **Cold build.** The indexer walks `sources` and does `corvint.markers.setdefault(key, []).append(...)`
  (`src/context_corvint_index.py`, the `MARKER_RE.finditer` loop), so the dict's insertion order is
  first-discovery order: source enumeration order, then line number.
- **Cache write.** The payload serializes markers through `sorted(corvint.markers.items())`
  (`src/context_corvint_index.py`, the `"markers"` member of the cache payload) — lexicographic by the
  `(kind, id)` relation key.
- **Cache restore.** The loader reinserts in that serialized order (`src/context_corvint_index.py`, the
  `for marker in list(payload.get("markers", []))` loop), so after any cache hit the dict's insertion
  order is lexicographic by relation key, not discovery order.

`src/context_corvint.py` (the `for key, markers in corvint.markers.items()` loop that fills
`package_test_markers`, then slices `marked[:MAX_EVIDENCE]` unsorted) iterates in whichever order is
in force, so the emitted evidence order follows the cache state.

**Consequence: the two runtimes AGREE on a cold index.** Measured 2026-08-30 against this entry's
own pinned invocation — `impact cmd/corvint/main.go --limit 10`, corvint corpus at `58a2806e`,
materialized the way `perf-v0` materializes it, sanitized environment, private `CORVINT_CACHE_DIR`:

| oracle, cold index | oracle, warm index | candidate `58a2806e` |
|---|---|---|
| `825c9e0eb19c…` | `4544606de591…` | `825c9e0eb19c…` |

The cold digest is byte-identical to the candidate's. Those are the same two digests decision 0007
D9's grant records as `candidate-stdout` and `oracle-stdout`, which identifies the recorded
divergence as entirely a product of the oracle's cache-restore path. `perf-v0` discards five warmups
before measuring, so every measured sample sees the warm order and the cold order is never observed.
Reproduced identically on the beamfall corpus at `impact internal/graph/graph.go --limit 10`: cold
oracle and candidate agree on evidence order, warm oracle does not.

**The adjudication does not move.** `python-defect` stands, and is strengthened. The amended
`GPK-V0-027` requires a deterministic total order derived from stable content and says an order
derived from index insertion or map iteration does not satisfy it "even where stable within one
runtime". Both oracle orders are map-iteration orders, so **both** fail the clause; the cold one
merely coincides with a conforming order on these corpora. Go is unaffected and `src/` stays
unrepaired.

What is new is that the oracle is **not idempotent**: `corvint impact` emits different bytes on its
first run against a tree than on every later run. So this entry's own "why it is not cosmetic"
argument — that which evidence survives `MAX_EVIDENCE` truncation depends on incidental iteration
order — is true *within* the oracle, not only between runtimes. Not yet observed as a content
divergence: at the beamfall invocation, result `[4]` (`internal/graph/graph_consoleconfig_test.go`)
carries exactly `MAX_EVIDENCE` = 10 entries and the cold and warm sets are equal. The ceiling is
reached, not exceeded.

**Outstanding.** The discriminating case is not yet in any corpus. `impact` expectations in
`cli-parity-v0` are oracle-captured, so a case exercising this ordering cannot live there once
Python is known-divergent — it needs a **spec-authored** corpus, the way DR-0001's case lives in the
LRF corpus with a spec-authored expectation. (Partially superseded 2026-08-29: DR-0007 added a
`knownDivergence` declaration to `cli-parity-v0`, so the corpus CAN now carry a known-divergent
stdout case. It does not resolve this entry on its own — the declaration states the exact divergent
bytes, and an evidence-array **reordering** is not expressible as a bounded byte rewrite. A
spec-authored ordering case is still what DR-0004 needs.) Until that exists, the standing evidence is
`conformance/perf-v0`, which compares stdout on every sample and types this divergence
`unequal-stdout`. Tracked in `docs/agent-memory/tests.md`. That case MUST pin a **warm** index: on a cold
index the two runtimes agree byte for byte, so a cold case would not discriminate at all.

**Scope note.** `impact` reads `PASS` in the corpus inventory. With this entry landed rather than
open, it no longer blocks that row or `impact`'s contribution to the `GPK-V0-025` retirement window.
Note the weaker claim that `PASS` now carries for this clause: no corpus case would fail if the
ordering requirement were violated, so it is adjudicated but unverified — exactly the gap the
"agreed-wrong cases" section above describes.

**Perf grant (2026-09-03).** `docs/decisions/0040-perf-grants-dr0004-dr0009-2026-09-03.md` ratifies
measuring `conformance/perf-v0` task `beamfall-impact-path` (`impact internal/graph/graph.go
--limit 10`) across this entry, relaxing `unequal-stdout` alone. The grant is pinned to that exact
invocation and does not travel.

### DR-0005 — `harness event --event user-prompt`: the third result differs from the oracle's (Python symbol context window)

- **Status:** CLOSED — the symbol context window was repaired 2026-08-30, and the independent stdout
  encoding defect that kept this entry open was repaired under DR-0013 on 2026-09-01.
  Adjudicated **`go-defect`** on the **re-measurement of 2026-08-29 at head `ed2fca2`** under
  decision 0007 D3, which authorized that re-measurement and pre-approved adjudication **A** — the
  `user-prompt` context block is bound by `GPK-V0-002` exact parity and MUST carry symbol results —
  *unless the re-measurement contradicted A*. It does not; see **Adjudication** below. The repair
  landed at `fix/dr0005-repair-0830`; see **Repair landed** below for its measurement.
  The new `harness-user-prompt-non-ascii` parity row independently discriminates the transport
  spelling; DR-0005 no longer carries that second defect as a reason to remain open.
- **Command:** `harness event --event user-prompt`. **Discovered:** 2026-08-28, by
  `conformance/perf-v0` while measuring `beamfall-harness-user-prompt`; the harness compares stdout
  on every sample and typed it `unequal-stdout` rather than reporting a timing anomaly.
  **Re-measured:** 2026-08-29.

**Reproduction.** The pinned Beamfall corpus is materialized exactly as `materializeCorpus`
(`conformance/perf-v0/corpus.go`) builds it — `git archive` of revision
`fa3b1e7fe5bc6c10e4b09b2729f364780f567a48`, then `git init` / `git add --all --force` / `git commit`
— and the rebuilt tree is asserted equal to the pinned tree `3a4251e0f0ebe70a489e296b32189634250f69d4`,
which it is. Both runtimes then receive the manifest's own argv and stdin for
`beamfall-harness-user-prompt`:

```
harness event --host claude-code --host-version 1.0.0 --surface plugin \
  --adapter-version 0.1.0 --event user-prompt --input -
```
```
{"task":"Identify the active work queue, required workflow gates, and minimum context needed to safely take the next roadmap ticket"}
```

under `sanitizedEnvironment` (`conformance/perf-v0/process.go`), oracle as `python3 -m corvint_cli`
with `PYTHONPATH=src`, candidate built `GOTOOLCHAIN=local CGO_ENABLED=0 go build ./cmd/corvint`.

**Divergence, before and after the re-measurement.** Same corpus, same argv, same stdin:

| | oracle | candidate 2026-08-28 | candidate 2026-08-29 (`ed2fca2`) |
|---|---|---|---|
| results | 3 | 1 | 3 |
| result 1 | `instructions AGENTS.md` 438 | identical | identical |
| result 2 | `symbol script/bf_tools.py:cmd_roadmap_ticket` 166 | **absent** | identical, 166 |
| result 3 | `symbol script/bf_tools.py:cmd_context_packet` 108 | **absent** | `symbol script/bf_tools.py:cmd_gate_summary` 120 |
| stdout bytes | 5527 | not recorded | 5520 |
| `coverage.packet_bytes` | 4828 | not recorded | 4824 |
| stdout equal | — | no | no |

The candidate no longer omits symbol results: two of its three results are symbols, and the
higher-scoring one is identical to the oracle's in score and every field. What remains is a
**ranking difference at rank 3** — the candidate admits a symbol the oracle's own admission filter
rejects, and that symbol displaces the oracle's third result — plus one independent encoding
difference recorded separately below.

**Mechanism** (re-derived at head; cited by function name, not line, because this entry's previous
line anchors had all drifted onto unrelated code). Every claim below was read at `ed2fca2`.

- `harnessIndexedContext` (`cmd/corvint/harness_context.go`) routes `user-prompt` to
  `contextindex.BuildEval` followed by `contextindex.EvalQuery` — **not** to
  `contextindex.BuildQuery`, which serves the authority-start `query` verb and never reaches this
  event. The function's own comment states the choice and its reason. The previous mechanism
  section's premise — that the candidate "has no symbols on this path by construction" — was true of
  `BuildQuery` and has been false of this event since `BuildEval` landed. DR-0006 already records
  the correct routing.
- `BuildEval` compiles through `compileEval` → `compileTables(false)` → `compileSources`, which
  populates `Index.Symbols` in full. The only table `compileEval` narrows is whole-repository import
  extraction, and `compileMarkerTestImports` restores the marker-test subset that
  `featureImplementationCandidates` dereferences. Symbol extraction is not narrowed at all.
- Measured symbol populations on the pinned corpus, candidate against oracle: `.py` 2618 = 2618,
  `.ts` 2317 = 2317, `.tsx` 2686 = 2686, `.mjs` 209 = 209, and `.go` **16041 against 17761**
  (23871 against 25591 in total, counted after `evalIndexWithSymbols` appends web symbols). The Go
  gap is 3250 oracle-only rows — 3177 of them `var`, matching the oracle's line-regex acceptance of
  indented function-local `var`/`const` declarations — against 1530 candidate-only rows.
- `evalPrepareSymbol` (`internal/contextindex/eval_query.go`) builds a per-symbol context window from
  the pinned source lines at rank time: `start = Line-4`, `end = min(len(lines), Line+20)`,
  truncated to 2000 runes. It uses that one window for **every** language.
- The oracle uses three (`src/context_corvint_index.py`): `_go_symbols` takes `lineno-4 … lineno+20`;
  `_web_symbols` takes `lineno-3 … lineno+20`; `_python_symbols` takes
  `lineno-3 … min(len(lines), node.end_lineno, lineno+20)` — **clamped to the AST declaration end**.
  The candidate therefore matches the oracle on `.go` symbols and, for `.py` and web symbols, starts
  one line earlier and (for `.py`) never clamps.

Both consequences are measured on `script/bf_tools.py` in the pinned corpus, where
`cmd_gate_summary` is declared at line 377 with its body ending at 385 and `cmd_context_packet` is
declared at 388:

- **The missing end clamp admits `cmd_gate_summary`.** The oracle's window is lines 375–385 and stops
  at the declaration end, giving `overlap = {gate}` and `context_overlap = {gate}`. The admission
  filter in `_query` (`src/context_corvint.py`) drops a symbol when
  `not exact_name_present and len(overlap) < 2 and len(context_overlap) < 3`, so the oracle drops it:
  it is absent from the oracle's ranking even at `--limit 25`. The candidate's window is lines
  374–397, which absorbs `cmd_context_packet`'s declaration and body; the borrowed `context`,
  `roadmap` and `ticket` terms lift `context_overlap` to 3, clearing the filter, and it is admitted
  at 120.
- **The one-line start offset re-scores `cmd_context_packet`.** The candidate's extra leading line is
  385, `emit({"tool": "gate_summary", ...})`, which contributes the term `gate`. That single extra
  context term adds 6 through `len(context_overlap) * 6` and 6 more through
  `int(120 * len(context_overlap) / len(terms))` (3 → 4 over 18 query terms) — exactly the
  108 → 120 delta.

The two symbols then tie at 120, and `evalRankSymbols` breaks a tie on the composite
`path:line:name` id as a string, so `script/bf_tools.py:377:cmd_gate_summary` sorts ahead of
`script/bf_tools.py:388:cmd_context_packet`. `EvalQuery` admits only `confident[:2]` behind a
document result — the same two-symbol shape the oracle produces — so the tie **evicts** the oracle's
third result rather than appending beside it. Measured candidate ranking, for the record:
`cmd_roadmap_ticket` 166, `cmd_gate_summary` 120, `cmd_context_packet` 120, against the oracle's
166 / dropped / 108.

**Why the previous "a partial symbol set cannot close it" argument no longer applies.** Both of its
reasons assumed a *narrowed* symbol build carrying no per-symbol context. Neither holds: the
candidate does a complete symbol build on this path and computes the context window from pinned
source bytes at rank time, and it reproduces the oracle's top two results exactly. The associated
performance argument is moot in the same direction — the run that recorded this divergence measured
this event at 448.9 ms p95 for the candidate, inside `GPK-V0-017(b)`'s 500 ms cap, against the
oracle's 2139.9 ms. Every per-term weight in this case is at the `max(6, …)` floor, so the
`symbol_term_frequency` denominator difference implied by the `.go` population gap is invisible
here; that is a property of this case, not a proof that the tables agree.

**A second, independent divergence measured in the same case: non-ASCII stdout encoding.** With the
symbol window aligned (throwaway probe, below) the two stdouts still differ by exactly 3 bytes. The
oracle's `_emit` (`src/corvint_cli.py`) calls `json.dumps` **without** `ensure_ascii=False`, so it
writes `—` for the em dash in `AGENTS.md`'s title; the candidate's `emit`
(`cmd/corvint/main.go`) writes stdout through `gokernel.CanonicalJSON`, which deliberately
reproduces `ensure_ascii=False` — correct for the harness receipt digest, which
`context_corvint_harness.py` does compute with `ensure_ascii=False`, and wrong for stdout. Measured:
oracle 5527 B, zero bytes ≥ 0x80, one `\u` escape; candidate 5520 B, three bytes ≥ 0x80, no escape.
This is general to every command whose stdout payload carries a non-ASCII character, it is not
DR-0005's subject, and it **needs its own register entry**. It is recorded here because it is why
this case's stdout stays unequal even after the symbol defect is repaired.

**Adjudication: `go-defect`** (decision 0007 D3, whose condition is satisfied). The `spec-gap` rested
on one unanswered question: whether the `user-prompt` context block inherits `GPK-V0-028`'s explicit
exclusion of symbol ranking or `GPK-V0-002`'s exact-parity duty. D3 answers it as **A** —
`GPK-V0-002` governs and the block MUST carry symbol results — so the spec now determines the
observable, and under `GPK-V0-033` the runtime that departs from it is the defect. The candidate is
that runtime. Python is unchanged and MUST keep cross-checking.

The re-measurement does not contradict A. It strengthens it three ways:

1. The block already carries symbol results, and its top-ranked symbol is byte-identical to the
   oracle's. A is a description of a reachable state, not an aspiration.
2. The residual difference is a **scoring input** — one context window, wrong by one line at the
   start and unclamped at the end — not an absent capability. Nothing in it engages `GPK-V0-028`'s
   "symbol ranking" exclusion as a reason the block may lack symbols.
3. The entry's old scope argument — that the divergence is invisible to the `query` slice's own
   parity rows because `GPK-V0-028` requires `--limit 1` — is retired by decision 0007 D5, which
   widened that profile to `--limit` 1–10. Measured at this head: `query --limit 1` is byte-equal
   between the runtimes on this corpus, and `query --limit 10` returns typed
   `{"code":"unsupported-query-option","error":"native Go authority-start query requires --limit 1"}`
   against the oracle's three results. That is D5 implementation work rather than a divergence — a
   typed refusal, not an approximation — but it means the same symbol-ranking question will arrive
   on the `query` verb, inside a clause that already demands the oracle's exact canonical bytes,
   unless one repair serves both.

**Repair (authorized; deliberately not in the change that re-measured this).** Sized by measurement,
not estimate:

1. **Give `.py` symbols the oracle's window.** `contextindex.Symbol` carries `Kind, Name, Path,
   BlobHash, Line` and no declaration end, and `pythonDefinitions.Add`
   (`internal/contextindex/pythonsymbols.go`) receives only a start line from `internal/pythonsyntax`.
   The end clamp therefore needs a declaration-end fact carried from the grammar into `Symbol`, or
   derived at rank time — a change to the symbol fact shape, not a one-line edit in
   `evalPrepareSymbol`.
2. **The start offset is load-bearing, and decision 0007 D4 as ratified does not close this case.**
   D4 clamps the window end and explicitly declines to ratify the start offset, recording it as a
   separate question. This is the measured answer. A throwaway probe applying **only** the end clamp
   reproduced the oracle's three result identities but scored `cmd_context_packet` **120 against the
   oracle's 108** — still `unequal-stdout`. A probe applying the end clamp **and** the Python start
   offset (`Line-3`) produced a packet identical to the oracle's in every field including
   `packet_bytes` 4828, leaving only the 3-byte encoding difference above. `_web_symbols` also starts
   at `lineno-3`, so the same offset applies to web symbols and is not exercised by this case.
3. **The `.go` symbol population gap** (16041 against 17761) perturbs `symbol_term_frequency`
   denominators. It changes nothing here because every weight saturates at the floor, but exact
   parity across a corpus is not provable while it stands.
4. **The non-ASCII stdout encoding**, which no symbol repair reaches.
5. **Cost.** Neither probe added measurable cost — the window change only narrows the text
   tokenised. An unpreregistered 12-run probe on a loaded host measured head at 596.6 ms median and
   the both-changes probe at 505.0 ms median. That is a sanity check, not `GPK-V0-016` evidence, and
   is recorded only to show the repair is not a performance trade.

**Repair landed 2026-08-30** (branch `fix/dr0005-repair-0830`), as items 1 and 2 above and nothing
else. The candidate now keeps the oracle's three windows instead of one: `symbolContextWindow`
(`internal/contextindex/eval_query.go`) opens a `.py` window at `Line-3` and clamps it to the
declaration end, opens a web window at `Line-3`, and leaves the `.go` window at `Line-4`. Languages
the oracle extracts no symbols for keep the Go window, because no oracle window exists to port for
them.

The declaration-end fact is carried, not derived. `Symbol` (`internal/contextindex/index.go`) gained
`EndLine`, and the closed grammar reports it through a new **optional** sink half:
`DefinitionSpanSink` (`internal/pythongrammar/native.go`), which the parser calls only when the
collector it was handed implements it. `FactSink` is unchanged, so `analyzerpython` and every other
collector are untouched. `Parser.parse` keeps a stack of open definitions and reports each one's end
at the dedent that closes its suite, taking the line from the last content token's own end offset so
a multi-line string does not report the line it started on. Measured against CPython `ast` over
1,174 files of the Python 3.9 standard library the closed grammar accepts: **25,967 declarations,
zero mismatches** against `node.end_lineno`, and 581 of 581 over this repository's own `src/`.

Re-measured on the pinned Beamfall corpus rebuilt exactly as the Reproduction section above
specifies (rebuilt tree `3a4251e0f0ebe70a489e296b32189634250f69d4`, asserted equal), same argv and
stdin:

| | oracle | candidate before | candidate after |
|---|---|---|---|
| result 3 | `symbol script/bf_tools.py:cmd_context_packet` 108 | `symbol script/bf_tools.py:cmd_gate_summary` 120 | `symbol script/bf_tools.py:cmd_context_packet` **108** |
| stdout bytes | 5527 | 5520 | 5524 |
| `coverage.packet_bytes` | 4828 | 4824 | **4828** |
| stdout equal | — | no | no — 3 bytes, encoding only |
| JSON equal | — | no | **yes** |

The two stdouts now parse to the same value, and every remaining byte difference is the
`ensure_ascii` one: the oracle writes one `\u2014` escape and no byte above 0x7F, the candidate
writes the three UTF-8 bytes of the same em dash and no escape. Nothing else differs.

**What the ratified fix shape did not cover.** Decision 0007 D4 states that the change moves receipt
bytes and that "the corpus expectation recapture is part of the work". No recapture was needed: a
full unfiltered `cli-parity-v0` replay at the repaired head is `parity=104` with every row PASS and
no manifest byte moved. That is consistent with this entry's own note that the parity corpus does not
reach these paths — the corpus holds no case where a `.py` or web declaration is scored against a
query — and it is the same coverage hole the discriminating case below exists to close, not evidence
that the repair is inert. The Beamfall measurement above is the evidence that it is not.

**Discriminating case required before `harness` may be promoted** (`GPK-V0-034`). It is now
authorable: A is ratified, and `GPK-V0-002` names the oracle's output as the expected observable for
a supported parity case, so the expectation comes from spec text rather than from whichever runtime
was read first. The case needs a `harness` corpus at a context limit above 1 on a repository whose
symbol corpus actually scores against the task — the Beamfall corpus does and the Corvint corpus does
not (`harness-user-prompt` on `corvint` is `state: valid` with an agreed digest in the same run) — and
it MUST contain a `.py` declaration immediately followed by another declaration, because that
adjacency is the only thing the window difference exploits.

**Measurement impact (consequence, not rationale).** Unchanged by the re-measurement:
`GPK-V0-016` invalidates measurements with unequal outputs, so `gpkV0017Criteria` in
`conformance/perf-v0/results/beamfall-userprompt-divergence-2026-08-28/report.json` still records
`GPK-V0-017(b)` / `beamfall-harness-user-prompt` as `NOT_RUN`, "measurement invalid:
unequal-stdout", with the run's outcome `insufficient_evidence`. A threshold not measured is
`NOT_RUN`, not waived, and it stays `NOT_RUN` until **both** divergences above are repaired.

**Note.** DR-0004 and DR-0005 were both surfaced by `conformance/perf-v0` typing a divergence
`unequal-stdout`, on the same day, on the same Beamfall corpus. That is two of five register entries
found by the performance harness rather than by the conformance corpus, which is worth reading as a
statement about corpus coverage: the perf harness compares full stdout across 100 sample pairs on
real repositories, and the parity corpus does not yet reach these paths at all. The re-measurement
adds a second reading: this entry's code citations were line anchors, and every one of them into
`internal/contextindex/index.go`, `cmd/corvint/harness_context.go` and
`docs/specs/go-production-kernel-migration-v0.md` had drifted onto unrelated text within a day,
carrying a false mechanism with them. Entries cite by name.


### DR-0006 — `harness event --event file-change` on a Python path: candidate silently drops reverse-import results and still reports complete coverage

- **Status:** CLOSED 2026-08-29 — adjudicated **`go-defect`** on the coverage claim, repaired on the
  coverage claim, and then closed outright when the shortfall itself was repaired. The candidate
  first stopped asserting complete coverage over an answer whose reverse-import dimension it did not
  compute; decision 0007 (D2) then widened `GPK-V0-027` to name the `.py` and web resolution rules,
  and porting them retired the shortfall. Re-measured on a clean copy of this repository at
  `ed2fca2c`, same argv and stdin as the discovery run: `h-file-change` now returns the oracle's five
  results in 4,890 bytes where it returned two in 3,891, and is **byte-equal to the oracle**. The
  coverage note survives, narrowed: it now fires only for a suffix `GPK-V0-027` names no rule for,
  which `harness event` can still reach and the `impact` command refuses.
- **Command:** `harness event --event file-change`. **Discovered:** 2026-08-29, by the
  `GPK-V0-021` Corvint self-dogfood
  (`conformance/perf-v0/results/corvint-selfdogfood-2026-08-29/`), case `h-file-change`.

**Divergence.** Changed path `src/context_corvint_harness.py` on a clean pinned copy of this
repository (tree `323b3f06…`):

| runtime | results | coverage |
|---|---|---|
| Python | `path` 1000, `spec` 825, and **three `reverse-import`**: `src/corvint_cli.py` 700, `tests/test_context_corvint_harness.py` 650, `tests/test_harness_protocol_equivalence.py` 650 | `included=5 omitted=0 requested=5` |
| Go | `path` 1000, `spec` 825 | `included=2 omitted=0 requested=2` |

The two shared results match in every field. Stdout differs by 1,092 bytes (4851 vs 3759).

**Why this is a `go-defect` and not only a parity gap.** The candidate reports
`omitted_results: 0`, `uncertainty: []`, and `critical_missing: []` — that is a positive assertion
that nothing was left out. Three results were left out. The candidate is not *withholding* them
under a declared boundary; it does not know they exist, so it reports complete coverage of a
smaller universe. `GPK-V0-023` requires a partial slice to remain **visibly** experimental, and the
whole point of the coverage block is to make incompleteness legible. A silent shortfall reported as
full coverage is the failure mode this register's opening section calls "silent divergence behind a
green suite", one layer down: green coverage over an incomplete answer.

**Mechanism** (verified, not inferred). `sourceImports` (`internal/contextindex/parse.go`, the
`.py` arm) returns an empty set for `.py` by deliberate choice, commented "Supported impact
repositories have slash-qualified Go module targets. No valid Python import can equal such a
target." `reverseImporters` (`internal/contextindex/impact.go`) then builds its match target as
`index.Module + "/" + path.Dir(changedPath)` — always a Go import path — so for
`src/context_corvint_harness.py` the target is `github.com/corvint-context/corvint/src`, which no Python
`import` statement can produce. The Go-path control case (`h-file-change-go`) is byte-equal at
6079 B, confirming the mechanism is language-specific and not a general `file-change` fault.

The comment is still true as written and is still not the whole reason. Go's target is unmatchable
by a Python import, but the deeper point is that Go builds **no** Python-shaped target at all: the
oracle's `_reverse_importers` (`src/context_corvint.py`) has an explicit `.py` arm that searches for
the dotted module candidates `src.harness`, `harness` and the `__init__` forms, and an explicit web
arm besides. The shortfall is therefore not confined to `.py`. Every non-Go changed path reaches the
same single Go-shaped target, so `.ts`/`.tsx`/`.js`/`.jsx`/`.mjs`/`.cjs` changed paths carried the
identical false coverage claim — and more sharply since `internal/contextindex/webimports.go`
landed, because web sources now populate `Index.Imports` with real lexed specifiers that this search
still cannot match.

The inconsistency that makes this a defect rather than a design choice: at discovery the **`impact`
command** already refused this exact input explicitly, exiting 2 with
`{"code":"unsupported-impact-path","error":"native Go impact currently supports only .go paths"}`
(a message the closing change replaced, because it stopped being true).
The **`file-change` event** reaches the same reverse-import machinery through a different route —
`harnessIndexedContext` sends `user-prompt` to `BuildEval`/`EvalQuery` but `file-change` to full
`Build` plus `EvalImpact` (`cmd/corvint/harness_context.go`) — and applied no such refusal. One
entry point declared the boundary; the other crossed it silently. `range impact` was already honest
here, and supplied the repair's vocabulary: it reports non-Go changed paths through
`coverage.uncertainty` as "`N` non-Go changed paths are outside the native Go range profile"
(`internal/contextindex/range_impact.go`). Note also that `importEvidenceLine`
(`internal/contextindex/impact.go`) already carries Python-aware handling, which suggests the case
was expected to be reachable.

**Repair.** `setCoverage` now derives, from the receipt's own mode and request, how many requested
changed paths have a reverse-import universe the native Go profile cannot compute — a path that is
an indexed source, is not a test path (neither runtime runs the reverse-import stage for those), and
is not `.go` — and names that count in `coverage.uncertainty`:

> `reverse-import results for N non-Go changed paths are outside the native Go impact profile`

Refusing the input, the other option the adjudication allowed, was rejected: it would discard the
two results the runtimes agree on and fail an event the oracle answers, which is a worse parity
outcome than a declared shortfall. `omitted_results` is deliberately left at its honest value. The
engine cannot count results it never computed, and fabricating a number there would replace a false
completeness claim with a false quantity; the count remains what the ranking ceiling dropped, and
`uncertainty` now names the dimension that count does not cover. The note is emitted only when it is
earned, so a Go-only repository's receipt stays byte-identical to one built before it existed — the
same property `unparsed` and `extraction` hold.

**After the repair**, re-measured on the same pinned copy (tree `323b3f06`), same argv and stdin:

| | oracle | candidate before | candidate after |
|---|---|---|---|
| `h-file-change` stdout bytes | 4851 | 3759 | 3851 |
| `coverage.omitted_results` | 0 | 0 | 0 |
| `coverage.uncertainty` | `[]` | `[]` | `["reverse-import results for 1 non-Go changed paths are outside the native Go impact profile"]` |
| `h-file-change-go` stdout bytes | 6079 | 6079 | 6079 (byte-equal) |

The other seven measured `harness event` cases are byte-unchanged.

**What the corpus could not discriminate, and now can.** At discovery, both
`conformance/cli-parity-v0` `file-change` cases (`harness-file-change`, `harness-empty-paths`) sent
`pkg/sample.go`, and all three `impact` cases sent `pkg/sample.go`; the `impact` CLI refused a
non-`.go` argument before the kernel was reached, so no parity case exercised a non-Go changed path
through this surface at all. The closing change adds four cases over two new fixtures:
`impact-python-module`, `impact-python-package`, `impact-web-component` and `impact-web-directory`.
The regression is held by `TestImpactNeverClaimsCompleteCoverageOverUncomputedReverseImports`
(`internal/contextindex/impact_reverse_import_coverage_test.go`), which asserts the durable
invariant rather than the shortfall of the day: a receipt may resolve the reverse-import universe or
declare that it did not, but may never assert complete coverage while omitting the dimension. It now
passes through its **first** branch, which is what the entry predicted a repair would do rather than
falsify. Python is unchanged and keeps cross-checking.


### DR-0007 — `impact` / `range impact`: a citing change set mints its own `accepted-contract` authority

- **Status:** LANDED — opened as `spec-gap`, closed by ratifying the owning-spec amendment, then
  re-adjudicated `python-defect` / known-divergent. `CF-V0-031` and `CF-V0-032` were ratified
  `accepted` on 2026-08-29 under repository-owner delegation
  ([decision 0006](../docs/decisions/0006-caller-authored-authority.md)), which closes the gap that
  required the park. Both surfaces now satisfy the clause; `src/` is deliberately unrepaired, and the
  disagreement ships as the discriminating parity case `impact-self-authored-adr`.
- **Command:** `impact` (oracle-shared) and `impact --base` (`corvint-range-impact/0`, Go-only).
  **Discovered:** 2026-08-29 by red-team review.

**Divergence class: agreed-wrong.** This is not a Go/Python disagreement. It is the class this
register names at "Known limitation: agreed-wrong cases" — both runtimes resolve governing-document
authority at the *target* revision, which is the same tree the caller is editing, so both mint
authority from caller-authored bytes. When this entry was opened the parity corpus could not
discriminate it at all: no fixture repository under `conformance/cli-parity-v0/fixtures/` contained a
`docs/adr/` tree, so the ADR authority branch was never reached by any of the 97 cases. Fixture
`impact-adr` and case `impact-self-authored-adr` close that hole (see **Discriminating case** below),
and with `CF-V0-031` ratified the class is no longer agreed-wrong: the candidate satisfies the clause
and the oracle does not, which is an ordinary `python-defect`.

**The laundering path.** An actor adds `docs/adr/9999-*.md` with `status: accepted` **in its own
change set**, cites `ADR-9999` from a changed hunk, and receives
`"confidence": "authoritative", "authority": "accepted-contract"` for its own change.

- Go, `range impact`: `citedRangeDecisions` (`internal/contextindex/range_impact.go:473-537`) reads
  `status` from `index.Documents`, which is resolved at the captured target HEAD.
- Go, `impact`: `recordResult` (`internal/contextindex/impact.go:306-337`) reads `status` from
  `index.Documents` for the ADR named by a ledger record's `adr:` field.
- Python, `impact`: `_record_result` (`src/context_corvint_index.py:1345-1367`) is byte-equivalent in
  this respect — `document = corvint.documents.get(source.path)`, `status = ... document.fields`,
  `binding = status == "accepted"`, `authority = "accepted-contract"`. The oracle has the same hole.

The oracle therefore decides nothing here, which is what makes the outcome `spec-gap` rather than
`go-defect`: with both runtimes agreeing, only spec text can adjudicate, and no spec text existed.

**Adjudication: `spec-gap`.** No clause in any spec under `docs/specs/` defined the
`accepted-contract` authority label or constrained the revision at which a governing document's
status is resolved — `grep -rl "accepted-contract" docs/` returned nothing before this amendment.
The observable was undetermined, so neither runtime won by default. The nearest governing text is
`docs/specs/change-frontier-v0.md:166` (`CF-V0-014`), which freezes caller-reported evidence as
non-closing precisely so an actor cannot certify its own work — but it states that invariant only
over the TCQ harness relation, never over authority resolution. `CF-V0-031`
(`docs/specs/change-frontier-v0.md`, "Caller-authored authority") is the amendment that closes the
gap: a governing document confers authority only when resolved from a revision the caller did not
author, and a document inside the declared change set withholds authority and records one named
uncertainty entry instead.

**Repaired surface: `range impact` (Go-only).** `impact --base` has no Python counterpart — the
oracle's `impact` accepts only explicit paths (`src/corvint_cli.py:115-118`) and no `--base` — so it is
not a parity case and no oracle disagreement is being resolved toward the candidate. It also has the
one input the clause requires: an immutable base commit whose ancestry is verified, from which the
true change set is computed by Git rather than declared by the caller. `callerAuthoredPaths` derives
that set from the diff, and a cited ADR inside it now yields
`"ADR-NNNN cited by a changed hunk is authored by the same change set and confers no authority"`
in `coverage.uncertainty` and no result. Adversarial case:
`TestRangeImpactRefusesAuthorityFromSelfAuthoredADR`
(`internal/contextindex/range_impact_test.go`), which mints `docs/adr/9999-self-minted.md` inside
the range and asserts both the withheld authority and the named abstention. Verified to FAIL with
the guard removed (`decision:docs/adr/9999-self-minted.md` is emitted as an authority) and PASS with
it, and a pre-existing accepted ADR cited by the same change set keeps its authority, so the refusal
is scoped rather than a blanket kill.

**Previously parked surface: `impact` (oracle-shared).** Kept for the record — this was the state
while the entry was open, and both reasons were correct at the time. Reason 1 lapsed when the owner
ratified the clause on 2026-08-29; reason 2 did not lapse and is why the repair is an abstention
rather than a guard. See **Resolution** below for what shipped.

1. `GPK-V0-033` is explicit: "Both runtimes stay as they are until the amendment lands; a
   `spec-gap` never authorizes a repair to either side." `CF-V0-031` is owned by the Frontier spec
   and was authored against the Frontier invariant; the `impact` wire is not a Frontier surface, and
   binding it to that clause is an owner decision, not a builder decision. Repairing Go here would
   also be the exact failure mode `GPK-V0-033` names — a `spec-gap` resolving toward the candidate
   "because the candidate is more convenient, newer, or already written."
2. The repair is not available at this input contract. `Impact(index, paths, limit)` receives no base
   revision and no diff; `paths` is the change set the caller *declares*. A rule keying on `paths`
   would downgrade an honest caller who declares the ADR it touched, and would not touch an attacker,
   who simply omits it — the ADR reaches `recordResult` through a ledger record's `adr:` field, not
   through `paths` at all. Such a rule closes nothing while appearing to, which `CF-V0-025` forbids
   asserting and `CF-V0-031` explicitly addresses: a surface with no caller-independent revision
   "cannot satisfy this clause and MUST NOT claim to."

**Resolution (2026-08-29).** Both halves of the required action are done, and the entry no longer
blocks `impact`'s `PASS` row or its `GPK-V0-025` contribution.

- **Owner decision on the `impact` input contract.** `CF-V0-031` admits exactly two dispositions for
  a surface with no caller-independent revision, and decision 0006 takes the second: `impact` does
  not gain a base revision, so its accepted-authority labels are reported as unverified.
  `recordResult` (`internal/contextindex/impact.go`) now emits confidence `low` with authority
  `unverified-contract` in place of `accepted-contract` / `partially-superseded-contract`, and
  `setCoverage` (`internal/contextindex/receipt.go`) derives one named `coverage.uncertainty` entry
  per withheld citation from the emitted evidence, so a projection that drops the citation drops its
  explanation with it. The withholding is **unconditional**, not a predicate over `paths`: `paths` is
  the caller's own declaration, so a predicate on it would downgrade an honest caller and never touch
  an attacker, who reaches `recordResult` through the ledger record's `adr:` field. That is the
  control `CF-V0-025` forbids asserting, and refusing to build it was the correct call when this
  entry was opened.
- **Sibling surface `rangeMarkerResult`.** Repaired under the same clause and, because `range impact`
  does have a verified immutable base, scoped rather than blanket: the `authoritative`/
  `canonical-ledger` label is withheld (to `low` / `unverified-ledger`, plus a named uncertainty
  entry) only where the ledger file declaring the record is inside the change set Git computed.
  `TestRangeImpactWithholdsSelfAuthoredLedgerAuthority`
  (`internal/contextindex/range_impact_test.go`) asserts both halves in one receipt — the
  self-declared record loses its authority and a pre-existing record keeps it.
- **Discriminating case.** Fixture `conformance/cli-parity-v0/fixtures/impact-adr` is
  `impact-go` plus `docs/adr/0012-token.md` (`status: accepted`) and a ledger record citing it, so
  the authority branch is reachable for the first time. Case `impact-self-authored-adr` runs
  `impact pkg/sample.go --limit 10` against it. Its expectation is captured from the pinned Python
  oracle by `cli-parity-v0 capture`, never from the candidate; the full recapture that produced it
  reproduced all 97 existing rows byte-identically (55 added lines, zero removals), which is the
  evidence that no other expectation moved.

**Adjudication: `python-defect` (known-divergent, no repair).** With `CF-V0-031` ratified, the oracle
contradicts binding spec text while the candidate satisfies it. `src/context_corvint_index.py:1345-1367`
is NOT repaired: it is scheduled for deletion under `GPK-V0-025`, it no longer supplies the
expectation, and repairing it would erase the recorded evidence that the runtimes differ. Go MUST NOT
be regressed to reproduce the oracle's behavior.

**How the corpus carries a stdout divergence.** DR-0004 recorded that `cli-parity-v0` had no way to
hold a known-divergent case, because every expectation there is oracle-captured. This entry adds the
narrowest mechanism that does: a case may declare a `knownDivergence` naming its register entry, its
clause, and the exact byte regions where the two runtimes differ. Each declared rewrite states what
the clause requires the candidate to emit and what the oracle emits instead; each must occur exactly
**once** in the candidate's stdout; and once every rewrite is applied the whole stdout must equal the
oracle's byte-for-byte. Three regions are declared here — the withheld authority label, its withheld
confidence, and the derived uncertainty entry. The fourth moving member, the receipt's own
`packet_bytes` count, is **not** declared: no clause can author a byte count, and transcribing it
would take an expected byte from the candidate, so the runner instead requires the two counts to
differ by exactly the length the declared rewrites account for. Any unaccounted byte anywhere in the
receipt breaks that identity and fails the case.

The declaration is pinned in code (`validKnownDivergence`, `conformance/cli-parity-v0/manifest.go`)
to this case ID, to `DR-0007` / `CF-V0-031`, and to the exact shape of each rewrite, so it cannot be
widened by manifest data alone. It is directional and fail-closed in both directions, which two tests
assert: a candidate that stops emitting the withheld form fails on the uniqueness check
(verified by replaying the pre-repair candidate, which fails with `occurs 0 times`), and a candidate
whose receipt moved for any other reason fails on the byte-count identity.

**Residual weakness, stated rather than hidden.** One declared rewrite — the uncertainty entry's full
text — is the candidate's rendering of a form `CF-V0-032` requires but does not word. `CF-V0-032`
pins the part that carries meaning (the entry must be derived from the emitted evidence and must name
the withheld reason), and `validKnownDivergence` refuses any declaration whose candidate side does not
carry it. The remaining wording is pinned by the manifest so it cannot change silently, but it is not
spec-authored, and a `PASS` on this case does not claim that it is.

**Residual weakness closed (2026-09-12).** Decision 0101 amends `CF-V0-032`
(`docs/specs/change-frontier-v0.md`) to state the entry's exact text on a surface with no
caller-independent revision -- `<reason>: no caller-independent revision is available at which to
resolve the cited decision status, so accepted authority is withheld` -- plus its deduplication, byte
order, and leading placement. The manifest's declared candidate side is now spec-authored text, and
`TestImpactWithholdsUnverifiableADRAuthority` asserts it byte-exactly. The adjudication is unchanged.

**Parity effect.** The Go repair alone changed nothing: replayed against the unchanged 97-case
corpus the abstaining candidate printed the identical
`SUMMARY parity=97 accepted-divergences=3 location-normalizations=13 structural-fields=1
unsupported-refusals=4 full-gpk-v0-005=PARTIAL detached-descendants=NOT_RUN`, which is not evidence
of safety — it is the measurement of how blind the corpus was. Adding the discriminating case is what
moves the line, to

```
SUMMARY parity=98 accepted-divergences=3 known-divergences=1 location-normalizations=13 \
structural-fields=1 unsupported-refusals=4 full-gpk-v0-005=PARTIAL detached-descendants=NOT_RUN
```

`parity` 97 -> 98 is the new case; `known-divergences` is a new counter reporting declared stdout
divergences, at 1. No count was edited by hand: both are derived from the manifest by the runner, and
the case they count is registered here. The same replay run against the **pre-repair** candidate
fails that case, so the number is backed by a corpus that can now express the defect.

### DR-0008 — `harness event --event user-prompt`: a packet no result's own words support is published as `READY`

- **Status:** LANDED — adjudicated `python-defect` / known-divergent under a newly ratified clause.
  `GPK-V0-039` (`docs/specs/go-production-kernel-migration-v0.md`) requires the withdrawal the
  candidate performs; the oracle publishes instead. The disagreement is **EXPECTED and MUST NOT be
  reported as a failure** (`GPK-V0-033`). `src/` is deliberately unrepaired.
- **Command:** `harness event --event user-prompt`, and every other surface that compiles a query
  packet. **Discovered:** 2026-08-29 by dogfood.

**Divergence class: agreed-wrong.** This is the class this register names at "Known limitation:
agreed-wrong cases", and the second entry to reach it. Both runtimes published the same wrong bytes
from the same input at the same revision, so nothing in the `GPK-V0-033` disagreement loop could
surface it: the case passed, and the spec was never consulted. It was found by reading a packet, not
by running the corpus.

**The accident.** A query and a repository share exactly one word, and that word arrives by
accident — `terms()` splits a hyphenated canonical id into ordinary English, so `a11y-high-contrast`
contributes the word `high`, and the query "how do I bake sourdough bread at high altitude" matches
it on `high` and on nothing else. The packet comes back `READY` with confident results. Score cannot
separate this case: it is an unnormalised sum of term counts, id bonuses and position bonuses, so it
is not comparable between two different queries and any threshold over it is a constant fitted to one
sample.

**Observed bytes (2026-08-29, both runtimes, fixture `harness-out-of-scope`).** Re-derived by running
the pinned oracle (`src/` at `f47db8f2`, whose `src` tree `24004426` is byte-identical to the tree at
this branch head) and a candidate built from this branch against the same materialized fixture, with
the manifest's sanitized environment. The full stdout of each is one canonical JSON line; the members
that move are:

| member | Python oracle | Go candidate |
|---|---|---|
| `context.state` | `READY` | `OUT_OF_SCOPE` |
| `context.results` | one entry, `feature:a11y-high-contrast`, `score` 180, three evidence rows | `[]` |
| `context.coverage.authoritative_results` | `1` | `0` |
| `context.coverage.included_results` | `1` | `0` |
| `context.coverage.requested_results` | `1` | `0` |
| `context.coverage.critical` | `["feature:a11y-high-contrast"]` | `[]` |
| `context.coverage.packet_bytes` | `1720` | `838` |
| `context.verification` | `["go test ./pkg/...","make coverage-ratchet","make scenario-ledger STRICT=1","make gate"]` | `["make gate"]` |

Total stdout: oracle 2495 bytes, candidate 1613 bytes — the oracle's is the manifest's captured
expectation for this case, `stdoutSha256 c06551bbb896234da1e966bd0f381d63289456ca2f3990c87402d787fa9053ca`.
Every other byte of the
receipt — adapter, receipt id, repository identity, freshness, learning, exclusions, request digest —
is identical, which is why the divergence is expressible as a closed set of member rewrites.

The oracle's single result is the feature itself, admitted on the word `high` alone. The candidate
withdraws the packet because no emitted result rests on two words of the query as written:
`bake`, `sourdough`, `bread` and `altitude` match nothing in the fixture.

**`abstention.reason` is not observable at the CLI surface.** The receipt shaper
(`internal/contextindex/receipt.go`) reduces `abstention` to `{"active": …}` for every state except
`NEEDS_WIDENING`, and drops it entirely under repository intent. Both receipts above carry neither
`intent` nor `abstention`, which is that branch. So the internal reason `below-relevance-floor` — the
name `GPK-V0-039` gives the abstention —
never reaches stdout, and the parity case discriminates on `state` and the emptied packet instead.
That is the whole observable consequence of the clause on this surface; the reason is recorded here
so a future receipt change that surfaces it has a name to surface.

**Adjudication: `python-defect` (known-divergent, no repair).** `GPK-V0-039` is the ratified clause,
and it is the *reason* this is no longer agreed-wrong: with it in place the oracle contradicts binding
spec text while the candidate satisfies it. The clause requires a packet to be withdrawn — no
results, `state` `OUT_OF_SCOPE`, abstention reason `below-relevance-floor` — unless some single
emitted result rests on at least two words of the query as written, counted per result rather than
across the packet, in the query's own words rather than in derived terms, and with results carried in
behind another result contributing nothing. It also fixes what a withdrawn packet's other members
say: zero coverage counts, an empty critical set, and the verification plan a repository with no
cited result implies, which is the detected profile's gate alone.

`src/` is **NOT** repaired. It is scheduled for deletion under `GPK-V0-025`, it no longer supplies
the expectation, and repairing it would erase the recorded evidence that the two runtimes differ. Go
MUST NOT be regressed to reproduce the oracle's behavior.

**Why the corpus could not see it.** The 98-case corpus was green in both directions, and its green
said nothing here. No case issued a below-floor query: `harness-user-prompt`, the only case that
compiles a query packet, sends "how do I run the required workflow gates" against a fixture whose
`AGENTS.md` answers it in several words, so it clears the floor with room to spare and is
byte-identical before and after the change. Under `GPK-V0-034` a corpus that cannot discriminate a
known divergence must gain a case that can before the affected command is promoted.

**Discriminating case: `harness-user-prompt-out-of-scope`.** Argv is `harness-user-prompt`'s, with
stdin `{"task":"how do I bake sourdough bread at high altitude"}`. Fixture
`conformance/cli-parity-v0/fixtures/harness-out-of-scope` is `impact-go`'s shape with one change that
matters: the canonical id is `a11y-high-contrast`, a hyphenated id with a common English component,
so the one-word accident reproduces **hermetically** rather than depending on a downstream corpus.
That independence is load-bearing here in a way it was not for DR-0007: the commit that added the
floor also wrote the sample query into `internal/contextindex/eval_query.go`'s own doc comment, so
the Corvint repository can no longer reproduce the accident against itself — the query now has real
lexical support there, and the candidate correctly answers `READY`. Verified: against a clean
snapshot of this branch head the candidate returns one symbol result for that query, matched through
the comment that documents it.

Expected bytes are captured from the pinned Python oracle by `cli-parity-v0 capture`, never from the
candidate. The declaration is a `knownDivergence` with seven rewrites — one per member in the table
above, excluding `packet_bytes` — each naming what `GPK-V0-039` requires the candidate to emit and
what the oracle emits in its place, each required to occur exactly once, with the whole stdout
required to equal the oracle's after substitution.

**One mechanism fix the case forced.** `packet_bytes` is reconciled arithmetically rather than
declared, because no clause can author a byte count. The identity was "the two counts differ by
exactly what the declared rewrites account for" — which is wrong whenever a rewrite moves the count
across a digit boundary, because the count includes its own encoded member and the packet therefore
grows by that member's width too. DR-0007 never exposed it — its two counts have the same
width, which is why that case reconciles identically before and after this change; DR-0008 moves
1720 to 838 and was off by exactly one. `reconcilePacketBytes` now adds the member's own width
delta to the accounted side. The term is zero for equal-width counts, so no existing case moves, and
two tests pin both directions: a correctly withdrawn packet whose count crosses the boundary
reconciles, and one extra unaccounted byte in the same packet still fails.

`validKnownDivergence` (`conformance/cli-parity-v0/manifest.go`) pins this declaration to this case
ID, to `DR-0008` / `GPK-V0-039`, to `harness`, and to the exact candidate side of all seven
rewrites — the withdrawn forms the clause names, not bytes read off the candidate. Each oracle side
is required to name the same JSON member and to differ from the candidate side, so a rewrite can
only change one named member's value and can never introduce, drop, or no-op one.

**Proof that the case discriminates.** Replayed against a candidate built with the floor removed —
`evalStrongestSupport` stubbed to return a value that always clears — the case FAILS:

```
FAIL harness-user-prompt-out-of-scope known divergence: declared divergence
"\"authoritative_results\":0" occurs 0 times in candidate stdout
```

(one line, wrapped here.) The neutered candidate emits the oracle's bytes exactly — `READY`, one
result, `packet_bytes` 1720 — so the very first declared rewrite finds nothing to rewrite.

A candidate that stops withdrawing the packet therefore fails on the uniqueness check rather than
passing quietly, which is what keeps the declaration from becoming an exclusion.

**Parity effect.**

```
before: SUMMARY parity=98 accepted-divergences=3 known-divergences=1 location-normalizations=13 \
structural-fields=1 unsupported-refusals=4 full-gpk-v0-005=PARTIAL detached-descendants=NOT_RUN
after:  SUMMARY parity=99 accepted-divergences=3 known-divergences=2 location-normalizations=13 \
structural-fields=1 unsupported-refusals=4 full-gpk-v0-005=PARTIAL detached-descendants=NOT_RUN
```

`parity` 98 -> 99 is the new case and `known-divergences` 1 -> 2 is its declaration; every other
count is unchanged. Both are derived from the manifest by the runner, not edited. The full recapture
that produced the new row reproduced all 98 existing rows byte-identically (81 added lines, zero
removals), which is the evidence that no other expectation moved. The only hand edit to the manifest
outside the new case is the `harness` inventory reason, which now names the deliberately
known-divergent row; it is prose, not an expectation.

**Amendment 2026-09-02 (GPK-V0-045, decision 0036).** Both runtimes now keep `abstention.reason`
through budget compaction, so the withdrawn harness receipt carries
`"abstention":{"active":true,"reason":"below-relevance-floor"}` where the oracle's published one
carries `{"active":false,"reason":"none"}`. The declaration for `harness-user-prompt-out-of-scope`
therefore leads with that abstention rewrite exactly as `query-repository-out-of-scope` already
did (eight rewrites, +16 bytes accounted), and `validRelevanceFloorDivergence` pins it on both
wires. The manifest was re-captured at `b5707ee` from the repaired oracle.

### DR-0009 — `impact`: coverage is denominated in the truncated output, so `omitted_results` is structurally zero

- **Status:** LANDED — adjudicated `python-defect` / known-divergent under a newly ratified clause.
  `GPK-V0-040` (`docs/specs/go-production-kernel-migration-v0.md`) requires the denomination the
  candidate uses; the oracle measures its own narrowed output instead. The disagreement is
  **EXPECTED and MUST NOT be reported as a failure** (`GPK-V0-033`). `src/` is deliberately
  unrepaired.
- **Command:** `impact`, `range impact`, and every other surface that compiles a context receipt
  under a ranking ceiling or a packet budget. **Discovered:** 2026-08-29 by a code audit of the
  receipt shaper.

**Divergence class: agreed-wrong.** This is the class this register names at "Known limitation:
agreed-wrong cases", and the third entry to reach it. Go was ported by reading the Python, inherited
the same arithmetic, and both runtimes emitted the same wrong bytes from the same input at the same
revision — so nothing in the `GPK-V0-033` disagreement loop could surface it. It was found by
reading the shaper, not by running the corpus.

**The defect, in the oracle's own lines.** `src/context_corvint.py:64` opens `_receipt` with
`results = results[:limit]`, narrowing the ranked list before anything measures it. The receipt it
builds then reaches `compile_receipt`, which at `src/context_corvint_learning.py:319` takes
`requested_results = list(receipt["results"])` — the ALREADY-TRUNCATED list — and passes
`len(requested_results)` to `_set_coverage`. `_set_coverage` computes `omitted = requested -
len(included)` over that same narrowed list, so the subtraction is structurally zero and
`omitted_results` can never report a result the ceiling dropped. The count that names the universe,
`requested_results`, instead names the output. The narrowing happens twice and is measured twice,
which is why repairing one site alone would not have closed it.

**The Go fix.** `e8abeb4` captured the pre-truncation count in `receipt` and passed that to
`setCoverage`, and gave `compileReceipt` an `admittedResultCount` that recovers the admitted count
an earlier stage recorded rather than remeasuring the results in hand. It also removed
`RangeImpact`'s after-the-fact restatement of the same three fields, leaving `setCoverage` the
single writer. That commit reported `parity=98 ... known-divergences=1` unchanged, and correctly so:
no corpus case ranked past its limit, so no receipt in the corpus moved a byte.

**Observed bytes (2026-08-29, both runtimes, fixture `impact-truncated`, `impact pkg/sample.go
--limit 3`).** The full stdout of each is one canonical JSON line; the members that move are:

| member | Python oracle | Go candidate |
|---|---|---|
| `context.coverage.requested_results` | `3` | `7` |
| `context.coverage.included_results` | `3` | `3` |
| `context.coverage.omitted_results` | `0` | `4` |
| `context.coverage.uncertainty` | `[]` | `["4 ranked results omitted by packet budget"]` |
| `context.coverage.packet_bytes` | `2427` | `2470` |

**Amendment 2026-09-04 (DR-0024).** The candidate's uncertainty entry now reads "omitted by
result limit" on this null-budget request, one byte shorter: candidate `packet_bytes` 2469 and total
stdout 2524, 42 bytes accounted. The oracle side and every other member above are unchanged.

Total stdout: oracle 2482 bytes, candidate 2525 bytes (2524 after `DR-0024`) — the oracle's is the manifest's captured
expectation for this case, `stdoutSha256
55452553ebfad4af850078433f28873d15c6351dc30ec036aff0c857ddb33a26`. The two runtimes emit the same
three results in the same order with the same evidence; `included_results` agrees, because the
packet each publishes is identical. Only the denomination of the counts differs, which is why the
divergence is expressible as a closed set of member rewrites.

**Adjudication: `spec-gap` first, then `python-defect` (known-divergent, no repair).** The
classification order matters and was got wrong once. `e8abeb4` landed the candidate repair and
proposed `python-defect`, which skipped a step: at that moment no clause in any spec under
`docs/specs/` and no entry under `docs/decisions/` defined how a receipt's coverage counts are
denominated, so the observable was undetermined and **neither runtime won by default**. That is
`spec-gap`, and "the candidate is more correct" is never itself authority under `GPK-V0-033`. This
is the same ordering `impact-self-authored-adr` was held to: it was `spec-gap` until `CF-V0-031` was
ratified and only then became `python-defect`.

`GPK-V0-040` is now ratified, and it is what makes this a `python-defect`: the oracle contradicts
binding spec text while the candidate satisfies it. The clause requires coverage counts to be
denominated in the universe of results the ranking ADMITTED, never in the narrowed set the receipt
emits — `requested_results` names what ranking considered, `included_results` what the packet
carries, `omitted_results` the difference a ceiling or a budget removed — and requires a compilation
stage that narrows a receipt again to carry the earlier stage's admitted count forward rather than
remeasure. It also fixes the consequence a caller can act on: a nonzero omission must be named in
`uncertainty`, because a zero that was never measured is an assertion of completeness rather than a
count, and a consumer reading `omitted_results: 0` must be entitled to conclude that nothing was
dropped.

`src/` is **NOT** repaired. It is scheduled for deletion under `GPK-V0-025`, it no longer supplies
the expectation, and repairing it would erase the recorded evidence that the two runtimes differ. Go
MUST NOT be regressed to reproduce the oracle's behavior.

**Why the corpus could not see it.** The 99-case corpus was green in both directions before and
after `e8abeb4`, and its green said nothing here. No case ranked past its own limit: the two
`impact` rows both pass `--limit 10` against a fixture that admits four results, and every other
receipt-compiling row is likewise under its ceiling. With no truncation there is no omission to
count, the defective and the correct arithmetic return the same number, and the whole divergence is
latent and unexercised. Under `GPK-V0-034` a corpus that cannot discriminate a known divergence must
gain a case that can before the affected command is promoted.

**Discriminating case: `impact-ranked-past-limit`.** Argv is `impact pkg/sample.go --limit 3`.
Fixture `conformance/cli-parity-v0/fixtures/impact-truncated` is `impact-go`'s shape plus three
further same-package tests, `pkg/alpha_test.go`, `pkg/beta_test.go` and `pkg/gamma_test.go`, which
carry no markers and exist only to be ranked. A change to `pkg/sample.go` therefore admits seven
results — the changed path, the marked and conventional same-package test, the feature and scenario
it composes, and the three plain tests — and `--limit 3` ranks four of them out. It has its own
fixture rather than borrowing `impact-go`'s, because `impact-go` exists to serve an unqualified
byte-exact row at `--limit 10` and may legitimately change shape; the truncation this case turns on
must not depend on that.

Expected bytes are captured from the pinned Python oracle by `cli-parity-v0 capture`, never from the
candidate. The declaration is a `knownDivergence` with three rewrites — the omission count, the
admitted count, and the uncertainty entry the omission produces — each naming what `GPK-V0-040`
requires the candidate to emit and what the oracle emits in its place, each required to occur
exactly once, with the whole stdout required to equal the oracle's after substitution.
`packet_bytes` is not declared: it is reconciled arithmetically, and the two counts differ by
exactly the 43 bytes the uncertainty entry adds, with a zero width term because both counts are four
digits wide.

`validAdmittedUniverseDivergence` (`conformance/cli-parity-v0/manifest.go`) pins this declaration to
this case ID, to `DR-0009` / `GPK-V0-040`, to `impact`, and to the shape of all three rewrites. No
clause can author a fixture's own result count, so no number in the declaration is transcribed from
the candidate: the omission count is read out of the first rewrite, and the rest must satisfy the
identity the clause states — the oracle's `requested_results` IS its included count, the candidate's
must exceed it by exactly the dropped results, and the uncertainty entry must name that same number
in the reason text an omission carries. A declaration whose identity does not close is refused as
invalid rather than admitted as a divergence.

**Proof that the case discriminates.** Replayed against a candidate built from this branch head with
`e8abeb4`'s two source hunks reverted — `receipt` measuring the truncated slice again and
`compileReceipt` remeasuring the results in hand — the case FAILS:

```
FAIL impact-ranked-past-limit known divergence: declared divergence
"\"omitted_results\":4" occurs 0 times in candidate stdout
```

(one line, wrapped here.) The reverted candidate emits the oracle's bytes exactly — `requested_results`
3, `omitted_results` 0, an empty `uncertainty`, `packet_bytes` 2427 — so the first declared rewrite
finds nothing to rewrite. The same replay against the unreverted binary from the same tree passes as
`PASS-WITH-KNOWN-DIVERGENCE`.

A candidate that stops denominating coverage in the admitted universe therefore fails on the
uniqueness check rather than passing quietly, which is what keeps the declaration from becoming an
exclusion.

**Parity effect.**

```
before: SUMMARY parity=99 accepted-divergences=3 known-divergences=2 location-normalizations=13 \
structural-fields=1 unsupported-refusals=4 full-gpk-v0-005=PARTIAL detached-descendants=NOT_RUN
after:  SUMMARY parity=100 accepted-divergences=3 known-divergences=3 location-normalizations=13 \
structural-fields=1 unsupported-refusals=4 full-gpk-v0-005=PARTIAL detached-descendants=NOT_RUN
```

`parity` 99 -> 100 is the new case and `known-divergences` 2 -> 3 is its declaration; every other
count is unchanged. Both are derived from the manifest by the runner, not edited. The full recapture
that produced the new row reproduced all 99 existing rows byte-identically (56 added lines, one
removed), which is the evidence that no other expectation moved. The single removed line is the
`impact` inventory reason, which now names the second deliberately known-divergent row; it is prose,
not an expectation.


**First real-corpus instance (2026-08-30).** The synthetic `impact-truncated` fixture is no longer
the only exercise of this clause. On the beamfall corpus pinned by `conformance/perf-v0`
(`fa3b1e7f`, tree `3a4251e0`), `impact internal/graph/graph.go --limit 10` admits **205** results and
a `--limit 10` ceiling ranks **195** of them out:

| member | Python oracle | Go candidate |
|---|---|---|
| `context.coverage.requested_results` | `10` | `205` |
| `context.coverage.included_results` | `10` | `10` |
| `context.coverage.omitted_results` | `0` | `195` |
| `context.coverage.uncertainty` | `[]` | `["195 ranked results omitted by packet budget"]` |
| `context.coverage.packet_bytes` | `12682` | `12730` |

The oracle's `requested_results` tracks its own included count exactly at every ceiling — `10` at
`--limit 10`, `50` at `--limit 50` — which is the structural-zero signature this entry names, while
the candidate reports the same admitted universe of `205` at both. The `packet_bytes` identity
closes as this entry requires: 48 bytes, being the 45-byte quoted uncertainty entry plus the three
digits the two widened counts add. Total stdout 12737 (oracle) vs 12785 (candidate).

This instance is 20.5x the ceiling rather than the fixture's 2.3x, so it is the stronger evidence
that `omitted_results: 0` is an assertion of completeness the oracle never measured. It is
**not** a second divergence and needs no register entry of its own: same command, same mechanism,
same closed three-member rewrite, already adjudicated here.

**Not present in the 2026-08-29 perf run.** `conformance/perf-v0/results/corvint-beamfall-2026-08-29`
pinned the candidate at `58a2806e`, which predates this entry's Go repair, so at that revision the
candidate reproduced the oracle's structural zero and this divergence did not fire. The
`unequal-stdout` recorded there for `beamfall-impact-path` was DR-0004's evidence ordering alone.
At `integration/agpl` head both fire at that one invocation, which is why a `perf-v0` grant for
that task cannot be satisfied by naming a single register entry — see
`docs/agent-memory/questions.md`.

**Perf grant (2026-09-03).** `docs/decisions/0040-perf-grants-dr0004-dr0009-2026-09-03.md` ratifies
measuring `conformance/perf-v0` task `beamfall-impact-path` (`impact internal/graph/graph.go
--limit 10`) across this entry, relaxing `unequal-stdout` alone. The grant is pinned to that exact
invocation and does not travel.

**Perf grant (2026-09-04).** `docs/decisions/0044-perf-grants-dr0009-beamfall-harness-and-snapshot-2026-09-04.md`
ratifies three more invocations across this entry, each diagnosed member by member with
`perf-v0 diagnose` and each relaxing `unequal-stdout` alone: `beamfall-harness-file-change`,
`beamfall-harness-file-change-snapshot-present` (both `harness event … --event file-change`), and
`beamfall-impact-path-snapshot-present` (`impact internal/graph/graph.go --limit 10`). The
snapshot-present impact receipt is byte-identical to the cold one on both runtimes.

### DR-0010 — `lrf --ocm`: a `.py` claim is verified against the closed Go grammar instead of `python-ast/1`

- **Status:** LANDED — adjudicated **`go-defect`**, repaired in Go under a newly ratified clause.
  `GPK-V0-014` admits exactly two treatments of a Python claim and names the candidate's third one
  as a parity failure; `GPK-V0-041` (`docs/specs/go-production-kernel-migration-v0.md`) names which
  of the two admitted treatments this path takes. `src/` is untouched, as it is for every outcome:
  the oracle is right here, and under `go-defect` it also stays the only independent signal against
  a candidate defect.
- **Command:** `lrf` (the `--ocm` leg). **Discovered:** 2026-08-28 while porting OCM; adjudicated
  2026-08-29.

**Divergence.** For an OCM map carrying a `.py` claim, the two runtimes disagree on what counts as
valid Python, because they consult different grammars.

- Python re-extracts claims with `ast.parse` under the host's frozen Python 3.9 grammar.
  `_python_candidates` (`src/context_corvint_claims.py:351-387`) returns `[]` on `SyntaxError`, so a
  blob CPython rejects offers no claims and `src/context_corvint_ocm.py:745` raises
  `claim-not-reextractable`.
- Go consults `pythonsyntax.SourceSyntaxValid` through `pythonSyntaxValid`
  (`internal/lrfrepo/ocm.go:897`), reached from `verifyClaims` (`:698`) and `claimExtractableIn`
  (`:794`). That is the analyzer's deliberately closed fact-extraction subset, not `python-ast/1`.
  It is wrong in both directions: it accepts sources CPython rejects, and it conservatively rejects
  valid Python outside the fact profile.

**Adjudication: `go-defect`, and the ordering matters.** The first question is not which runtime
looks better; it is whether a clause governs the observable. One does, and it governs it by name:

- `docs/specs/go-production-kernel-migration-v0.md:205-208` (`GPK-V0-014`) — "Before any TCQ-, OCM-,
  claim-, or frontier-consuming command moves to Go, its parser MUST implement the exact frozen
  `python-ast/1` Python 3.9 grammar and byte-offset behavior or abstain exactly where the accepted
  TCQ contract requires. Calling an installed Python, **accepting the Go host's idea of Python
  syntax, or silently narrowing Python support fails parity**."

`lrf` is an OCM- and claim-consuming command that has moved to Go, and its parser was neither of the
two admitted things. The clause names the candidate's actual behavior — a host grammar, narrowed —
as the failure. The oracle satisfies the clause by construction: `python-ast/1` is frozen to the
Python 3.9 `exec`-mode grammar (`docs/specs/test-claim-qualification-v0.md:171`), and the pinned
oracle runtime is Python 3.9.6. So the spec decides, and it decides against the candidate. Nothing
here rests on preferring a runtime, and `src/` is not repaired under any outcome.

**Why not `spec-gap`.** It would have been, had `GPK-V0-014` not existed — and the register's own
worked example (`impact-self-authored-adr`) stayed parked as `spec-gap` until `CF-V0-031` was
ratified, precisely because "the candidate is more correct" is never itself authority. Here the
clause predates the divergence and disposes of it, so the outcome is available immediately. The
ratification `GPK-V0-041` performs is narrower: `GPK-V0-014` offers two remedies with an `or`, and
the OCM claim path had no named abstention point of its own. `GPK-V0-041` names it — the same typed
`unsupported-ocm-python-claims` refusal `GPK-V0-037` already requires of `ocm status|verify|report`,
on the same stated condition, "until an exact OCM-owned Python grammar exists". The clause selects
between remedies the spec already permits; it does not create the adjudication.

**Confirmed by execution (2026-08-29), function level.** On the blob

```
from pkg.app.app import VALUE


def test_constructor_selection():
    """HTTP-001: the second-generation server constructor must be used."""
    assert VALUE == 2


LIMIT = +/ 2
```

`ast.parse` raises `SyntaxError: invalid syntax`; `pythonSyntaxValid("x.py", blob)` returns `true`.
The same split reproduces for `x = 1 {2}` and `return (,)`. The reverse direction — valid Python the
closed grammar rejects — is recorded in `docs/agent-memory/bugs.md`; one direction is enough to
adjudicate, and the abstention closes both.

**Confirmed by execution (2026-08-29), CLI level.** Scratch repository `pydiv`, that blob committed
at both revisions, with an OCM map linking its docstring claim (identity computed with the oracle's
own `claim_id`, so the input is not candidate-authored), `lrf --cem .corvint/change.cem.json --ocm
.corvint/change.ocm.json --expected-base de9f167a --target 6f1029ac`:

| runtime | exit | stdout | stderr |
|---|---|---|---|
| Python oracle | `2` | empty | `{"code": "claim-not-reextractable", "error": "claim cannot be re-extracted at target", "ok": false}` |
| Go candidate (pre-repair) | `0` | full LRF evaluation, three issues and two results | empty |

The candidate did not merely differ in a field: it published a complete evaluation of a claim the
oracle refused to re-extract at all.

**The Go repair.** `verifyOptionalOCM` (`internal/lrfrepo/ocm.go:106`) now passes
`rejectPythonClaims: true` to `verifyOCM`, so the `lrf` OCM leg reaches the same categorical
`.py` refusal in `verifyOCMClosure` (`:459`) that `ocm status|verify|report` reaches through
`ocm_read.go:133`. One line, and it deletes an entire divergent surface rather than narrowing it.
The refusal is deliberately categorical rather than per blob: a candidate cannot decide whether its
own approximation is close enough on exactly the inputs where the approximation is wrong.

**What the repair costs, stated plainly.** An invocation the oracle evaluates is now refused. On
fixture `lrf-ocm-python` — a valid Python claim, linked by the oracle's own `ocm link` — the
pre-repair candidate's 1370 stdout bytes were **byte-identical to the oracle's**, and the repaired
candidate returns exit 2 with empty stdout and a 154-byte typed refusal. That is not a regression
being hidden: `GPK-V0-014` does not admit "agrees on the blobs we tried" as parity when the grammar
underneath is a different grammar, and `GPK-V0-037` already paid this exact cost for the read slice.
The `lrf` `.py`-claim profile is `UNSUPPORTED`; the corpus records it as a refusal, never as parity.

**Discriminating evidence** (`GPK-V0-034`): the refusal row `lrf-ocm-python-claim-refusal`, argv
pinned in `validLRFPythonClaimRefusalArgv`. Against a candidate built from this head with the single
`rejectPythonClaims` argument flipped back to `false`, and resolved from `PATH` as `corvint` the
way the runner resolves it, the row FAILS with `not a bounded nonzero typed refusal`; against the
repaired binary it reports `UNSUPPORTED lrf-ocm-python-claim-refusal
code=unsupported-ocm-python-claims`.

**Split from this landed entry by decision 0012.** The different Frontier/TCQ surface and contract
is now DR-0014. DR-0010 remains scoped to the landed `lrf --ocm` categorical refusal and carries no
open Frontier status.

**Corpus counts.**

```
before: SUMMARY parity=100 accepted-divergences=3 known-divergences=3 location-normalizations=13 \
structural-fields=1 unsupported-refusals=4 full-gpk-v0-005=PARTIAL detached-descendants=NOT_RUN
after:  SUMMARY parity=100 accepted-divergences=3 known-divergences=3 location-normalizations=13 \
structural-fields=1 unsupported-refusals=5 full-gpk-v0-005=PARTIAL detached-descendants=NOT_RUN
```

`unsupported-refusals` 4 -> 5 is the new row. `parity` does not move, and neither does any
divergence or normalization count: a `go-defect` is repaired, not declared, so it earns a refusal
rather than a `knownDivergence`. Expected bytes were not authored for it at all — a refusal is
candidate-only evidence — and the full oracle recapture that accompanied it reproduced all 100
existing rows byte-identically.

### DR-0011 — index import table: the oracle records a JavaScript comment, string, or template literal body as a module specifier

- **Status:** LANDED — adjudicated **`python-defect`** / known-divergent, and still **latent**: the
  two runtimes disagree about the contents of `Index.Imports` for web sources, but no measured CLI
  invocation renders that disagreement and **no parity case discriminates it**. The corpus gained
  `.ts` sources on 2026-08-29 with the `impact-web` fixture, which the `GPK-V0-027` widening
  required; every specifier in it is a live `import`/`export` statement, so the oracle's positional
  rule and Go's lexer agree edge for edge and all four web and Python cases replay byte-exact. The
  entry is kept because that agreement is a property of one hand-authored fixture, not of the two
  rules: it takes one commented-out or quoted specifier to separate them, and this register's
  opening failure mode is reading a corpus that cannot discriminate a divergence as evidence that
  none exists.
- **Command:** any command that compiles the full index — `impact`, `harness event --event
  file-change`, `feature`. **Discovered:** 2026-08-29, by corpus measurement.

**Divergence.** For the six web suffixes the oracle itself names (`src/context_corvint.py:100`), the
oracle's import fallback matches `from` or `import` followed by a quoted run **anywhere in the
bytes**, with no lexical state. Go now lexes the source first, so a specifier that appears inside a
`//` or `/* */` comment, inside a `'`/`"` string literal, or inside a template literal is not an
edge.

Measured over three real corpora (`git ls-files`, tracked sources only):

| corpus | files | edges: oracle rule | edges: Go lexer | removed | added |
|---|---|---|---|---|---|
| Beamfall workspace `.js`/`.jsx`/`.ts`/`.tsx` | 980 | 5,281 | 5,257 | 24 | 0 |
| Beamfall workspace `.mjs`/`.cjs` | 61 | 267 | 264 | 3 | 0 |
| this repository, all six suffixes | 33 | 90 | 90 | 0 | 0 |

All 27 removed edges were opened and confirmed fabricated. Four are representative:

- `beamfall-design-system/_from-app/components/Screens/_shared/image-slot.js:70`, a comment reading
  `tell "never set" from "just deleted"`, recorded as a module named `just deleted`.
- `beamfall-web/test/routes.test.ts:167`, `spawnSync(process.execPath, ['--import', 'tsx', …])`,
  where the match opened on the word `import` inside one string literal and closed on the opening
  quote of the next, recording the `, ` between them as a module name.
- `beamfall-web/test/tizen-avplay-smoke.ts:76`, the `import { defineConfig } from 'vite'` line of a
  vite config this file *generates* inside a multi-line template literal.
- `beamfall/internal/web/app/src/areas/enroll/index.ts:5`, a comment showing consumers how to
  import the module, recorded as the module importing itself.

**Why it is latent.** `reverseImporters` (`internal/contextindex/impact.go`) matches an edge against
`index.Module + "/" + path.Dir(changedPath)`, always a slash-qualified Go import path. No removed
edge has that shape — none contains a `host.tld/` prefix at all — so no `impact`, `file-change`, or
`feature` result moved on either corpus. `Index.ApproximateImports` is written but read by no
receipt shaper, so it contributes no bytes either. Latent is not absent: an edge fabricated from a
comment that happens to quote a Go import path would be observable, and nothing prevents one.

**Adjudication: `python-defect`.** Two spec-side citations, neither of them "the candidate is more
correct":

- `docs/specs/typescript-javascript-affected-adapter-v0.md:46-49` (`TJAA-V0-005`) makes the edge set
  for these six suffixes "static relative `import`, re-export, and `require` edges", and requires
  unparsed source to raise a deterministic frontier rather than silently shorten the set. Text
  inside a comment or a string literal is not a static import under any reading of that clause, and
  a paragraph of program text captured across two literals is not a module specifier under any.
- The oracle contradicts **itself**. It already carries `_strip_web_noncode`
  (`src/context_corvint.py:107`) — a comment/string/template state machine — as its model of what
  counts as code in a web source, and applies it before extracting web import *bindings*
  (`src/context_corvint.py:168`). `_extract_imports` (`src/context_corvint_index.py:1213-1217`) simply
  does not use it. The oracle therefore holds both positions at once; the spec-consistent one is
  the one it applies everywhere else.

**Citation audit (2026-09-12): the first citation is weak.** `TJAA-V0-005` is not an accepted
clause that governs this observable. Its spec is intent `proposed`
(`docs/specs/typescript-javascript-affected-adapter-v0.md:5`), and it governs the affected-test
adapter under `internal/liveverify/affected/typescript`, not the `contextindex` import table. The
accepted clause that does govern web edges, `GPK-V0-027`(c)
(`docs/specs/go-production-kernel-migration-v0.md:213-215`), names "relative specifiers" and the
configured alias prefix but states no lexical rule for comments, string literals, or template
literals. The second citation is the oracle's internal inconsistency, which is not spec text. The Go
lexer (`378c33e7`, 2026-08-29 21:40) also landed before decision 0007's ratification commit
(`866bba88`, 22:19), so the lexer was not written against a ratified clause. On spec text alone, whether a quoted specifier inside a comment or literal is an
edge is undetermined, so this entry reads as `spec-gap` for that byte region. The outcome is left
at `python-defect` rather than reopened, and the reason is stated here rather than applied silently:
an OPEN `spec-gap` would block `PASS` for `impact`, `harness event --event file-change`, and
`feature`, and the retired oracle (decision 0088) means no discriminating case could ever close it.
The disagreement is latent (no row renders it), so leaving it at `python-defect` changes no
expectation. The repair is an owning-spec amendment of `GPK-V0-027`(c) that states the lexical
rule.

**Citation audit closed (2026-09-12).** Decision 0101 amends `GPK-V0-027`(c)
(`docs/specs/go-production-kernel-migration-v0.md`) with that rule: a specifier is read only from a
`'`- or `"`-quoted literal in live code after an import/export clause's `from`, directly after
`import`, or as the literal argument of `import(`; comment, string, and template-literal bodies
contribute no edge, `require(...)` is unrecognized, and an unterminated block comment or template
literal records `WEB_SOURCE_UNPARSED`. The `python-defect` adjudication now rests on accepted spec
text rather than on `TJAA-V0-005` and the oracle's internal inconsistency.

`src/` is deliberately unrepaired, per `GPK-V0-033`.

**Candidate behaviour.** `webImports` (`internal/contextindex/webimports.go:238`) lexes the source
into words, punctuation and completed string literals, and reads a specifier from exactly the three
positions the oracle regex targeted: a quoted string after `from`, directly after `import`, and
inside `import(`. It recognises nothing new — `require()` is deliberately still unrecognised, so
that no new edge rides in a change whose claim is that it only removes fabricated ones. Every other
extension keeps the oracle-mirroring scanner unchanged, which
`TestSourceImportsKeepsTheScannerOffTheWebPath` pins.

A dynamic `import("./literal.js")` remains an edge. `TJAA-V0-005` distinguishes a static specifier
from "computed loading", and a string-literal argument is the former: `integrations/opencode/src/
index.js:272` names a file that exists in this repository. Dropping it would be an
under-approximation, the one direction `goImports` (`internal/contextindex/parse.go`) documents as
unsound for the impact closure. `import(expression)` names no module and contributes none, as
before.

**Countability.** A file that ends inside an unterminated block comment or template literal had its
tail consumed as literal text, so its edge set is short. That is reported through the existing
channel as `Unparsed{Facts: "imports", Reason: WEB_SOURCE_UNPARSED}`, which reaches the receipt's
`unparsed` block, rather than returning the short set silently. Zero of the 1,074 measured files
took that branch.

**What would create a discriminating case.** A `cli-parity-v0` fixture with one web source whose
comment or string body quotes a specifier that resolves onto the changed path, so that `impact`
compiles a reverse-import closure over an edge only one runtime has. `impact-web` is now such a
fixture in every respect except that one: its specifiers are all real statements, deliberately, so
that the widening's own parity evidence is not entangled with this divergence. Adding the quoted
specifier is a separate change, because it converts four byte-exact rows into declared divergences.


### DR-0012 — `impact` web rule: the oracle resolves the `@/` alias into one named repository's source root from every repository

- **Status:** LANDED — adjudicated **`python-defect`** / known-divergent under a newly ratified
  clause, and **latent**: the disagreement needs a repository that both contains
  `internal/web/app/src/**` and is NOT recognised as the `beamfall` profile, and no such repository
  exists in the corpus or in either measured product tree.
- **Command:** `impact`, and `harness event --event file-change` through the same kernel.
  **Raised:** 2026-08-29 while porting the `GPK-V0-027` web resolution rule.

**Divergence.** `_web_import_target` (`src/context_corvint_index.py:1518-1520`) resolves an aliased
specifier with a string literal:

    target = ("internal/web/app/src/" + imported[2:] if imported.startswith("@/") else ...)

Neither the alias `@/` nor the root it expands to is read from anything about the repository under
analysis. A repository that declares a different alias root — or no alias at all — has `@/x` resolved
into `internal/web/app/src/x` regardless, and a reverse-import edge is reported whenever a file
happens to sit at that path.

**Spec authority.** `GPK-V0-027`, as amended by decision 0007 (D2), names the web rule as
"resolution of relative specifiers and of the configured alias prefix, **where that prefix MUST be
read from the project profile and MUST NOT be hardcoded to any one repository's source root**". That
sentence exists because the oracle's behaviour is what it forbids: the clause was written after the
hardcode was found, and names it.

**Adjudication: `python-defect` (known-divergent, no repair).** `src/` is frozen under `GPK-V0-033`
and is deliberately NOT repaired. The candidate reads the pair from `internal/projectprofile`:
`Profile.WebAliasPrefix` and `Profile.WebAliasRoot`, carried by the `beamfall` row and left empty by
the fallback. A profile with no configured alias admits no aliased specifier, which is the
fail-closed direction the same clause requires of an unruled suffix.

**Observability.** For the `beamfall` profile the two runtimes are identical by construction, since
that row carries exactly the oracle's literal. For any other profile they differ only where
`internal/web/app/src/` exists anyway; where it does not, the oracle resolves to a path no source
occupies and reports the same empty result the candidate does. Measured: on this repository at
`ed2fca2c` all four tracked `extensions/vscode/src/*.ts` reverse-import receipts are byte-equal, and
on the `impact-web` fixture — which does carry `internal/web/app/src/**` *and* the profile signals —
so are both web cases.

**What would create a discriminating case.** A `cli-parity-v0` fixture holding
`internal/web/app/src/**` and an `@/` specifier but omitting `testing/features.yaml` and
`testing/scenarios.yaml`, so the fallback profile is detected. That case would be a
`knownDivergence` row whose rewrites are a whole result element rather than a field value, which is
a shape `validKnownDivergence` (`conformance/cli-parity-v0/manifest.go`) does not yet admit;
authoring it is a corpus change, not a kernel one, and is deliberately not folded into the widening.
It is held instead by `TestImpactWebAliasComesFromTheProjectProfile`
(`internal/contextindex/impact_reverse_import_rules_test.go`), which runs the same tree under both
profiles and asserts that the aliased specifier resolves under one and not the other while the
relative specifiers resolve under both.

### DR-0014 — `frontier`/TCQ verifies `.py` claims against the closed Go grammar

- **Status:** CLOSED 2026-09-04 — adjudicated **`go-defect`**, remedy landed and measured.
  Decision 0012 rejects categorical universe refusal; the required remedy is the edge-local
  `unsupported-python-grammar` abstention in `TCQ-V0-047`. The Frontier-surface abstention is
  implemented and pinned by `TestTCQV0047PythonGrammarAbstainsPerEdge` and its siblings
  (`internal/frontierrepo/frontierrepo_test.go:539`), and the corpus discriminates both
  directions: `python-grammar-edge-abstains` for the over-permissive half and
  `python-crlf-edge-processes` for the under-permissive half. Residual 1 was fixed in
  `internal/tcq`; residual 2 was measured unreachable rather than assumed. No item remains.
- **Command:** `frontier`, `tcq` — the `internal/frontierrepo` surface. **Split:** 2026-09-01 from
  DR-0010 by repository-owner decision 0012.

**Divergence.** `VerifyUniverse` (`internal/lrfrepo/universe.go:92`) still passes
`rejectPythonClaims: false`, so Frontier/TCQ claim verification can reach the closed
`pythongrammar.SourceSyntaxValid` approximation. That accepts some source rejected by
`python-ast/1` and rejects valid Python outside its fact profile; the mechanism and function-level
probes are recorded in DR-0010.

**Adjudication.** `GPK-V0-041` scopes command-level `unsupported-ocm-python-claims` refusal to OCM
claim verification and explicitly assigns this surface to `TCQ-V0-047`. `TCQ-V0-047` requires an
affected Python edge to abstain as `unsupported-python-grammar` while Go processing remains
available. Publishing a qualification through the approximate host grammar is therefore a
`go-defect`; refusing the whole universe would also violate the accepted edge-local contract.

**Corpus evidence (2026-09-04).** `conformance/frontier-v0` fixture `python-grammar-edge-abstains`,
case `python-post-3.9-edge-abstains-beside-a-go-edge`, builds a real repository whose OCM binds two
linked obligations: `CF-V0-001` to a docstring claim in `tests/test_claims.py`, a blob whose trailing
statement is a Python 3.10 `match`, and `CF-V0-002` to an ordinary Go table case in
`tests/claims_test.go`. The Go host's own Python grammar accepts that blob —
`pythonsyntax.SourceSyntaxValid` returns true for it, measured — so the shared verification admits
the claim and the approximate-grammar path is exactly what the case exercises. The required
projection is four items: the Python edge's `INTENT_TEST` carries `TEST_CLAIM_UNSUPPORTED` /
`PROFILE_REQUIRED` / `DEFINE_SUPPORTED_PROFILE`, and the Go edge's carries `TEST_NOT_MATCHED` /
`ACTIONABLE` / `SUPPLY_TEST_OBSERVATION`, so an abstention that suppressed unrelated Go processing
would fail the case as loudly as a published qualification.

**Discrimination (measured 2026-09-04).** With `isPost39Grammar` (`internal/tcq/python.go:123`)
neutered locally, the same universe re-runs and the case fails on three members of item 2:
`reasons` `[TEST_NOT_MATCHED]` for `[TEST_CLAIM_UNSUPPORTED]`, `resolutionClass` `ACTIONABLE` for
`PROFILE_REQUIRED`, `nextAction` `SUPPLY_TEST_OBSERVATION` for `DEFINE_SUPPORTED_PROFILE`. Without
the frozen-grammar refusal the Python edge associates to a unit and publishes an ordinary
qualification derived through the host grammar, which is the `go-defect` this entry names. The local
change was reverted and is not in the tree. Before this fixture the corpus carried
`unsupported-python-grammar` only as a declared TCQ diagnostic in
`intent-test-reason-union-and-priority`, whose case skips under `tcq-diagnostic-set`, so nothing
measured it.

**Residual 1 — CLOSED 2026-09-04.** The per-edge reason was `unsupported-anchor-profile` because
`analyzePython` returned an empty analysis on a grammar refusal, so `associate` exited before it
read `blob.failure`. `internal/tcq/python.go` now retains the anchor candidates through a grammar
refusal exactly as the offset path does, so `blob.failure` and `association.reason` are both
`unsupported-python-grammar` while the units stay withheld and sibling Go edges keep ordinary
processing. Pinned by `internal/tcq/analysis_test.go` and the Frontier seam tests; no wire golden or
conformance fixture moved.

**Residual 2 — NOT A DEFECT, measured 2026-09-04.** The universe-level refusal it described is
unreachable: `verifyUniverseClaims` (`internal/lrfrepo/universe.go:167`) decides extractability from
claim shape alone and never consults the approximate grammar, and every caller of
`verifyOCMClosure` passes `rejectPythonClaims: true` (`ocm.go:116`, `ocm_read.go:129`), so the
Frontier leg does not reach `verifyClaims` at all. The under-permissive input the residual needs is
real and constructible (`SourceSyntaxValid` rejects any CR byte, which `python-ast/1` and CPython
both accept) and is already pinned to ordinary edge processing by
`TestTCQV0047DoesNotUseTheOCMGrammarAsAProfileGate`
(`internal/frontierrepo/frontierrepo_test.go:609`) over `pythonCRLFDocument`.

**Closure evidence (2026-09-04).** The second direction now has its corpus case.
`conformance/frontier-v0` fixture `python-crlf-edge-processes`, case
`python-crlf-edge-processes-beside-a-go-edge`, binds a CRLF Python claim edge — a blob
`python-ast/1` and CPython accept and `pythonsyntax.SourceSyntaxValid` rejects on its CR guard —
beside an ordinary Go edge, and requires ordinary processing for both (`TEST_NOT_MATCHED` /
`ACTIONABLE` / `SUPPLY_TEST_OBSERVATION`), so neither a universe refusal nor a blanket abstention
can pass it. Discrimination was measured by forcing the CRLF blob down the abstention path
locally: the case failed on `reasons` (`TEST_CLAIM_UNSUPPORTED` for `TEST_NOT_MATCHED`),
`resolutionClass` (`PROFILE_REQUIRED` for `ACTIONABLE`) and `nextAction`
(`DEFINE_SUPPORTED_PROFILE` for `SUPPLY_TEST_OBSERVATION`); the local change was reverted and is
not in the tree. This entry no longer blocks Frontier/TCQ `PASS`.

### DR-0015 — `query`: a `vN` task token empties the symbol universe of an unversioned tree

- **Status:** LANDED — adjudicated `python-defect` / known-divergent under an amended clause.
  `GPK-V0-043` (`docs/specs/go-production-kernel-migration-v0.md`, decision 0017) requires a
  `vN` token to narrow the symbol universe only where some symbol path carries a version term; the
  oracle narrows unconditionally. The disagreement is **EXPECTED and MUST NOT be reported as a
  failure** (`GPK-V0-033`). `src/` is deliberately unrepaired.
- **Command:** `query` under `repository` intent, and every other surface that ranks symbols
  through `EvalQuery`. **Discovered:** 2026-09-01 by the Agent Retrieval Bench abstention probe
  (`docs/agent-memory/bugs.md`, since removed).

**Divergence class: agreed-wrong.** Both runtimes carried the same filter
(`src/context_corvint.py` symbol loop, `version_terms`; `internal/contextindex/eval_query.go`
`evalRankSymbols`), so no corpus case could disagree. It was found by reading the bench's
per-sample abstention reasons: 9 of Corvint's 12 abstentions on `v2_abstention` and all 3 on
`v2_code2test` were `no-relevant-candidates` on tasks whose only version vocabulary was a `v18`,
`v20`, or `v3` from an issue's "Versions" section, against repositories with no versioned path.

**Observed bytes (2026-09-01, both runtimes, fixture `query-version-token`).** The fixture is one
Go module with `internal/auth/session.go` declaring `EnforceSessionRevocation` and no ledger; the
task is `enforce session revocation v18`. The members that move:

| member | Python oracle | Go candidate |
|---|---|---|
| `context.abstention` | `{"active":true,"reason":"no-relevant-candidates"}` | `{"active":false,"reason":"none"}` |
| `context.state` | `OUT_OF_SCOPE` | `READY` |
| `context.results` | `[]` | one `symbol` entry, `internal/auth/session.go:EnforceSessionRevocation`, `score` 464 |
| `context.coverage.authoritative_results` | `0` | `1` |
| `context.coverage.included_results` | `0` | `1` |
| `context.coverage.requested_results` | `0` | `1` |
| `context.verification` | `["git diff --check"]` | `["go test ./internal/auth/...","make gate"]` |

`packet_bytes` moves with them and is reconciled by the runner's width identity, not transcribed.
Every other byte — revision, freshness, learning, intent, request — is identical.

**Adjudication: `python-defect` (known-divergent, no repair).** A version token is a path
narrowing; with nothing to narrow by it carries no information about which symbol answers the
task, and skipping every symbol on it is an invented certainty that the repository does not answer
(product invariant 2). Decision 0017 amends `GPK-V0-043` to say so, and the candidate satisfies
the amended text. Where a versioned path exists the two runtimes still agree. `src/` is **NOT**
repaired: it is scheduled for deletion under `GPK-V0-025` and no longer supplies the expectation.

**Discriminating case:** `query-version-token` (`knownDivergence`, seven rewrites pinned in
`validVersionTokenDivergence`). Before it, every query fixture was unversioned but no task carried a
`vN` token, so the unconditional and the conditional filter were indistinguishable.

**Record repair (2026-09-12).** The two amendments below were written on 2026-09-02 but placed
under `## Adjudicated and closed`, where later entries separated them from this entry, so DR-0015
appeared to lack the `GPK-V0-046` rewrite that `conformance/cli-parity-v0/manifest.json` declares
for `query-version-token`. They are moved here unchanged. The observed-bytes table above is the
2026-09-01 capture. The seven rewrites currently declared against `DR-0015` / `GPK-V0-043` are:
`abstention`, `included_results`, `requested_results`, `uncertainty`, `results`, `state`, and
`verification`. `authoritative_results` is no longer a rewrite, and the verification plan's
candidate side ends in `git diff --check`, not `make gate`.

*Provenance of the uncertainty line.* `GPK-V0-046`
(`docs/specs/go-production-kernel-migration-v0.md:455-465`) requires "exactly one deterministic
`coverage.uncertainty` entry naming that condition" and gives no wording. The exact string was
authored in `b5707eee` (decision 0036), which added it to both runtimes at once, as
`SyntaxOnlyUncertainty` in Go and `SYNTAX_ONLY_UNCERTAINTY` in the oracle, under the clause's
identical-repair sentence. The oracle side of this rewrite is `[]` because the oracle abstains on
this fixture and so never reaches the syntax-only condition, not because it lacks the line.

**Amendment 2026-09-02 (GPK-V0-046, decision 0036).** Every result the candidate answers with here
rests on `authority: syntax` alone, so the candidate now counts
`"authoritative_results":0`, the same zero the abstaining oracle reaches, and names the condition:
`"uncertainty":["all results are syntax matches; no project-owned authority corroborates the task"]`
against the oracle's `[]`. The declaration swaps the authoritative-count rewrite for that
uncertainty rewrite (still seven, 448 bytes accounted) and `validVersionTokenDivergence` pins the
line verbatim. Included and requested counts, the results array, the state and the verification
plan diverge as before.

**Amendment 2026-09-02 (verification plan, decision 0036 wave 3).** The fallback profile no longer
assumes `make gate`: the candidate's closing gate on this fixture is `git diff --check`, so the
verification rewrite becomes `"verification":["go test ./internal/auth/...","git diff --check"]`
against the oracle's `["git diff --check"]`. Still seven rewrites; the byte identity is
reconciled from the strings at replay time.

### DR-0018 — `eval`/`query`: members of a parenthesized Go `const (...)`/`var (...)` group are symbols in the candidate and invisible to the oracle

- **Status:** LANDED 2026-09-03 — the amendment was accepted as `GPK-V0-048`
  (`docs/decisions/0039-go-declaration-set-2026-09-03.md`), so the entry is re-adjudicated
  **`python-defect`** / known-divergent. The candidate satisfies the clause unchanged and `src/` is
  not repaired.
- **Command:** `eval`, `query` (the shared symbol table since wave 3, decision 0036 item 10).
  Development-corpus cases cobra-active-help and beamfall exact-feature-pairing. **Discovered:** 2026-09-02 as non-critical selector drift; root cause
  isolated 2026-09-03.

**Observed bytes (2026-09-03).** The oracle's `_go_symbols` (`src/context_corvint_index.py:1058`) is a
per-line regex and emits no symbol for a name declared inside a parenthesized group: over cobra's
`active_help.go` it emits none of `activeHelpMarker`, `activeHelpEnvVarSuffix`,
`activeHelpGlobalDisable`, and over Beamfall's `internal/pairing/pairing.go` none of
`enrollAttemptAction`, `enrollAttemptWindow`. The candidate's `goSymbols`
(`internal/contextindex/parse.go:280`, `goDeclaredNames`/`goGroupNames`) uses `go/parser` and emits
one symbol per declared name, so its candidate pool is wider and the ranked non-critical selectors
on the two cases differ. Contract metrics are identical under both engines (0/31 critical misses,
recall 1.0); byte-weighted precision 0.780332 Go / 0.804952 Python is unchanged by this class.

**Adjudication: `spec-gap`.** No clause of `go-production-kernel-migration-v0.md` or the index
snapshot spec states which Go declarations are symbols; `GPK-V0-031` names a "Go-symbol feature
compiler" without defining the declaration set, and `GPK-V0-047` ranks symbols without defining
them. Neither runtime wins by default. Proposed amendment (owner acceptance required): a Go source
contributes one symbol per name declared at file scope by `go/ast` (`FuncDecl`, and every
`ValueSpec`/`TypeSpec` name of a `GenDecl`, grouped or not), so a parenthesized group is not a
scope boundary; on a parse error the lossy scanner's symbols are recorded with an `Unparsed` row.
Once accepted this entry becomes `python-defect` / known-divergent with no repair to `src/`.

**Accepted 2026-09-03.** The owner accepted the amendment verbatim ("accept the DR-0018 clause");
it is `GPK-V0-048` of `docs/specs/go-production-kernel-migration-v0.md`, and the clause added
`type (...)` to the groups named and made the `Unparsed` row on a parse error a MUST rather than a
description. No code changed: `goGroupNames` already flattens every group and
`TestGoSymbolsNamesEveryGroupMember` already pins it against a fixture the lossy scanner fails, so
the candidate passed the new clause the day it was written. The oracle stays frozen and wrong under
`GPK-V0-033`, which is what known-divergent means here.

### DR-0019 — `query`: the closing verification command ignores a Makefile the repository declares

- **Status:** CLOSED 2026-09-03 — adjudicated **`go-defect`**, repaired in Go under `GPK-V0-033`.
  `src/` is unchanged.
- **Command:** `query` (the standalone verb only). **Discovered:** 2026-09-03 by
  `conformance/perf-v0`, as the sole `unequal-stdout` member of task `query-authority-start`.

**Observed bytes (2026-09-03).** On the `corvint` corpus at `ca37e75a`, oracle and candidate agreed on
every member of the receipt except two: `context.verification[0]` was `"make gate"` in the oracle and
`"git diff --check"` in the candidate, and `context.coverage.packet_bytes` differed by exactly seven,
the length delta of those two strings. The oracle's `_gate_command`
(`src/context_corvint_learning.py:210`) reads the repository's own Makefile and names the first
conventional target it actually declares; Corvint's Makefile declares `gate`.

**Root cause.** Go carries the same rule — `gateCommand` and `makefileTargetCommand`
(`internal/contextindex/receipt.go`), with the oracle's target list and target pattern — and its
unit tests passed, because they hand the function a `Source` whose `Data` is already populated. The
standalone query index does not populate it: `querySourceInventory`
(`internal/contextindex/index.go`) records every admitted path as
`Source{Path, BlobHash, Mode}` and loads content only for the entries the query ranks. An unloaded
source's `Text()` returns `("", true)`, so the Makefile scan read an empty file, found no target,
and fell through to the signal-free profile's gate. The `impact`, `feature` and harness paths build
a full index and were never affected, which is why only one task in the matrix carried the reason.

**Repair.** `queryGateEntries` names the gate file and the query build loads it through the same
`pinSources` path the authorities use, so divergence and generated-header handling are identical.
`TestQueryIndexLoadsTheGateFileItDerivesTheClosingCommandFrom` builds a real query index over a
fixture repository whose Makefile declares `gate`, and fails before the repair with "the Makefile is
admitted but carries no content". End to end on this repository, both runtimes now emit
`["make gate"]`.

**Residual, not repaired here.** `Source.Text()` still reports an unloaded source as an empty file
rather than as absent, so any future rule that reads a source the query build did not load will
degrade the same silent way. Separating "not loaded" from "empty" changes the `Source` contract for
every caller and needs its own decision; it is recorded in `docs/agent-memory/fixes.md`.

### DR-0022 — `verification`: the eight-command ceiling can drop the mandatory gate in the oracle

- **Status:** LANDED 2026-09-04 — adjudicated **`python-defect`** under `GPK-V0-033`; repaired in Go,
  `src/` unchanged. No parity case reaches eight hints, so no manifest row is known-divergent.
- **Command:** every receipt that carries `verification` (`query`, `impact`, `harness event`).
  **Discovered:** 2026-09-04 by the AT-13 source investigation (Astra, read-only), confirmed at
  `internal/contextindex/receipt.go` and `src/context_corvint_learning.py:280-283`.

**Observed behavior.** Both runtimes derive one test hint per evidence directory, append the
repository gate (`make gate` or the profile's gate), then truncate the list to eight. With eight or
more distinct hints the gate is the ninth entry and is cut: the receipt keeps advisory hints and
loses the one command the repository declares mandatory. No parity case reaches eight hints, so
the disagreement was never observed as bytes.

**Repair.** Go now truncates the hints to seven before appending the gate, so the gate always
survives and a hint is dropped instead (`TestVerificationDerivesCommandsFromWhatTheRepositoryProves`,
case "eight or more hints drop a hint, never the gate"). The oracle keeps its order; a parity case
with nine Go directories would now differ in the eighth entry (`make gate` versus the eighth hint).

### DR-0020 — `harness event --event file-change`: a Go-only disclosure list outbids the evidence it discloses about

- **Status:** CLOSED 2026-09-03 — adjudicated **`go-defect`**, repaired in Go under `GPK-V0-033`.
  `src/` is unchanged.
- **Command:** `harness event --event file-change`, and every budgeted receipt whose index has an
  `unparsed` or `extraction` table. **Discovered:** 2026-09-03 by `conformance/perf-v0`, in task
  `harness-file-change`.

**Observed bytes (2026-09-03).** On this repository, changed path `cmd/corvint/main.go`, budget
5,952. The oracle's fifth result is `docs/specs/index-snapshot-v0.md` with an
`authority: repository-spec` row reading "explicitly references changed path cmd/corvint/main.go";
the candidate's fifth is `cmd/corvint/affected.go` with `authority: syntax`, and the document is
absent from its results entirely.

**Root cause, and what it is not.** Not ranking: `Impact` scores the document 825 and the reference
775, in that order, in both runtimes. Not the selector: the Go budget loop and the oracle's
`compile_receipt` are the same greedy critical-first pass that skips a result which does not fit and
continues. Not result size: the first four results are byte-identical across runtimes at
283/595/760/341 bytes, and the document itself is 2,864 bytes in both. The difference is the
envelope — oracle 726 bytes, candidate 1,627 — and 925 of that 901-byte excess is the candidate's
`unparsed` samples list. With it, the document no longer fit, so the selector skipped it and took
the 1,211-byte `syntax` result further down instead.

`unparsed` and `extraction` carry the same `{count, samples}` shape as `exclusions`, which
`compactBudgetEnvelope` has always compacted to `{count, samples_omitted_by_budget}`. They were
never added to it because they are Go-only members and the oracle's envelope has nothing to mirror,
so no parity case could observe the omission. The effect is that a disclosure the caller cannot act
on outbids the evidence it is disclosing about, and a `syntax` result displaces a project-owned one —
against AGENTS.md invariant 3.

**Repair.** `compactBudgetEnvelope` compacts both members alongside `exclusions`. The count and the
fact of omission survive; only the samples go. All five results now match the oracle byte for byte,
and the receipt still reports `"unparsed":{"count":7,"samples_omitted_by_budget":7}`.
`TestBudgetedEnvelopeCompactsTheGoOnlyDisclosureMembers` fails before the repair.

### DR-0021 — receipt disclosure: a source whose grammar refused it is disclosed by the candidate and invisible to the oracle

- **Status:** LANDED 2026-09-03 — adjudicated **`python-defect`** under `GPK-V0-033` and
  **known-divergent**. `src/` is NOT repaired. Owning clause `GPK-V0-049`, accepted by
  `docs/decisions/0041-perf-grants-dr0021-corvint-harness-2026-09-03.md`.
- **Command:** every command compiling a context receipt over a repository holding a source some
  grammar refused, observed on `harness event --event user-prompt` and
  `harness event --event file-change`. **Discovered:** 2026-09-03 by `conformance/perf-v0`, tasks
  `harness-user-prompt` and `harness-file-change`, diagnosed offline against the pinned corpus.

**Observed bytes (2026-09-03).** Corpus `corvint` at `d995231`, materialized the way the runner
materializes it — `git archive` of the pin, extracted, `git init` + `git add --all --force` + one
commit, verified to rebuild the pinned tree `02eab903` — with the oracle source and the candidate
binary both built from that pin and both run under the runner's sanitized environment. On
`--event user-prompt` the receipts differ in exactly three leaves: the candidate carries
`"unparsed":{"count":7,"samples_omitted_by_budget":7}`, the oracle carries no such member, and
`coverage.packet_bytes` is 3,967 against 3,914. Nothing else differs. On `--event file-change` the
same member appears alongside two disagreements already registered elsewhere (`DR-0009`'s coverage
denominators, `requested_results` 10 against 60; `DR-0004`'s two permuted test-marker evidence rows
inside `results[2]`).

The seven are `internal/pythonsyntax/testdata/ast-parity/reject-{brace-trailer, dangling-ifelse,
decorator-order, exponent-dot, for-target, leading-comma, unary-binary}.py`, all
`PYTHON_SOURCE_UNPARSED` withholding `symbols`. They are fixtures whose entire purpose is to be
invalid Python, so the candidate is right to refuse them and right to say so.

**Adjudication.** `GPK-V0-049` states the requirement the spec previously carried only for Go
(`GPK-V0-048`): a source the index admits and counts but extracts no facts from MUST be disclosed,
whatever the refusing grammar. The oracle has no such member at all — `unparsed` does not occur
anywhere in `src/` — so it contradicts the clause and the candidate satisfies it unchanged. This is
adjudicated from the clause, not from the candidate's output: the clause's reasoning is
`GPK-V0-048`'s own, extended to the languages it left undetermined, and AGENTS.md invariant 2
independently forbids a receipt that drops facts and claims coverage anyway.

**Why the register cannot close it, and what the corpora show.** The oracle is frozen, so the member
can never appear on its side; no repair closes this. Narrowing the index's admission rules so the
fixtures are never taken in would close it by suppressing an accurate disclosure and by reading the
candidate's convenience as the authority, which `GPK-V0-033` forbids in terms.

The consequence for measurement is that `GPK-V0-016`'s equal-stdout predicate makes every task over
such a repository invalid **by construction** rather than merely unmeasured. The two corpora
discriminate this exactly: the Beamfall clone reports zero unparsed sources, so the member is absent
from both receipts and its harness tasks are `valid`; this repository reports seven, and its harness
tasks are not. That difference, and not any difference in the measured code paths, is the whole of
why `beamfall-harness-user-prompt` passes and `harness-user-prompt` does not.

**Grant.** Decision 0041 ratifies `DR-0021` for the two corvint harness invocations by exact argv, and
additionally ratifies `DR-0004` and `DR-0009` for `harness-file-change`, which carries all three.
The grant relaxes `unequal-stdout` alone. No other task may name the entry without its own
ratification.

**What would discriminate a regression.** The entry is pinned to the seven fixtures and to the
`{count, samples_omitted_by_budget}` shape `GPK-V0-045` compaction leaves. A candidate that stopped
emitting the member, emitted it with a different count, or emitted its samples uncompacted would
change the candidate's bytes and fail the parity and budget tests named in `GPK-V0-049`'s
traceability row before this grant could hide it.

### DR-0023 — receipt disclosure: `exclusions.count` omits the tracked paths the suffix allow-list never read

- **Status:** LANDED 2026-09-12 (decision 0166). Adjudicated `python-defect` / known-divergent under
  `GPK-V0-063` (`docs/specs/go-production-kernel-migration-v0.md`), which requires the count the
  candidate emits. The oracle counts exclusion rows only. The disagreement is **EXPECTED and MUST
  NOT be reported as a failure** (`GPK-V0-033`). `src/` is deliberately unrepaired. This supersedes
  the 2026-09-04 deferral in decision 0051.
- **Command:** every receipt, meaning `query`, `feature`, `impact`, `range impact`, and the `harness`
  context blocks. **Discovered:** 2026-09-04, by the AT-02 receipt-disclosure slice
  (`docs/agent-memory/fixes.md`, since removed).

**Root cause.** Both runtimes admit a tracked path only when its suffix is on the text allow-list.
They skip every other path before reading it and write no exclusion row for it. The receipt counts
only exclusion rows. A `.gitignore`, `.tsv` or `.lock` the index never opened is therefore absent
from `exclusions.count`, and the receipt claims more coverage than the index holds (invariant 2).

**The Go repair.**
- `admittedEntries` in `internal/contextindex/index.go` returns the number of skipped-suffix paths.
- `Index.UnsupportedSuffixCount` persists that number in every snapshot encoding.
- `receipt` adds it to the count. It adds no `exclusions.samples` entry, so the samples and every
  other member are unchanged.

**Observed bytes (2026-09-12).** In each of the 20 cases below, the two runtimes' stdout differs in
exactly one member:

| member | Python oracle | Go candidate |
|---|---|---|
| `…exclusions.count` | `0` | `1` |

The one unread path is the fixture's `.gitignore`. It was enumerated from `fixture.json`
independently of the candidate, as decision 0166 records. `packet_bytes` does not move, because the
rewrite keeps the width.

**Discriminating cases.** Each case carries one `exclusionCountDivergence` rewrite, a separate
declaration from `knownDivergence` so that it composes with `DR-0015`, `DR-0025` and `DR-0035`:
- `harness-file-change`, `harness-session-start-compact-rehydrated`, `harness-user-prompt`;
- `query-authority-start-default`, `-learned`, `-limit-50`, `-non-ascii`, and
  `query-clean-authority-start`;
- `query-repository-agent-tooling`, `-budget-selection`, `-default`, `-limit-1`, `-limit-10`,
  `-limit-50`, `-non-ascii`, `-trace-budget-selection`, `-trace-matching`, `-trace-mixed-worktree`,
  `-trace-nonmatching-nonpassed`;
- `query-version-token`.

`validExclusionCountDivergence` (`conformance/cli-parity-v0/manifest.go`) pins the declaration:
- to this closed case set and to `DR-0023` / `GPK-V0-063`;
- to one stdout rewrite of the `"exclusions":{"count":N,` prefix;
- to a candidate integer larger than the oracle's.

A candidate that stopped counting the unread paths therefore fails replay rather than passing
quietly.

### DR-0025 — `query`: coverage is denominated in the truncated output at `--limit 1`

- **Status:** LANDED — adjudicated `python-defect` / known-divergent under `GPK-V0-040`
  (`docs/specs/go-production-kernel-migration-v0.md`), which requires the denomination the candidate
  uses; the oracle measures its own narrowed output instead. The disagreement is **EXPECTED and MUST
  NOT be reported as a failure** (`GPK-V0-033`). `src/` is deliberately unrepaired.
- **Command:** `query`. **Discovered:** 2026-09-04, when the `GPK-V0-040` repair reached the query
  receipt and `query-repository-limit-1` stopped matching the frozen oracle bytes.

**Root cause: the same defect DR-0009 records, on the query path.** `query`
(`src/context_corvint.py:696`) builds its receipt through the same `_receipt`, whose first statement is
`results = results[:limit]` (`src/context_corvint.py:64`), and that already-narrowed list is measured
again by `compile_receipt` (`src/context_corvint_learning.py:403`, `:406`). `requested_results` therefore names
the emitted output rather than the admitted universe, `omitted_results` is structurally zero by
subtraction, and no `uncertainty` line can ever name a result the ceiling dropped. DR-0009 pins the
`impact` observation of this defect; this entry pins the `query` one, and neither runtime's ranking,
ordering, or evidence differs.

**The Go repair.** `EvalQuery` passes the full admitted result universe to `receipt`, so
`setCoverage` denominates coverage in what ranking considered — exactly what `GPK-V0-040` requires of
every receipt-compiling surface, not only `impact`.

**Observed bytes (2026-09-04, both runtimes, fixture `query-repository`, `query --task "session
expiry device revocation enforcement" --limit 1`).** The full stdout of each is one canonical JSON
line; the members that move are:

| member | Python oracle | Go candidate |
|---|---|---|
| `context.coverage.requested_results` | `1` | `2` |
| `context.coverage.included_results` | `1` | `1` |
| `context.coverage.omitted_results` | `0` | `1` |
| `context.coverage.uncertainty` | `[]` | `["1 ranked results omitted by packet budget"]` |
| `context.coverage.packet_bytes` | `2005` | `2048` |

Total stdout: oracle 2059 bytes, candidate 2102 bytes — the oracle's is the manifest's captured
expectation for this case, `stdoutSha256
8ce1fb16de7c1a381febd51922d365f35512647fa1bac577f12b28cfe77b41c6`. Both runtimes publish the same
single result with the same evidence; `included_results` agrees, and only the denomination of the
counts differs, which is why the divergence is expressible as three member rewrites (43 bytes
accounted; the `packet_bytes` delta is reconciled arithmetically by the replay). Amended
2026-09-04 by `DR-0024`: the candidate entry reads "omitted by result limit" on this null-budget
request, so candidate `packet_bytes` is 2047, total stdout 2101, and 42 bytes are accounted; the
oracle side is unchanged.

**Discriminating case: `query-repository-limit-1`.** The fixture admits two results for the task and
`--limit 1` ranks one of them out, so the omission is nonzero and the two denominations cannot
return the same number. `validQueryAdmittedUniverseDivergence`
(`conformance/cli-parity-v0/manifest.go`) pins the declaration to `query`, to `DR-0025` /
`GPK-V0-040`, to the identity `admitted == emitted + dropped`, and to the verbatim uncertainty line,
so a candidate that stopped emitting the form the clause requires fails here rather than passing
quietly.

### DR-0024 — a null-budget omission is named as a budget the request never set

- **Status:** LANDED — adjudicated `python-defect` / known-divergent under `GPK-V0-053` (accepted
  by decision 0051 item 2, `docs/specs/go-production-kernel-migration-v0.md:507-515`), which requires an omission line to
  name the ceiling that actually dropped the results. The disagreement is **EXPECTED and MUST NOT be
  reported as a failure** (`GPK-V0-033`). `src/` is deliberately unrepaired.
- **Command:** every surface that compiles a context receipt under a `--limit` ceiling with no
  `--budget`: `query`, `impact`, `range impact`, `feature`, and the `eval` and `harness` paths that
  reuse them. **Discovered:** 2026-09-04, auditing the wording `DR-0009` pins.
- **Amends:** `DR-0009` and `DR-0025`. This entry does not add a divergent case; it restates the
  candidate side of the uncertainty rewrite both of those entries pin.

**The defect.** `src/context_corvint_learning.py:363` writes `f"{omitted} ranked results omitted by
packet budget"` for every nonzero omission, whatever produced it. With `budget_bytes` null no packet
budget exists: the only ceiling that can drop a ranked result is the `--limit` value the request
carries. Naming a budget the caller never set is the failure mode product invariant 2 forbids — a
receipt asserting a cause it never measured — and it is worse than silence, because a caller reading
"packet budget" will raise its budget and see the same omission.

**The Go repair.** `setCoverage` (`internal/contextindex/receipt.go`) selects the cause from the
budget it was handed: `omitted by result limit` when `budget == nil`, `omitted by packet budget`
otherwise. Budgeted receipts are byte-unchanged, so no budgeted parity expectation moves and
`conformance/go-query-start-v0`'s frozen budget vectors are untouched.

**Amended candidate fragment.** `DR-0009`'s and `DR-0025`'s third rewrite keeps its oracle side
`"uncertainty":[]` and its shape; only the candidate text changes.

| case | oracle | candidate (before) | candidate (after) |
|---|---|---|---|
| `impact-ranked-past-limit` | `"uncertainty":[]` | `["4 ranked results omitted by packet budget"]` | `["4 ranked results omitted by result limit"]` |
| `query-repository-limit-1` | `"uncertainty":[]` | `["1 ranked results omitted by packet budget"]` | `["1 ranked results omitted by result limit"]` |

`packet budget` is thirteen bytes and `result limit` twelve, so each quoted entry loses exactly one
byte and each entry's arithmetic closes one byte tighter than it did:
`impact-ranked-past-limit` candidate `packet_bytes` 2470 -> 2469 and total stdout 2525 -> 2524
against the oracle's unchanged 2427 and 2482 (43 bytes accounted -> 42);
`query-repository-limit-1` candidate `packet_bytes` 2048 -> 2047 and total stdout 2102 -> 2101
against the oracle's unchanged 2005 and 2059 (43 -> 42). The `2026-08-30` Beamfall real-corpus
instance recorded under `DR-0009` moves the same way: 48 accounted bytes -> 47, candidate
`packet_bytes` 12730 -> 12729 and total 12785 -> 12784. No oracle expectation and no case status
moves, so the manifest's `stdoutSha256` values and the SUMMARY `known-divergences` count are
unchanged.

**Discriminating cases.** The two above. `admittedUniverseUncertaintyMark`
(`conformance/cli-parity-v0/manifest.go`) pins the amended text, so both
`validAdmittedUniverseDivergence` and `validQueryAdmittedUniverseDivergence` refuse a declaration
that keeps the old wording, and `runner.go`'s exactly-once check refuses a candidate that emits it.
`TestQueryCoverageCountsAdmittedCandidatesPastLimit`
(`internal/contextindex/eval_query_coverage_test.go`) holds the unbudgeted query path to the new
wording while `TestQueryWithheldDisclosureFitsInsidePacketBudget` in the same file still requires
`omitted by packet budget` under a real budget, so the pair discriminates the two causes.

## Adjudicated and closed

### DR-0013 — stdout non-ASCII encoding: the oracle escapes, the candidate emits UTF-8

- **Status:** CLOSED 2026-09-01 — adjudicated **`go-defect`**, repaired under `GPK-V0-033`.
- **Command:** `harness` and the other JSON stdout paths corresponding to the Python CLI's `_emit`,
  including CEM and OCM envelopes. Observed on `harness event --event user-prompt` against the pinned
  Beamfall corpus. ASCII-only cases cannot detect it. LRF is explicitly outside this entry: both its
  Python and Go canonical document paths deliberately retain raw UTF-8 and bypass `_emit`.

**Divergence.** The oracle's `_emit` (`src/corvint_cli.py`) calls `json.dumps(payload, sort_keys=True,
separators=(",", ":"))` and passes **no `ensure_ascii` argument**, so Python's default of `True`
applies and every non-ASCII character is written as a `\uXXXX` escape. The candidate's `emit`
(`cmd/corvint`) serialises through `gokernel.CanonicalJSON`, which deliberately reproduces
`ensure_ascii=False` and writes the character's UTF-8 bytes. On the measured case the oracle emits
5527 bytes with one `—` escape and zero bytes >= 0x80; the candidate emits 5524 bytes with three
bytes >= 0x80 and no escape. Every JSON field is equal; only the transport spelling differs.

**Adjudication: `go-defect`.** `GPK-V0-002` binds the candidate to the oracle's exact stdout bytes,
and the oracle's spelling is the authority whether or not it is the more attractive one. The
candidate does not match it. Under `GPK-V0-033` a `go-defect` MUST be repaired; the oracle is frozen
and is not touched.

**Scope of the repair — read before changing `CanonicalJSON`.** `CanonicalJSON`'s `ensure_ascii=False`
behaviour is **correct where it is used today** and MUST NOT simply be flipped. It computes the
receipt digest basis, and changing it would move every emitted `receiptId` — a far larger blast
radius than this divergence, and a wire-visible change in its own right. The repair separates the two
concerns: stdout emission adopts the oracle's escaping, while the digest basis keeps the canonical
form it already has. Any repair that changes both with one edit is wrong even if it makes this case
pass.

**Why this is filed separately from DR-0005.** DR-0005's subject was repaired on 2026-08-30 (the
symbol context window) and its two stdouts are now JSON-identical in every field including
`packet_bytes`. This encoding difference is the *only* remaining reason that case's stdout is
unequal, and it is a different mechanism with a different adjudication. DR-0005 is held open until
this entry exists precisely so `harness` does not stop being blocked while an unequal stdout remains.
**DR-0005 should be closed in the same change that closes this entry, not before.**

**Discriminating case.** A `cli-parity-v0` fixture whose stdout carries non-ASCII characters on a
path both runtimes answer. No case did before this repair, which is why the prior green suite never
detected it.

**Repair and evidence.** CLI stdout now escapes the already-encoded JSON transport without changing
`gokernel.CanonicalJSON`, so the receipt digest basis remains raw UTF-8. The new
`harness-user-prompt-non-ascii` row carries `é`, `—`, and `🙂`, pinning `\u00e9`, `\u2014`, and the
astral pair `\ud83d\ude42`. Before repair its candidate stdout digest was
`6daab2bc4bad6bf261a262449d979d7fb7889f5c460c5bd921eb848ac654833b` against the oracle's
`6036094e49cefbb4881ea0cd8ec92aaaee160f95bdbf864d48347a234ce637e0` in the original fixture. After
independent fixture regeneration, the focused row replayed byte-identically at the current oracle's
`985964fa039bd9fb4a44366d1398bfb251bdb00f978b92f34b16808eba1376fb` and 2,473 bytes. A later
repeated replay hit the standing
`internal/contextindex` Git process-order flake (`repository-object-unavailable` from the oracle);
that occurrence is process evidence, not an encoding mismatch.

### DR-0016 — `query`: the oracle refuses a task over 2,000 characters that the candidate answers

- **Status:** LANDED — adjudicated `python-defect` / known-divergent under an amended clause.
  `GPK-V0-028` (decision 0023) bounds a task at 8,000 characters; the oracle's `MAX_QUERY_CHARS`
  stays 2,000 and `src/` is deliberately unrepaired. The disagreement is **EXPECTED and MUST NOT
  be reported as a failure** (`GPK-V0-033`).
- **Command:** `query`, and every surface that validates a task through `ValidateQueryCommand`.
  **Discovered:** 2026-09-02 by the confidently-wrong trial's first observation
  (`benchmarks/results/cw-trial-heldout-v1-first-run.json`): 11 of 20 trace2code tasks, whose
  task is a test failure excerpt of 2,085 to 4,310 characters, were refused as
  `query text exceeds 2000 characters` and the refusal became the corvint arm's context.

**Divergence class: python-defect, fixtured on stderr.** Both runtimes carried the same bound, so
no corpus case disagreed until the candidate's rose: `query-repository-task-oversized` carried a
2,001-character task the candidate then answered. The case now carries an 8,001-character task that
both runtimes refuse with exit status 2 and an empty stdout, and is declared known-divergent under
one `stderrRewrites` entry (`exceeds 8000 characters` against `exceeds 2000 characters`), validated
by `validTaskBoundDivergence`, which also requires the task to exceed the candidate's bound. Between
2,001 and 8,000 characters the oracle exits non-zero and the candidate emits an ordinary packet;
no parity case sits in that band.

**Adjudication: `python-defect` (known-divergent, no repair).** A bound on the task is a bound on
what the repository can be asked, and 2,000 characters excludes the tasks an agent actually holds
(a stack trace, a failing test's output). The tokenisers are linear and the packet budget bounds
the answer, so the bound protected nothing the budget does not.

### DR-0017 — `impact`: a root-package `.go` file resolves to `module/.` in the oracle and to the module path in the candidate

- **Status:** LANDED — adjudicated `python-defect` / known-divergent under an amended clause.
  `GPK-V0-027` (decision 0023) admits a root-package path and resolves its import path to the
  module path; the oracle admits the path but builds the target `f"{module}/{package}"` with
  `package == "."`, which no import statement names. The disagreement is **EXPECTED and MUST NOT be
  reported as a failure** (`GPK-V0-033`). `src/` is deliberately unrepaired.
- **Command:** `impact` (one-shot). `range_impact.go` keeps a root refusal of its own and is out of
  scope here. **Discovered:** 2026-09-02 by the trial's first observation: two gin-gonic/gin
  change tasks (`logger.go`, `tree.go`) were refused as `unsupported-impact-path`.

**Observed bytes (2026-09-02, fixture `impact-go-root`).** One module `example.test/rootpkg` with
root `logger.go`, `logger_test.go`, and `cmd/tool/main.go` importing `example.test/rootpkg`. Before
this change the candidate refused with exit status 2 and the oracle answered. After it, on the
case as captured (`impact logger.go`), both emit the changed path and the same-package test row;
the oracle's `_reverse_importers` target is `example.test/rootpkg/.` and finds nothing, and the
candidate's target is `example.test/rootpkg`, which `cmd/tool/main.go` imports and so appears as a
third, `reverse-import` row (score 700, `imports example.test/rootpkg`) with one more result in each
coverage count, 338 more packet bytes, and `go test ./cmd/tool/...` first in `verification`. The
parity case is known-divergent under five rewrites validated by `validRootPackageDivergence` (the
338-byte packet count delta is reconciled by the replay), and
`TestImpactAdmitsARootPackagePath` (`internal/contextindex`) pins the same importer.

**Adjudication: `python-defect` (known-divergent, no repair).** The Go module reference makes the
module path the root package's import path; `module/.` is not an import path. The candidate
satisfies the amended clause; the oracle's silence on root importers is an invented absence.

### DR-0026 — `query`: a withheld test-path symbol disclosure has no oracle counterpart

- **Status:** LANDED — adjudicated `python-defect` / known-divergent under `GPK-V0-052`
  (`docs/specs/go-production-kernel-migration-v0.md`, accepted by decision 0052, 2026-09-04), and
  still **latent**: the oracle has no line answering to this clause at all, but **no
  `cli-parity-v0` manifest case discriminates it yet** — the register requires spec lines, not
  runtime output, and this entry records the owning clause ahead of the first case that exercises
  it.
- **Command:** `query`. **Discovered:** 2026-09-04, registering the Go-only receipt member
  `GPK-V0-052` accepted while auditing decision 0052 against the specs and register.

**Divergence.** `GPK-V0-052` requires that when a query term exactly names a symbol
`isTestPath` withholds from ranking, `coverage.uncertainty` MUST name the withheld count so a
caller can tell test evidence exists from a true absence. `evalWithheldTestSymbols`
(`internal/contextindex/eval_query.go:360`) computes this count before the receipt is built, and
`EvalQuery` (`internal/contextindex/eval_query.go:361-362`) appends `"%d test-path symbol
candidates withheld from query ranking; the context verb serves test evidence"` to
`coverage.uncertainty` whenever it is nonzero. The Python oracle (`src/context_corvint.py`) carries
no equivalent computation and never emits this line under any input, so the two runtimes disagree
by construction rather than by a repairable defect: the oracle predates `GPK-V0-052` and cannot
satisfy a clause it does not implement. This is an ordinary `python-defect` per the outcome table:
the oracle contradicts the (now-accepted) spec, and the candidate matches it.

**Not yet observed.** No fixture in `conformance/cli-parity-v0/manifest.json` names a task whose
exact token matches a symbol `isTestPath` withholds, so no case's stdout carries this line and no
`stdoutSha256` expectation needs a rewrite yet. This entry is filed ahead of that measurement, on
the same authority `DR-0011` uses for a structurally-present but not-yet-discriminated
disagreement: this register's opening failure mode is reading a corpus that cannot discriminate a
divergence as evidence that none exists. The first fixture or corpus task whose named term matches
a withheld test-path symbol must register the observed bytes here and add a discriminating case
under `GPK-V0-034`, following `DR-0025`'s and `DR-0024`'s worked pattern.

**Observed 2026-09-11 (Packet 5 only).** Decision 0086 retains actual diagnostics and stdout
under `conformance/perf-v0/testdata/dr-0026/` for Beamfall cold and snapshot-present user-prompt
at pin `fa3b1e7fe5bc6c10e4b09b2729f364780f567a48`, candidate built from `d4f69c2feb29c99ba75e7367d585b9dc0f742991`.
Both reproduce oracle SHA-256 `7d18ce2eafd164a6b1cf5f92e6569bb224f88988c74ea3d3cc5397c9e54df6ff`
and candidate `cddea558337c499a3f71637000c52a0f0be099a7b8afcd119c7770488b5364a0`.
The uncertainty list gains the nine-symbol disclosure; packet_bytes changes from 4844 to 4942.
The prior “not yet observed” paragraph describes the CLI parity corpus: its discriminating case
remains `NOT_YET_AUTHORED`, and query promotion remains blocked under GPK-V0-034. These harness
observations support only the exact prospective timing grants, never retroactive absorption.

**Wording owned (2026-09-12).** `GPK-V0-052` fixed the count and placement but not the line's text.
Decision 0101 amends it to state the exact line, `N test-path symbol candidates withheld from query
ranking; the context verb serves test evidence`, with no line at zero; a future discriminating case
authors that region from the clause.

**Adjudication: `python-defect` (known-divergent, no repair).** `GPK-V0-052` is the accepted
clause; the oracle's silence on withheld test-path symbol candidates is an invented absence, and
`src/` is deliberately unrepaired (`GPK-V0-033`).

### DR-0027 — `query`: lowercasing the task before camel splitting erases identifier boundaries

- **Status:** ADJUDICATED — `python-defect` / known-divergent under accepted `GPK-V0-055`.
  The focused Go regression is present; a `cli-parity-v0` discriminating row is **NOT_YET_AUTHORED**,
  so query promotion is **BLOCKED** until that row exists and passes; this latent divergence cannot
  contribute to promotion (`GPK-V0-034`).
- **Command:** `query`, `eval`, and `harness event --event user-prompt`, through their shared
  `EvalQuery` ranking. **Discovered:** 2026-09-05 by independent retrieval review and confirmed by
  source inspection, a failing regression, isolated runtime output, and the pinned blind-v3 run.

**Divergence.** Both runtimes formerly trimmed and lowercased the task before passing it to helpers
that split camel/acronym boundaries. `stripFinalNewline` therefore became the indivisible term
`stripfinalnewline`, while the declaration `stripNewline` was split from its original case. Go now
passes the still-cased trimmed task to both term derivations
(`internal/contextindex/eval_query.go`); the oracle still lowercases first
(`src/context_corvint.py`) and is deliberately unchanged.

**Adjudication: `python-defect` (known-divergent, no repair).** `GPK-V0-055` requires the shared
camel/acronym split before Python-compatible lowercase for both pipelines. Ranking terms retain
their stem and alias expansion; ordered relevance-floor words retain the unexpanded task-word order
required by `GPK-V0-039`. The candidate satisfies that clause. The oracle contradicts it, so `src/`
MUST remain unchanged under `GPK-V0-033`.

**Observed bytes (2026-09-05).** The fixture reconstructed by
`TestEvalQueryCamelSplitsTaskBeforeLowering/GPK-V0-055` was queried at limit 1 with `How does
stripFinalNewline behave for object-mode output?`. With author and committer dates fixed at
`2000-01-01T00:00:00Z`, the fixture commit is
`9ecf963b5f818e1ac2a5da51cc957fc9e699c4e7` and the packet's Git-tree revision is
`4ba1cc85f6a6c6b98e7b89b32bd5c4d3b2b9dfaa`. Two independent reconstructions produced identical
bytes. The oracle and pre-repair candidate were byte-identical: 1,599 stdout bytes, SHA-256
`f64d5a3a957f3dbbb0953d3f2aebac62dbea29b8a508e326a40df299a3643653`, with result
`symbol:lib/validate-file-object-mode.js:validateFileObjectMode`, score 220, and
`coverage.packet_bytes` 1,545. The repaired candidate emitted 1,604 bytes, SHA-256
`8064151747a1aa849642dd405dee382b04e848706910acbe417ae33bc7d73c7c`, with the spec-required
`symbol:lib/io/strip-newline.js:stripNewline`, score 280, and `packet_bytes` 1,550. These hashes are
reproducible observations, not frozen parity expectations; the missing manifest row remains explicit
above.

**Measured corpus consequence.** Against the exact five development checkouts, the stable Corvint
case bytes and non-latency metrics were byte-identical before and after this correction (SHA-256
`fdd615e10038a86a244d35e77385b0b4d60fccadd55e6c8794065edcf6fdd525`). On blind-v3, recall moved
0.181818 -> 0.272727, critical misses 8 -> 7, top-five success 0.6 -> 0.8, and serialized-result-byte
precision 0.134069 -> 0.159548. The already-observed partition is development evidence, not a new
held-out result.

### DR-0028 — `impact`: same-package tests ignore declaration-reference evidence and tie by path

- **Status:** ADJUDICATED — `python-defect` / known-divergent under accepted `GPK-V0-056`.
  The focused Go regression is present; a `cli-parity-v0` discriminating row is **NOT_YET_AUTHORED**,
  so impact promotion is **BLOCKED** until that row exists and passes; this latent divergence cannot
  contribute to promotion (`GPK-V0-034`).
- **Command:** path `impact`, `EvalImpact`, and `harness event --event file-change` for the path
  profile. **Discovered:** 2026-09-05 by independent retrieval review and confirmed by source
  inspection, a failing regression, isolated runtime output, and the pinned blind-v3 run.

**Divergence.** The oracle and pre-repair candidate exclude `_test.go` candidates from their
same-package declaration-name scan, assign every non-twin convention test score 550, and therefore
break ties by path. Go now includes tests in that scan, keeps the convention row, and uses the
bounded match count and its syntax evidence to rank the row. Test paths do not become a second
`reference` result. The oracle remains flat (`src/context_corvint.py`) and is deliberately unchanged.

**Adjudication: `python-defect` (known-divergent, no repair).** `GPK-V0-056` requires test-inclusive
reference scanning, reference-count ordering below the exact-twin and marker tiers, and evidence
that explains the score. The candidate satisfies that clause. The oracle contradicts it, so `src/`
MUST remain unchanged under `GPK-V0-033`.

**Observed bytes (2026-09-05).** The fixture reconstructed by
`TestImpactRanksSamePackageTestsByDeclarationReferences/GPK-V0-056` was queried at limit 2. With
author and committer dates fixed at `2000-01-01T00:00:00Z`, the fixture commit is
`1112aab20296f468a3dff56686b40b5b8c20f99d` and the packet's Git-tree revision is
`0f6cc99a73e2fc8a20359d0d30824bd842f9b2b6`; two independent reconstructions produced identical
bytes. The oracle emitted 1,310 stdout bytes, SHA-256
`600abbc8a0f0ee90cf1bbb69fd47bb7c153e2adce6f073600e60f870b2570135`; the pre-repair candidate
emitted 1,352 bytes, SHA-256
`d8b559ea132535ff2159cc7660bb9fac23e57dadab5c8b0471913a642974ef81`. Their ranking agreed on
`test:pkg/a_unrelated_test.go`, score 550; the 42-byte envelope difference is the pre-existing
omission-cause divergence registered by DR-0024, not this repair. The repaired candidate emitted
1,772 bytes, SHA-256 `a8f0b5932fc7c16fc17b29db4da8ec48d7b8a9a3419d4c0c516e3abc530807b8`, ranking
`test:pkg/z_relevant_test.go` at 552 with its convention evidence followed by the two
`ImportantDeclaration` syntax-reference rows. These hashes are reproducible observations, not frozen
parity expectations; the missing manifest row remains explicit above.

**Measured corpus consequence.** With both DR-0027 and DR-0028 repairs present, blind-v3 recall
moved 0.181818 -> 0.363636, critical misses 8 -> 6, top-five success 0.6 -> 0.8, and
serialized-result-byte precision 0.134069 -> 0.211877. Development recall stayed 1.0 with zero
critical misses; impact packets changed as this clause requires, and serialized-result-byte
precision moved 0.803314 -> 0.819964. This observed partition is development evidence, not a new
held-out result.

### DR-0029 — `impact`: an index-admitted suffix without a reverse-import rule is refused by the oracle

- **Status:** ADJUDICATED — `python-defect` / known-divergent under accepted `GPK-V0-027` as amended
  by decision 0057. The focused cross-surface Go regression is present; a `cli-parity-v0`
  discriminating row is **NOT_YET_AUTHORED**, so impact promotion remains **BLOCKED** until that row
  exists and passes (`GPK-V0-034`).
- **Command:** path `impact`, `prove PATH`, snapshot `batch` impact, and `harness event --event
  file-change`. **Discovered:** 2026-09-05 by independent surface review and accepted in decision
  0057.

**Divergence.** The Go index admits `.rs`, but deliberately has no Rust reverse-import rule. The
candidate returns the impact packet and names the missing dimension in `coverage.uncertainty`; the
Python oracle excludes `.rs` from `TEXT_SUFFIXES` (`src/context_corvint_index.py`) and then refuses the
path as absent from the captured revision (`src/context_corvint.py`). The candidate's standalone and
harness context blocks are byte-identical for the same `.rs` path, tree, and dirty set.

**Adjudication: `python-defect` (known-divergent, no repair).** Amended `GPK-V0-027` requires an
index-admitted path to receive the disclosed packet and reserves `unsupported-impact-path-suffix`
for a suffix the index does not admit. This grants no Rust reverse-import rule. The oracle
contradicts the accepted clause, so `src/` remains unchanged under `GPK-V0-033`. The discriminating
Go witness is `TestImpactDisclosesUnruledAdmittedSuffixAcrossSurfaces_GPKV0027`; its `.rs` fixture
also asserts the exact `outside the native Go impact profile` disclosure.

**Clause correction (2026-09-12).** `GPK-V0-027` required the disclosed packet and, in the same
clause, said expected bytes for every rule "are captured from the Python oracle", which cannot hold
for a path the oracle refuses. The clause now excepts the unruled-suffix disclosure and authors that
packet's expected bytes from its quoted text
(`docs/specs/go-production-kernel-migration-v0.md:222-225`), as decision 0057 item 1 already
prescribed. The outcome is unchanged. This correction does not settle whether the literal
`native Go` in that wire text is consistent with `GPK-V0-003`'s "no `go` wire variant".

### DR-0030 — trace consumers: bounded-window candidates carry a Go-only diagnostic and count

- **Status:** ADJUDICATED — `python-defect` / known-divergent under accepted `LTA-V0-005`
  (`docs/specs/learned-trace-admission-v0.md:63-70`). The focused Go regression is present; a discriminating `cli-parity-v0` row
  is **NOT_YET_AUTHORED**, so affected command promotion remains blocked under `GPK-V0-034`.
- **Command:** `record`, repository `query`, `eval`, and harness user-prompt trace loading.
  **Discovered:** 2026-09-05 while implementing Decision 0058's retention carry-over.

**Divergence.** When a candidate is an ancestor older than the bounded replay window, Go attaches
read-failure kind `replay-window`, emits `candidate revision predates the bounded replay window`,
and discloses the number of ancestry rows truncated by its bounded probe
(`internal/tracerecordrepo`). The Python oracle instead rejects the ancestry globally once it
exceeds 10,000 rows and has no distinct candidate-window kind or truncated count
(`src/context_corvint_trace.py`). Repository query still maps the Go kind to
`unsupported-query-trace-state`, so `GPK-V0-044`'s external failure semantics do not change.

**Adjudication: `python-defect` (known-divergent, no repair).** Accepted `LTA-V0-005`
(`docs/specs/learned-trace-admission-v0.md:63-70`) requires the distinct bounded-window diagnostic
and count while `LTA-V0-003` (`:56-62`) uses the same classification only inside `record` retention.
That text was an unnumbered trust-boundary paragraph drafted in the implementing commit `e80b9736`;
decision 0093 gave it a requirement ID on 2026-09-12 without changing its words. Decision 0058
item 5, which predates that commit, permits registering Go learned-path changes but never names
`replay-window`.
The oracle is deliberately unchanged under the owner charter. The
`TestReadBoundsTraceReplayWithoutRefusingLargeRepositories/candidate_outside_bounded_replay`
regression discriminates the Go read kind, message, count, and the separate unreachable-revision
case; a CLI parity row remains required before promotion.


### DR-0031 — `ocm link`: the oracle publishes a claim whose anchor lacks the exact obligation ID

- **Status:** ADJUDICATED — `python-defect` / known-divergent under frozen `OCM-V0-005` exact-ID
  linkage (`docs/specs/ocm-v0-dogfood.md`, proposed/experimental, not promoted by this entry) and
  accepted `GPK-V0-008` rejected-mutator no-effect rules. The focused Go regression is present; a
  discriminating `cli-parity-v0` row is **NOT_YET_AUTHORED** — not merely unwritten, but
  inexpressible under the corpus's current closed item vocabulary (see *Why the parity row cannot be
  authored yet* below) — so `ocm link` promotion remains **BLOCKED** under `GPK-V0-034`.
- **Command:** `ocm link`. **Discovered:** 2026-09-07 from dogfood artifact
  `model-selection-ocm-invalid-links-01.json` (SHA-256
  `aba0bee90e549ae6392da31d7acc5046a8a5d07479a9dda87d0a9ff3bc580aa8`), whose published map the
  verifier then rejected `claim-obligation-mismatch`.

**Divergence.** Given an intent obligation `TM-V0-008` and a test anchor `_TM-V0-008_`, both
producers previously validated only the old map, then published the new link without validating
the candidate closure (Go `internal/lrfrepo/ocm_write.go` `LinkOCM`; Python
`src/context_corvint_ocm_workflow.py` / `src/context_corvint_ocm.py` `link_obligation`). The Go
candidate now reparses the encoded candidate and runs the existing `verifyOCMIntent` /
`verifyOCMClosure` before choosing or writing any destination: exit 2, empty stdout, typed
`claim-obligation-mismatch` on stderr, repository-plus-`.git` entry/content/mode digest unchanged,
original map bytes/mode unchanged, and an absent nested `--output` parent left absent. The unchanged
Python oracle on byte-identical fixture input exits 0 with empty stderr and publishes the invalid map
(SHA-256 `cee82888a7039df6a4bbc7a52cedf1b905eee495aa9677e3eed2d8c83ad795e0`, mode `0600`), which
its own `ocm verify` then rejects `claim-obligation-mismatch`. Receipts: pre-fix Go matched the
oracle (`ocm-link-red-final.log`, SHA-256
`f62a6152e123da73820842ecf3c813c5591cf6f12ca5b1ab6a022ac7421b136b`); post-fix
`ocm-link-green-final.log` (SHA-256
`d6700548cbf40f5f6fd5da54b223de28cb670f2e54bbff9a92058a0e41321d56`) records Go stderr SHA-256
`340ac1b569060b2897b6bd9636858e03a6b28345b17411928fee73bbca4a789f`, Go tree digest
`def7b50f…` before and after, Python stdout SHA-256
`4fc92de25ceb7241ea0fd22558745092b6eb007fe1b549e3e1554698cd259bcf`, and Python tree digest
`c8ae3e7e…` -> `9786dc6e…`.

**Adjudication: `python-defect` (known-divergent, no repair).** `OCM-V0-005` requires the exact
obligation ID in the anchor and the spec's CLI contract states no `link` write occurs before the
binding/obligation/hunk/claim checks complete; `GPK-V0-008` requires a failed `link` to leave the
exact old artifact. A producer that publishes a map its own verifier rejects contradicts both. The
exact-ID verifier grammar is unchanged and no requirement ID or rule is added. `src/` remains
unchanged under `GPK-V0-033`. The discriminating Go witness is
`TestOCMLinkRejectsClaimWithoutExactObligationIDBeforePublication` (`cmd/corvint/ocm_test.go`),
whose `OCM-V0-005` and `GPK-V0-008` subtests probe the original oracle on identical pristine fixture
bytes and whose delimiter-valid control still links.

**Why the parity row cannot be authored yet.** `conformance/cli-parity-v0` admits four item shapes and
this divergence fits none of them. The obstruction is structural: authoring a row today would require
changing the conformance contract itself, not adding data to the manifest.

1. *Plain parity case.* Replay requires exact process bytes and independent before/after repository
   snapshots to match. Go exits 2 with empty stdout and leaves the tree digest unchanged
   (`def7b50f…` before and after); the oracle exits 0, writes stdout, and publishes the invalid map,
   moving the tree digest `c8ae3e7e…` -> `9786dc6e…`. The case fails by construction, which is the
   divergence itself rather than an authoring defect.
2. *`acceptedDivergence`.* `conformance/cli-parity-v0/manifest.go:364` rejects the declaration when
   `item.ExitStatus == 0`, and the class is defined for an oracle that scaffolds exactly one declared
   path *before it rejects*, with `candidateMutation: "none"`. This oracle does not reject: it exits 0
   and completes a real publication. A published artifact is not a scaffold, and no declared
   oracle-only path describes it.
3. *`knownDivergence`.* `validKnownDivergence` (`manifest.go:538-548`) closes admission to a pinned
   table of case IDs, and the class is defined for an observable that is **stdout**: each rewrite must
   occur *exactly once* in the candidate's stdout, and after all rewrites the candidate's stdout must
   equal the oracle's byte-for-byte. Go's stdout here is empty, so no rewrite can occur even once. The
   class also cannot express divergent exit status or divergent repository mutation, both of which this
   divergence carries. `stderrRewrites` exists for `query-repository-task-oversized` alone, where both
   runtimes refuse and only the named bound differs.
4. *`refusals`.* `validateRefusalCase` (`manifest.go:1076-1098`) requires `support: "unsupported"` with
   an `unsupported-*` `expectedType`, and its `switch item.Argv[0]` admits only `query` and `lrf`, with
   a `default` returning "refusal command is outside the seed". `ocm` is outside that seed, and Go's
   rejection is a supported-path domain refusal (`claim-obligation-mismatch`), not an `unsupported-*`
   capability refusal.

What would unblock it is a new, operator-accepted divergence class for *oracle-success* divergence: one
where the candidate refuses on a supported path, the oracle succeeds, and the declaration pins the
divergent exit status, the candidate's empty stdout and typed stderr, the oracle's empty stderr, and
the exact artifact the oracle publishes and the candidate does not, so the row proves the rejection
structurally instead of by byte comparison. That is a substantive contract extension governed by
AGENTS.md invariant 8. It is proposed here and in `docs/agent-memory/ideas.md`; this entry does not
accept it. Until an owner accepts such a class, `NOT_YET_AUTHORED` is the accurate state and
`GPK-V0-034` continues to block promotion.

### DR-0032 — `ocm link`: a missing case selector reports its normalized fragment

- **Command:** `ocm link`.

- **Status:** OPEN — user-requested native diagnostic; no parity adjudication or promotion.
  `ocm link` remains **BLOCKED** for promotion and retirement-window contribution under
  `GPK-V0-033`/`GPK-V0-034`. A discriminating frozen CLI corpus row is **NOT_YET_AUTHORED**.
- **Request provenance:** the owner's 2026-09-08 orchestrator handoff, Task 2, explicitly asks
  "Print the normalized fragment on a miss." The transferred root coordinator confirmed that
  narrow request; it does not accept a new profile, divergence class, or broader product contract.
- **Owning clause:** `docs/specs/ocm-v0-dogfood.md`, `OCM-V0-007` and its native selector diagnostic
  amendment in Traceability. Only the supplied `/case:` suffix is normalized using the existing
  extractor. The bounded hint is emitted only on an exact-selection miss and only if different;
  it does not imply an existing or unique matching claim. No matching rule changes.

**Observed bytes.** On the same isolated supported-CEM fixture, both runtimes receive
`--claim test:TestWork/case:WQO-V0-032` and exit 2 with empty stdout. The frozen Python oracle emits
this stderr, followed by LF:

```json
{"code": "claim-selector-out-of-range", "error": "claim selector is outside the extracted worklist", "ok": false}
```

The native candidate emits this stderr, followed by LF:

```json
{"code": "claim-selector-out-of-range", "error": "claim selector is outside the extracted worklist; normalized case fragment: wqo-v0", "ok": false}
```

The same-fixture CLI probe passed its observation assertions in 3.000s; this is evidence of the
difference, **not a parity PASS**. Private log and wrapper exit-0 receipt:
`/private/tmp/corvint-ocm-repair-evidence/diagnostic.log` and its `.exit.json` companion.
`TestOCMClaimSelectorMiss` covers exact matching, ambiguity refusal, missing normalized targets,
bounded messages and unchanged unrelated misses. The Python oracle and manifest pins are unchanged.
The amendment is limited to the explicitly requested UX repair. The register does not select a
GPK-V0-033 outcome by treating the candidate's bytes as authority; adjudication and a spec-authored
discriminating corpus row remain required before promotion. DR-0031 above records the separately owned lane L evidence of false-anchor publication and cannot stand in for this diagnostic difference.
On 2026-09-25 (V1-0274) the native hint for every `/case:` miss also appends the supported Go case anchor shapes, which widens this same OPEN stderr difference; the observed bytes above predate that suffix, and the status and promotion hold are unchanged.


### DR-0033 — `ocm link`: Go ignored the reader's invalid binding verdict

- **Status:** ADJUDICATED `go-defect` against existing OCM-V0-006; focused repair verified.
  A discriminating frozen CLI corpus row is **NOT_YET_AUTHORED**. This unit does not close the
  GPK-V0-033/034 promotion hold or broaden the supported profile. DR-0031 and DR-0032 remain unchanged.
- **Owning clauses:** OCM-V0-006/007 (verify before linking), GPK-V0-008 (rejected mutation has no
  effects), GPK-V0-002/004 (frozen oracle), and GPK-V0-033/034 (record/adjudicate differences).
- **Observation:** at `eec7e2ed`, canonical OCM bytes with only `cem.mapSha256` set to 64 zeros
  produce reader `valid=false` / `cem-map-digest-mismatch`. Go nonetheless links in place or
  overwrites an existing destination with exit 0. Frozen Python rejects the identical input with
  exit 2, empty stdout, and the following stderr plus LF:

```json
{"code": "cem-map-digest-mismatch", "error": "CEM map digest does not match", "ok": false}
```

The independently supplied target also exposes the bypass: moving only OCM `targetRevision` to
an existing base commit yields reader and Python `target-mismatch` / `caller target differs from
OCM target`. Old Go can publish instead. An absent nested output parent previously refuses with
`unsafe-output`; an invalid claim/test path can mask binding failure. Neither is the required first
binding refusal. Original private reproduction: `invalid-ocm-reader-repro.json` in
`/private/tmp/corvint-orchestration-20260908/`; the frozen Python source is unchanged.

`TestOCMLinkRejectsInvalidReaderVerdict` now checks the actual reader's first issue, exact Python/Go
exit/stdout/stderr, and complete repository plus `.git` entry/content/mode digests before and after
both refusals on identical input. Its two binding cases cover in-place, existing destination, and
absent nested output, plus combined invalid binding/claim/test/output precedence. All seven cases
failed before the guard and pass afterward. Existing exact-anchor success and false-anchor refusal
checks pass. A separate noncanonical control preserves Go's existing exact refusal and no effects;
Python's pre-existing normalization behavior is not relabeled parity.

Focused `go test -count=1 ./internal/lrfrepo ./cmd/corvint -run TestOCM -v -timeout 180s` passes
(10.308s and 51.262s). Qualified supervisor exit 0 / no signal and raw red/green logs are retained
under `/private/tmp/corvint-orchestration-20260908/ocm-hardening/`. These observations restore existing
Python refusal behavior for the targeted binding failures; they do not change the oracle, add a
new divergence class, establish a full-gate result, or claim broader OCM promotion.

### DR-0034 — `contextindex` index admission: no spec text settles whether `.claude` belongs in `forbiddenParts`

- **Status:** LANDED 2026-09-12 — opened as `spec-gap`, closed by amending the owning spec
  (`IDX-SNAP-V0-018`, `docs/specs/index-snapshot-v0.md`, decision 0095), then re-adjudicated
  `python-defect` / known-divergent. Go is unchanged. No discriminating parity row can be authored:
  the Python oracle is retired (decision 0088) and `GOC-V0-002` forbids reconstructing it.
- **Command:** `query`, `eval`, `index`, and every surface built on `contextindex.Build`'s admission
  screen (`admittedEntries`, `internal/contextindex/index.go:473-491`, calling `forbiddenPath` at
  `:1282-1294`, which reads the `forbiddenParts` map at `:73-77`). **Discovered:** 2026-08-29
  (`docs/agent-memory/fixes.md`, decision 0007 D10). **Re-adjudicated:** 2026-09-12.

**Divergence, as originally filed.** `internal/contextindex/index.go`'s `forbiddenParts` excludes
`.claude`; the (now-deleted) oracle's `FORBIDDEN_PARTS`
(`src/context_corvint_index.py:60`, historical Git `9ca27f9a`) did not. Decision 0007 D10
(`docs/decisions/0007-omnibus-ratification-2026-08-29.md:169-173`) ratified "leave until W15,"
reasoning "Go's behaviour is the better one... No register entry is opened." That reasoning is an
owner preference over runtime behavior, not spec text, and this register exists precisely to stop
adjudication from drifting toward the candidate on that basis (see "Known limitation" above), so it
is re-examined here against spec text alone.

**Spec search (2026-09-12).** No clause in any file under `docs/specs/` enumerates `forbiddenParts`'
content or says whether an agent-tooling directory name belongs in it. The two closest texts both
fall short: `IDX-SNAP-V0-001` (`docs/specs/index-snapshot-v0.md:37-45`) governs the searchable-text
*suffix* allowlist ("The searchable-text admission allowlist includes `.rst`, `.mdx`, and `.txt`")
and says nothing about path-*component* exclusion. `SBQ-V0-008`(b)
(`docs/specs/snapshot-batch-v0.md:109-110`) describes the *consequence* of the classification --
"dropped by `admittedEntries` (`index.go:506-517`) as forbidden, oversize, or carrying no admitted
suffix" -- but treats "forbidden" as an opaque fact of that code citation rather than defining which
components qualify. `grep -rn "forbiddenParts\|FORBIDDEN_PARTS" docs/specs/*.md` returns nothing.

**Adjudication: `spec-gap`.** The observable -- whether `.claude` (or any other agent-worktree
directory name) is excluded from the index -- is undetermined by spec text, so neither runtime wins
by default (`GPK-V0-033`). This was already effectively true when decision 0007 D10 was written; the
decision avoided opening this entry on pragmatic grounds (measured impact zero) rather than
resolving the underlying gap, which is the shortcut the register's "Known limitation" section warns
against. It cannot be closed via the register's normal discriminating-case mechanism (`DR-0007`'s
worked example) because the second runtime no longer exists to compare against, and `GOC-V0-002`
forbids reconstructing it to try.

**Practical effect of oracle retirement.** `GPK-V0-034`'s stated blocking rule -- an open entry
blocks `PASS` for its command and blocks `GPK-V0-025` retirement contribution -- is itself superseded
for the Go-only product by decision 0088 (`docs/specs/go-only-cutover-v0.md`, "Rollout and
rollback": "The owner explicitly supersedes GPK-V0-025's Python coexistence windows and
GPK-V0-033's mandatory live cross-check for this cutover; independent spec conformance remains
binding."). So this entry no longer gates a live promotion pipeline, but the underlying question is
real and unresolved: whichever spec is judged to own index admission --
`docs/specs/index-snapshot-v0.md` is the closest existing candidate, since `IDX-SNAP-V0-001`'s
traceability row already names `admittedEntries` as one of its implementing functions -- needs a
clause stating whether `.claude`-named (or more generally, agent-worktree-named) paths are excluded
from the index, and why. Until that clause exists, Go's current exclusion of `.claude` is unchanged
and unchallenged; this entry does not authorize removing it, and no runtime was modified to produce
this entry.

**Closure (2026-09-12).** Decision 0095 adopts the screen Go applies as `IDX-SNAP-V0-018`: the
eleven whole components `.git`, `vendor`, `node_modules`, `app-dist`, `dist`, `build`, `coverage`,
`.next`, `.cache`, `target`, `.claude`; the `internal/store/migrate/` and
`internal/conformance/testdata/` prefixes; and the `generatedPath` pattern, each with its recorded
reason and in that order. The clause now decides the observable: `.claude` components are excluded.
The historical oracle's `FORBIDDEN_PARTS` omitted `.claude`, so it contradicts the clause and the
candidate satisfies it. `TestForbiddenPathScreenIsTheAcceptedSet`
(`internal/contextindex/index_test.go`) pins the Go set to the clause. The learned-trace screen in
`internal/trace` is outside this entry and the clause.

### DR-0035 — `query`: a learned-path evidence reason names the commit its trace was recorded against

- **Status:** LANDED 2026-09-12 — adjudicated `python-defect` / known-divergent under `GPK-V0-044`
  (`docs/specs/go-production-kernel-migration-v0.md`), as amended by commit `6bc54ff2` ("fix: disclose
  a learned trace's recorded revision in its evidence reason"). The disagreement is **EXPECTED and
  MUST NOT be reported as a failure** (`GPK-V0-033`). No oracle was reconstructed (`GOC-V0-002`).
- **Command:** `query`. **Discovered:** 2026-09-12, when `TestGPKV0002ManifestReplay` failed on
  `query-repository-trace-matching` (stdout `c3bf4572…`, 3365 bytes, against manifest `108e9719…`,
  3287 bytes) at integration `e16afda2` and `7381b726`.

**Clause.** The `GPK-V0-044` amendment requires that each learned-path evidence item's `reason`
"MUST therefore also name that trace-recorded commit", because the evidence blob pins to the
path's current indexed content, which may postdate the commit the trace observed. The frozen
expectation for this case was captured from the Python oracle at `00c37afb` (2026-09-01), before
the clause existed, and its reasons end at the matched-term count. The oracle therefore contradicts
the amended clause and the candidate satisfies it.

**Causal commit and bisection.** A candidate built at `00c37afb` reproduces the manifest bytes
exactly (`108e9719…`, 3287 bytes) under the same replay fixture; the candidate at `7381b726` does
not. `6bc54ff2` is the only commit introducing the text `trace recorded at commit`
(`git log -S`), it changed `internal/contextindex/eval_query.go` and the two `QueryTrace`
construction sites, amended `GPK-V0-044` in the same change, and did not update the parity manifest.

**Observed bytes (2026-09-12, fixture `query-repository` with one passed trace `ba3234c6a44a`
recorded at the fixture commit `d6a676aa5bf4`).** Three members move:

| member | frozen expectation | Go candidate |
|---|---|---|
| changed-path `reason` | `…changed this path; matched 2 task terms` | same, then `; trace recorded at commit d6a676aa5bf4` |
| opened-path `reason` | `…opened this path; matched 2 task terms` | same, then `; trace recorded at commit d6a676aa5bf4` |
| `context.coverage.packet_bytes` | `3233` | `3311` |

78 bytes are accounted by the two rewrites (39 each); `packet_bytes` is reconciled arithmetically by
the replay. Every other byte, including ranking, scores, and evidence blobs, is unchanged.

**Discriminating case: `query-repository-trace-matching`.** `validTraceRevisionDisclosureDivergence`
(`conformance/cli-parity-v0/manifest.go`) pins the declaration to `query`, to `DR-0035` /
`GPK-V0-044`, to exactly one changed-path and one opened-path rewrite, and to a candidate side that
is the oracle reason with only the disclosure inserted, naming the manifest's own `commitRevision`
rather than bytes read off the candidate. `TestTraceRevisionDisclosureDivergenceNamesTheManifestRevision`
refuses a declaration naming any other revision.

### DR-0036 — `query`: a third implementation per feature and the frontier kept for a one-part named root

- **Status:** ADJUDICATED 2026-09-12 — `python-defect` / known-divergent under `GPK-V0-040` and
  `GPK-V0-047` (`docs/specs/go-production-kernel-migration-v0.md`), as amended by decision 0157
  (`docs/decisions/0157-ranking-cap-three-and-multi-part-named-root-2026-09-12.md`). The focused Go
  regressions are present; a `cli-parity-v0` discriminating row is **NOT_YET_AUTHORED**, so this latent
  divergence cannot contribute to promotion (`GPK-V0-034`). No oracle was reconstructed
  (`GOC-V0-002`); `src/` is absent and unrepaired.
- **Command:** `query`, `eval`, and `harness event --event user-prompt`, through their shared
  `EvalQuery` ranking. **Discovered:** 2026-09-12 by the offline measurement in
  `docs/plans/ranking-cap-frontier-2026-09-12.md`, re-run for decision 0157.

**Clauses.** The `GPK-V0-040` amendment requires each competitive feature record to admit its three
strongest implementation declarations and not a fourth. The `GPK-V0-047` amendment requires the
named-root frontier drop only when the root's identifier has two or more parts. The pre-0157
candidate admitted two per feature and dropped the frontier for any contained root name; per the
`GPK-V0-047` trace row, that candidate's `evaluation` output was byte-identical to the oracle's for
`execa`, so the oracle is inferred to carry both pre-amendment rules. It therefore contradicts the
amended clauses and the candidate satisfies them.

**Observed movement (benchmark corpus, not the parity manifest).** On the 23 frozen `query` cases of
`benchmarks/manifest.json`, 4 packets move: `execa-kill-descendants` (root `kill`, frontier now kept:
`killDescendantsUnix`, `killDescendantsWindows`, `subprocessKill` emitted) and
`natural-language-session-revocation` (`scopeFromCookies` admitted as a third implementation) in
`results` and `coverage`; `ambiguous-reveal-navigation` and
`blind-beamfall-immediate-hard-delete-abstention` in `coverage` counts only.

**Parity corpus.** `go run ./conformance/cli-parity-v0 replay --candidate <worktree build> --only
query-` and `--only harness-` both exit 0 with no `FAIL` row: no frozen `cli-parity-v0` case
discriminates either amendment, which is why the discriminating row above is still owed.

### DR-0037 — CLI dispatch: argparse's option-prefix abbreviations stay refused in Go

- **Status:** CLOSED 2026-09-13 — adjudicated **intentional divergence**, kept under `GPK-V0-064`
  (`docs/specs/go-production-kernel-migration-v0.md`, decision 0173,
  `docs/decisions/0173-option-values-follow-argparse-classification-2026-09-13.md`).
- **Command:** every `cmd/corvint` command parser. **Discovered:** 2026-09-13, alongside the
  option-value classification fix (`GPK-V0-064`); carried forward from `GPK-V0-062`'s
  non-goal list (decision 0172).

**Divergence.** The retired oracle's `argparse` accepts an unambiguous option-prefix abbreviation
(`--he` for `--help`, `--ta` for `--task`) in place of the full spelling. `cmd/corvint`'s parsers
match only the exact registered flag spelling; an abbreviation is an unrecognized argument and
exits 2.

**Adjudication: intentional divergence.** Not repaired. Accepting prefix abbreviations requires
resolving the match against every option currently registered on that parser at each call; adding a
future option whose name shares a prefix with an existing one would then silently change which
option an already-shipped abbreviated invocation resolves to, or make a previously-unambiguous
abbreviation newly ambiguous — a breaking change for any script relying on the shorter form, with
no compile-time or test-time signal at the call site introducing the new option. `GPK-V0-001` pins
compatibility to the retired oracle's *observable behavior*, not to reproducing every one of
`argparse`'s parsing conveniences, and no `cli-parity-v0` fixture exercises an abbreviated flag.

**Scope of the repair.** None: this entry closes the gap `GPK-V0-062` left open by recording the
non-goal as a deliberate, adjudicated choice rather than leaving it as latent unresolved behavior.

**Discriminating case.** `corvint query --he` (or any other unambiguous prefix of a registered
long option) exits 2 with `unrecognized arguments: --he` in Go, versus printing help under the
retired oracle's `argparse`.

**Repair and evidence.** None required; `GPK-V0-064`'s amendment text states the non-goal is
unchanged from `GPK-V0-062`.

### DR-0038 — cli-parity-v0: the candidate carries one name; frozen oracle inputs keep the old one

- **Status:** CLOSED 2026-09-17 — adjudicated **intentional divergence** under decision 0308
  (`docs/decisions/0308-single-name-corvint-identity-2026-09-17.md`, `CRB-DEC-002`).
- **Command:** `harness` (ten cases, declared); `cem`, `eval`, `migrate-traces`, `ocm`, `query`,
  `record` (twenty-nine cases, retired); `lrf` and `query` (two refusal items, retired).
  **Discovered:** 2026-09-17, replaying the committed corpus against the renamed candidate.

**Divergence.** The retired oracle emitted the product's previous name in three places the frozen
corpus pins: the harness event profile `atlas-harness-event/0`; the state directories `.atlas/`
(CEM and OCM maps, evaluation corpus) and `.context-atlas/traces` (local trace store) that the
frozen argv, fixtures and expectations address; and the agent-tooling term set, which the frozen
task `atlas agent context roadmap` matched on the product name. The candidate emits
`corvint-harness-event/0`, keeps its state under `.corvint/` and `.context-corvint/`, and matches
the tooling term `corvint`.

**Adjudication: intentional divergence.** Decision 0308 gives every identity one name and admits
no alias. The frozen oracle inputs are evidence bytes: the fixture builder identity and format, the
twenty-three fixture files, every per-case argv, setup and path in `manifest.json`, the
`oracleCommand` provenance and the argv pins in `manifest.go` keep their frozen `.atlas` forms, in
the same preserved category as the sealed benchmark partitions and the `atlas-eval-split-v0` split
rule. Editing them would silently re-author the oracle's expectations from the candidate, which
`GOC-V0-002` forbids.

**Scope of the repair.** Two declarations, each pinned to a closed set in `manifest.go`, the
second carried by cases and by refusal items alike:

- `identityRenameDivergence` on the ten harness cases whose only differing bytes are the profile
  member. `validIdentityRenameDivergence` admits exactly one rewrite,
  `"profile":"corvint-harness-event/0"` for `"profile":"atlas-harness-event/0"`, on exactly those
  ten ids; the member lies outside the counted packet, so `packet_bytes` is untouched and the
  declaration coexists with the DR-0008 and DR-0023 declarations three of the cases carry. The
  runner applies it exactly once before any other rewrite and reports
  `PASS-WITH-IDENTITY-RENAME`.
- `retired` on the twenty-nine cases whose frozen argv, fixture or expectation addresses the
  oracle's state paths or the product-name tooling term: `cem` 16 of 37 (the seven `cite` evidence
  outcomes, three `mark` hunk cases, `prepare` create and resume, `status` bound and
  unresolvable-expected-base, `verify` bound and policy-limit), `eval` 1 of 3 (`eval-frozen-corpus`),
  `migrate-traces` 2 of 5 (both legacy cases), `ocm` 5 of 9 (link, mark, prepare-resume,
  verify-cem-binding, verify-linked), `query` 4 of 27 (the three `.context-atlas/traces` cases and
  `query-repository-agent-tooling`), `record` 1 of 3 (`record-create-trace`). `validRetired` admits
  only decision `0308`, a stated reason and a pinned id; `validateManifest` refuses a manifest whose
  retired or renamed count differs from the pinned sets. A retired case is neither executed nor
  scored: the runner prints `RETIRED <id> decision=0308 reason=…`, never a `PASS` vocabulary, its
  frozen expectation stays in the manifest unchanged, and every `SUMMARY` count ranges over executed
  cases only (`parity=104 retired=29 identity-renames=10 … known-divergences=24
  location-normalizations=1 structural-fields=0`). The six commands with a retired case carry
  inventory status `PARTIAL` with the retired count in their reason (`GPK-V0-026`).
- `retired` on the two refusal items whose frozen argv or setup addresses the same paths:
  `lrf-ocm-python-claim-refusal` names `.atlas/change.cem.json` and `.atlas/change.ocm.json`, so the
  candidate refuses the absent `.corvint/` maps before it reaches the typed
  `unsupported-ocm-python-claims` refusal; `query-present-trace-store-refusal` creates
  `.context-atlas/traces`, which the candidate never reads, so the `unsupported-query-trace-state`
  refusal is unreachable. `validRetiredRefusal` and `retiredRefusalCases` pin exactly those two
  ids; the runner prints the same `RETIRED` line and the `SUMMARY` reports
  `unsupported-refusals=1 retired-refusals=2`. Both typed refusals remain in the candidate; `lrf`
  joins the `PARTIAL` rows. `query-missing-authority-refusal` still executes.

**Discriminating case.** `harness-user-prompt-non-ascii`, previously an unqualified byte-exact row,
now differs from the frozen bytes by exactly the two-byte profile rename; `cem-mark-unknown-hunk`
refuses with `excludedPath must be the exact string ".corvint/change.cem.json"` where the frozen
argv excludes `.atlas/change.cem.json`.

**Repair and evidence.** `TestGPKV0002ManifestReplay` asserts the retired rows equal the two pinned
sets, that no retired id is reported as passing or unsupported, and the new `SUMMARY` line;
`TestDecision0308DeclarationsAreClosed` holds all three validators to their pinned sets and refuses
a manifest that drops any declaration. The alternative, keeping `.atlas/` and `.context-atlas/` as
the state-directory names and `atlas` as a tooling term to retain full oracle parity, was rejected
by decision 0308's one-name rule; re-executing a retired case requires new fixtures captured under
an accepted successor to `GOC-V0-002`, not an edit to the frozen bytes.

### DR-0039 — `cem mark`: the candidate's invalid `--reason` refusal enumerates the four `cem/0.3` structural reasons

- **Status:** CLOSED 2026-09-22 — adjudicated **intentional divergence** (a Go extension) under
  decision 0338 (`docs/decisions/0338-cem-0-3-structural-mechanical-reasons-2026-09-22.md`,
  `CEM-SM-001` in `docs/specs/cem-0.3-structural-mechanical.md`).
- **Command:** `cem mark` (one case, `cem-mark-invalid-reason`). **Discovered:** 2026-09-22 by the
  full gate after ticket V1-0087 landed: `TestGPKV0002ManifestReplay` reported
  `FAIL cem-mark-invalid-reason stderrSha256: candidate=531926b2… manifest=b03c3c67…`.

**Divergence.** `CEM-SM-001` makes `cem/0.3` the `cem/0.2` wire shape plus the four structural
reasons `rename`, `move`, `import-reorder`, `formatter-only`, so decision 0338 added them to the
`--reason` choice set in `internal/cem/cli/cli.go`. An unknown reason is still refused by both
runtimes with exit status 2, an empty stdout, and the `invalid-arguments` envelope; only the choice
list inside the refusal differs. Observed bytes on the frozen argv (`--reason nope`):

- candidate (sha256 `531926b2159ab354360a813536b84ac99e5cd448d6b49575f2886229e0fbc3f1`):
  `{"code": "invalid-arguments", "error": "argument --reason: invalid choice: 'nope' (choose from
  'conflicting-evidence', 'formatter-only', 'import-reorder', 'insufficient-evidence',
  'line-ending-only', 'move', 'no-evidence', 'rename', 'whitespace-only')", "ok": false}`
- oracle (frozen `stderrSha256` `b03c3c674a71033198e48de3aed1bdeaae63554283f6b6109d9da9100ed6fbfe`):
  the same envelope with `(choose from 'conflicting-evidence', 'insufficient-evidence',
  'line-ending-only', 'no-evidence', 'whitespace-only')`.

Replacing the candidate's choice list with the oracle's, once, reproduces the frozen digest exactly.

**Adjudication: intentional divergence (Go extension).** Not repaired on either side. The retired
oracle predates `cem/0.3` and cannot know the structural reasons; the candidate satisfies
`CEM-SM-001`. This is not a `python-defect` against the oracle's own spec, and `GPK-V0-033`'s three
outcomes name no "extension" case, so the entry is adjudicated under `GPK-V0-033`'s rule that spec
text, not the oracle, is the authority, with `CEM-SM-001` as the clause the candidate bytes are
authored from. No `GPK-V0-02x`/`03x` clause governs option-choice enumeration (`GPK-V0-027` and
`GPK-V0-028` govern `impact` paths and the `query` task bound); `DR-0037` is the nearest CLI-surface
precedent and, like this entry, is an intentional divergence carried without repair. Re-authoring
the frozen expectation from the candidate is forbidden by `GOC-V0-002`.

**Scope of the repair.** One `knownDivergence` on `cem-mark-invalid-reason` with no stdout rewrite
and one `stderrRewrites` entry (the nine-reason list for the five-reason list), validated by
`validMarkReasonDivergence`, which pins the case to `cem`, register `DR-0039`, clause `CEM-SM-001`,
exactly one stderr rewrite, and the exact candidate and oracle byte strings. The runner requires the
candidate string to occur exactly once, so a candidate that drops or reorders a reason fails the
uniqueness check rather than passing by rewrite. Together with `DR-0040` the `SUMMARY` line moves
from `known-divergences=24` to `known-divergences=26` and the `cem` inventory row reports 19
byte-exact cases and names both declarations. `conformance/cli-parity-v0/README.md`'s
known-divergence list was not updated in this change (outside its ownership) and should gain
`DR-0039` and `DR-0040` bullets.

### DR-0040 — `cem`: the candidate's invalid-subcommand refusal enumerates the Go-only actions

- **Status:** CLOSED 2026-09-22 — adjudicated **intentional divergence** (a Go extension) under
  decision 0347 (`docs/decisions/0347-patch-coverage-witness-from-a-local-coverprofile-2026-09-22.md`,
  `TCQ-V0-051` in `docs/specs/test-claim-qualification-v0.md`).
- **Command:** `cem` (one case, `cem-invalid-subcommand`). **Discovered:** 2026-09-22 by the
  `cli-parity-v0` replay once `DR-0039` was declared: the runner reports the first failing case,
  and `cem-invalid-subcommand` was the next.

**Divergence.** `TCQ-V0-051` adds `corvint cem cover`, `TCQ-V0-055` adds `cem discriminate`, and
`FPK-V0-037` adds `cem anchor` and `cem provenance` (decisions 0347, 0353, 0355), and `RCB-V0-001`
adds `cem export` (`docs/specs/receipt-bundle-v0.md`), all listed after the oracle's actions in
`cemActionOrder`, so the candidate's refusal of an unknown action enumerates twelve actions where
the retired oracle enumerated seven. (Declared 2026-09-22 with the eight-action list of decision
0347 alone; widened the same day when the 0.7.0 integration added three more actions, and on
2026-09-23 for `cem export` under `RCB-V0-001` in `docs/specs/receipt-bundle-v0.md`.) Both runtimes
refuse `bogus` with exit status 2, an empty stdout, and the `invalid-arguments` envelope. Observed bytes on the frozen argv (`cem bogus`):

- candidate (sha256 `ea0102e20cd4ddc4a1cc39a9a87d2f65d365f22e29ce4a80b2405bb6e23b46e3`):
  `{"code": "invalid-arguments", "error": "argument cem_command: invalid choice: 'bogus' (choose
  from 'begin', 'prepare', 'cite', 'mark', 'verify', 'status', 'report', 'cover', 'discriminate',
  'anchor', 'provenance', 'export')", "ok": false}`
- oracle (frozen `stderrSha256` `2a7db57bf388b00caf69fe67aa0db6d2eae74e1f461d9db50243e2f17aabe749`):
  the same envelope with `(choose from 'begin', 'prepare', 'cite', 'mark', 'verify', 'status',
  'report')`.

Replacing the candidate's action list with the oracle's, once, reproduces the frozen digest exactly.

**Adjudication: intentional divergence (Go extension).** Not repaired on either side, on the same
reasoning as `DR-0039`: the retired oracle predates the coverage witness, the candidate satisfies
`TCQ-V0-051`, and `GOC-V0-002` forbids re-authoring the frozen expectation from the candidate.

**Scope of the repair.** One `knownDivergence` on `cem-invalid-subcommand` with no stdout rewrite
and one `stderrRewrites` entry (the twelve-action list for the seven-action list), validated by
`validCEMActionDivergence`, which pins the case to `cem`, register `DR-0040`, clause
`TCQ-V0-051`, exactly one stderr rewrite, and the exact candidate and oracle byte strings.

### DR-0041 — `cem`: the candidate's unreadable-patch and unreadable-map refusals carry their code

- **Status:** CLOSED 2026-09-25 — adjudicated **intentional divergence** under decision 0398 (on
  PR #221, not yet merged) and the proposed `CCF-V1-004` amendment in
  `docs/specs/core-compatibility-freeze-v1.md`, which makes adding an optional `code` member to a
  codeless refusal a compatible change. Awaits owner acceptance with that amendment.
- **Command:** `cem` (two cases, `cem-begin-unreadable-patch` and `cem-unreadable-map`).
  **Discovered:** 2026-09-25 by the `cli-parity-v0` replay after ticket V1-0336 (panel finding D3)
  stopped stripping the code from the two CEM read failures.

**Divergence.** The retired oracle read the patch and the map in its argument layer and refused an
unreadable one with a fixed message and no code. The candidate had copied that codeless envelope in
`emitCEMError`; it now keeps the fixed message and adds `patch-unavailable` or `map-unavailable`.
Both runtimes refuse with exit status 2, an empty stdout and the same `error` text. Observed bytes:

- `cem-begin-unreadable-patch`: candidate (sha256
  `1acd2bf58876e6f98e0e2be019356107b0b6515f8c962572cad738c33c779557`)
  `{"code": "patch-unavailable", "error": "cannot read patch", "ok": false}`; oracle (frozen
  `stderrSha256` `0888ad4fb1b7d6bbe7fc946f4635000eaefef456daf31fbbf95b7d81605a9ec4`)
  `{"error": "cannot read patch", "ok": false}`.
- `cem-unreadable-map`: candidate (sha256
  `40471a42524faefc48409df3af0ed3d0adbe5f54361d032619c15c9305b99081`)
  `{"code": "map-unavailable", "error": "cannot read CEM map", "ok": false}`; oracle (frozen
  `stderrSha256` `517162f19cf5e3136607599ddcb699dd1870bead0fb88fc1a137d21d658edcea`)
  `{"error": "cannot read CEM map", "ok": false}`.

Replacing the candidate's leading `{"code": "<code>", "error": ` with `{"error": `, once,
reproduces each frozen digest exactly.

**Adjudication: intentional divergence.** Not repaired on either side. The oracle's codeless form
left a caller unable to branch on the failure without matching message text; decision 0398 treats
that as a defect, and the proposed `CCF-V1-004` amendment authors the candidate bytes. As with
`DR-0039` and `DR-0040`, this is adjudicated under `GPK-V0-033`'s rule that spec text, not the
oracle, is the authority, and `GOC-V0-002` forbids re-authoring the frozen expectations from the
candidate.

**Scope of the repair.** One `knownDivergence` on each case with no stdout rewrite and one
`stderrRewrites` entry, validated by `validReadFailureCodeDivergence`, which pins the two cases to
`cem`, register `DR-0041`, clause `CCF-V1-004`, exactly one stderr rewrite, and the exact candidate
and oracle byte strings for each case's code. The `SUMMARY` line moves from `known-divergences=26`
to `known-divergences=28`, and the `cem` inventory row reports 17 byte-exact cases. The same change
also emits the `repository-*` and `unsupported-git-object-format` codes from `emitError`; no
`cli-parity-v0` case reaches those refusals, so they need no declaration here.
