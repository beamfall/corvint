# ctx-w58: bounded spans, pre-budget merging and fixed-membership RRF

Base: `5d1ed52b17d61147bd8c04b08afa6da39fa80afb` (`origin/main`). Branch:
`claude/lane-ctx-w58-1103`. Human inputs: V1-0114, V1-0116 and V1-0219 in the lane charter.
New requirements: **TCP-V0-2220..2223**, accepted under the owner's standing auto-accept,
not an owner read-back or promotion. Delivery stays **experimental and opt-in**.

## Decision

**Keep all three experiments opt-in.** RRF improves the history-backed rank-1–5 gold count
by one on both sets while retaining final membership and top-20 gold. The full gate is still
unmet: three frozen releases are unavailable, the history replay contains invocation errors,
and frozen trace2code recall@5 falls under RRF. Exact-hit fallback loses gold-span coverage;
merged provenance increases six individual packets despite reducing aggregate bytes. These
are retained failed or incomplete evaluations, never promotion evidence.

The implementation now locates a hit before selecting its smallest indexed enclosure, returns
an explicit exact-hit fallback when the declaration/cumulative line budget cannot fit, and
coalesces the full ranked span candidate list before line truncation. Compact `contributors`
records retain every contributing slot and hit. RRF uses k=60 and first per-relation candidate
positions, breaks ties by admission slot then path, and only reorders the final rows selected
by corroboration. Reserved and graph positions remain fixed. No new product flag, command,
index encoding, persisted state or default dependency was added. Rollback unsets RRF and
selects span mode `on` or unsets spans.

## Frozen retrieval reading: MEASURED on trace2code, full gate incomplete

Input: `/mnt/d/Atlas-worker/data-arb-v2-trace2code-20260914T001809/extracted`.
The frozen release has 101 samples, 127 gold paths and 5072 annotated gold lines.
Sample SHA-256: `9d0ff50155fa4f65c1bbb632abc4f1f11393c50fa7b0a9249e06128b368b6266`.
Lane binary SHA-256: `1ea30ae6a3325d953fce2eb99e0a74759a706b81b666b0467b241cc9f259e042`.
The same binary serves every arm, with unrelated experiments removed from its environment.
The registered benchmark engine, source packet digests, raw captures, reports and fixed-budget
reading are retained under `.agent-evidence/ctx-w58/frozen/`; `summary.json` is the short reading.

| arm | recall@5 | recall@10 | recall@20 | MRR | total complete response bytes |
|---|---:|---:|---:|---:|---:|
| corroboration, spans off | .445545 | .516502 | .764026 | .345737 | 1808815 |
| spans `on` | .445545 | .516502 | .764026 | .345737 | 2296704 |
| spans `hits` | .445545 | .516502 | .764026 | .345737 | 2409454 |
| spans `merged` | .445545 | .516502 | .764026 | .345737 | 2407945 |
| RRF, spans off | .412541 | .488449 | .764026 | .346189 | 1909699 |

Every arm has zero errors and zero abstentions in the existing frozen scorer. Final membership
differs on **0/101** samples between corroboration and RRF. Macro recall@20 above is the
existing runner metric; the budget counts below are micro hit numerators, not that macro metric.

Fixed budgets count the **entire actual CLI response**, including its envelope and newline:
if it exceeds the budget, no evidence is delivered. This offline whole-message admission rule
adds no product flag and never truncates JSON. At 4096 and 8192 bytes no arm fits. At 16384 bytes
only spans-off corroboration fits (10 samples, 12/127 gold paths); RRF fits none, losing delivery
on those 10 samples. Neither span experiment fits at 16384 bytes.

| budget | span mode | delivered samples | file hits / 127 | span hits / 5072 | delivered bytes |
|---|---|---:|---:|---:|---:|
| 32768 | on | 100 | 91 | 357 | 2260447 |
| 32768 | hits | 100 | 91 | 193 | 2372369 |
| 32768 | merged | 100 | 91 | 193 | 2371332 |
| 65536 | on | 101 | 92 | 357 | 2296704 |
| 65536 | hits | 101 | 92 | 193 | 2409454 |
| 65536 | merged | 101 | 92 | 193 | 2407945 |

At 32768 and 65536 bytes, seven `hits` samples lose gold-span lines against `on`; micro coverage
falls from 357/5072 to 193/5072 and mean per-sample coverage from .080661 to .062397.
`merged` loses no file or span hits against `hits` at any measured budget. Its aggregate size
falls 1509 bytes, but six individual responses grow (largest +712 bytes), violating V1-0116's
never-rise criterion. The six sample IDs and exact costs remain in the per-sample report.
`code2test`, `comment2context` and `edit2ripple` are **NOT_RUN**, not zero-recall measurements.
The full four-release promotion gate remains unchanged and incomplete.

## Decision-0027 history-backed two-arm reading: MEASURED

History source: `/mnt/d/Atlas-worker/batch-driver-34d8605/source`; all 63 base commits exist.
Each task is checked out in a private history-sharing clone, preserving history and leaving
source inputs untouched. Both arms receive the identical original task, subject and limit 20.
The frozen v1 manifest is checked as an identical 34-task subset of the 63-task v2 manifest.

- v2 task SHA-256: `9bf5adab6b6018adb9949f8615ba640359d45e82c5358cd917b156312480d246`.
- v1 task SHA-256: `28846af5b521d91a16ba3b5d177f81c27637da3190db19cc4794520a37901050`.

