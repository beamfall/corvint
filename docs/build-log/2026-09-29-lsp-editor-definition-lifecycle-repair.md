# Repair backend lifetime cleanup under editor backpressure

Independent integration review of 1e22d60b84b786fc61236c2b8c270fe3e575a2cf found P2:
backend death/session expiry was observed only by the dispatcher, which could remain blocked
writing a reply to an unread editor pipe. Parent cancellation released it, but backend lifetime
completion alone did not. This violated LED-V0-005. Review and original scratch reproduction are
retained at `/tmp/lsp-semantic-integration-independent-review.md` and
`/tmp/lsp-semantic-backpressure_test.go`.

Before repair, the retained regression `TestSemanticBackendDeathUnblocksReply` failed after 3.19s
(`/tmp/lsp-integration-repair-before.log`). It waits until the reply enters the output writer,
then kills the owned fake backend while leaving the editor input open and response unread.
The repair independently observes backend Done, cancels owned transport I/O, and joins the
observer on exit. Intentional shutdown disarms and joins the observer before client.Shutdown,
so expected backend exit cannot consume the editor's normal shutdown reply.

Targeted race verification passes (2.184s):
`GOTOOLCHAIN=local go test -race -count=1 -timeout 30m ./internal/lspstdio -run 'TestSemantic(BackendDeathUnblocksReply|ShutdownReplyBeforeExit|EOFDuringInitialize|CancelAndChange)$'`.
It covers the observed blocked-output failure, shutdown reply followed by exit, startup EOF and
pending request cancellation/change. `go vet ./internal/lspstdio` also passes. The independent
reviewer's original Go-overlay reproducer passes after repair; no source was altered by that
scratch test. Original outputs remain under `/tmp/lsp-integration-repair-after.log`,
`/tmp/lsp-integration-repair-vet.log`, and `/tmp/lsp-integration-review-repro-after.log`.

Affected advice was retained before testing as `/tmp/lsp-integration-repair-affected.json`.
This is one bounded repair cycle, not new qualification. Existing live overlay evidence remains
bound to the pre-repair source; actual-client runs and fresh independent repair review remain
pending. Intent stays proposed, delivery experimental. Final CEM/seal, publication and task
completion remain with the parent coordinator. No repository-wide gate was run.
