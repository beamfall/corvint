# 2026-10-07: planted-receipt clock race in the writer-route tamper test (V1-0952)

## Intent

Make `TestCALV0116_WriterRouteTamperAtFastStages` deterministic. It failed intermittently in
`internal/tasks/store`, including on hosted CI for
[beamfall/corvint#666](https://github.com/beamfall/corvint/pull/666) (go-product-shard 0). The
failure was a `MALFORMED: transaction: recordedAt … is earlier than the head receipt's …` refusal in
its `*-receipt-planted-after-model` subtests.

## Cause

Both planted subtests append a receipt stamped with the wall clock in the middle of a write. The
write had sampled its own timestamp earlier. When the planted receipt landed in a later second, the
write's timestamp read as a clock step backward, and CAL-V0-012 refused it.

- **Lease.** The lease writer already samples its live clock again against the head it holds
  (`recordedAt`). The test, however, ran it without the live clock that every command-line writer
  carries (`internal/tasks/cli/clock.go`).
- **Mutate.** Mutate sampled once, after taking the lock, and both its writer route and its
  complete route used that sample. When the writer route declined because the head had moved, the
  complete route planned against the newer head with the earlier sample. That contradicts the
  failure-mode row "The writer samples its live clock again against the head it holds
  (CAL-V0-012)". (The lease writer route already samples again, in `writer_route.go`.) With a
  lock-respecting writer the head cannot move under the lock, so the refusal needs an out-of-band
  receipt (tamper). Any other fallback to the complete route now also records the later, still
  monotonic, time at which it plans.

## Change

- `mutateLocked` re-reads the live clock (`recordedAt`) before the complete route, after the writer
  route declines.
- The planted subtests run the write under `WithClock(ctx, WallClock)`, as the CLI does. Before
  planting, they wait for the wall clock to reach the next second, so the planted receipt always
  postdates the write's first sample. The race is now forced on every run, not left to chance.

## Evidence

- Regression, forced boundary, without the live clock: both subtests fail (`-count=2`, 4 of 4).
- With the live clock but without the `mutate.go` re-sample: `mutate-receipt-planted-after-model`
  fails 5 of 5 (`-count=5`). With both changes: 5 of 5 pass.
- `go vet ./internal/tasks/store/` is clean, and the full `internal/tasks/store` and
  `internal/tasks/cli` packages pass with `-timeout 30m`.

## Limits

- NOT_RUN: `make gate`.
- The subtests now wait up to one second each for the second boundary.
