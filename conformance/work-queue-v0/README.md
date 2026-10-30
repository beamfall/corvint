# Native work-queue conformance fixtures

The work-queue fixture producer and qualification harnesses are native Go commands:

```text
go run ./conformance/work-queue-v0/cmd/cli-adapter snapshot|details|verify
go run ./conformance/work-queue-v0/cmd/cli-replay ACTION --output /private/tmp/... [--source ... --binary ... --build-receipt ...]
go run ./conformance/work-queue-v0/cmd/cli-prose prepare|selftest|run --output /private/tmp/... [...]
go run ./conformance/work-queue-v0/cmd/cli-scope prepare|run|finalization-selftest --output /private/tmp/... [...]
```

`cli-replay` retains the former action boundary: `prepare`, `baseline`, `replay`, `controls`,
`environment-controls`, `cleanup-selftest`, `cleanup-probe`, `oracle-selftest`, and
`fixture-selftest`. Evidence output remains restricted to a private temporary directory. The
producer accepts only `snapshot`, `details`, and `verify`; it writes canonical JSON, records an
immutable ordinal spy, reconstructs the committed repository source, and emits the same frozen
independent fixture wires checked by `native_fixture_test.go`.

All production capture claims remain `UNKNOWN/EMPTY` qualification evidence. The native fixture
selftests do not promote eligibility, dispatch, mutation containment, network absence, rollout, or
external Beamfall behavior.
