# Batch C: compact affected plan consumers (2026-10-08)

The batch C bind lifecycle found two `cmd/corvint` tests that failed deterministically after the
V1-0943/V1-0995 compact default (`affected-plan/1`, AFP-V0-035/038) merged. Both passed at base
`0c94c66c`.

- `TestAFUV1024StrictAndCoverageBytesUnchanged` compared default output against goldens captured
  before S4, which are the full `affected-plan/0` document. AFU-V1-024 pins `strict` and `coverage`
  bytes against the `e2e-safe` profile, not against the default output format, so the test now
  passes `--full` and its goldens are unchanged.
- The OpenCode cockpit `affected` read (`integrations/opencode/src/runtime.js`, AHI-034) validates
  and renders the full plan arrays and accepts only `affected-plan/0`. Its fixed argv now passes
  `--full`, so the cockpit keeps its contract instead of degrading to `malformed-cockpit-output`.

CI's advisory order planner (`.github/cishards/order.go`) already accepts both profiles.

Evidence: `go test -run 'TestHostAdapterJavaScript|TestAFUV1024' ./cmd/corvint` rc=0 at this commit.