| set / arm | samples attempted | gold 1–5 | gold 6–10 | gold 11–20 | gold top 20 | gold absent | invocation errors |
|---|---:|---:|---:|---:|---:|---:|---:|
| v2 corroboration | 63 | 47 | 51 | 52 | 150 | 531 | 15 |
| v2 RRF | 63 | 48 | 55 | 47 | 150 | 531 | 15 |
| v1 corroboration | 34 | 21 | 20 | 18 | 59 | 134 | 7 |
| v1 RRF | 34 | 22 | 21 | 16 | 59 | 134 | 7 |

Gold totals are 681 (v2) and 193 (v1). Gold absent at the pinned base is separately retained:
273 paths in v2 and 68 in v1; 408 v2 gold paths exist at base. No missing path is silently
renamed or removed from the denominator. Fifteen v2 tasks (seven in v1) have a subject path
absent at base, including `cmd/corvint/main.go`; both actual invocations fail and count zero
retrieved gold. These are errors, not abstentions. There are 48 successful paired v2 tasks
and 27 successful paired v1 tasks, with **zero membership differences**. The per-task revision,
history size, rankings, errors, packet digests and both all-gold and present-at-base buckets
are retained in `.agent-evidence/ctx-w58/history-final/reading.json`.

## Verification, authority and limits

The exact runnable commands and per-criterion disposition are in `ctx-w58.handback.md`.
Focused context/task-context tests pass in `internal/contextindex` and `cmd/corvint`; eight
Python scorer controls pass. Scoped vet, spec generation/checking, requirement collision check,
line-citation check and flow-guide check pass; changed Go files are gofmt-clean. Obligation
counts were checked before adding IDs: task-context-packet 91/256 -> 95/256; context-ranking
12/256 unchanged. Four new IDs have no collision across 42 open refs.

Pre-change context/query/impact and installed build-49 affected receipts are retained under
`.agent-evidence/ctx-w58/verification`. The affected plan remains UNKNOWN; this is focused
coverage, not repository-wide qualification. No `make gate` or exhaustive suite was requested.
No shell source changed; the bash-3.2 gate is not applicable.

The shared Git directory is read-only in this sandbox: normal staging and `dogfood-change`
refused writes there. Required staged checks use a private writable index/object store; Git
redirection is removed from test subprocesses so fixture repositories stay independent.
Citation checking required 14 mechanical line repins in core-compatibility-freeze-v1.
No commits, pushes, Tasks calls or Codex subprocesses were issued. Final CEM binding/sealing,
independent review, publication and native ticket completion are **NOT_PRODUCED**. This entry
records experimental implementation and measurements, not completed tickets, owner promotion,
external outcome validation or agent-time savings.

The raw frozen-evaluation outputs under .agent-evidence/ctx-w58/frozen/ (17 MB) are not committed; their digests stay in .agent-evidence/ctx-w58/SHA256SUMS and the E2 command above regenerates them.

## Review fixes

An independent Codex review of `83f066e38` raised three P2 findings. All three were confirmed and fixed.
The fixes are uncommitted on top of that commit.

- **Merged rows could exceed the 80-line cap (TCP-V0-2221).** `coalesceSpans` merged with no
  limit, so `rank` could emit a union longer than the per-span cap. In a fixture, two adjacent
  50-line declarations produced a 100-line row (lines 3–102). Now, under `merged`, `capMerged`
  checks each coalesced row before budget selection. If the row is past the cap, it falls back to
  the union of its contributors' hit ranges and states the reason. If that hit range also exceeds
  the cap, the row is counted as omitted. Every contributor is still listed. The modes `on`,
  `hits` and unset are unchanged. Test: `TestContextMergedRowCap_TCPV02221`.
- **Enclosure lookup was quadratic (TCP-V0-2220).** For symbols without an end line, such as Go
  declarations, `nextSymbolLine` rescanned the file's symbols from the start each time. It now
  uses a binary search over the symbols, which are already sorted by line. Test:
  `TestContextEnclosingManySymbols_TCPV02220`, which uses 200,000 declarations. It takes 0.04 s
  with the fix and still had not finished after 10 s without it.
- **The scorer ignored capture headers and footers (TCP-V0-2223).** `captures` now checks five
  things before scoring a capture:
  - the header profile;
  - that the header registration equals the report's registration exactly;
  - that the invocation identities match the registered sample IDs in order;
  - that each record has `COMPLETE` stdout with a matching byte count and digest;
  - that the footer is `COMPLETE`, with the true invocation count, `total_bytes` and prior-bytes
    SHA-256.

  The report check now also binds `registration.corvint_sha256` and `limit`. The new control
  rejects six bad captures: wrong binary, wrong arm, `PARTIAL` footer, missing footer, missing
  header and a tampered record. The old scorer accepted all six. All five retained trace2code
  captures still pass the new check (101 samples each), and a cross-arm capture is refused.

**Retained limit.** The frozen `merged` reading above came from the defective binary. Exactly one
retained `merged` packet has a row past the cap: sample `30a18c73cca02d3672705be7`,
`tokio/tests/sync_mpsc.rs`, lines 50–130 (81 lines). No `on` or `hits` packet has one. The
frozen arms were **not re-measured** after the fix, so the `merged` byte and coverage numbers
above describe the uncapped implementation. The decision is unchanged: every span mode stays
opt-in.
