# Host gate once: V1-4743 and V1-4745

The owner authorized consolidating the repeated host Go subset while preserving mandatory Node
coverage, and repairing the separately observed Pi protected dependency-test cwd failure.
Base: `dba1f4d6c9a33098470ccf57794fb66f989f03c8`.

## Contract and implementation

The full gate's fixed prefix now begins with `host-adapter-node-test`. Its root Go suite remains
unchanged and includes every host Go test. The standalone `host-adapter-test` and declared TypeScript
command retain the Node suites and focused Go subset. Full gate prerequisites inherit mandatory
Node coverage, including through recursive make and an explicitly empty caller assignment.
GNU make 3.81 needs separate target override and export statements; its recursive command-line
assignment replay also needs the gate's mandatory value appended to `MAKEOVERRIDES`.

Every ledger key binds raw invocation `CORVINT_REQUIRE_NODE` and `CI` values, including resolved
packages and unresolved whole-tree passes. The Node-only step keeps conservative whole-tree scope.
An uncached Node availability check before receipt creation prevents a cached mandatory pass from
hiding a subsequently missing executable. Other tool-identity expansion and narrower scopes are
excluded. Requirements amended: GL-V0-001/787, STR-V0-004/920, ASD-V0-644.

The three Pi protected SDK paths derive from `import.meta.url` with `fileURLToPath`, preserving
all highlighter, YAML, HTML, upstream-version and license assertions. This repair is a separate
source commit. No dependency lock or dependency assertion changed.

## Evidence and limits

Before edits, the exact focused Go host command (`CORVINT_REQUIRE_NODE=1`, `-count=1`, `-timeout
30m`) passed at the base: package 83.399 seconds, wall 95.73 seconds. Its 37 observed top-level
`cmd/corvint` test identities all overlap the unfiltered root-module `./...` command in
`.github/cishards/test-commands.json`. The retained JSON lists each identity. This is one local
fresh-subset observation, with ordinary shared-host load and a warm compilation cache; it is not
a measured full-gate speedup or an estimate of CI savings. Removing that invocation removes its
37 duplicate top-level executions on a fresh full-gate miss without changing the root Go command.

Focused verification:

- `go test -count=1 -timeout 30m ./tools/gate-ledger ./tools/test-commands-check`: PASS
  (4.474 seconds and 9.909 seconds), through `script/heavy.sh`.
- `make gate-receipt-test`: PASS, including clear ordering, ledger on/off, caller-empty override,
  recursive mandatory inheritance, standalone optional behavior and missing-Node receipt refusal
  after a simulated cached mandatory pass.
- `TestASDV0644RequireNode`: PASS, directly checking the existing optional/missing/fatal helper.
- `make windows-vet`: PASS on configured WSL2 offload, Go 1.27.3, 35.8 seconds.
- Root invocation of `dependencies.test.mjs`: PASS, 5 tests, zero skips, Bun required.
- Full `make host-adapter-node-test`: PASS, wall 187.58 seconds. OpenCode 55 passed; Pi
  173 passed/4 existing optional skips; protected Pi 26 passed/2 existing optional skips;
  zero failures. The dependency tests themselves ran with Bun required and passed.
- Staged `make spec-requirements BASE=...`: PASS, no generated changes. Existing capacity-margin
  warnings remain visible; this change adds no requirement.

Node v22.23.4, npm 12.2.1 and Bun 1.3.11 were observed. Locked `npm ci --ignore-scripts` installs
used the existing pinned npm installation and did not modify tracked dependency files. An initial
npm 10.9.9 attempt failed the package's engine check and was corrected without overriding it.
Initial restricted filesystem cache/lock failures were retained; authorized retries used existing
shared caches. Original prechange context/query receipts were recorded before editing. Makefile
impact refused `unsupported-impact-path-suffix`; a separate supported Go/mjs impact receipt passed.
`corvint affected` retained `self-binary-stale` uncertainty for installed build88. Retrieval/ranking,
learning, external providers and host qualification are unchanged; their frozen evaluations are
inapplicable. No exhaustive `make gate` was run under the owner's scoped verification preference.

Private evidence: `/private/tmp/corvint-4743-evidence/`, including `host-baseline.json`,
`host-baseline.time`, `overlap.json`, prechange receipts/logs, selected-check logs and enrollment.
These observations do not substitute for independent review, final committed CEM/OCM checking,
remote integration or native ticket completion, which remain pending at source review.

Rollback: restore the original first gate step, mandatory setting and prior key inputs together;
restore cwd-relative SDK paths only if intentionally reinstating the prior invocation limitation.
No stored ledger records or dependency state are deleted.


## Independent-review repair: isolated missing-Node fixture

The first independent review found that the mandatory gate probe appended `/usr/bin:/bin` to
its fake-tool PATH. Removing its Node stub therefore did not hide a system Node installation,
including typical Linux installations. The repair limits that probe to its private `bin`
directory, links only `env`, `make` and `stat` for the real Makefile, and keeps its existing Go
and Node stubs. After deleting Node, a separate `command -v` assertion verifies fixture absence
before the gate refusal is exercised. Product code and the prior verification inputs are unchanged.

The full `sh -x script/gate-receipt_test.sh` passed locally on macOS. Its retained trace proves
successful fixture initialization, both ledger modes' mandatory environment inheritance through
recursive make with a caller-empty assignment, standalone optional behavior, and both missing-Node
diagnostics with no receipt. The private bin inventory is exactly `env`, `make`, `stat`, `go`,
`node` before deletion and the same set without `node` afterwards; the trace inventory and refusal
assertions were checked independently. Evidence: `repair-r1-receipt-trace.log` and
`repair-r1-summary.json` under the existing private evidence directory. `corvint affected` ran
before this check and retained its installed-binary `self-binary-stale` uncertainty in
`repair-r1-affected.json`. `git diff --check` passed. Linux execution is `NOT_RUN`: the configured
`heavy.sh` offload accepts Go test/vet and Windows vet, not a shell-only test. The isolated PATH
has no system-directory fallback on either platform. No full gate, Node suite or Windows gate
was repeated for this test-only correction. Fresh independent review and root-coordinated
CEM/OCM closure remain pending.
