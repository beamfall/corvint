# Decision 0148 — Contained analyzer staging is content-addressed and reused

Date: 2026-09-12. Status: accepted. Authority: delegated call, owner instruction 2026-09-12.

After decision 0145 the Darwin payload ran, but `TestNewStaticCoreUsesContainedNativeInvocation`
still failed: 573 contained probes in 60 s all ended `TIMED_OUT` at the 100 ms plan cap.
`internal/analyzerexec` staged a fresh copy per invocation. Measured on macOS 26.6.2 (12 CPUs,
load 35-50, 3.8 MB Go payload, 30 interleaved samples): first exec of a fresh copy took p50 186 ms
unsandboxed and p50 199 ms sandboxed; a reused path took p50 4.4 ms unsandboxed and p50 13.5 ms
sandboxed (p90 22 ms). A second exec of the same fresh file took 4.3 ms, a hard link to a warm
inode 26 ms, and `clonefile` or `cat` copies paid the full cost, so it is per inode. Copy plus two
hashes took p50 7.6 ms; one SHA-256 pass took 2.2 ms at load 104. Killing the first exec at 20,
60, or 100 ms did not cancel the work (the next exec took the remainder), and `syspolicyd` gained
0.70 s CPU over 20 fresh execs, so the cost is an out-of-process exec policy assessment of a
freshly written (`com.apple.provenance`) file. It is not attributable to the sandbox profile.

The call: the staged executable persists under an owner-private directory named by its pinned
digest and is reused. A new entry is written into a random `0700` temporary directory, verified,
then published by rename; a losing concurrent publish is a terminal race. On reuse Core still hashes
the registry source against the pin and re-verifies the directory, identity, `0500` mode, and bytes
at staging, and again immediately before launch, so every ACC-V0-002 pre-launch check is kept. An
entry that fails reuse verification is removed during cleanup; a verified entry is never removed.
ACC-V0-002 and its failure-mode table state the rule.

Disclosed residue: the staging parent keeps one entry per distinct pinned executable digest, and
the first contained invocation of a digest on Darwin timed out in every measured sample (fresh-exec p10 173 ms).
With reuse the test passed 20/20 at load 60 and 15/20 at load 84-127, where the single-shot Core
invocation spent about 45 ms before launch and exhausted the cap without starting.

Rollback: restore the per-invocation random staging directory removed at cleanup, the two reuse
tests, and the ACC-V0-002 sentences. Contained Darwin invocations then time out again.
