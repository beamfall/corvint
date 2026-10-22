# Docs corpus claim audit: sampling, independent verdicts and measured accuracy (V1-2316)

GitHub #1481; ticket V1-2316. Builds on the `docs corpus eval` surface (DCP-V1-250..257, PR #1498).
Requirements `DCP-V1-260..268` in `docs/specs/documentation-corpus-v1.md` are accepted under the
owner's standing auto-accept (2026-10-11).

## Problem

A corpus states a trust label per claim, but nothing measured how often claims are actually right.
There was no reproducible sample, no record of who judged a claim and how, and no way to keep a
generator's self-assessment out of an accuracy figure.

## Decisions

- **Verdicts are a provider record, joined at build time.** `corvint-corpus-claim-audit/1` is a
  closed, manifest `/2` provider member, using the same pattern as test knowledge. The join is a
  separate artifact member, so a verdict never rewrites the claim or its trust label. Evidence
  anchors resolve against declared inputs: a forged binding refuses the build, and an undeclared
  path is kept as an `unanchored` limitation.
- **Staleness is a span hash.** The audit span hash covers claim text plus each evidence anchor's
  repository, path, range and span hash. Any later change makes the verdict `stale`, and a stale
  verdict is excluded from the headline figure.
- **The headline figure takes only current, independent, non-generator verdicts.**
  - Independence needs the record flag, and needs both the reviewer and the audit provider to
    differ from the claim's provider.
  - A generator-method verdict is a `model` verdict on a `generated` claim.
  - Excluded verdicts are reported beside the headline figure with their own accuracy.
- **Sampling is deterministic.**
  - Rank within each stratum by SHA-256 of seed and claim ID.
  - Allocate proportionally by largest remainder.
  - Emit claims sorted by ID.
- **Accuracy is correct / (correct + incorrect + unsupported-by-citation), with a 95% Wilson
  interval.** `unverifiable` is kept separate. A zero denominator abstains; there is no value.
- **Views only.** `claim_audits` appears beside claims on v1 and v2 receipts and through the MCP
  tools, and the trust label never changes.

## Evidence

- **Focused tests:**
  - `TestDCPV1260`, `1261`, `1262`, `1263` and `1265` (`./internal/doccorpus`).
  - `TestDCPV1263MCPGetShowsClaimAudit` (`./internal/mcp/corpusbridge`).
  - `TestDCPV1268CorpusAuditCLI`, plus the flag-matrix and help tests (`./cmd/corvint`).
- **Fail-without-fix:** I mutated `claim_audit.go` so that the headline figure accepted any
  non-unresolved verdict and receipts attached no views. `TestDCPV1261`, `TestDCPV1262` and
  `TestDCPV1263` failed, and passed again once the change was restored.
- **`corvint affected` tier 0+1 (cap 15), all `ok`:**
  - Filtered runs: `./internal/doccorpus` (its full suite exceeds 600 s on this host) and
    `./cmd/corvint` with `-run 'TestDocsCorpus|TestDCPV1|TestCorpus'`.
  - Full packages:
    - `./internal/mcp/corpusbridge`, `./internal/corpusrepublish`
    - `./cmd/corvint-corpus-mcp`, `-parity`, `-republish`
    - `./internal/corpusindex`, `corpusserve`, `doccorpus/selfcorpus`, `mcp/docsbridge`
    - `./internal/flowdocs`, `appflows`, `flowcoverage`, `delta`
- **Static checks:** `go vet` on the touched packages and `gofmt -l` are clean.

## Limits

- Generator method is inferred only as a `model` verdict on a `generated` claim.
- Reviewer independence beyond the provider and reviewer identity checks is provider-reported and
  unauthenticated.
- The pooled figure across strata is not population-weighted, and the report says so.
- No measured accuracy on a real corpus is claimed. No independent verdict set exists yet, so a
  report over real verdicts is `NOT_RUN`.

## NOT_RUN

- Unfiltered `./internal/doccorpus` and `./cmd/corvint` suites.
- Tier 1 units over the cap: `internal/mcp/bridge`, `internal/mcp/testvaliditybridge`,
  `internal/playwrightminimize` and `tools/heading-nav-bench`.
- All 96 tier 2 units, left for batch CI.

## Rollback

Revert the change.
- A corpus without a claim-audit provider builds byte-identically, because the member is omitted
  when empty.
- Artifacts that do carry audits must be rebuilt.
- The `audit` operations disappear from the CLI.
