# Generated claims get a mechanical mention check and corpus pattern signals

Date: 2026-10-23. Lane: claude/claimcheck-gh1584-1586 (base 0b7d59ba6). GitHub #1584, #1586.
Requirements `DCP-V1-340..352` in `docs/specs/documentation-corpus-v1.md` are accepted under the
owner's standing auto-accept (2026-10-11).

## Problem

A generated claim can name code that its cited span does not contain, or code that does not exist
anywhere in the corpus. Nothing in the corpus showed this. The audit sampler (`DCP-V1-265`) also
drew claims in proportion to their population. It could not over-sample the claims most likely to
be wrong: templated text, many claims on one anchor, whole-file anchors, claims whose mentions miss
their span, and uniform claim shapes.

## Decision

- One documented ASCII extractor (`ExtractCodeTokens`, `DCP-V1-340`) feeds both issues. The build
  resolves each token in three stages (`DCP-V1-342`):
  1. in the claim's own cited spans;
  2. then to a corpus subject, an indexed symbol or a tracked path;
  3. otherwise it is unresolved.
- The result is a pinned artifact member, `claim_mentions` (`DCP-V1-341`). `Open` rederives the
  member and refuses a tampered one. Readers serve it beside the claim, and the check also feeds
  typed gaps and coverage rows (`DCP-V1-343..345`).
- The member is omitted when no checked claim has a token, so existing corpora keep their bytes.
- Native claims are not examined. Neither are supplied test-run receipts, which `DCP-V1-211`
  already excludes from coverage.
- The reference half of #1584 already existed as build closure. `DCP-V1-347` pins it with a test
  instead of adding a second check.
- `docs corpus patterns` reports five fixed-threshold signals over the same examined claims
  (`DCP-V1-348`, `DCP-V1-349`). The `token-disjoint` signal reuses the extractor.
- `--stratify patterns` weights each flagged stratum 4 and `unflagged` 1 (`DCP-V1-350`). Because
  that over-samples, `audit report` adds a population-weighted figure for any stratified sample.
  The figure abstains when a populated stratum has no defined accuracy (`DCP-V1-351`).
- The build thresholds are opt-in (`DCP-V1-346`, `DCP-V1-352`). They run after `BuildAdmitted` in
  the CLI. A refusal writes nothing and names every exceeded rate.
- No check or signal raises trust or changes a claim. A flag is a sampling signal, not a verdict.

## Evidence

Focused runs, GOTOOLCHAIN=local (go1.27.2), all PASS:

| Package | Scope | Result |
| --- | --- | --- |
| `./internal/doccorpus/` | full package | ok, 514.9 s |
| `./internal/mcp/corpusbridge/` | full package | ok, 124.5 s |
| `./cmd/corvint/` | `-run 'TestDCPV1349\|TestDocsCorpus\|TestDCPV1268'` | ok, 6.9 s |

`go vet` is clean on the touched packages.

## Limits

- The check makes no semantic judgment, and the token rules are ASCII-only. A PascalCase word with
  no internal case transition is not extracted.
- Template comparison is exact after normalization, and thresholds are fixed.
- The weighted figure has no interval. Stratum populations come from the sample document, which is
  validated but not re-counted against the corpus.
- The HTTP companion shares the `Query`/page readers but has no separate claim-mentions test.

## Independent review fixes

A single Codex review round on PR #1592 found five issues. All are fixed, each with a test.

- **Linear trimming** (`DCP-V1-340`): `trimChunk` now works out once which openers the chunk
  contains, then trims from the right in one pass. The old version rescanned the chunk for every
  closing bracket it stripped, which was quadratic. A test runs a 1 MiB closing-bracket run.
- **Members before file names** (`DCP-V1-340`): dotted members now take precedence over paths. A
  backticked dotted name counts as a file name only when its last part is in a fixed extension
  list, so `os.stat` is a member and `main.go` is a path.
- **Strata reconciliation** (`DCP-V1-351`): `audit report` now refuses a sample whose strata do not
  reconcile with its claims and totals. This closes the dropped-stratum case, where a forged sample
  still produced a defined weighted figure.
- **Empty threshold values** (`DCP-V1-346`, `DCP-V1-352`): an empty value for either threshold
  flag is now an argument error. It used to be treated as an absent threshold.
- **Tamper test** (`DCP-V1-341`): the test now recomputes the artifact digest, so the refusal it
  sees comes from `Open`'s rederivation, not from the digest check.

After the fixes, these focused runs pass:

| Package | Scope | Result |
| --- | --- | --- |
| `./internal/doccorpus/` | `-run '^TestDCPV1'` | ok, 437.8 s |
| `./internal/doccorpus/` | `-skip '^TestDCPV1'` | ok, 79.5 s |
| `./cmd/corvint/` | `-run 'TestDCPV1349\|TestDocsCorpus\|TestDCPV1268'` | ok, 5.7 s |
