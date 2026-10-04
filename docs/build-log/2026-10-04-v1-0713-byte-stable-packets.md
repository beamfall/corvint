## 2026-10-04 V1-0713 AHI-043: Claude context packets are byte-stable across time

Human-owned intent: the owner asked to start V1-0713, filed from the claude-mem finding that
minute-granularity timestamps changed injected context every 60 s and defeated prefix caching. The
ticket asks whether any Corvint SessionStart or UserPromptSubmit field varies across identical
invocations, and requires a test, a disposition for any varying field, and recorded byte counts.

### Finding

No model-visible field varies with the wall clock. The packets carry Git revisions, path-set and
request digests, coverage counts and the session key, and no timestamp, elapsed time or process
identity. The first measurement used the dogfood binary built from this branch on a clean checkout
of this repository, run twice 61 s apart in real time: session-start wrote 169 identical bytes and
user-prompt wrote 3,481 identical bytes. That session-start run hit the 2 s hook deadline and
returned the FALLBACK packet, which shows the one real source of variance: whether an invocation
finishes inside its deadline depends on host load, so the same state can produce the full packet
once and a deadline or stale-index FALLBACK another time. Each outcome is itself byte-stable. The
deadline variance is already tracked as V1-0607 (with V1-0396), so it is documented as an
exemption, not refiled.

### Change

- `AHI-043` in `docs/specs/agent-harness-integration-v0.md` requires byte-identical SessionStart
  and UserPromptSubmit stdout for an unchanged tree, worktree, hook input and session key, forbids
  per-invocation values in the model-visible packet, and names the deadline-outcome and
  changed-state exemptions.
- `cmd/corvint/host_adapter_stability_test.go` runs `runHostAdapter` for session-start (startup
  and resume) and user-prompt twice inside a `testing/synctest` bubble, 61 s apart on the fake
  clock, against a committed fixture with an unchanged untracked file. Each run must produce a full
  repository envelope, not a deadline or stale-index fallback, so equal fallbacks cannot pass
  vacuously. Recorded byte counts: session-start 3,336 and 3,336 for both sources, user-prompt
  3,646 and 3,646 (they vary slightly with the temporary directory length).
- A mutation check that appended `time.Now()` to the adapter's stdout made the test fail on its
  first case. The mutation was reverted before commit.

### Limits

The fixture has no task store, so its packets report `frontier-authority-unavailable`. A frontier
or policy-bearing packet is covered by the no-wall-clock rule but not by this test's bytes. Cache
impact is mainly a SessionStart concern: a UserPromptSubmit packet lands in the newest message,
after any cached prefix. Actual Claude Code cache-hit rates were not measured (`NOT_OBSERVED`).

Rollback: delete the test, `AHI-043` and its trace row, and regenerate `REQUIREMENTS.tsv`. Runtime
behaviour is unchanged by this entry.
