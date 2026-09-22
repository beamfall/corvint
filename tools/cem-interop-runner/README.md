# Native CEM 0.1 interop runner

`cem-interop-runner` is the active Corvint-free external consumer harness for the frozen
`interop/cem-0.1` packet. It uses only the Go standard library and exposes the same workflow:

```text
go run ./tools/cem-interop-runner doctor
go run ./tools/cem-interop-runner start --lane consumer --output /absolute/private/observation.json
go run ./tools/cem-interop-runner consumer --implementation /absolute/path/to/consumer --observation /absolute/private/observation.json
go run ./tools/cem-interop-runner consumer --retry --implementation /absolute/path/to/consumer --observation /absolute/private/observation.json
go run ./tools/cem-interop-runner inspect --observation /absolute/private/observation.json
```

The runner verifies the frozen manifest and all 51 artifact digests before reading the consumer,
then snapshots all case inputs and runs the ordered 7 valid, 19 invalid, and 6 drift cases in fresh
private directories. Observations use an owner-only lock, bounded CAS updates, atomic publication,
and at most one explicit retry.

`interop/cem-0.1/runner.py` remains unchanged because its bytes belong to the historical packet
identity. It is retired from active execution. Receipts created with its packet digest remain
historical evidence and are not current native-runner receipts. The native runner does not claim
external qualification, producer support, sealed qualification, or the documented multi-evidence
extension.
