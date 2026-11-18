# V1-4733: continue independent archive build caches

Date: 2026-11-18. Owner intent: the m8-archive lane charter, ticket V1-4733.
Requirements: GAG-V0-9960..9963 and ARTIFACT-GO-V0-001, 003, 005..007.

The base already runs bounded target workers, preserves manifest-order failures, and cancels
speculative suffix work. Its remaining duplicate work is recompiling each target's independent
A/B inputs from cold caches again after the loose gate passes.

Decision: each raw-export build continues only the matching loose cache for its target and side
within this invocation's private scratch. A and B still start cold and never share entries;
neither targets nor invocations share entries. The raw exports, Go build commands, private Git
indexes, temporary directories, double assembly, byte comparisons, offline verifier, smoke checks,
toolchain checks, clean-state rechecks and ordered failure collector remain unchanged.

The pinned Go toolchain validates build action inputs before reuse. Its local implementation
describes action IDs as hashes of inputs in `cmd/go/internal/work/buildid.go`. This relies on the
same trusted toolchain as compilation itself. Matching compiled actions need not run twice again;
changed inputs recompile independently in each side. No live-root binary replaces a raw build.
This matters for ignored source: clean Git status alone cannot establish input equivalence.

`TestArchiveCacheContinuationPreservesIndependentBuilds` exercises real initially cold loose A/B
builds and matching raw builds, checks cache isolation and byte/provenance equality.
`TestIgnoredLiveSourcePassesLooseGateButArchiveBindingRefuses` now primes the actual continued
caches with both committed-ignore and local-exclude source; all previous assertions remain and
require raw builds to omit that source and return `loose-gate-report-mismatch`.
The existing worker, cancellation and clean-state tests retain their assertions.

Rollback: restore fresh `cache-a/<target>` and `cache-b/<target>` paths in `buildArchiveTarget`.
There are no wire, field, error-code, stderr, golden, target or publication changes.

## Measurement

Host: PC, WSL2/Linux, Ubuntu 26.04, kernel 6.18.33.2-microsoft-standard-WSL2,
AMD Ryzen 9 5950X, 32 visible logical CPUs, Go 1.27.4. Default selects five target workers.
This is not a hosted Ubuntu 24.04 runner.
Baseline commit: `84a977dc5721a840159c369f1ab4fce91ce2a2d1`.
Implementation commit measured after: `17d164041475cb133149138fa9b806a77d07055c`.
Each gate creates private empty caches. `/usr/bin/time` runs inside `script/heavy.sh`, excluding
semaphore waiting. The default run admitted immediately. Initial Linux load1 was 0.77;
Windows workload/CPU was NOT_OBSERVED. No native Windows workloads were started by this lane.

The four-CPU run sets `GOMAXPROCS=4 CORVINT_ARCHIVE_WORKERS=4` and uses `taskset -c 0-3`.
Affinity also constrains compilers: closed build environments do not inherit `GOMAXPROCS`.
It is a local four-CPU experiment, not evidence of hosted runner performance.

Raw logs and timing files live in `/home/russell/lanes/m8-archive-evidence/`.
Commands use `TMPDIR=/home/russell/lanes/m8-archive.tmp`:

```sh
script/heavy.sh /usr/bin/time -f 'wall=%e user=%U sys=%S maxrss=%M' \
  -o "$evidence/before.time" make go-archive-gate
script/heavy.sh taskset -c 0-3 env GOMAXPROCS=4 CORVINT_ARCHIVE_WORKERS=4 \
  /usr/bin/time -f 'wall=%e user=%U sys=%S maxrss=%M' \
  -o "$evidence/before-4.time" make -C "$lane.tmp/before" go-archive-gate
```

Here `lane=/home/russell/lanes/m8-archive` and `evidence=$lane-evidence`.
The four-CPU baseline uses a separate clean local clone at the baseline commit, allowing edits
in the lane without changing measurement inputs. After measurements use the lane checkout,
with identical wrapper/options and `after` / `after-4` log names; the four-CPU after command omits
`-C` because it runs in the lane checkout.

These are single observations, not statistical estimates. Default CPU user+system time fell from
3119.25 to 1700.62 seconds; four-CPU user+system time fell from 2099.98 to 1082.50 seconds.
All four heavy admissions waited zero seconds. Their recorded integer load1 values were
0 / 12 before (default / four CPUs), and 5 / 20 after, respectively. The four-CPU after result
is 4m46.86s, below the 8-minute target locally. Native execution on cross-compiled platforms remains
outside this claim.

