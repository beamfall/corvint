# Heavycap independent review repair: path-qualified env on the receiver

Date: 2026-11-07. Repair starts at `e6872ca1598bd673af115a13126f1186433341b2`.
Affected and CEM base: `c3b29cbb54d31c8526d3cce4ad74356b6fb9b943`.
Owner intent: the supplied independent review P1, V1-5007 / ASD-V0-2478.

## Finding and decision

The receiver's leading-command scan recognized literal `env`, while admission and
`command_goflags` recognize `env` by basename. With `/usr/bin/env GOFLAGS= go test`,
the scan stopped before the empty assignment. It therefore failed to replace that
assignment with the sender's validated bound, allowing receiver-only GOENV exclusions
to hide an existing test broken by the source edit.

Recognize `*/env` in that same receiver prefix scan. This matches the already admitted
path-qualified command forms and preserves the existing GOFLAGS binding, argv transport,
diagnostics and exit behavior. No frozen wire contract, golden or assertion changes.

Extend `TestASDV02478OffloadKeepsValidatedFlags` with `/usr/bin/env GOFLAGS=` and a
receiver-only GOENV `-skip=TestExisting`. The real offload fixture must return the broken
existing test's failure remotely; its existing assertions also reject local fallback.
The new case fails against the unfixed receiver: the remote fixture package incorrectly
passes and returns zero. This is execution evidence, not only a `go env` probe.

Rollback: revert this repair's receiver case arm, regression row and spec evidence-table
update together. V1-5007 remains PARTIAL for the previously evidenced absent pre-commit
entry point. The other heavycap tickets receive no implementation changes in this round.

## Verification and limits

The affected receipt selects tools/test-commands-check as DIRECT_TEST_CHANGE. Its 69
unknown entries remain retained; no broad selection or repository gate is claimed.
The installed binary reports self-binary-stale, so the plan retains that limitation.
All Go tests run through heavy.sh, one package per command, without exported commit
identity. The focused negative control returns 1 with the expected assertion failure.

Raw evidence stays under `/home/russell/lanes/heavycap-evidence/`, and scratch uses
TMPDIR=/home/russell/lanes/heavycap.tmp, mode 0700. Existing pre-change evidence is reused
without a new capture. Final CEM binding/checking follows all source and evidence edits.
Independent review, broad host checks, publication, Tasks writes and sealing remain
owned by the orchestrator or explicitly excluded by the charter.

The final whole tools/test-commands-check package passes with every existing test and
no -run filter: 55.448 s, wrapper rc=0, queue=8.0 s. The negative control fails in
6.303 s, wrapper rc=1, queue=64.1 s, with the remote fixture incorrectly returning zero.
`make -s spec-requirements-check line-citations-check` passes: internal/specindex whole
1.204 s (45.0 s queue), existing internal/lrfrepo selection 0.098 s. Citation repin after
staging found no moved citations; no second repin was needed. `bash -n script/heavy.sh`,
`git diff --cached --check` and the go1.27.3 toolchain identity check pass.

Commands (TMPDIR as above; HEAVY_WHOLE_REASON records the affected touched-package basis):

```sh
script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run '^TestASDV02478OffloadKeepsValidatedFlags$/absolute-env-explicit-empty$' ./tools/test-commands-check
script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./tools/test-commands-check
script/check-line-citations.sh --repin
make -s spec-requirements-check line-citations-check
```

Friction: unchanged configured slots were saturated for the waits above. The runner also
warned of low /tmp space but admitted the command using its configured cache and scratch
policy. No semaphore or cache override was introduced. External pre-commit integration
remains unavailable as previously recorded. Final CEM/report results belong to the handback.

Evidence SHA-256 (external directory named above):

- absolute-env-affected.json: `4363ae516b6f869a48f4fb44f336813e7c589a123e730842c73120edba68022f`
- absolute-env-affected.stderr: `0ee4ade61bddb0b8c027b78fa8737c82aace500a40af0cd6dfa11e8abed5f83e`
- absolute-env-red.log: `7b64a13675ab89ea994e167197f8c85c09f869fa1e0b0b0f0c99a3e7054b6003`
- absolute-env-package.log: `b63d0f3eac5d3c562bd284693c4a874eb1176df3e3a0850f392e4c84a16162dd`
- absolute-env-repin.log: `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133`
- absolute-env-spec-lines.log: `fae5b0e1e96db0e4f86da83d7ba8d12419d2ba224e8cef73164d1265a752e58b`
