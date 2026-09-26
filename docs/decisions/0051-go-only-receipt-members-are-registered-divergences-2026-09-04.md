# Decision 0051 — Go-only receipt members are registered divergences, not oracle edits

Date: 2026-09-04. Status: accepted. Authority: repository owner, standing instruction to make the
open calls and record them (decision 0048 lineage). The AT-02 slice 2 builder stopped at the
oracle boundary and parked the question in `docs/agent-memory/questions.md`; this record answers it.

## The call

The two remaining silent omission stages from AT-02 (`docs/agent-memory/fixes.md`, "three silent
omission stages") are surfaced by Go-only receipt members. The slice 2 build registers each in
`conformance/divergence-register.md` as a `python-defect` known divergence under `GPK-V0-033`
("the oracle contradicts the spec while the candidate matches it"), with an owning semantic
clause proposed in `docs/specs/go-production-kernel-migration-v0.md` in the same change (the
register requires spec lines, not runtime output), exact byte rewrites in the `cli-parity-v0`
manifest with a pinned validator exactly as `DR-0025` (`query` coverage at `--limit 1`) was
registered today, and the observed bytes `GPK-V0-034` requires. Neither entry exists yet; this
record authorizes the shape, not the adjudication.

1. `exclusions.unsupported_suffix_count` (the count of tracked paths dropped by the suffix
   allow-list) is emitted by Go and registered as `DR-0023` under a proposed clause extending
   `GPK-V0-040`'s denomination rule to exclusions. The oracle never counts them.
2. For a null budget the oracle emits `"uncertainty":[]` and the Go candidate today says
   "omitted by packet budget", the candidate fragment `DR-0009` pins
   (`conformance/cli-parity-v0/manifest.json:3640@ddcf22f8`, `manifest.go:713`). The candidate wording
   changes to "omitted by result limit"; `DR-0024` amends `DR-0009`'s pinned candidate fragment
   and the manifest constant to the new wording, and the rewrite is composed as oracle `[]` to
   the candidate line, never as a phrase replacement inside oracle bytes.

The frozen oracle in `src/` is not edited (product invariant: python-defects are registered, never
repaired). A Go unit test that compares bytes against the live oracle
(`internal/contextindex/eval_query_test.go`, `TestEvalQueryMatchesPythonForGoRepository`) applies
the registered rewrites to the oracle bytes through one helper that names the `DR` ids, so the
comparison stays byte-for-byte after the declared rewrite and no fixture expectation is silently
changed. The `cli-parity-v0` manifest declares the same rewrites per affected case with a pinned
validator, as `DR-0025` does.

Rejected: keeping the misleading wording (invariant 2: missing evidence must not read as
certainty), and repairing `src/` (GPK-V0-033).

## Rollback

Delete `DR-0023`/`DR-0024` and their proposed clauses, the two Go members and the test helper, and
restore the manifest cases and `DR-0009`'s wording;
`make gate` (`interop-gate`, `internal/betarung`) then passes again against the unchanged oracle.

## Amendment 2026-09-04 (same day): `DR-0023` deferred on measurement

Building the `exclusions.unsupported_suffix_count` member and replaying `cli-parity-v0` at
`01aa66a` moved 19 otherwise-passing cases (`eval-frozen-corpus`, three `harness` context rows, six
`query-authority`/`query-clean-authority` rows, nine `query-repository` rows) and stopped
`query-version-token`'s pinned divergence from closing. Registering them as `DR-0023` would declare
13 of `query`'s 27 parity cases known-divergent and remove them as `GPK-V0-034` promotion evidence
for `GPK-V0-028`, a consequence this decision did not weigh when it authorized the one-case
`DR-0025` model. Call: `DR-0023` is deferred; the member is not built. The next slice proposes a
narrower count (suffix exclusions no accepted clause already admits, for example the `.gitignore`
paths `GPK-V0-050` names; about two cases) through its own Gate A before any register entry.
`DR-0024` stands as written above and landed. The implemented-and-reverted measurement is recorded
in `docs/agent-memory/fixes.md` ("two silent omission stages").

Note (fresh review of slice 2, 2026-09-04): the live-oracle fixtures in
`internal/contextindex/eval_query_test.go` produce no omission (limits 10 and 8 on a fixture that
drops nothing), so the rewrite helper named above is not yet required; it becomes necessary with the
first live-oracle fixture that omits results.

## Amendment 2026-09-12: `DR-0023` accepted by decision 0166

Decision 0166 settles the deferral above differently from the proposed narrower count. It adds no
`unsupported_suffix_count` member. Instead, the existing `exclusions.count` includes every
skipped-suffix path (`GPK-V0-063`), and `DR-0023` is registered with one `exclusionCountDivergence`
rewrite per case. That separate declaration composes with `query-version-token`'s `DR-0015` one, so
the divergence still closes. On the 2026-09-12 fixtures the count moves 20 cases.
`eval-frozen-corpus` is not among them, because its fixture drops no suffix. Every other byte of
those 20 cases still replays byte-exact, so they remain `GPK-V0-034` evidence for `GPK-V0-028`.
