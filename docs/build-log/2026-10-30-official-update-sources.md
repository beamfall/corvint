# V1-3184: official RC and developer update sources, and an explained UNKNOWN

## Finding

The updater has no algorithm defect: Core discovery already admits prerelease `v*` tags and Tasks
reads `tasks-dev-*`, so an RC is checkable without a published stable `v1.0.0`. The blocker is
that the single-root `beamfall/corvint` has no published releases at all (anonymous
`GET /repos/beamfall/corvint/releases` returned `[]` on 2026-10-30; `beamfall/corvint-history`
returned 404 anonymously). Before this change a check printed only `no matching release;
freshness UNKNOWN`, which did not say which source was read, how many releases it held, or what
would resolve it.

## Decision

- Keep one fixed official source, `beamfall/corvint`, with the Core `v` channel (RC and stable)
  and the Tasks `tasks-dev-` channel, each checked independently (`UPD-V0-367`). No
  `--repository` override: a configurable source would let a fork or mirror pass as official.
- Every result names `source` and `channel`; an inventory read records `releasesObserved`; an
  UNKNOWN carries `unknownReason` naming the empty channel and that publication is an
  owner-authorized action, or the unreadable-source cause and retry hint, or the missing network
  grant (`UPD-V0-367`).
- `--github-token-env NAME` authenticates release API reads, sent to `api.github.com` only
  (`UPD-V0-368`). It raises the rate limit; it is not publisher authentication.
- `docs/RELEASE-RUNBOOK.md` now lists the tags and assets each channel must publish, and
  `docs/INSTALL.md` the source and the explained UNKNOWN.

## Evidence

Focused: `go test -count=1 ./internal/update` PASS (WSL2 Linux amd64, go1.27.3), including
`TestUPDV0367CheckExplainsSourceAndUnknown` and `TestUPDV0368TokenReachesOnlyTheAPIHost`.

Live, unauthenticated, Linux amd64, updater built from this change, installed build 7 (93b9f5b08):

```
corvint-update check --allow-network --component core   -> exit 1
{"component":"core","installedVersion":"Corvint 1.0.0-rc.3.dev (build 7, commit 93b9f5b08ce841875c9c81d17905908d91346f99)","installedBuild":7,"freshness":"UNKNOWN","source":"beamfall/corvint","channel":"v","unknownReason":"beamfall/corvint lists 0 published release(s) and none has a tag starting \"v\" with asset corvint_linux_amd64.tar.gz; an official Core RC or stable (vX.Y.Z-rc.N) release must be published (an owner-authorized action, docs/RELEASE-RUNBOOK.md) before freshness can be known","action":"check","qualification":"release evidence only; no broad qualification; atomic visibility, no power-loss durability guarantee","publisherIdentity":"NOT_VERIFIED"}
corvint-update check --allow-network --component tasks  -> exit 1
{"component":"tasks","installedVersion":"0.0.0-tcp01-unverified+build.7","installedBuild":7,"freshness":"UNKNOWN","source":"beamfall/corvint","channel":"tasks-dev-","unknownReason":"beamfall/corvint lists 0 published release(s) and none has a tag starting \"tasks-dev-\" with asset corvint-tasks_linux_amd64.tar.gz; an official Tasks developer (tasks-dev-YYYYMMDD.N) release must be published (an owner-authorized action, docs/RELEASE-RUNBOOK.md) before freshness can be known","action":"check","qualification":"release evidence only; no broad qualification; atomic visibility, no power-loss durability guarantee","publisherIdentity":"NOT_VERIFIED"}
```

## Remaining (ticket stays open)

- Criterion 1: sources and per-channel assets are documented; providing them means publishing an
  RC `v1.0.0-rc.3` and a `tasks-dev-*` release, which needs the owner's separate authorization.
- Criterion 2: unauthenticated check retained above; an authenticated live check is NOT_RUN (no
  token on the lane host).
- Criteria 3 and 4 (verified apply, retained manifests and rollback, independent update-route
  evidence for both components and the final RC) are NOT_RUN until a release is published. The
  verification behaviour they need already exists (`UPD-V0-002`, `UPD-V0-004`, `UPD-V0-005`).
- A Core dev label `1.0.0-rc.3.dev (build 7)` orders above a published `v1.0.0-rc.3` by SemVer
  precedence, so check would say `LOCAL_NEWER` (apply still proceeds when the candidate build is
  higher). Reported as friction, not changed here.
