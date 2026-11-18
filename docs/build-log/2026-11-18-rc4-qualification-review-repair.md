# 2026-11-18: rc.4 qualification review repair

Requirements: RCF-V1-008, RCF-V1-010, RCF-V1-003. Change: `rc4-candidate`, tasks 2.2,
2.4 and 4.2. The lane charter assigns no native ticket IDs to these tasks.

## Independent findings and decision

The independent review found three unsound qualification claims: live symbol spans could escape
verification, answers could come from a replacement artifact, and equal porcelain status could
hide checkout mutations. It also found that affected selection can skip the live qualification
without `CORVINT_RCF_QUALIFY`.

The repair closes the receipt-form class with a closed search-receipt decoder. Its results are
Subjects, as requested by the frozen query runner without `include=claims`. It admits only the
named search fields in `rcfSearchReceipt` and their typed nested records, under strict JSON
unknown-member rejection. Unknown schemas, result variants, null results and unknown nested
fields fail qualification. The passage allowlist is top-level citations, Subject evidence
anchors, rank match anchors, trust-envelope citations and symbol-anchor live locations. Each
location reaches the same pinned-blob and span-digest verifier. An extension needs an explicit
qualification review; unfamiliar forms never silently count as verified.

Artifact admission validates the canonical corpus and its embedded content digest. Every CLI
receipt must name that digest; parity binds both MCP copies to it. The report keeps its existing
file-byte SHA-256, and a second read rejects changed artifact bytes. Replacement tests use a
second valid corpus at the same repository revision, including replacement answers followed by
restoration of the original artifact. Restoring the file does not excuse misbound answers.

The checkout comparison now covers HEAD, read-only `git ls-files --stage -z` index entries and
actual tracked/nonignored-untracked worktree bytes and modes, in addition to porcelain. The
filesystem allowlist is regular files, symlink targets and absent tracked entries. Directories
at tracked paths, gitlinks and other special forms fail closed. Artifact files under `.git` are
covered separately. `git status` disables optional index writes. No write-tree is used. These
are before/after comparisons, not a claim to observe a transient mutation restored between
snapshots.

For rerun enforcement, the charter explicitly assigns the candidate gate to the readiness record
and forbids changing the env-gated test. Row 12 of `2026-11-18-rc4-readiness.md` is now NOT_RUN,
requiring configured qualification plus both ranking runs on candidate inputs. Skipped or missing
runs and changed trigger inputs retain HOLD. This is an explicit candidate-record gate, not an
implemented automatic promotion check; the orchestrator owns that disposition and independent
review. Historical porcelain digests alone do not prove checkout immutability.

## Regression evidence

The Task 2.2 spec list names the new negative tests:

- `TestRCFV1008LiveSymbolSpanFailsTheRun`: valid live-location baseline, then identical span
  corruption in all three receipt copies.
- `TestRCFV1008UnknownReceiptFormFailsClosed`: unknown root/result/evidence/live fields,
  unknown result variant, null result and unknown schema.
- `TestRCFV1008ArtifactReplacementFailsTheRun`: changed bytes, replacement answers, and
  replacement answers after original bytes are restored.
- `TestRCFV1008CheckoutMutationFailsTheRun`: dirty tracked content, untracked content, index
  entries and HEAD; every mutation preserves the original porcelain output.
- `TestRCFV1008UnknownCheckoutFormFailsClosed`: a directory replacing a tracked file is refused.

No shipped JSON shape, output golden or existing assertion changed. The first test run exposed
an incorrect new fixture assumption: its provider symbol was unresolved, so no live location
existed. The regression now constructs a valid pinned location and proves its baseline before
corrupting it. The repaired focused run passed every RCF test it ran; the unconfigured owner-run
test skipped. The existing profiling test failed independently because this sandbox exposes
`/home/russell/lanes/.git`, making the mandated TMPDIR appear inside a repository to PRB-V0-013.
The guard and its assertions remain unchanged.

## Final verification

- Toolchain assertion: PASS, go1.27.4.
- `corvint affected --base 55decc6a041a1df3f8ae43e95ba21cea669e7d80`: NARROWED,
  313 selected rows and 58 retained frontier unknowns. The touched docs package is selected by
  DIRECT_TEST_CHANGE; internal/specindex is selected too. Broader coverage is not claimed.
