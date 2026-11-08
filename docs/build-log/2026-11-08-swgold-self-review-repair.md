# V1-4710 self-review repair: preserve must-run dependency selections

Scope: AFP-V0-9322; base `8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734`.

## Finding and decision

The earlier disconnected fixtures did not exercise typed dependency slicing.
`mustRunOf` retains an existing dependency witness; the last `testTypedSlices`
pass then overwrote the whole-package slice established by `testGoSlices`.
The typed pass now excludes every declared must-run unit before admitting a
candidate, preserving the existing whole-package reason for any witness kind.
This implements the existing AFP-V0-9322 obligation without changing its contract,
wire fields, error/exit codes, stderr, frozen outputs or committed goldens.

The soundness class is narrowing a project-owned obligation using a weaker
selection witness. The closed admission set is Go selections with no must-run
obligation, exactly `DEPENDENCY_PATH`, and one of the existing relative packages
`cmd/corvint`, `internal/tasks/cli`, or `internal/tasks/store`. All existing typed
proof checks remain necessary. Unknown witness kinds and packages outside that
allowlist retain the whole-package selection; no list of forbidden witness forms
was introduced. Rollback of the guard reopens the reported omission; the connected
regression detects that loss, so rollback requires an explicit disposition of
AFP-V0-9322 rather than removing or weakening the regression.

## Regression evidence

`TestAffectedFrozenProfilesDependencySlices_AFPV09322` extends the disposable
fixture with `TestOther` calling `engine.Value`, then changes `Value`. Both the
declared and undeclared plans must actually retain `DEPENDENCY_PATH`. The declared
case must keep the whole package and the frozen-test reason. The undeclared control
must demonstrate typed narrowing to `^(TestOther)$`. A future, unknown witness kind
must stay whole with and without the must-run declaration. Existing disconnected
omission controls and assertions remain intact.

Before the fix, the declared dependency case failed: typed slicing overwrote the
must-run reason. After the fix it passed, as did both unknown-witness cases and the
earlier AFP-V0-9320/9322 regressions. The focused suite remains failed because the
sandbox prevents reading the undeclared control's base: `the base of
engine/graph.go is unreadable: cannot be listed at base`. That assertion remains
intact for host qualification; no typed-narrowing success is claimed locally.

The selected test package is `cmd/corvint`, confirmed by `corvint affected --base`
against the base above (UNKNOWN scope; 349 selected units). Go tests run only through
`script/heavy.sh`, one package per command, with the required TMPDIR and without
exported commit identity. No pre-change capture was repeated. Independent review,
repository-wide gates, publication, native Tasks writes and sealing remain reserved
for or excluded by the orchestrator's instructions.

## Whole-package run and retained limits

The unfiltered `./cmd/corvint` run executed the existing package tests and failed
in 269.335 s (wrapper 270.7 s), with 255 top-level failures. These include process
containment refusals, unreadable checkout Git metadata, unsafe scratch ancestry,
the previously observed frozen `advice.whole_packages` mismatch, and the new
undeclared dependency control's unavailable base. Neither declared regression
case nor the unknown-witness cases failed. Full package qualification remains
PARTIAL pending the host rerun and the previously recorded baseline-golden
resolution; the failed local result is not a passing gate.

`make line-citations-check`, `make changed-checks BASE=...`, and `git diff --check`
passed. `script/check-line-citations.sh --repin` found no moved unchanged citation;
180 pre-existing unresolved-basename citations remain skipped. The Go runtime is
`go1.27.3`. No spec requirement or line citation changed in this repair round.

Raw artifacts remain outside the repository under
`/home/agent/lanes/swgold-evidence/`:

| Artifact | Bytes | SHA-256 |
|---|---:|---|
| `repair-red.log` | 1751 | `db2b6f9fdd52cd7a2b0b20d3f6464a44201439baf8cb5fef88ade36ad8acd82f` |
| `repair-focused.log` | 1275 | `463a13c21a6d88cdbc2ef7db477e4f84cd2bf163cb9f3bee6a70364b627f3880` |
| `repair-whole.log` | 320678 | `c32ba39b182b19da26806cf6ee24e7594d3241ac377285c2ec75a2eff5443c7e` |
| `repair-affected.json` | 150262 | `3a9caf781b0a2f111c5cefa542e97737f977a602652474c29b69fc76d969f30a` |
| `repair-changed-checks.log` | 1273 | `b77e46218bc42df0c10254e25f4dc3847c4c89457464ae20496dabf637688162` |
| `repair-citations.log` | 118 | `855a4551b199f407e4e7cec705511746022a39aa0f70383fc13aac9acc748bba` |

Final CEM and check outcomes, including every NOT_PRODUCED reason and the final
head, are retained in the lane handback at `/home/agent/lanes/swgold.handback.md`.
