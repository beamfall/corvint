# Decision 0417 — AFU-V1 run-evidence remainders: registry, `/3` profile, observer gap, JUnit timeout

Date: 2026-09-25. Status: accepted. Authority: repository owner answer "separate registry, approve
/3 profile, accept 014 gap, add JUnit timeout" (2026-09-25, relayed by the coordinating agent for
PR #250), answering the four open questions of V1-0249. Spec:
`docs/specs/application-flow-understanding-v1.md`.

1. `AFU-V1-013`, "separate registry": combining repeated runs under the `DCP-V1-023` counters and
   the `DCP-V1-024` policy takes the planned repetition ordinals, the planned or manual run kind,
   the infrastructure failure class and the policy binding from a separate repository-owned local
   registry. The `test-run-evidence/0` wire record is not revised. The registry is its own
   spec-first change (branch `claude/v1-0249-run-registry`).
2. `AFU-V1-012`, "approve /3 profile": the `corvint-playwright-external` reporter may gain a `/3`
   profile revision that carries every attempt's detail, with `/2` still readable. Its promotion
   still needs the live reporter qualification (branch `claude/v1-0249-playwright-profile-v3`).
3. `AFU-V1-014`, "accept 014 gap": the AFU-V0-010 observer is flow-scoped and has no test key, so it
   emits no per-test `LOCALLY_OBSERVED` record. The requirement keeps its ID and states the gap;
   the schema still accepts the authority.
4. JUnit timeout, "add JUnit timeout": a JUnit `failure`, `error`, `flakyFailure`, `flakyError`,
   `rerunFailure` or `rerunError` element is a `timedOut` attempt when its `message` attribute
   contains the case-sensitive phrase `timed out after `. The phrase is what JUnit 4
   `TestTimedOutException` ("test timed out after %d %s") and JUnit Jupiter `TimeoutException`
   ("%s timed out after %s") write. It is the narrowest signal the adapter already reads: body text
   and other attributes are not consulted, so an assertion log that mentions a timeout stays
   `failed`. A runner that reports a timeout without that phrase stays `failed`, never guessed.

Evidence: `TestAFUV1JUnitTimedOutAttempt` and the JUnit timed-out control in
`TestAFUV1AdapterObservesFailedControl` (`internal/appflows/runevidence_test.go`).

Rollback: remove the `junitTimeoutPhrase` mapping from `internal/appflows/runingest.go` and its
spec sentence (JUnit timeouts return to `failed`), and restore `AFU-V1-014` to partial. Items 1
and 2 are rolled back by not merging their branches.
