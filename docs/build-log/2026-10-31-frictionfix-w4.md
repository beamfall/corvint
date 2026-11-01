# Lane frictionfix-w4: heavy.sh, batch-precheck, analyzer-schema and spec-gate friction

Lane `frictionfix-w4` (Unraid host), base `origin/main` 8daec98ce4349c1583186692726d2c3a782ac647,
branch `claude/lane-frictionfix-w4-1031`. New requirement IDs come from the lane block 890-899:
`ASD-V0-890` to `ASD-V0-895`, `IDX-SNAP-V0-890` to `IDX-SNAP-V0-892`, `SRG-V0-890` and
`SRG-V0-891`, each checked free on `origin/main` and in the worktree before use.

## Tickets

- V1-3570 (heavy.sh remote GOMAXPROCS bound): BLOCKED. The bound it tunes, V1-1628 AC3's
  `GOMAXPROCS` set to the remote `-p`, is not on this base: `git grep GOMAXPROCS -- script/heavy.sh`
  is empty at 8daec98ce and `git log --all -S GOMAXPROCS -- script/heavy.sh` finds nothing on this
  host. It lives on the unmerged scripts-w2 lane branch (head 0e15029fe, not fetched here), and its
  acceptance needs timings on the wsl-lanes remote, which this lane cannot reach.
- V1-3506 (one refusal for every missing heavy.sh reason): delivered, `ASD-V0-890`.
- V1-3397 (heavy.sh takes the corvint test selection as its reason): delivered, `ASD-V0-891`.
- V1-3479 (`-run Test` and similar count as whole-package): ALREADY-DELIVERED on the base by
  f6992f6b7 (`ASD-V0-860`, `run_selects_all`), pinned by the `lane-scripts-test` loop over `Test`,
  `-run=.`, `^Test.*`, `.*/Sub` and the empty pattern, which all refuse, and `-run TestAll`, which
  stays selected. The lane-host copy `~/lanes/heavy.sh` carries the same rule.
- V1-3572 (batch-precheck skipped-row overhead): delivered, `ASD-V0-892`.
- V1-3260 (batch-precheck unknown list on one line): delivered, `ASD-V0-893`.
- V1-3387 (batch-precheck_test TMPDIR and step counts): delivered, `ASD-V0-894`; the macOS run of
  the default `TMPDIR` is left to the orchestrator's Mac.
- V1-3574 (analyzer-schema pin line conflicts): delivered, `IDX-SNAP-V0-890`, `IDX-SNAP-V0-891`.
- V1-2976 (analyzer drift learned only after the seal): delivered, `IDX-SNAP-V0-891`.
- V1-2356 (markers, blob shards, bump outside the semaphore): delivered, `IDX-SNAP-V0-892`.
- V1-3575 (SRG-V0-980 refusal text and the charter rule): delivered, `SRG-V0-890`. The charter rule
  is in `docs/LANE-ORCHESTRATION.md`'s requirement-ID block paragraph; the orchestrator's own
  charter template on the Mac is outside this repository and needs the same sentence.
- V1-3295 (a definition outside `## Requirements` passes spec-requirements): delivered,
  `SRG-V0-891`.
- V1-3391 (test-read-scopes over-declaration): delivered, `ASD-V0-895`.

## Decisions

- heavy.sh reasons (V1-3506): both rules run first and each keeps its own stderr line and log
  line; only the stdout marker is joined (`REFUSED whole-reason-missing+local-reason-missing`),
  plus one `set both` line, so every existing grep on the single-rule refusals still holds.
- Selection as reason (V1-3397): heavy.sh reads the `corvint test --dry-run --manifest` JSON
  rather than calling Corvint, so the shell never decides the selection. A manifest is accepted
  only for the HEAD it was planned at (`source.head`), which makes a stale manifest refuse after a
  commit instead of admitting packages the new commit may not select; regenerating costs 2.6 s
  here. `--plan` prints one command per unit so a 200-package procgroup selection becomes a
  per-package plan. `corvint test`'s own `testWholeReasons` (STR-V0-073) already prints a reason
  line for a whole run; the manifest route covers `--dry-run` users without retyping it.
- Skipped rows (V1-3572): the cause is `gate-ledger run`'s per-step key, a temporary index of the
  whole worktree (8 git calls over 10,533 tracked files). Measured here: 586 to 593 ms per call,
  hit or miss, against 2 ms for the bare `printf`; under the Mac's parallel step load it was 3 s.
  A print-only row now runs directly. The ledger key itself is unchanged.
- Analyzer pins (V1-3574): the input-digest pin line is removed instead of being moved, because
  any pinned value derived from the input bytes conflicts when two branches edit inputs. Since
  `IDX-SNAP-V0-107` the golden output already decides whether a number moves, so the input pin
  only forced a mechanical repin. `TestAnalyzerSchemaInputs` keeps discovering and reading every
  input and logs their digest; the schema number pin moved into `TestAnalyzerSchemaOutput`.
- Shard coverage (V1-2356): shards stay off by default (decision 0073), so their facts get their
  own pin, `auditedShardSHA256`, over `blobShardPayload` (the bytes `writeBlobShards` stores,
  factored out unchanged) instead of being folded into the pack digest, which would have forced a
  needless schema bump. A base without the shard pin compares the pack only, so adding the pin
  needed no bump: `script/analyzer-schema.sh check --output` reports "no bump needed" against
  origin/main. The corpus already yielded one marker (`feature:seed-demo`); the new test proves a
  marker-only index change moves the pack digest.
