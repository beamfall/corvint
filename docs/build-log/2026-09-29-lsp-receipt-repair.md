# LSP integration receipt repair

CI on PR #356 found that the task-orientation implementation receipt still pinned the previous bytes of `cmd/corvint/taskcontext.go`. The standalone focused documentation selection had omitted the use-case receipt checks.

Run the repository repinner to update only the changed subject digest, receipt revision, and ledger receipt digest. Keep the reviewed LSP source and its existing sealed CEM unchanged. This repair has a separate evidence binding based on the implementation seal, `39ce4c57e0eec0dbf83f35c370986268594337f5`.

Acceptance uses `make use-case-receipts-check use-case-receipts-test` and `GOTOOLCHAIN=local go run ./conformance/use-cases-v0`, plus hosted PR CI. Repinning does not add product promotion evidence or establish new semantic usefulness. The independent prelanding review found no actionable source defect in the implementation diff. Rollback is the inverse of the two receipt-file changes; it restores the known stale-pin failure.