- Final whole `./cmd/corvint-docs` run through `script/heavy.sh`, with `go test -json -count=1
  -timeout 30m` and `CORVINT_RCF_QUALIFY` set: FAIL in 219.213 s. All RCF tests pass; only
  `TestPRBV0008EveryNamedDocsCommandWritesProfiles` fails for the sandbox path refusal above.
  There are 267 passing test/subtest events, one failure and one existing skip
  (`TestTrustedNavCLITrustBoundary`). No existing assertion or golden was altered.
- `TestRCFV1008Qualification`: PASS in 178.45 s, on the retained three-repository configuration.
  Corvint: 25 queries, 504 verified passages; Beamfall: 25, 570; Vue: 25, 886. Total: 75 queries,
  1,960 verified passages, zero unverified, zero failures. This qualifies these inputs with the
  repaired runner, not an unchosen candidate commit or the unrun twin ranking gate.
- Runner source SHA-256: `b2fb06db48e01a88ad4d2bf8dbc98fe58edf2ac89e1e52b107905c606972e2a4`. The run started from parent `ccaa6b6597b5db45784ed2f825b04bd16d391868`
  with this repair applied. Report schema and fields are unchanged.
- `script/check-line-citations.sh --repin`: PASS, no moved citation. `make -s
  spec-requirements-check line-citations-check`: PASS; internal/specindex 1.103 s. Both retain
  184 pre-existing skipped basename citations.
- `make -n dogfood-finish`: exit 2, no such target. This is a dry-run availability check;
  no bind or seal happened. CEM remains NOT_PRODUCED. The final dogfood-check attempt is retained
  in the lane handback.

Raw evidence stays outside the repository at `/home/russell/lanes/rc4-promo-evidence/`:

| File | Bytes | SHA-256 |
| --- | ---: | --- |
| `affected.json` | 256890 | `926665afa7b7b466031a43bd5996dd1bcfa047466620bf9e8238c775dd2ce909` |
| `package-test.log` | 3083 | `e2fb05847bcc52cdd90f1f2461fba6d02c714386c17850e589dd63f12d9f447e` |
| `focused-test.log` | 893 | `3b7d243812e99385c3ffcb230f2574e3400dcbec5213cf2ac6cbbcbe82be0a44` |
| `package-final.jsonl` | 301803 | `56508def4679afc84ac426b8dabef1cd0ccc66239225ff4c43d3082431d8641b` |
| `qualification-report.json` | 16725 | `51800e0dcdfb250e23acf0deab35e9a75a533283022b7f6ffb3f69bab92138bd` |
| `repin.log` | 190 | `dce4b7fe3eadc26103692787d6775a7701ed62b33f01ccc227f7182433b8ea7a` |
| `spec-citations.log` | 909 | `48b84d78723368bd82ceb8ceb6256526034fc18178bc4f85d6a9d6558aee3d89` |
| `bind-target.log` | 58 | `90a3afd47056e12c98b6ef08efc05b38deb080824a87904bd08d51ed95b0a791` |


## Rollback and remaining gates

Revert this repair commit to restore the earlier runner; doing so also restores the review's
known soundness gaps and does not authorize promotion. Production binaries and contracts are
unchanged. Current-candidate ranking qualification and independent review remain orchestrator
work. CEM binding and dogfood-check cannot run in this tree: `docs/DOGFOOD.md` records their
removal under decision 0594. No workflow was recreated and no seal was attempted.

## Fix-only re-review outcome

The fix-only re-review (gpt-6.1-sol, high, read-only, FIX_BASE `ccaa6b659`) confirmed that
live-span verification, artifact binding and checkout identity are fixed, each with a regression
test that would fail without the fix. P1-3, executable enforcement of the RCF-V1-010 rerun,
remains open. Two review-fix rounds have run, so it is rescoped rather than repaired again.
V1-5692 tracks a fail-closed release-candidate gate. Until that gate lands, readiness row 13 keeps
the rc.4 tag on HOLD.
