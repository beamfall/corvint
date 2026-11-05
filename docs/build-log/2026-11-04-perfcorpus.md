# Read-path performance lane, 2026-11-04

Base: `d436dff76de1b8e75c427069ae7f80003ae99bc2`. Work proceeds in ticket order.
Measurements are Linux/amd64 WSL2, Ryzen 9 5950X, 32 logical CPUs, Go 1.27.3.
Shared-host load is recorded alongside raw samples; none qualifies a quiet-host target.
No publication, ticket-store writes, model-backed review or seal is part of this lane.
The independent review belongs to the orchestrator.

## V1-4230: allocation-bounded stamp capture

`DCR-V1-8000` preserves the existing stamp metadata and abstentions. Decision: use a
chunk-local directory descriptor with no-follow `newfstatat` on Linux/amd64 and a borrowed
metadata buffer; retain the portable `DirEntry.Info` reader elsewhere. Every entry is still
statted. No directory-mtime shortcut can detect an in-place descendant edit. Reserve record
buffers and compact the entry list in place before formatting.

Before: `BenchmarkDCRV1847CheckoutStamp`, 3 x 10 captures, 54.341 / 42.373 / 38.152 ms,
44.60–44.74 MB and about 206,800 allocations per capture. After (same fixture):
29.711 / 28.563 / 30.798 ms, 16.98–17.08 MB and about 97,700 allocations. Starting load1:
13.03 before, 11.59 after. This is a local improvement; the <15 ms quiet-host target is
NOT_QUALIFIED. Raw samples and subsequent final samples are retained with this entry.

`TestDCRV18000StampEntryReader` compares the exact records for regular files, directories,
symlinks, FIFOs and special permission bits, refuses vanished entries, and bounds allocations.
Replacing its stat loop with the original reader fails: 300 allocations per 100 stats (budget 1).
Focused existing stamp, coarse/skew and symlink tests passed (6.859 s package time).
Rollback: restore the portable reader and buffer growth; no persisted state changes.

## Workflow evidence and limits

`make dogfood-change BASE=<base>` ran before edits. Pre-change context/query/impact receipts
are bound in the worktree-private Git directory. The first impact call used directory paths
and refused `unsupported-impact-path-suffix`; the concrete-file retry succeeded.
The affected plan retains build-constraint and shell frontier uncertainty. Focused checks are
selected from it; the exhaustive repository gate is NOT_RUN by explicit lane instruction.

## V1-2588: worker allocation and canonical comparison

`DCR-V1-8001`: move each split batch's allocation to its worker, and compare claims and subjects
using a typed, reusable canonical encoder. The encoder uses `jsontext.AppendQuote`, emits every
field in declaration order, and is differentially tested against `Encode` over 1,000
reflection-populated records (controls, escapes, Unicode, nil/empty arrays, optional IDs and
negative integers). Other records retain `EncodeTo`. The closed decoder still validates every
record before canonical comparison, so an invalid later record outranks an earlier mismatch.
No disk state, artifact bytes, digest algorithm or admission rule changes.

Decode-only, same retained 66,752,193-byte fixture, 3 x 10: before 98.47 / 113.32 / 248.20 ms,
139–140 MB allocated and about 661,000 allocations; after 139.40 / 98.52 / 106.08 ms,
130–134 MB allocated and about 530,400 allocations. Load exceeded 75 before and heavy.sh waited
120 s; these unpaired wall samples establish no latency win. The paired encoder benchmark and
20-pair CLI experiment are recorded below. Existing refusal/parallel/streamed/index tests pass
unchanged (6.998 s); race results are retained separately. A first repair used a comparer helper
that expects a terminal newline for newline-free split elements; the existing tests caught it,
and the original length comparison was restored for those elements.

### Fresh-process corpus qualification (V1-2588, V1-2571)

Frozen attested artifact: 66,752,223 bytes, built by the clean BASE binary. Base binary at
`d436dff76de1b8e75c427069ae7f80003ae99bc2`; candidate at
`9631b0e42f950270b725de2652d328937a0094a8`. Twenty interleaved pairs per operation,
order reversed on alternate rounds, load1 55.70–64.31:

| Operation | Base / candidate process p50 ms | Base / candidate `--timings` p50 ms | Candidate max RSS MB |
| --- | --- | --- | --- |
| flow | 234.64 / 217.60 | 173.30 / 159.30 | 229.73 |
| gaps | 224.63 / 220.06 | 161.75 / 157.05 | 231.83 |

Both stdout digests match across all 40 calls per operation. Candidate peak RSS is below the
3.5x artifact bound (233.63 MB). The canonical microbenchmark's old encoder is 2,914 / 3,525 /
3,279 ns, 72 bytes and 2 allocations; typed encoder 937 / 1,054 / 944 ns, zero bytes and zero
allocations. Focused corpus race tests pass (101.570 s). No wall target is qualified under this
load; both tickets retain PARTIAL quiet-host qualification. The original DCR-V1-551..553/559
parallel scan is already on BASE, with adversarial strings/escapes/nesting and sequential/refusal
parity tests unchanged and passing. No redundant boundary scanner is introduced for V1-2571.

## V1-4199: retained /3 versus /4 format gate

The ticket's round-3 note predates delivered `IDX-SNAP-V0-811` (V1-4235). BASE already uses `/4`:
raw source bodies, leased deferred reads, copying and blob verification under the fault guard,
with no escaping alias. `/3` is a header-mismatch miss. The original accepted gate is retained in
`docs/specs/index-snapshot-v0.md` and `2026-11-01-rdp-w33.md` round 4; its 52% figure is absolute
decode CPU per call, not a halving of decode's share of total CPU.

Decision: measure the exact historical format pair again on this host, rather than invent another
encoding. Before `7472261d2eb671607202ec5edd8e8d89d985dc7c`, after
`af63abf4d4284f84a0273319da6a10dd1bc81fe4`, both clean binaries; frozen before tree
`a5cb39f85afc4d1995559983c75231a805c6b35b` (9,602 sources, 115,277,150 source bytes).
Scratch stores only. `/home/russell/lanes/perfcorpus-evidence/raw/measure.py` retains command resource samples, binary hashes,
root identity, load and stdout hashes. Cold means an empty derived store, not dropped OS pages.

| Measure | /3 | /4 |
| --- | --- | --- |
| Snapshot bytes | 186,663,394 | 186,683,152 |
| Cold index x3 p50 / p95 ms | 4,740.91 / 4,828.66 | 3,759.00 / 9,519.84 |
| Reuse index x3 p50 / p95 ms | 3,029.29 / 4,507.58 | 3,213.29 / 5,222.89 |
| Context x20 p50 / p95 ms | 1,017.76 / 1,266.70 | 932.27 / 1,095.64 |
| Context mean process CPU s | 1.6815 | 1.3930 |
| Context max RSS MB | 548.05 | 288.51 |
| User-prompt hook x10 p50 / p95 ms | 622.00 / 823.92 | 602.64 / 741.49 |

