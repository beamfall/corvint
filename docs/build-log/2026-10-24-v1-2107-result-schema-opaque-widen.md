# 2026-10-24 V1-2107 and V1-1761: RESULT.schema.json admits opaque/0; one install route for both verifiers

## V1-2107: recorded widening of the runner receipt profile

CEM-V1-310..313 let a map whose runner receipt was not admitted carry the `opaque/0` runner receipt
label, and `stable.schema.json` already admits it. Both byte-identical `RESULT.schema.json` copies
pinned `$defs/runnerReceipt/properties/profile` to `{"const":"corvint-test-runner-receipt/0"}`.
A verify result that referenced an `opaque/0` receipt was therefore ACCEPT, but it did not match
its own published result schema (Codex P2 on beamfall/corvint#1338).

The sealed chain (CEM-V1-091) had no mode that changes a schema value. Hand edits are refused by
`make cem-stable-regen-check`. CEM-V1-620 adds `--widen --record FILE`
(`make cem-stable-regen-widen RECORD=...`). It widens one string `const` or `enum` node, named by
a JSON pointer, in both copies. It checks the node's predecessor digest, keeps every existing
value, case and cell, and re-binds the same chain as the other modes. The packet status becomes
`SCHEMA_ENUM_WIDENED`.

The committed run (CEM-V1-621):

- Record: `docs/build-log/evidence/v1-2107/widen-record.json`, sha256
  `08deed4e4bb28eab94ba18e4d9ee8be631023f0f42bed916c2526aed78cc1e25`. It adds only `opaque/0`.
- Node `/$defs/runnerReceipt/properties/profile`, compact-JSON sha256
  `ee614710112c1b723eb3e032a0f7a103be2470b0c2e405cceb870d4ad3cf9f47` ->
  `2dcc5dcebe9c2714f5f0d3afb39a407590ae28c4dab65430a439b7d1b03f41bb`. It now admits
  `corvint-test-runner-receipt/0` and `opaque/0`, and nothing else.
- Packet manifest predecessor `c6a5cbe0899fe31968f42e740db00da6005989c3e7ce1954c1e832e5045834f4`.
  The schema copies change from `2334f05d...` to `5849832e...`. The packet `CHANGES.md` section
  lists every file digest. The old schema bytes stay in Git history at the base commit.
- Re-bound: both stable and packet manifests, `REPOSITORY-ENVELOPE.md`/`.cells.json`, and the
  go1.27.2 and go1.27.3 structural tuples. `make cem-stable-regen-check` passes on the result.

CEM-V1-622 adds `TestCEMV1622OpaqueReceiptResultValidates` in `internal/cem/verify`. It runs
`Stable` on the neutral case with each label, then validates each result against both committed
copies. It uses a strict draft 2020-12 subset validator that refuses unknown keywords. The
`opaque/0` receipt now validates at `references.runnerReceipts[0].profile`. A label `opaque/1`, and
the sealed legacy `const`, are both still refused there.

### Open limit: plain criterion bindings (V1-2615)

The full result for an unadmitted map still fails the committed schema, at
`references.criterionBindings[0]` (undeclared member `evidenceId`). ALGORITHMS.md CEM-V1-020 and
CEM-V1-083 define two binding forms: plain (`id`, `criterionIndex`, `criterionSha256`,
`evidenceId`) and Tasks. `$defs/criterionBinding` declares only the Tasks form, so every plain-map
result fails, whatever its receipt label. This gap predates V1-2107. The ticket allowed widening
only the two receipt profiles, so it is filed as V1-2615. The test pins the current refusal, and
also shows that both results validate once a plain form is added (an in-memory overlay only). When
V1-2615 is fixed, that assertion must change.

## V1-1761: install route for corvint and cem01-go

TCB-V0-045 (78824fc6d) already makes the cem-verdict gate name a stale or modified consumer
`verifier-build-mismatch`. The remaining criterion was a route that rebuilds and installs both
tools together. `corvint-update` installs release archives only, and release archives do not ship
`cem01-go`. TCB-V0-046 adds `script/install-cem-verifiers.sh` as the documented route:

- It refuses a dirty checkout.
- It builds both tools from HEAD.
- It refuses any build that does not record HEAD and `vcs.modified=false`.
- It installs each tool through `install-local-bin.sh`, keeping a rollback copy.
- It re-reads the installed pair.

`--check` is the read-only preflight.

Tested with a stub `GO` in `script/lane-scripts_test.sh` (`make lane-scripts-test`). The real-build
rehearsal follows.

### Rehearsal

Run on Darwin arm64 (go1.27.3) from the clean lane worktree at commit `a94be9e5c`, into a
scratch bin directory, not `~/.local/bin`:

- Install: `script/install-cem-verifiers.sh --bin-dir <scratch>` built both tools and installed
  them, then printed `MATCH a94be9e5c393e82e8e2c643912eb7ff45fa19929`. It took 8.4 s wall time.
- `--check` on the same directory printed `MATCH`, exit 0.
- Stale consumer: replacing the scratch `cem01-go` with the operator's installed one (built from
  `ec41e4e9c`) made `--check` print `MISMATCH corvint=a94be9e5c... cem01-go=ec41e4e9c...`,
  exit 1. This is the V1-1761 condition, named before the gate runs.
- `--check` against `~/.local/bin` (read-only) printed `MATCH ec41e4e9c...`. The installed pair
  is current, so nothing was reinstalled there.
