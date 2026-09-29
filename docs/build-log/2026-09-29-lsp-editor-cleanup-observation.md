# Bounded editor cleanup observations and safe signal ownership

Owner authorized one bounded V1-0489 diagnosis/repair, independent review and both-client reruns.
The actual VSCode run at sealed tooling `e9277132` and server `cb324729` had valid context/wire
and unchanged fixture evidence, but failed cleanup for PID98981 `(Code)`. Later exact-PID observation
found it absent. Original process state, retirement latency and root cause remain UNKNOWN; the
original raw failed report is unchanged. The deterministic seam proves the prior0.6-second stage
can fail before a process disappears at0.8 seconds, and command transitions suppress later signals.
Late-child and identity drift are reproduced algorithm risks, not asserted original causes.

Independent Gate A accepted a safer boundary: no indirect PID signals or sampled atomic identity
claims. The single-threaded unreaped Popen owner guards every direct group TERM/KILL, including
communicate escalation and outer exception fallback. Indirect descendants remain observation-only.
A fixed5-second monotonic stage samples diagnostic start/state/command, retains changed-command
candidates until absence, and admits new children only through a currently unchanged sampled parent.
PID-generation/lineage ambiguity, missing identity or timed-out inventory fails closed as UNKNOWN;
a lasting live child fails with its hold. Zombies must disappear. Inventories/sleeps use remaining
budget; new children cannot reset it. A bounded1s pre-signal inventory retains attributable children before directTERM/drain; missing
setup is sticky UNKNOWN. Existing3s+3s direct stages compose with5s observation and do
not establish a universal aggregate5-second promise.

The fresh cleanup worktree retains approved public-stack base `e9277132`; current origin/main
advanced independently and is NOT_INTEGRATED. Fresh query/impact/affected/baseline and keyed enrollment
precede edits. Sol/low matches this bounded harness repair; independent Astra/medium reviews ownership.
Controlled-clock checks witness delayed disappearance, live/late children, unrelated/reused PID
exclusion, lineage ambiguity, inventory timeout and zero indirect signals. Direct-owner fixtures
verify TERM/KILL and reaped-leader held-pipe escalation/fallback skipping. Actual outer INT/TERM
fixtures remain required. These are harness checks, not real editor interruption qualification.

Fresh actual clients remain NOT_RUN until root-inspected review/binding/seal. Default native-Go
Core/server source and pinned dependencies are unchanged. All tuples remain UNQUALIFIED. Rollback
reverts cleanup-only tooling/docs and preserves original failures, reports and cleanup holds. No
new installs, full gate, server tests, native Tasks writes, publication or nested agents occurred.
