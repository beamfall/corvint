# V1-0747: bound host-produced evidence reads before allocation

Ticket V1-0747 is from the V1-0712 output-class inventory
(`docs/build-log/2026-10-04-v1-0712-output-class-inventory.md`). Four readers that take in
host- or model-produced evidence checked their size limit only after reading the whole input.
This is the same class of bug as V1-0691.

## Change

- `internal/opencodequalification`:
  - `readRows` and `readObject` now share `readBounded`, which reads at most 16 MiB + 1 byte
    and refuses anything longer with the existing `evidence stream exceeds bound` error.
  - Before this change, `readObject` had no bound at all. It now covers every package-local
    report, probe, screen and manifest read.
- `internal/doccorpus` `loadInput`: below `gitauth.MaxBlobBytes` it calls the new
  `gitauth.Repository.BlobBytesWithin`. That method refuses an over-bound blob from its
  `cat-file --batch` header, before the body is allocated, and the refusal keeps the
  existing `corpus-refused` / `input bound exceeded` code. Limits at or above
  `MaxBlobBytes` (the v2 128 MiB corpus limit) keep the memoised `BlobBytes` read, which is
  already capped at 64 MiB.
- `internal/cem/gitauth`: `BlobBytesWithin` reports a present header over the limit as
  `over=true` with no error. Every other failure is returned exactly as `BlobBytesBounded`
  returns it, and `BlobBytesBounded`'s error types are unchanged (DLT-V0-003).
- `internal/evalrepo` `loadTraceFixture`: the fixture is read through
  `io.LimitReader(trace.MaxTraceStoreBytes+1)` and refused when it is over 16 MiB. This is
  the same bound as the trace store whose rows a fixture freezes.

## Unknown members (AC2)

OpenCode qualification rows and reports decode into open `Object` maps on purpose. Their
producers (the OpenCode host, the harness and the gate probes) may add fields, and every
gate reads only the fields it names. So unknown members are **accepted**: the comment on
`decode` and the AHI-032 traceability row now say so, and a test asserts it.

## Evidence

- Focused tests, each feeding bound+1 bytes and also checking the exact bound:
  - `TestEvidenceReadsRefuseOverBoundFiles`
  - `TestLoadInputRefusesOverBoundBlobBeforeBody`
  - `TestTraceFixtureRefusesOverBoundFile`
  - `TestBlobBytesWithinReportsOverBoundHeader`
- Full packages: `internal/cem/gitauth`, `internal/doccorpus/...`, `internal/evalrepo`,
  `internal/opencodequalification`, `internal/cem/verify`.
- `corvint affected` selected 176 units, because `gitauth` is imported widely. The units
  outside the list above are `NOT_RUN`, following the owner's preference for focused tests.

## Limits

- The tests show that oversized input is refused at the bound. They do not measure peak
  allocation.
- Real OpenCode qualification runs are `NOT_OBSERVED`.