Context and hook stdout are byte-identical. Index receipts differ in their measured timings,
engine/store identity and the format's byte count, as expected; those are not packet output.
Load1: 16.36–51.37 for the timing rows, 13.60–13.71 for five interleaved CPU-profile pairs.
Aggregated parent-process CPU samples: /3 3.77 s total, body decode 1.68 s and trailer hash
0.48 s; /4 2.68 s total, body decode 1.09 s and trailer hash 0.18 s. Decode+hash absolute CPU
falls 41.2%, but its share falls only from 57.3% to 47.4% (17.3% relative). **The ticket's
halving-of-share gate is NOT MET on this run.** This is a completed losing spike, not a new
format promotion or a change to BASE's previously delivered `/4`. No new encoding ships.
Rollback for the delivered format remains the owning spec's `/3` inline-body writer and
whole-file trailer. Focused current `/4` parity, corruption, truncation and old-header tests
pass (1.197 s). Independent review remains with the orchestrator.

## V1-2746: share the successful Go parse

The linear reference/import scanners and their regex differential oracle are already on BASE.
The remaining duplicate parser work is removed under `GPK-V0-8000`: the collector takes imports
from the successful full symbol parse; a failed full parse still runs ImportsOnly independently.
Thus a malformed body preserves valid import edges and the original import-refusal semantics.
No source admission, approximation, analyzer output or snapshot format changes.

The new differential test covers every tracked Go source and malformed-import/body fixtures;
the allocation regression requires shared import extraction to allocate less than half as much
as a separate imports parse. Shared extraction matches independent extraction (4.582 s package).
The old path is retained in the paired benchmark: 3,719–4,947 ns, 3,456 bytes and 68 allocations
per Go source before; 2,150–2,203 ns, 2,040 bytes and 38 allocations after. Analyzer output check
passes with the existing schema pins. Whole-checkout race/profile results follow after commit.

### Whole-checkout build measurement

`TestCorpusSelfDocumentation` through heavy.sh `-race -cpuprofile`: BASE 75.67 s (package
77.263 s), candidate `150fa7c1f6f7a4f126fd7056287e33c40c967a47` 71.70 s (package 73.505 s).
Starting load1 26.06 / 10.94, so this pair does not establish a causal wall-time improvement.
Both CPU profiles are dominated by race instrumentation and map/posting work. Regexp's largest
self entry is about 1.18% after (rank 18), so it is no longer dominant, although an assertion
of no regexp in the first 20 rows would be false for this race profile. The original linear
scanner differential passes unchanged. The <60 s CI target is NOT_OBSERVED: no CI publication
is permitted here. V1-2746 remains PARTIAL on that external qualification.

## V1-4231: exact hook wall-stage profile

`PRB-V0-8000` adds fixed standard-Go trace regions, inactive without tracing, without a new CLI
flag, state, payload text, receipt field or deadline. It covers both the legacy harness and the
local-completion route. The opt-in fixture creates exactly 3,000 tracked files, modifies 700,
adds 767 untracked files and asserts 1,467 dirty paths. It refuses an existing fixture directory.
Twenty calls per event retain identical stdout within each event; `go test -trace` and
`perfcorpus/trace_regions.py` retain wall intervals, including overlapping reads.

Final trace, load1 19.63–21.83:

| Stage, p50 ms | User-prompt | Post-tool |
| --- | --- | --- |
| Complete in-process adapter call | 47.83 | 38.50 |
| Git status interval union | 44.99 | 35.02 |
| Git identity interval union | 6.69 | 4.90 |
| Snapshot acquisition | 6.63 | not used |
| Packet construction | 0.67 | not used |
| Git profile read | not used | 2.32 |
| Normalization | 0.004 | 0.045 |

These are inclusive/overlapping intervals, not additive serial costs. Status runs twice to
preserve opening/closing currentness. The 200–370 ms hook cost is not reproduced on this host;
the isolated single-status result from unraid cannot be substituted for these two WSL reads.
The dominant observed stage is the union of the two isolated status reads, not packet rendering.
Proposed follow-up for the orchestrator: **reduce the two-status union p50 to <=20 ms on this
exact 1,467-dirty-path fixture, preserving both observations and byte-identical output**.
Measure private-index preparation and racily-clean rehash work separately; do not remove a
closing observation or add fsmonitor state. Native follow-up filing is NOT_PRODUCED because
this lane explicitly forbids ticket writes. V1-4231 is PARTIAL only on that external write.
Existing adapter normalization, deadline, oracle, joined-corpus and kernel tests pass.

## V1-2477: current retained-artifact path and MCP evidence

The new canonical-worker optimization also serves these CLI reads. The 66.75 MB CLI samples,
RSS, stdout parity and host limitation are above: 159.3/157.05 ms `--timings` p50, maximum
231.83 MB (target <=3.5x artifact =233.63 MB); quiet-host wall qualification stays PARTIAL.
`DCR-V1-432` already records each remaining path's disposition:

| Path | Disposition and reason |
| --- | --- |
| corpus_integration | streamed verified artifact; generated-documentation receipt without authority |
| delta current side | verified read under DCR-V1-460..464; explicit rederive option and byte-identical receipt |
| flowdocs | rederive: generation manifest/check/finalize consume it as source-validated |
| flowcoverage | rederive: corpus inventory controls the pass/fail verdict |
| corpusindex | rederive: producer declares source-rederived-at-producer |
| republish | rederive: only full source verification can mint VerifiedCorpus |

Large-fixture MCP retest (`TestDCRV1416CorpusMCPFirstAndWarmCalls`, artifact 66,752,193 bytes,
load1 30.22): first call 308 ms; warm p50 62 ms, p95 107 ms, max 125 ms; last warmed stamp call
5 ms. Full rederivation 2.588 s, so the first-call 3x speedup assertion passes. These are loaded
host samples, not promotion of a quiet-host percentile target. The existing joined-artifact and
refusal tests pass; no producer is silently changed into an integrity-only consumer.

## Final loaded-host measurements and verification

Final code binary at `a4a974f4618f159f60d7ba2701d919019a90702b`, interleaved with clean BASE,
20 pairs per operation, identical stdout in every pair:

| Operation | Base / candidate process p50 ms | Candidate p95 ms | Candidate max RSS MB |
| --- | --- | --- | --- |
| corpus flow | 210.03 / 192.19 | 224.67 | 229.09 |
| corpus gaps | 205.17 / 204.75 | 236.77 | 231.78 |
| user-prompt hook | 100.39 / 91.67 | 101.09 | 45.55 |
| post-tool hook | 68.52 / 69.20 | 75.06 | 39.62 |

