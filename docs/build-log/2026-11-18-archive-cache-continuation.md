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
32 visible logical CPUs, Go 1.27.4. This is not a hosted Ubuntu 24.04 runner.
Baseline commit: `84a977dc5721a840159c369f1ab4fce91ce2a2d1`.
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
in the lane without changing measurement inputs. After measurements use the lane checkout.

| Mode | Before wall seconds | After wall seconds | Result |
| --- | ---: | ---: | --- |
| Default workers, 32 CPUs | 169.08 | pending | baseline PASS |
| Four workers, four-CPU affinity | 550.61 | pending | baseline PASS |

Hosted Ubuntu 24.04 before/after qualification is NOT_RUN: this lane has no authorized hosted
runner execution, and publication is explicitly forbidden. The ticket's hosted under-eight-minute
target remains unqualified until the orchestrator obtains that evidence.

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
Real-producer injection and after measurements await the clean implementation commit.
Independent review is reserved for the orchestrator by the lane charter.
