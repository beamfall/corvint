# Decision 0149 — The test-validity execution report gains a SKIPPED outcome

Date: 2026-09-12. Status: accepted. Authority: repository owner instruction, 2026-09-12, via
delegation.

The shared projector accepted only `PASSED`, `FAILED`, and `INCOMPLETE` as execution-report
outcomes, although its execution axis already had a `SKIPPED` state. A Go `go test` skip therefore
projected `UNSUPPORTED`/`unsupported-execution-outcome`, and the editor showed it as an unknown.
Two related gaps sat beside it: Go per-test entries had no `file:line` anchor, so they produced no
diagnostic, and the editor did not decode the session-level `projection`.

The call has three parts.

1. `SKIPPED` becomes a report outcome (`LPCV-V0-052`). With no cause it projects execution
   `SKIPPED` with reason `test-skipped`, the TCQ report term. It never projects a passing state. A
   `SKIPPED` report that carries a cause is outside the vocabulary and projects `UNSUPPORTED`.
   Freshness still follows currency. The vectors `skipped-reported-result` and
   `skipped-with-cause-abstains` join the pinned minimum set, and both projectors reproduce them.
   The Go session's `skip` action maps to it (`GLTP-V0-052`).
2. A Go per-test anchor comes from source, not from the stream (`GLTP-V0-052`). `go test -json`
   records no declaration location, and `Output` lines are written by the test, so neither can
   establish one. The session keeps the per-file digests behind the run's identity and anchors a
   test only when every `_test.go` file in its package directory is watched, unchanged against that
   digest, and parseable, and exactly one top-level function has the test's name. Subtests,
   duplicate declarations, packages outside the module, and stale events stay unanchored.
3. The editor decodes the session projection but takes the outcome from the state
   (`VSC-V0-068`). A projection that classifies differently marks the scope items `errored` with an
   `UNKNOWN` message, so the projection cannot upgrade a state. Anchored Go entries become
   file-located test items and feed the same diagnostics ledger as JavaScript records
   (`VSC-V0-069`).

Disclosed residue: subtests have no anchor; a package directory with any unwatched test file
anchors nothing; the anchor line is the `func` keyword, not the first statement; and the
real-provider Electron run remains `NOT_RUN`. The session stays preview tier under `GLTP-V0-048`.

Rollback: remove the `SKIPPED` case from `executionFromReport` and its TypeScript mirror, the two
vectors and their pinned names, `sourceLocator` and the digest retention in
`internal/liveverify/session/session.go`, and `classifyGoSessionEvent` and the Go anchor recovery
in the extension, then revert the four requirement IDs. A Go skip then abstains again, and Go
entries return to unanchored.
