# Decision 0166 — A receipt's exclusion count includes the suffixes the index never read

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12). Supersedes the `DR-0023` deferral recorded in the
2026-09-04 amendment to decision 0051.

`docs/agent-memory/fixes.md` (2026-09-04, AT-02) found that `admittedEntries` in
`internal/contextindex/index.go` skips every tracked path whose suffix the `textSuffixes`
allow-list does not admit, and records no exclusion row for it. As a result, `exclusions.count` in
a context receipt left out paths the index never read. Decision 0051's amendment built an
`exclusions.unsupported_suffix_count` member, measured that it moved 19 frozen `cli-parity-v0`
cases, and deferred `DR-0023` rather than weigh what that would cost the `GPK-V0-034` evidence.

The owner call: invariant 2 requires a receipt to disclose what it excluded. A count that leaves
out unread paths asserts more coverage than the index has. So the existing `exclusions.count`
counts skipped-suffix paths too, with no new member and no new sample. `DR-0023` is accepted as a
`python-defect` under `GPK-V0-033`, and each affected `cli-parity-v0` case declares one rewrite.
`src/` stays unrepaired.

## Change

- `GPK-V0-063` (new, `docs/specs/go-production-kernel-migration-v0.md`) states the rule.
- `admittedEntries` now returns its skipped-suffix count.
- The `Index.UnsupportedSuffixCount` field carries that count through the gob, event, sectioned and
  pack snapshot encodings. It must be persisted because an event-loaded index has no tracked table
  to recount from.
- `receipt` in `internal/contextindex/receipt.go` adds the count to `len(index.Exclusions)`. This
  one change covers every receipt surface: `query`, `feature`, `impact`, `range impact`, and the
  `harness` context blocks.
- `analyzerSchemaID` moves to `corvint-analyzer/40` (numbered /38 on its lane; renumbered on merge after /39 landed, and its clause renumbered from `GPK-V0-062` to `GPK-V0-063` because decision 0172 took 062). The pack key therefore changes, and no cache
  built before this change is reused.

## Manifest declaration

- Each of the 20 cases declares its rewrite as a new, separate `exclusionCountDivergence` field,
  not as `knownDivergence`. Three of the cases already carry a `knownDivergence` whose validators
  pin exact rewrite counts: `query-repository-limit-1` (`DR-0025`),
  `query-repository-trace-matching` (`DR-0035`) and `query-version-token` (`DR-0015`).
- `validExclusionCountDivergence` in `conformance/cli-parity-v0/manifest.go` closes the declaration:
  - by case ID;
  - to `DR-0023` / `GPK-V0-063`;
  - to exactly one stdout rewrite and no stderr rewrites;
  - to a candidate `"exclusions":{"count":N,` whose canonical integer is larger than the oracle's.
- The two declarations compose, so `query-version-token`'s `DR-0015` divergence still closes. This
  is the obstacle the 2026-09-04 amendment recorded.

## Independent derivation (`GOC-V0-002`)

No expected value was taken from the candidate. A scratch Python enumeration, not committed, read
only each fixture's `fixture.json` under `conformance/cli-parity-v0/fixtures`. For each fixture it:

1. kept entries of type `file`;
2. set aside paths that already carry an exclusion row (forbidden segments, the migrate and
   conformance-testdata prefixes, generated paths), which the oracle count already includes;
3. set aside `Makefile`, `go.mod`, and paths whose lowercased final extension is in a transcription
   of the `textSuffixes` allow-list;
4. counted the rest.

Results:

- `harness`, `query-authority`, `query-repository` and `query-version-token` each drop exactly one
  path, `.gitignore`.
- `eval`, `harness-non-ascii`, `harness-out-of-scope`, `query-no-authority` and every `impact-*`
  fixture drop none. That is why `eval-frozen-corpus` and the two `DR-0008` out-of-scope cases no
  longer move, unlike in the 2026-09-04 measurement.
- Fixtures whose commands emit no receipt (`cem`, `ocm`, `lrf`, `record`, `migrate-traces`) show
  nonzero counts, but no receipt carries them.
- The unchanged-binary replay proves the oracle count is 0. The expected candidate count is
  therefore 0 + 1 for each of the 20 cases:
  - `harness-file-change`, `harness-session-start-compact-rehydrated`, `harness-user-prompt`;
  - the four receipt-emitting `query-authority-start-*` cases and `query-clean-authority-start`;
  - the eleven receipt-emitting `query-repository-*` cases, which run on the `query-repository`
    fixture. `-out-of-scope` runs on `harness-out-of-scope`, and the other cases are refusals with
    no receipt. The closed set is `exclusionCountDivergenceCases`.
  - `query-version-token`.

Every case was derived, so no case stopped the slice.

## Proof that only the count moved

- The `manifest.json` diff is 220 insertions and 0 deletions: 20 declaration blocks, and no frozen
  expectation, digest or byte count changed.
- Each of the 20 cases was diffed per case: the unchanged binary's stdout and the new binary's
  stdout differ in exactly one substring, `"exclusions":{"count":0,` → `"exclusions":{"count":1,`.
  The other 113 cases match, apart from the temporary-directory and latency bytes that replay
  already normalizes.
- `cli-parity-v0 replay` with the new binary exits 0:
  - `parity=133`
  - `known-divergences=29` (9 + 20)
  - every stream is byte-exact against the unchanged `stdoutSha256`
- With the unchanged binary, `--only harness-user-prompt` exits 1 because the declared rewrite
  occurs 0 times. So the declaration cannot pass a candidate that stops emitting the required
  count.
- `packet_bytes` does not move: it is reconciled arithmetically, and a 0 → 1 rewrite changes no
  width.

## `GPK-V0-034` evidence stays valid

Only the disclosed count changed. Every other byte of these 20 streams is still compared
byte-exact against the frozen oracle digests after the one declared rewrite. That includes results,
ranking, evidence, coverage, `state`, `verification` and `packet_bytes`. The cases therefore still
discriminate the `GPK-V0-028` query behaviour they were promotion evidence for. They are marked
`PASS-WITH-KNOWN-DIVERGENCE` so the replay summary counts them honestly, not because any compared
behaviour was waived.

## Consequences

- `docs/specs/go-production-kernel-migration-v0.md` gains `GPK-V0-063` and its traceability row.
- `conformance/divergence-register.md` gains `DR-0023`.
- `conformance/cli-parity-v0/README.md` inventories the new declaration.
- Decision 0051 gains a pointer to this decision.
- The AT-02 `fixes.md` entry is removed.

## Rollback

Revert this decision's commit. That removes the persisted field, the receipt addition, the
declaration field and its validator, the 20 manifest blocks, `GPK-V0-063` and `DR-0023`, and
restores `corvint-analyzer/37`. Replay then passes again with `known-divergences=9` against the same
frozen expectations, which this decision never edited.
