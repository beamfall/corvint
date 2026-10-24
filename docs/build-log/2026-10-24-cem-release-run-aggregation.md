# cem-release-gate binary envelopes, per-platform aggregation and readiness binding (V1-2578, V1-2579, V1-2581)

Date: 2026-10-24. Tickets: V1-2578, V1-2579 and V1-2581. Spec: `docs/specs/cem-stable-v1.md`
CEM-V1-609..614, proposed; SRR-V1-005 amended by cross-reference. Revision
`27e9d9807214029a1535a5135f3401b6c6d45bec`, stacked on #1703. Nothing is promoted.

## Decisions

- **Binary envelope rows run (V1-2578).** The 29 repository-envelope cells are prose recipes the
  S0E harness rebuilds, so the gate runs the portable consumer's cell recipes once per built binary
  with that binary as each cell's verifier (`CEM01_ENVELOPE_BINARY`). A failing binary run refuses
  with exit 1; only 29 distinct `RUN_BINARY` passes make the row `PASSED`.
- **Aggregation over run records (V1-2579).** One run observes one platform. Each run can write a
  `cem-release-run/1` record, and `--aggregate` judges readiness over several records of one
  revision. The declaration is read again from that revision's committed objects, never from a
  record. A runner row passes the merge only when every record observed it `PASSED`.
- **Records are unauthenticated.** Neither encoding is signed. The binary digests let a reviewer
  rebuild at the revision and compare; nothing here proves who ran a record.
- **Readiness binds the gate verdict (V1-2581).** The SRR catalogue gains `gate/cem-release-gate`,
  appended last. Its evidence must be a READY `cem-release-gate-outcome/1` for the candidate
  commit, checked at build and at verification.

## Encoding changes

- New `cem-release-run/1` (tools/cem-release-suites) and `cem-release-gate-outcome/1`
  (`internal/companionrelease`). Both are new profiles with strict canonical readers; no earlier
  artifact used them.
- `corvint-stable-readiness-record/1` keeps its profile. The catalogue for `1.0.0-rc.1` and
  `1.0.0-rc.2` is unchanged, so their existing records still verify byte for byte. For every later
  version, a record without the gate row is refused as a catalogue that is not closed. No such
  record exists: the current candidate version is `1.0.0-rc.3` and none has been built.
- Consequence: an rc.3 readiness record cannot be built until the gate is READY, which needs the
  external consumer row (V1-0014, EXTERNAL). This is the intended refusal.

## Evidence at 27e9d98

Files under `artifacts/cem-release-27e9d98/`.

| Run | Host | Binaries (corvint, cem01-go) | Envelope rows | Record SHA-256 | Outcome SHA-256 | Verdict |
| --- | --- | --- | --- | --- | --- | --- |
| single | darwin/arm64 go1.27.3 | `8125324e…`, `d7bd12f0…` | both PASSED 29/29 | `b5e50d8e1da7e872f13917f26a70f93133b630dbe41440633f5867a0558eb261` | `b2848b1cf7c938568c48db2b5889376f00dd1195c9caef1f669368e889cbab3a` | REFUSED `release-platform-incomplete` (linux/amd64) |
| single | WSL2 linux/amd64 go1.27.3, node v22.22.1 | `4cc9b830…`, `7b1900da…` | both PASSED 29/29 | `b6c4f020d3800c7469ad0261d62a6461fab5305c1ff074c79cece06049682041` | `8d3fae32c469f836aba0940bb01b8c7fb85e0786a6bc6fec4543566ac9b52726` | REFUSED `release-platform-incomplete` (darwin/arm64) |
| aggregate | darwin/arm64 | both records | all 11 runner rows PASSED; both platforms PASSED | — | `26451ef98464305f9847686710877d18a2f8dd6650ad33a92685246ac679420b` | REFUSED `release-external-incomplete` (56 accepted results), exit 3 |

The aggregate outcome binds declaration `5511e011e0133a5ca07a03942501403302ad8855683d2ecbda2e06c36dd1f763`
and both record digests. `independent-external-consumer` stays `NOT_OBSERVED` (V1-0014).

### linux/amd64 runner receipts (V1-2579)

These were produced on WSL2 linux/amd64 with `corvint-test-runner` built at 27e9d98. The executables
were go1.27.3 (`5f5eb6d0…`) and node v22.22.1 (`d0efb6fc…`). The cem-release-suites plan declares
`/usr/bin/git` 2.53.0 (`ed8b2df9…`) as a tool, because the aggregation tests commit a fixture
repository. A first attempt without it failed those three tests, since the runner passes no host
PATH.

| Index | Runner and target | Plan SHA-256 | Receipt SHA-256 | Result |
| --- | --- | --- | --- | --- |
| 0 | go-test `./internal/companionrelease` | `abef797ba73881766e227f5b9e6bc6a36e9168eaaf42588983fb7f52c12f88aa` | `5e827350fcf960b37b8eb58afa96013518c4b7d075afd33c5a0f67f660a3c473` | 59 PASSED, complete |
| 1 | go-test `./tools/cem-release-suites` | `30eca3482531ca59675ddcf1f2e59ccd699b995d5562f23a5c21d30c3c99f359` | `36b751315db326db162dc923aec7aace1c042e6e77ecabfdaedb911741406619` | 7 PASSED, complete |
| 2 | node-test `tools/cem-release-suites/readiness.test.mjs` | `87d2a53760661dedbde857f0e383c596e7db2d854fec80afbf5de8067cd52921` | `02acb4aa1d88229968ca790f6b686b29e520181dbb5b684067c48170502bab74` | 4 PASSED, complete |

Receipt authority and dependency closure are `CALLER_OBSERVED` and `NOT_OBSERVED`. The plans name
the remote temporary root, so they cannot be re-run elsewhere byte for byte.

## Limits

- External interoperability is `NOT_OBSERVED`, so the aggregate verdict is REFUSED. This is
  expected and is not a pass.
- No CI job runs the gate or the aggregation. V1-2580 is the owner's `.github/` change.
- The run records were copied between hosts by the operator. They are unauthenticated.

## Rollback

Revert the lane's commits. The flags, the outcome reader and the SRR gate row go away. Files under
`artifacts/cem-release-27e9d98/` and this entry are evidence only, and nothing reads them as
ranking, authority or promotion input. rc.1 and rc.2 readiness records verify under both versions.
