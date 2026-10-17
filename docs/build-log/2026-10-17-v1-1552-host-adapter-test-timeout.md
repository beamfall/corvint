# V1-1552: host-adapter-test timeout

`make host-adapter-test` ran `go test` for `./cmd/corvint` with no `-timeout`, so Go's 600s default
panicked it at load ~45 inside `make gate` (decision 0082). It now passes `-timeout $(GO_TEST_TIMEOUT)`.
Audit of gate-reachable `go test` lines: `go-test` (declared flags carry `-timeout`), `interop-gate`
(has it); `cem-stable-regen-capture` and `pi-protected-test` are not `GATE_STEPS` prerequisites.

Requirement STR-V0-019 (session-test-runner-v0) added; `tools/test-commands-check` dry-runs the
`GATE_STEPS` and refuses any `go test` command without `-timeout`. Owner standing auto-accept of lane
requirements (2026-10-11) applies. Rollback: revert the Makefile flag and the check. `host-adapter-test`
itself and `make gate` were NOT_RUN.