Corpus `--timings` candidate p50: flow 140.30 ms, gaps 154.45 ms. Mean CPU seconds per process:
flow 2.0505 -> 1.7395; gaps 1.9540 -> 1.7570. Corpus load1 18.93–20.46, hook load1
19.80–21.76. These measurements do not qualify quiet-host targets. The final stamp benchmark
(three means of ten captures): 26.277 / 26.527 / 43.073 ms, 16.88–17.04 MB and about 97,600
allocations per capture; before was 38.152–54.341 ms and about 44.7 MB / 206,800 allocations.
The <15 ms stamp target remains unqualified.

Focused units, the corpus race suite, snapshot refusal/parity witnesses, the self-documentation
race/profile experiment, analyzer output freeze, spec requirements/capacity checks, line-citation
check, targeted Go vet and Windows stamp-package vet pass. `make gate`, exhaustive Go tests/vet,
full Windows vet, the known failing batch precheck, independent model-backed review, publication,
native ticket writes and sealing are NOT_RUN under the explicit lane instructions. Independent
review belongs to the orchestrator. No ranking/learning behavior changes, so the learning-weight
training/held-out campaign is excluded; exact analyzer and corpus parity are the relevant frozen
checks here. Native ticket state is unchanged; lane statuses below do not claim native completion.

## Friction and bounded repairs

1. Default heavy.sh lock path `/home/russell/bench/rt/.locks` is read-only in this sandbox.
   Repro: the initial stamp benchmark through heavy.sh. Use the existing shared
   `/tmp/corvint-heavy-1000` lock pool and writable logs, retaining eight host-wide slots.
2. `dogfood prechange` rejects directory impact operands with `unsupported-impact-path-suffix`.
   Repro: PATHS=`internal/mcp/corpusbridge internal/doccorpus internal/contextindex internal/hook`.
   Retrying concrete `.go` paths before edits produced all three bound receipts.
3. Host load rose to 75 while collecting the decode baseline; heavy.sh waited its full 120 s.
   Repro and raw queue/load data: `decode-before.txt`. Quiet-host and CI targets remain external.
4. Go test profile flags leave a package test executable in the working directory unless `-o`
   is supplied. Repro: the first stamp CPU profile left `corpusbridge.test`, making affected bear
   unrelated language frontiers. It was moved to the evidence directory; later profiles use `-o`.
5. The ticket's V1-4199 round-3 note predates delivered `/4`; its 52% original gate value is
   absolute decode CPU, not CPU share. Repro: compare the ticket note with IDX-SNAP-V0-811 and
   the new aggregated parent-process profiles. Preserve the losing current share result.
6. Hidden adapter dispatch rejects a global `--root` before `adapter`, even while its error's
   valid-choice list contains `adapter`. Repro: the failed last row of `snapshot-gate.jsonl`.
   The harness now supplies the root through the adapter's own host environment.
7. Sandbox read-only `.git` marker directories under every writable root make profile path
   admission report `/home/russell/lanes` and `/home/russell/.cache` as Git worktrees. `/var/tmp`
   is read-only. Repro: CORVINT_CPUPROFILE under either root. Capture to an inherited anonymous
   Linux memfd, then retain its bytes after the child exits; the production guard is unchanged.
8. A minimal archive Git directory builds with Go but fails Corvint's reciprocal worktree and
   alternates contract. Repro: the first self-corpus baseline reports missing
   `<private-git-dir>/corvint/perf-base.git/gitdir`. Giving the archive its reciprocal back-pointer
   and relative commondir to the owned object store fixes it, entirely under the permitted
   `worktrees/perfcorpus` directory; no common metadata files were written.
9. `go tool trace -d=1` is obsolete in Go 1.27.3 (`want parsed, wire, footprint`). Use
   `-d=parsed`; retaining only task/region lines avoids a 245 MB debug projection from a 1.3 MB
   trace. The raw final trace and extracted intervals are both retained.
10. The shared encodingComparer.equal helper assumes a trailing newline, unlike split records.
    Existing refusal/parity tests caught that misuse immediately; the newline-free comparison
    retains its original byte count check. Likewise collector nil/empty slices require value
    comparison rather than a representation-only DeepEqual in the new differential test.
11. Spec regeneration warns about pre-existing specs at the 256-obligation cap and moves README
    citations. Repro: `make spec-requirements`; the new requirements stay in their owning
    under-cap specs, and the indicated line citations were repinned in the same commits.
12. `check-line-citations.sh --changed --base BASE` is invalid (`--base needs --repin`);
    `--changed` is a repinning selector. The ordinary check takes no arguments and passes.
13. A cleanup command containing `rm -rf` was rejected before execution. No file was written by
    that command. The measurement harness was added with apply_patch and syntax-checked without
    creating a bytecode-cache directory; no cleanup bypass was needed.
14. Native follow-up filing and completion writes, CI/PR publication, sealing, and independent
    review are prohibited or assigned to the orchestrator. They stay explicit handback obligations.
15. CEM preparation refuses an empty successful-command log as
    `metadata-only-change-unsupported empty-file-creation`. Repro: the first citation draft
    named `perfcorpus/analyzer-output.txt`. It now explicitly records command, exit 0 and empty
    stdout/stderr; no benchmark sample was changed.
16. Final line-citation validation found 40 moved/changed source references, beyond the README
    references the generator named. `--repin` fixed 30; diff-mapped `--repin --changed --base`
    fixed seven insertion-spanning pins after reading the changed functions. Three unanchored
    error-code references required explicit relocation to their actual error statements.
    The earlier `line-check-final.txt` is the retained failure report; the passing final check
    is `line-citations-final.txt`. These repairs change references only.
17. A local commit emitted a failed attempt to create
    `/home/russell/projects/corvint/.git/packed-refs.lock`, which is outside the writable Git
    metadata allowlist. The commit itself succeeded. The warning persisted even with automatic
    maintenance disabled per invocation; its internal cause is NOT_DIAGNOSED. No packed-refs
    file or repository configuration is changed.
18. Relocating the three old unpinned error-code references also requires pinning the new
    references and deleting their old rows from `script/line-citation-unpinned-inventory.txt`.
    Repro: the checker reports both new unpinned citations and stale inventory rows. This is
    repaired with exact source-statement hashes and deletion-only inventory maintenance; the
    final staged citation check now passes (180 basename-only historical references are skipped
    by the existing checker policy).

### Own-builder performance protocol

The common-artifact runs above isolate exact stdout parity. DCR-V1-430's performance protocol
also requires each binary to read its own built artifact, so a separate final 20-pair run uses
`corpus-attested.json` from BASE and `corpus-head.json` from candidate `a4a974f46`. Both are
66,752,223 bytes; decoded contents differ only in builder revision and the derived content digest.
Consequently their receipts have different artifact provenance and are not compared for byte
parity; common-artifact parity remains independently demonstrated above.

Own-builder flow: process p50 199.97 -> 189.79 ms, candidate `--timings` p50 137.85 ms and peak
RSS 229.49 MB. Gaps: process p50 207.86 -> 188.19 ms, candidate `--timings` p50 138.90 ms and
peak RSS 232.50 MB. Load1 10.86–13.19. Both RSS values remain below 233.63 MB. Even though the
process medians are below 200 ms in this local run, quiet-host targets remain unqualified under
the lane's stricter host rule. Raw own-builder samples name their deliberately incomparable
artifact-provenance digests rather than silently normalizing them.

