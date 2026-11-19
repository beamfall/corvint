# Line evidence: default output migration blocked by the frozen baseline

Ticket: V1-5697. Base: `9f4d5d89ab0648425db40e617f3d890598f9ff79`.
Requirements affected: LEV-V1-004, LEV-V1-009, LEV-V1-010, LEV-V1-012.
State: PARTIAL. The accepted requirement text remains unchanged.

The whole `cmd/corvint` package run failed in 183.761 seconds. Default tier, exact, instance and
reason fields violated `TestCoreVerbsEmitTheFrozenProfiles` and its hermetic-identity copy.
Unconditional reason lines violated the existing dry-run stderr assertions and summary budget.
Default annotations also disagreed with the frozen explicit-Go-index plan comparison and the
shared review-tail comparison. The lane forbids weakening assertions or changing frozen goldens;
the default absent-store output migration therefore remains blocked.

The retained implementation activates the additive overlay only when the private store exists,
or when watch explicitly supplies an index. Refresh initializes the store. Missing per-instance
files inside it still produce `line-evidence-absent`; stale and damaged inputs still widen. A
repository with no private store keeps its existing baseline output. This preserves existing
contracts but does not fulfill universal tier/reason annotations; task 2 is PARTIAL. Tasks after
it retain their executed evidence without sequential completion ticks. Task 5 additionally needs
live Ruby proof. Task 12 records the trace without claiming delivery of tasks 10 or 11.

The CLI conformance table now includes both new command nodes, and the clean-tree refusal table
includes refresh. These are added cases; every previous assertion and golden is retained. The
new package declares only reads in its own directory; the four existing tests that build the CLI
declare its newly imported collector/reader directories. No unbounded-reader exemption was added.

The run also exposed host isolation friction: `/home/russell/lanes/lev.tmp` is under the separate
`/home/russell/lanes` Git worktree. Tests expecting a non-repository directory discover that parent,
and profiling tests correctly refuse destinations beneath it. An attempted
`GIT_CEILING_DIRECTORIES=/home/russell/lanes/lev.tmp` did not isolate them: core Git reads sanitize
that variable away, and profile validation checks ancestor `.git` entries directly. The mandated
TMPDIR and Git metadata restrictions prevent a local test-environment repair. The host verifier
must run those fixtures with scratch space outside every repository. No product guard was relaxed.

Initial whole-run evidence is retained outside the repository as
`/home/russell/lanes/lev-evidence/cmd-whole.log`.
SHA-256: `378530b5e56928f02174400b71d61e3c60f22c21abf7c006d72509ff79b6ddcf`.
Final package and focused-repair results are recorded in the handback.