| Mode | Before wall seconds | After wall seconds | Result |
| --- | ---: | ---: | --- |
| Default workers, 32 CPUs | 169.08 | 108.72 | both PASS; 35.7% lower wall time |
| Four workers, four-CPU affinity | 550.61 | 286.86 | both PASS; 47.9% lower wall time |

Hosted Ubuntu 24.04 before/after qualification is NOT_RUN: this lane has no authorized hosted
runner carrying this unpushed commit, and pushes are explicitly forbidden. The ticket's hosted
under-eight-minute target remains unqualified until the orchestrator obtains that evidence.

## Verification

`corvint affected --base 84a977dc5721a840159c369f1ab4fce91ce2a2d1` selected the touched
package and archivebuild/archiveverify/archive-verifier. Its plan retained 56 language-frontier
unknowns; broader selections remain outside this lane's authorized focused checks.

Whole-package `go test -count=1 -timeout 30m` through `script/heavy.sh` passed for:

- `./conformance/release-artifact-v0`: 13.777 s, including both cache regression tests.
- `./conformance/release-artifact-v0/archivebuild`: 0.012 s.
- `./conformance/release-artifact-v0/archiveverify`: 0.016 s.
- `./conformance/release-artifact-v0/archive-verifier`: 0.012 s.

`script/heavy.sh GOTOOLCHAIN=local go vet ./conformance/release-artifact-v0` passed.
`make spec-requirements` found no generated ID/index changes. Citation repin moved two existing
anchors and its second pass found no further moves. The required
`make -s spec-requirements-check line-citations-check openspec-check` passed; OpenSpec retained
65 pre-existing baselined findings in 29 subjects and reported zero failures.
Both after gate measurements passed at the implementation commit above. The existing
`TestCorvintHostArchivePartialProof` skips its Darwin arm64-only case on this Linux host.
OpenSpec commit rules OSC-V1-003/004 remain NOT_RUN in the standard target (no base supplied).
`script/heavy.sh make go-archive-gate-injection-test` passed at that clean implementation
commit (169.74 s). The real build and unchanged replay passed; extra regular files, directories,
symlinks, a symlinked archive and a missing archive refused. The real interrupt exited 130 and
cleanup passed. The injection harness interrupt self-test also passed.

Git committed successfully but printed a sandbox refusal for
`/home/russell/projects/corvint/.git/packed-refs.lock`; no metadata override was used.
Independent review is reserved for the orchestrator by the lane charter.

## Retained raw evidence digests

SHA-256 digests bind the external logs and timing numbers above:

| File under m8-archive-evidence | SHA-256 |
| --- | --- |
| before.log | `19767ecf834877fa874cb483144ff3fe498a2d300f19527a4c05a3b8c755e467` |
| before.time | `ac8e091058d9b4d2887e9f953cb1da52af1612cec2fa47b254e70f5af6eeb994` |
| before-4.log | `3b47df81769ddfd495ae454b6d5dc5190ec0862a13c8257feee5a5a99aec2933` |
| before-4.time | `97094c9f695de17f91aecdf1381fe15a11c692b2a7a1f378f708acd44d7fb465` |
| after.log | `6693c0c448d33ba9a25460b5a9997e0a03630d10e5182ffd5fe3a9b903700edf` |
| after.time | `0589c8fc4166a43fcb77dffec9c8b1033794a46e15cccf6a0735abe95af2ac1d` |
| after-4.log | `ea63b23b732c111e812644c9fa129cad185a48d82059b704e67ed47a7cb90c93` |
| after-4.time | `b3ab7beb5b6306e79b91f43b645e536b34f103c81d6075e8732f7981e4f94df1` |
| package-tests.log | `63ca5b29c834057dcffeab5ef99b3eb30868e6debeee257bec0bf71fdadb36ed` |
| archivebuild-tests.log | `9471628e740f18529c7dd626e999b57d363882ce54151329e5bd9d4b67c50671` |
| archiveverify-tests.log | `ef45c30f05dc5c48bede92b952e4d61ed26019235da70a906bbac1b36335bc0d` |
| archive-verifier-tests.log | `8f315ac4d00aaca59348c091806ab1d353635815105b49006c00bfa76f294d80` |
| vet.log | `54ee1e389b2294d8c1df2092f392f220373a88762093870f6ed05cec2fdd69ae` |
| affected.json | `5a78bc66bb37aad5178fd843a260ba6714b79d87347b84234bb30073df6cc802` |
| injection.log | `87f4a91eece1c4a1930503f14386bbe3e52c2bb2c9503b83462569e51e3652c4` |
| injection.time | `4b2e822bbd9bf2fc7b233914ea5f337772953995711c0cd1cb373a02c7144d23` |