### V1-2746 final profile repair: document regex prefilters

Final review of the retained race profile found `documentRecord` still owning 9.11 s of regex
CPU. Rather than leave that observed internal cost behind an external CI limitation,
`GPK-V0-8001` adds necessary-condition title/heading/status gates. The original regexes still
make every matching decision, including Unicode whitespace and long-s case folding; an already
found title is not searched again. The preserved prefilter-free parser is an independent test
oracle over 1,014 tracked self-corpus documents and adversarial edge fixtures, with identical
full records. The backend-call regression uses 2,000 prose lines: restoring the original
unfiltered path fails at 2,004 title / 2,004 heading / 2,003 status calls; fixed counts are
1 / 2 / 1. This tests resource work without a flaky elapsed-time threshold.

Same-host benchmark, load1 7.25 at the start of this measurement round, three means of 20:
original 3.145 / 3.231 / 3.366 ms per document; gated 0.376 / 0.383 / 0.398 ms. Functional
parser, edge and budget checks pass (0.760 s), and analyzer output freeze remains unchanged.
The whole-checkout race profile below determines the remaining regex share and timing; this
microbenchmark is not a CI or quiet-host wall qualification.

### V1-2746 final whole-checkout result

At `fee2c39c21113dc86e50516ccc83f146b91191e9`, the full self-documentation test passes with
`heavy.sh -race` in 46.32 s (package 47.917 s), start load1 7.07. This is a measured local result
below 60 s, not CI evidence and not a causal wall comparison with the loaded 75.67 s baseline.
The new parser's focused race tests also pass (14.507 s); analyzer output and index vet pass.

The production (non-race) whole-checkout profiles, measured before/after through heavy.sh on
this host at starting load1 9.63, show the first regexp self entry moving from rank 16 (1.39%)
to rank 21 (1.09%): no regexp entry in the first 20 production self-cost rows. Native test times
are 9.55 / 9.22 s. The race profile remains dominated by instrumentation; there regexp is rank
14, so the production and instrumented profiles are retained separately. In the race profile,
documentRecord's regexp CPU falls from 9.11 s at the prior shared-AST head to 3.80 s after the
prefilters. Remaining regex work includes fence, marker and lexical-term handling. There is no
claim that every regex disappeared. The only remaining V1-2746 acceptance blocker is actual CI
qualification, which this no-publication lane cannot produce.

Corpus/hook qualification samples above remain bound to their named `a4a974f46` binary; the
later change only prefilters index document parsing, with full-record equality and frozen
analyzer-output evidence, and those earlier samples are not reattributed to this later commit.

The final obligation-link pass required literal numbered requirement IDs inside each
re-extractable test claim. Added named subtests for DCR-V1-8000, DCR-V1-8001,
GPK-V0-8000, GPK-V0-8001 and PRB-V0-8000, then reran the affected focused tests
through the shared heavy runner. All passed (raw `ocm-*-tests.txt`); production
behavior and measured samples are unchanged. This supplies source linkage, not a
formal guarding-test execution receipt or independent review.


## Fix round 1: bounded review repairs

The owner assigned only five review findings. DCR-V1-8000 now keeps the owning
`*os.File` live immediately after each descriptor-relative syscall; the release callback
is named `release`. GPK-V0-8000's allocation guard now exercises `compiledSources.collect`
and compares it with the same collector excluding imports plus an independent import parse.
This guards actual integration as well as the helper. Git-dependent GPK self-corpus parity
checks skip with an explicit reason when git is absent from PATH. No runtime contract,
format or performance threshold changes. Rollback: revert these review repairs; all original
lane performance claims keep their original measured revisions and host limitations.

Decision: retain raw evidence privately rather than in the source diff. All 83 files actually
present at review head `155185098279c1183e6abb188beb8337dbe27c78` (1,976,154 bytes)
were moved byte-for-byte into the directory below; the review's 92-file estimate differs from
the inspected tree. The source diff removes the entire `docs/build-log/perfcorpus/` directory.
The inventory gives each file's SHA-256, exact private path and supported result. Historical
raw rows may name their original capture locations; this relocation does not rewrite them.
These private paths are local retention, not portable evidence for a fresh clone.

Ownership: V1-4230, V1-2588, V1-2571, V1-4199 and V1-2746 remain as committed, with the
original measured failures, host/CI limits and no native completion claims. V1-2477 is MET
by the orchestrator's ownership decision: own-builder 20-pair process flow/gaps p50
189.79/188.19 ms, timings 137.85/138.90 ms, peak RSS 232.50 MB <233.63 MB;
large MCP first call 308 ms, warm p50/p95 62/107 ms, stamped call 5 ms, full rederive
2.588 s and passing first-call 3x assertion. This does not establish quiet-host qualification.
V1-4231 remains PARTIAL: the orchestrator must file the native follow-up to reduce the union
of the two isolated status reads to <=20 ms p50 on the exact 3000 tracked / 700 modified /
767 untracked fixture, retain opening/closing identity and status checks and byte-identical
receipts, and attribute private-index preparation and racily-clean rehash separately.
Native task writes are prohibited in this lane.

Fix-round prechange retry with concrete source paths refused `prechange-after-change`
because HEAD already differs from the original lane BASE. Existing BASE-bound prechange
receipts remain OBSERVED; no fabricated replacement receipt was created. The initial
`make dogfood-change` ran before repairs and reported missing bind inputs as expected.
Independent review remains with the orchestrator. Validation and final checked CEM identity
are recorded in the fresh private handback; no seal or publication is authorized.

Fix-round validation: the collector case passes at 43 versus 73 allocations (independent
imports: 32); restoring only the collector's independent parse makes it fail at 73 versus 73.
The optimized collector was restored byte-for-byte. Both GPK tracked-corpus tests skip with
explicit reasons under an empty child PATH. Broad existing checks pass for contextindex
(37.395 s), corpusbridge (34.951 s), doccorpus (9.798 s) and gokernel (expanded probe checks:
1.572 s). All 18 Go files touched by the cumulative lane diff pass `gofmt -l` with empty output.
Generated requirements, spec capacity, line citations and frozen analyzer output pass.
The broader command-package run fails in the existing JavaScript host harness: four of 81
Node subtests expect scratch paths outside Git, but `/home/russell/lanes/.git` is a read-only
sandbox marker above mandatory TMPDIR. Reproduce with the broad `TestHostAdapter` pattern;
native adapter checks pass separately (2.364 s). This environment failure is retained, not
silently treated as passing or repaired by changing unrelated adapter behavior. Formal
guarding-test receipts remain NOT_PRODUCED because their runner bypasses the required
heavy.sh execution route. Quiet-host, CI and other-platform qualifications are NOT_RUN.