- Before the bind (V1-2976): `dogfood-finish.sh` runs the static check and, only when it notes a
  changed input, `check --output` (about 8.5 s here). `dogfood-change.sh` names the changed inputs
  through the new `script/analyzer-inputs.sh`, so it does not reach `script/heavy.sh`, the
  shell plugin's unbounded reader; only `script/analyzer-schema_test.sh` (about 1 s) became an
  unbounded unit.
- Spec gates (V1-3575, V1-3295): the generator mirrors `check-requirement-definitions.sh`'s
  section tracking and reads its frozen RDG-V0-021 baseline, so both refuse the same lines.
- Fixture marker (V1-3391): a trailing `//corvint:fixture-path` comment on the literal's line, not
  a new JSON record, because the existing per-path `.corvint/test-read-scope-literals.json` is the
  file-level form and the ticket asked for a per-literal one. A token an unmarked literal of the
  same file also holds is still checked. `ASD-V0-895` is filed in the self-development spec, not
  `docs/specs/affected-plan-v0.md`, which the charter keeps out of intents until V1-3429.

## Evidence

- `make lane-scripts-test`: 36 groups passed; the V1-3506 group fails with the joint-refusal
  ordering reverted (mutant run).
- `script/batch-precheck_test.sh`: PASS with `TMPDIR` ending in `/`, run with a lane-local `node`
  stand-in because this host has no node (group 1b); the group 4b assertion fails with the
  print-only bypass removed (mutant run).
- `script/analyzer-schema_test.sh`: ok, including two output-neutral `receipt.go` branches that
  merge with no conflict and move no pin.
- `TestAnalyzerSchema*` and the blob-shard tests of `internal/contextindex`: pass; changing
  `blobShardVersion` alone fails `TestAnalyzerSchemaOutput` (mutant run).
- `script/gen-spec-requirements_test.sh`: ok (cases 13 and 14).
- `tools/test-read-scopes-check` and `tools/dogfood-inherit-citations` packages: ok; `make
  test-read-scopes-check` passes without the `docs/specs/` entry.
- `script/dogfood-change_test.sh`: PASS; `script/dogfood-cache_test.sh`: PASS with a lane-local
  `rg` to `grep` shim, since this host has no ripgrep.
- `make doc-gates`, `make line-citations-check`, `make windows-vet`, `gofmt -l`: clean at the
  final commit.

## Friction

1. `~/lanes/heavy.sh` (the Unraid lane copy) lacks the `ASD-V0-074` small-package exemption:
   `~/lanes/heavy.sh env GOTOOLCHAIN=local go test -count=1 ./tools/test-read-scopes-check/` is
   refused for a tools/ package with 31 KB of tests, which `script/heavy.sh` admits (limit
   49152 bytes); the lane typed a reason.
2. This Unraid host has no `node` and no `rg`: `make batch-precheck-test` fails at group 1b
   (`node: command not found`) and `script/dogfood-cache_test.sh` at line 98 (`rg: command not
   found`), before any change-related case runs. Stand-ins were needed to run them.
3. `TestRepositoryShellForeignCallersComplete_TSP_V0_005` says only "the shell plugin refused
   .corvint/shell-foreign-callers.json" when a caller list is unsorted; the cause (order) is not
   named. Repro: append a caller out of order to the `script/heavy.sh` entry and run `make
   shell-foreign-callers-check`.
4. Mentioning `script/heavy.sh` in a script comment makes it a declared mention under TSP-V0-005,
   and referencing it from a script reached by `dogfood-change.sh` turned
   `script/runtime-environment_test.sh` into an every-change unbounded unit; the only hint is the
   `unbounded-readers-check` import chain, found 6 minutes into `make doc-gates`.
5. `make doc-gates` takes 6 minutes on this host and stops at the first failing sub-gate's
   summary, so two unrelated failures (line citations, shell callers) cost two full runs.
6. `make dogfood-change` after the first commit exits 2 (expected, no citations yet), which reads
   like a failure in a lane log; the note it prints is easy to miss in the long report.
7. The installed `corvint` was `DEGRADED self-binary-stale` (built from f1680433b107, 21 Go paths
   differ); the lane built private binaries into `~/lanes/bin-w4`.
8. `corvint affected` on a dirty tree reports `scope=UNKNOWN` because the untracked
   `.corvint/change.cem.json` sidecar from `make dogfood-change` is an `UNOWNED_DIRTY_PATH`, which
   widens the selection to 184 units for a scripts-only lane.
9. V1-3570's ticket names a GOMAXPROCS bound that is on an unmerged lane branch; the ticket was
   assigned against a base that does not carry the code it tunes.

## Review fixes

- P2 numbered manifest slices: FIXED in bfe9fdd8c (spec a3b7b1654). `internal/sessiontest/plan.go` emits `I/N` slices with flag `-run` or `-skip`, but `--plan` rendered a pattern only for slice `selected` and printed duplicate whole commands that admission refused. `--plan` now prints each slice's own flag and pattern. A command with one `-skip PATTERN` (or `-skip=PATTERN`) is admitted when the manifest lists that `-skip` slice. A slice with no `-run`/`-skip` pattern is listed as skipped. The `lane-scripts-test` group `ASD-V0-891` fails 3 checks before the fix and passes after it.
- P2 missing test timeout: FIXED in bfe9fdd8c. Every `--plan` command carries `-timeout 30m`, the Makefile `GO_TEST_TIMEOUT` (decision 0082), and the plan-output test asserts it.