### Private raw evidence inventory

| SHA-256 | Private path | Key numbers / result supported |
|---|---|---|
| `1461966faedd328d4ee7d1c1aa672cdfbfff1bf98745988b1dbfcd7193e52f15` | `/home/russell/lanes/perfcorpus-evidence/raw/analyzer-output.txt` | Frozen analyzer output/schema check exit 0 with empty stdout/stderr; no schema bump. |
| `ca64fe979000afb8bc4c301108e995ebff222786d1f1c4f75a7fb2c477b47f28` | `/home/russell/lanes/perfcorpus-evidence/raw/build-tests-final.txt` | ok   github.com/Beamfall/corvint/internal/contextindex 16.037s |
| `4a1f6f065b419d8c84d7e2bed63c311354da6620252006ecb1e9a25190d92d63` | `/home/russell/lanes/perfcorpus-evidence/raw/canonical-benchmark.txt` | BenchmarkDCRV18001CanonicalComparison/Typed-32               10000        943.9 ns/op        0 B/op        0 allocs/op; PASS; ok   github.com/Beamfall/corvint/internal/doccorpus 0.204s |
| `1d919f6a5322d3e4a5a7bfd4fe2f57780de020cf4be565f890e3cf0d23e93e45` | `/home/russell/lanes/perfcorpus-evidence/raw/cmd-focused.txt` | ok   github.com/Beamfall/corvint/cmd/corvint 2.059s |
| `8ca88d316c2fee6c2f98df3696c67bbdb3fc18eefe9548eea5839bee225a184e` | `/home/russell/lanes/perfcorpus-evidence/raw/corpus-cli-pairs.jsonl` | 20 common-artifact pairs/op: candidate flow/gaps process p50 217.60/220.06 ms; identical stdout per operation. |
| `34706902d4c566be8bfe981f7fa3d3925acc7b5a3085213de8b4717630882315` | `/home/russell/lanes/perfcorpus-evidence/raw/corpus-final.jsonl` | 20 common-artifact pairs/op: flow p50 210.03 -> 192.19 ms, gaps 205.17 -> 204.75 ms; head max RSS 231.78 MB; byte parity. |
| `3a71306c96a7c4bf1fdcbd30815ded104cc05892c04b5164e89f86fe872061dd` | `/home/russell/lanes/perfcorpus-evidence/raw/corpus-own-builder.jsonl` | 20 pairs/op: flow/gaps candidate process p50 189.79/188.19 ms; timings 137.85/138.90 ms; RSS <=232.50 MB. |
| `3c16fbba892f315020d03f1fdaf87c9414f495c5a15af647de8ea5c42601ca39` | `/home/russell/lanes/perfcorpus-evidence/raw/decode-before.load` | Host-load qualification: 73.12 35.61 26.58 81/2132 107 |
| `a74390bd8d85e0faf3fb4de613e497d0d3f1f368dbadb73c711c0b0683a648a2` | `/home/russell/lanes/perfcorpus-evidence/raw/decode-before.txt` | BenchmarkDCRV18001ArtifactDecode-32           10  248204725 ns/op 138228552 B/op   660248 allocs/op; PASS; ok   github.com/Beamfall/corvint/internal/doccorpus 4.701s |
| `c08702922961f402dbe03658eb1823feb68b4c1c38ba0f710ba7f21b9c26c83e` | `/home/russell/lanes/perfcorpus-evidence/raw/decode-final.load` | Host-load qualification: 32.96 38.21 30.17 6/1409 3 |
| `d91c61d4eaf9b725c51d091a11c02ffd25a7e31e01eb1ce77bcb15bbdcccba99` | `/home/russell/lanes/perfcorpus-evidence/raw/decode-final.txt` | BenchmarkDCRV18001ArtifactDecode-32           10  106082932 ns/op 134038677 B/op   530532 allocs/op; PASS; ok   github.com/Beamfall/corvint/internal/doccorpus 3.536s |
| `283eac218941167677309fd1d521951082d30d9cecc1c7335fc525a5ecc43513` | `/home/russell/lanes/perfcorpus-evidence/raw/decode-race.txt` | ok   github.com/Beamfall/corvint/internal/doccorpus 101.570s |
| `ab28ae958419d7256b4fdf7251267ad4a14846df4cd4e49a13a01c7572f0e6fb` | `/home/russell/lanes/perfcorpus-evidence/raw/decode-tests-fixed.txt` | ok   github.com/Beamfall/corvint/internal/doccorpus 6.998s |
| `fb41958e916b2195d5d3bf3899c6984e7038446e7bdfb483da28707322547bdc` | `/home/russell/lanes/perfcorpus-evidence/raw/document-prefilter-benchmark.load` | Host-load qualification: 7.25 7.96 12.60 6/1193 3 |
| `eec9e560a0ceda7d80abcc5d2786697b37986ca12b34322b56ceb87f9c636f4a` | `/home/russell/lanes/perfcorpus-evidence/raw/document-prefilter-failing-first.txt` | Original path fails at title/heading/status regex calls 2004/2004/2003 versus budgets 1/2/1. |
| `fc0e2f9a5b537a0671de0cb74f0127fa1f4889961939b908bc958b8aa739304b` | `/home/russell/lanes/perfcorpus-evidence/raw/document-prefilter-final-benchmark.txt` | BenchmarkGPKV08001DocumentRecord/Prefiltered-32             20     398406 ns/op    33659 B/op       15 allocs/op; PASS; ok   github.com/Beamfall/corvint/internal/contextindex 0.242s |
| `4d2b42983744e53b5e473428685d5f65bd5331a7e3b57f2b995d74fbbcb94082` | `/home/russell/lanes/perfcorpus-evidence/raw/document-prefilter-final-pass.txt` | --- PASS: TestDocumentTitleUsesFirstH1WhenItEqualsStem (0.00s); PASS; ok   github.com/Beamfall/corvint/internal/contextindex 0.760s |
| `c106b3b1dd44a3319b38d81d5ebdf1e1f7be540816b128de633af0b2e0a128ad` | `/home/russell/lanes/perfcorpus-evidence/raw/document-prefilter-race.txt` | ok   github.com/Beamfall/corvint/internal/contextindex 14.507s |
| `f0e0c23d87193947e1521db729e390fcd53b9d30988811dd740867f26699847d` | `/home/russell/lanes/perfcorpus-evidence/raw/hook-cli.jsonl` | 20 fresh-process pairs/event: prompt p50 100.39 -> 91.67 ms, post-tool 68.52 -> 69.20 ms; byte parity. |
| `61186eacd518a42f2e822950db004e76dc0ef45d19b5fd57737e23afde345cb4` | `/home/russell/lanes/perfcorpus-evidence/raw/hook-final-regions.txt` | Parsed trace projection supporting the 20-call inclusive unions: prompt status 44.99 ms, post-tool status 35.02 ms p50. |
| `1a526cad1f37c4246911e03fd74ff02c17f4dacf0696854d32de49ebeb0f4aff` | `/home/russell/lanes/perfcorpus-evidence/raw/hook-final-stages.jsonl` | 20 calls/event: prompt wall/status p50 47.83/44.99 ms; post-tool 38.50/35.02 ms; inclusive unions. |
| `2ef19b241cd9f62f0427aa539268cfdda331f5afef7a3522235f9acbdbe46d4d` | `/home/russell/lanes/perfcorpus-evidence/raw/hook-final.txt` | --- PASS: TestPRBV08000HookStageProfile (3.29s); PASS; ok   github.com/Beamfall/corvint/cmd/corvint 3.328s |
| `865bb5f54fe53780759afd29c1856400994b0f89e4f3f95e8375f140e0cfc600` | `/home/russell/lanes/perfcorpus-evidence/raw/kernel-focused.txt` | ok   github.com/Beamfall/corvint/internal/gokernel 0.209s |
| `8c85a2d6966362db8098f35eeedef0a821e5d2f398e704c0ff3442e5a169c2d9` | `/home/russell/lanes/perfcorpus-evidence/raw/line-check-final.txt` | Line-citation diagnostic/check; line citations: these results read the staged (Git index) copy of every file, not the worktree; git add an edit to check it line citations: note: skipped 180 citation(s) whose basename is not tracked at repository root (--verbose lists them) |
| `855a4551b199f407e4e7cec705511746022a39aa0f70383fc13aac9acc748bba` | `/home/russell/lanes/perfcorpus-evidence/raw/line-citations-final.txt` | Line-citation diagnostic/check; line citations: note: skipped 180 citation(s) whose basename is not tracked at repository root (--verbose lists them) |
| `353c0a9e0eec50f31f491be1c89a49f359747827a91b9bcd0a16ea35cf4e2a22` | `/home/russell/lanes/perfcorpus-evidence/raw/mcp-large.load` | Host-load qualification: 30.22 28.94 29.44 6/1442 3 |
| `5bc74fdaf94f2d4781361e048471094b33f36c98a0d8e7c52397e0448c23e04e` | `/home/russell/lanes/perfcorpus-evidence/raw/mcp-large.txt` | 66752193-byte artifact: first 308 ms, warm p50/p95 62/107 ms, max 125 ms, final stamped call 5 ms; full rederive 2.588 s; 3x assertion passes. |
| `a00b0587ecb78bb720d722bf5ddf5b45640233fdd4b29e412dde590225efbf9e` | `/home/russell/lanes/perfcorpus-evidence/raw/measure.py` | Reproduction harness for the /3-vs-/4, corpus and hook paired experiments; no measurement by itself. |
| `a08c41c0c5759e19fa826d419b2e4a9255491a87ddb0643f5a8b8383d7d2971b` | `/home/russell/lanes/perfcorpus-evidence/raw/ocm-canonical-tests.txt` | ok   github.com/Beamfall/corvint/internal/doccorpus 0.019s |
| `da1158ff8b6644f434790058fe0d2233e904e7a8014dbbef2b436e8098384faf` | `/home/russell/lanes/perfcorpus-evidence/raw/ocm-hook-tests.txt` | --- PASS: TestPRBV08000HookStageProfile/PRB-V0-8000_profile (2.41s); PASS; ok   github.com/Beamfall/corvint/cmd/corvint 2.457s |
| `efed2d075b78c3a142aebcbe3c94efbb72ad7894737fe75a7e4b3a5b1b369fbd` | `/home/russell/lanes/perfcorpus-evidence/raw/ocm-index-tests.txt` | ok   github.com/Beamfall/corvint/internal/contextindex 5.156s |
| `5554746d610501f0bace5e0b8f15aaf87d9b7aee2a62fc32e9e958d1c223fb17` | `/home/russell/lanes/perfcorpus-evidence/raw/ocm-stamp-tests.txt` | ok   github.com/Beamfall/corvint/internal/mcp/corpusbridge 0.020s |
| `041db3f6cd94fdabf7f4b593e981337ff77676ca6aadd48ad1bb8bb37d73a191` | `/home/russell/lanes/perfcorpus-evidence/raw/profiles/base-context-0.cpu` | One of five base context CPU profiles: aggregate 3.77 s total, 1.68 s decode + 0.48 s hash; share 57.3%. |
| `c101c0d04f82d7d9eb36a3f2b27bf96a0ac075c0012ae3401f7c708a9ccf07e1` | `/home/russell/lanes/perfcorpus-evidence/raw/profiles/base-context-1.cpu` | One of five base context CPU profiles: aggregate 3.77 s total, 1.68 s decode + 0.48 s hash; share 57.3%. |
| `0d4eec2f0df2304161990e95d99d5df307c18896bf7ad2d98c083f0e9ef9ad07` | `/home/russell/lanes/perfcorpus-evidence/raw/profiles/base-context-2.cpu` | One of five base context CPU profiles: aggregate 3.77 s total, 1.68 s decode + 0.48 s hash; share 57.3%. |
| `d8c7735f42c35c2458b1e12c3d0dd83ca48fa17c7664aea13283762e78f3198b` | `/home/russell/lanes/perfcorpus-evidence/raw/profiles/base-context-3.cpu` | One of five base context CPU profiles: aggregate 3.77 s total, 1.68 s decode + 0.48 s hash; share 57.3%. |
| `7fb6f02fe881a6a7d5da304012f5ef3934b664543ba11e3eee4998f464260c62` | `/home/russell/lanes/perfcorpus-evidence/raw/profiles/base-context-4.cpu` | One of five base context CPU profiles: aggregate 3.77 s total, 1.68 s decode + 0.48 s hash; share 57.3%. |
| `5794c4daaebe976e1fcadf303756eae3f4f76a30806bac636ae156d59a16f7e6` | `/home/russell/lanes/perfcorpus-evidence/raw/profiles/head-context-0.cpu` | One of five head context CPU profiles: aggregate 2.68 s total, 1.09 s decode + 0.18 s hash; share 47.4%. |
| `a6278cb146cd51410124260510fdadc95b7d86965ff7715326ed99d4056a6fe8` | `/home/russell/lanes/perfcorpus-evidence/raw/profiles/head-context-1.cpu` | One of five head context CPU profiles: aggregate 2.68 s total, 1.09 s decode + 0.18 s hash; share 47.4%. |
| `b474ec73bf016008fefc866683968098d17c36b430122780d3dbdb27d9d2f072` | `/home/russell/lanes/perfcorpus-evidence/raw/profiles/head-context-2.cpu` | One of five head context CPU profiles: aggregate 2.68 s total, 1.09 s decode + 0.18 s hash; share 47.4%. |
| `7779e7eac8cc3328fcd7a8ac5b7025d50b286c84ffb9bd00c23988644b8bd22a` | `/home/russell/lanes/perfcorpus-evidence/raw/profiles/head-context-3.cpu` | One of five head context CPU profiles: aggregate 2.68 s total, 1.09 s decode + 0.18 s hash; share 47.4%. |
| `00926a51d868977e25f5a58b88d465555ad1e4d64ad233b3eccddc1932932ff6` | `/home/russell/lanes/perfcorpus-evidence/raw/profiles/head-context-4.cpu` | One of five head context CPU profiles: aggregate 2.68 s total, 1.09 s decode + 0.18 s hash; share 47.4%. |
| `a7244b24266fcf1a94b1b6bb346f21aa7031f5d680b6fd7f11120e1fec33cbc9` | `/home/russell/lanes/perfcorpus-evidence/raw/profiles/hook-final.trace` | Raw trace supporting 20 calls/event: prompt wall/status 47.83/44.99 ms; post-tool 38.50/35.02 ms p50. |
| `879fba44fdd0b3e18fc586bc538a19b08d7c5b945a9d2d2e2bb3686ae379d7e2` | `/home/russell/lanes/perfcorpus-evidence/raw/self-after-top.txt` | Shared-AST self-documentation race CPU profile: test 71.70 s, package 73.505 s; regexp rank 18. |
| `4f90024102149a2d340eab1df7c88c71e415305c877ab85869ce22cb5968e642` | `/home/russell/lanes/perfcorpus-evidence/raw/self-after.load` | Host-load qualification: 10.94 22.10 27.77 36/1439 170 |
| `fe14d1d3e6883712690ad7937cf9b3f973732e0a3d79d10add8f4c8809531df1` | `/home/russell/lanes/perfcorpus-evidence/raw/self-after.txt` | --- PASS: TestCorpusSelfDocumentation/DCP-V1-019_self (71.70s); PASS; ok   github.com/Beamfall/corvint/internal/doccorpus/selfcorpus 73.505s |
| `6df0b118152614b28a3baf3f5c5931fef9c7f65e606792e8688a4c6e0cca2c62` | `/home/russell/lanes/perfcorpus-evidence/raw/self-before-fixed.load` | Host-load qualification: 26.06 29.16 30.65 9/1383 5 |
| `dcc6bb46961c4e94a2dbad4548fb311bd678af5c643a4126b71126f0c4ba4e76` | `/home/russell/lanes/perfcorpus-evidence/raw/self-before-fixed.txt` | --- PASS: TestCorpusSelfDocumentation/DCP-V1-019_self (75.67s); PASS; ok   github.com/Beamfall/corvint/internal/doccorpus/selfcorpus 77.263s |
| `d91bea8bcc901526e9b65d28ebb8d5b8740caa07cc5c8d19042d515e73e7c1fd` | `/home/russell/lanes/perfcorpus-evidence/raw/self-before-top.txt` | BASE self-documentation race CPU profile; race test 75.67 s, package 77.263 s. |
| `0ccf9acd37661f0eeba3af386bbe7cee0d7921a3fb33bcb2e283145022b01981` | `/home/russell/lanes/perfcorpus-evidence/raw/self-native-after-top.txt` | Production profile after prefilters: first regexp self rank 21, 1.09%; test 9.22 s. |
| `2c1c5fb32b3e3842f533466d69ce92c39b7fce040637e53c420426d205d85363` | `/home/russell/lanes/perfcorpus-evidence/raw/self-native-after.txt` | --- PASS: TestCorpusSelfDocumentation/DCP-V1-019_self (9.22s); PASS; ok   github.com/Beamfall/corvint/internal/doccorpus/selfcorpus 9.357s |
| `efdb91f55a0eb95ac4878d49d8cbb3b2845c4ec2f12ab7a71f8a0973b3efd909` | `/home/russell/lanes/perfcorpus-evidence/raw/self-native-before-top.txt` | Production profile before prefilters: first regexp self rank 16, 1.39%; test 9.55 s. |
| `526f1c24625afb66d57b2f26072246e013bbf3c283d598c4ad947ee3810a3667` | `/home/russell/lanes/perfcorpus-evidence/raw/self-native-before.txt` | --- PASS: TestCorpusSelfDocumentation/DCP-V1-019_self (9.55s); PASS; ok   github.com/Beamfall/corvint/internal/doccorpus/selfcorpus 9.697s |
| `5c47ee12b03bb28bbfb01dce54bea0adf5491283563538a372eec1708a10c1f9` | `/home/russell/lanes/perfcorpus-evidence/raw/self-native-profile.load` | Host-load qualification: 9.63 10.74 12.05 4/1185 3 |
| `c1f9be0200b3c7dca113c181e278c7e70bd2be8be355e5373b8f6668749e1c30` | `/home/russell/lanes/perfcorpus-evidence/raw/self-prefilter-after-regexp.txt` | Race documentRecord regexp CPU 9.11 -> 3.80 s after prefilters. |
| `25afc54baeb8fc322d63d8d2a11192593da06cd6be25e64a7623271b79fe3d86` | `/home/russell/lanes/perfcorpus-evidence/raw/self-prefilter-after-top.txt` | Prefiltered race CPU profile; test 46.32 s, package 47.917 s; regexp rank 14. |
| `89562d79b42419f9461387542e0ffcfcbf854d1a9c692c5dc40019b863f77bd1` | `/home/russell/lanes/perfcorpus-evidence/raw/self-prefilter-after.load` | Host-load qualification: 7.07 9.64 11.91 5/1199 697 |
| `a4b3bc644503e14af921f5250768af37e1e4c418c344a6c6322242827bc52ff9` | `/home/russell/lanes/perfcorpus-evidence/raw/self-prefilter-after.txt` | --- PASS: TestCorpusSelfDocumentation/DCP-V1-019_self (46.32s); PASS; ok   github.com/Beamfall/corvint/internal/doccorpus/selfcorpus 47.917s |
| `0b3ae29dbcebf4b7f5233a7b67ec8762a14bb33a66442055f666f7f5dccb1b61` | `/home/russell/lanes/perfcorpus-evidence/raw/shared-parse-benchmark.txt` | BenchmarkGPKV08000GoFacts/Shared-32                  10000       2150 ns/op     2040 B/op       38 allocs/op; PASS; ok   github.com/Beamfall/corvint/internal/contextindex 0.222s |
| `d48e784ab0fadeedf506d1b8b62397a6086f8b5ad167c0d5ba577ba3318d2c26` | `/home/russell/lanes/perfcorpus-evidence/raw/shared-parse-failing-first.txt` | Original helper path fails at 32 shared versus 32 independent allocations. |
| `74935caa040b49b13f02611defad1bba5dc33393ae5d4acca5cf32367bd40cfa` | `/home/russell/lanes/perfcorpus-evidence/raw/shared-parse-tests.txt` | ok   github.com/Beamfall/corvint/internal/contextindex 4.582s |
| `0adfaa34771df15740c7be1eb5c0c46a240bb936dddaf5118ba893d8b9912759` | `/home/russell/lanes/perfcorpus-evidence/raw/snapshot-gate.jsonl` | /3 vs /4: bytes 186663394/186683152; context n=20 p50 1017.76/932.27 ms; max RSS 548.05/288.51 MB; retains failed adapter row. |
| `e616cff5c45b2b5220c8d744b83ffa7c74cbf5b4be1d573c885e68bd026d531b` | `/home/russell/lanes/perfcorpus-evidence/raw/snapshot-hooks-profiles.jsonl` | Hook CPU-profile samples, separate from the n=20 context and n=10 hook timing gate. |
| `ef54963f71c7dd44580675797f7fb7fdb9eef9b69fcf483a9366adecf65b00c9` | `/home/russell/lanes/perfcorpus-evidence/raw/snapshot-profiles-memfd.jsonl` | Five profiled pairs; raw process/profile identities for aggregate decode+hash CPU share 57.3% -> 47.4% (gate fails). |
| `574c6a61c0e683a14e6cb67893e04670bf7d50c9ba1225f2b5b570db31d88557` | `/home/russell/lanes/perfcorpus-evidence/raw/snapshot-tests.txt` | ok   github.com/Beamfall/corvint/internal/contextindex 1.197s |
| `a884598b0c332b71af12f5977f6143b6396a344fe418bbdb9360fda40842c52f` | `/home/russell/lanes/perfcorpus-evidence/raw/snapshot3-profile-top.txt` | Five /3 CPU profiles aggregate 3.77 s; body decode 1.68 s, trailer hash 0.48 s; share 57.3%. |
| `82cf44c20fccd8b299707f7902b50b43831bcdf15b0111ba49e2368e94b2c0e8` | `/home/russell/lanes/perfcorpus-evidence/raw/snapshot4-profile-top.txt` | Five /4 CPU profiles aggregate 2.68 s; body decode 1.09 s, trailer hash 0.18 s; share 47.4%. |
| `acf8b0f1c58f7b6b5f31eff8431e14004f215a862c0f96e0bacb154c2c3d5b90` | `/home/russell/lanes/perfcorpus-evidence/raw/spec-check.txt` | spec-readme-check: PASS; ok   github.com/Beamfall/corvint/internal/specindex 1.016s; ok   github.com/Beamfall/corvint/internal/lrfrepo 0.128s |
| `719fc2a135d728583e16df8e9296559f3a13cb70ad32bbed40786229bda6181f` | `/home/russell/lanes/perfcorpus-evidence/raw/stamp-before.load` | Host-load qualification: 11.09 19.02 21.61 8/1486 4 |
| `e9e27115e6b010b7c5e97b2f943c3316699242e877fd0c850ce536ff87436e00` | `/home/russell/lanes/perfcorpus-evidence/raw/stamp-before.txt` | BenchmarkDCRV1847CheckoutStamp-32           10   38152098 ns/op 44596738 B/op   206777 allocs/op; PASS; ok   github.com/Beamfall/corvint/internal/mcp/corpusbridge 32.054s |
| `6511f7fa9aec97388199f4211ad7f6749a919596d0955dea654304d0998a0c08` | `/home/russell/lanes/perfcorpus-evidence/raw/stamp-failing-first.txt` | Original reader fails at 300 allocations per 100 stats (budget 1). |
| `2fcd7f1b29a7fa13a2a76969257b9b97bd9df5a7f8551ec9411cc85229dba326` | `/home/russell/lanes/perfcorpus-evidence/raw/stamp-fast.load` | Host-load qualification: 11.59 16.68 20.38 4/1462 8 |
| `faf3c13c5cc09e295e3825c577004f992bf2af92be8f3a344f874f57724b11dc` | `/home/russell/lanes/perfcorpus-evidence/raw/stamp-fast.txt` | BenchmarkDCRV1847CheckoutStamp-32           10   30798358 ns/op 16982114 B/op    97631 allocs/op; PASS; ok   github.com/Beamfall/corvint/internal/mcp/corpusbridge 29.850s |
| `f00860b89af4b00e9989fd9a957d79cc2b6e087adf999a0486b0c9e529c0a23d` | `/home/russell/lanes/perfcorpus-evidence/raw/stamp-final.load` | Host-load qualification: 16.16 24.32 27.52 5/1296 114 |
| `8e5837e230e38acb1eb60f2b20141c5657e363253fb87868c58b50a206d26f2f` | `/home/russell/lanes/perfcorpus-evidence/raw/stamp-final.txt` | BenchmarkDCRV1847CheckoutStamp-32           10   43072531 ns/op 16880224 B/op    97498 allocs/op; PASS; ok   github.com/Beamfall/corvint/internal/mcp/corpusbridge 35.916s |
| `70f55745df6e01fef3c9235c46e0b6b502a7244c93ed37879cef38c03709b24e` | `/home/russell/lanes/perfcorpus-evidence/raw/stamp-tests.txt` | ok   github.com/Beamfall/corvint/internal/mcp/corpusbridge 6.859s |
| `c4853df76316e85a6733cd890800f80e328fb8de10549bd76e8ba18374ea4f90` | `/home/russell/lanes/perfcorpus-evidence/raw/trace_regions.py` | Reproduction harness for inclusive trace-region unions; overlapping stages must not be summed. |
| `6f8f304dbe7c2c6e93f8f8b960ce6ef96610b97e96f9af2d6a4b5b4f05b78c30` | `/home/russell/lanes/perfcorpus-evidence/raw/vet-cmd.txt` | Focused vet exit 0; no diagnostic (command/queue details in raw log). |
| `05bf2cbcb266223229096e184324da47a7f4f550874130f33ac2cf43db67f36a` | `/home/russell/lanes/perfcorpus-evidence/raw/vet-corpus.txt` | Focused vet exit 0; no diagnostic (command/queue details in raw log). |
| `282f3452a6bfe9160f876b89cb39dd34f68a4e20447c43b401f160fbe624b414` | `/home/russell/lanes/perfcorpus-evidence/raw/vet-index-prefilters.txt` | Focused vet exit 0; no diagnostic (command/queue details in raw log). |
| `5ead42b23449725a106400dbc37b41770e59ea23a653ee14ee1b5c323d712ef4` | `/home/russell/lanes/perfcorpus-evidence/raw/vet-index.txt` | Focused vet exit 0; no diagnostic (command/queue details in raw log). |
| `54545c600fe957af3854bc8af3eadde5131d73019acdc377290f4d38464256d6` | `/home/russell/lanes/perfcorpus-evidence/raw/vet-kernel.txt` | Focused vet exit 0; no diagnostic (command/queue details in raw log). |
| `d28acaf1452fc04da7d019c3f0b3557cfd309a35ac241aac088e90dca5d69a18` | `/home/russell/lanes/perfcorpus-evidence/raw/windows-stamp-vet.txt` | Focused vet exit 0; no diagnostic (command/queue details in raw log). |
