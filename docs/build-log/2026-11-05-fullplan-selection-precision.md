# Helper-source and retained-evidence selection precision

Base: `9db7a94169821d9b28701eaa88866aee2fec1083` (`origin/main` at lane start).
Intent: fullplan charter, 2026-11-05; AZU-V0-9920 through AZU-V0-9922 in
`docs/specs/affected-zero-unknown-v0.md`, refining AZU-V0-002 and the `.github/` slice of AZU-V0-005.

The Go and Python claimants recognised suffixes under `.github/`, but their shared source walk
skipped that hidden root. The shell plugin already used the bounded, cache-stamped `FilesUnder`
reader there. Use the same explicit reader for Go/Python helpers, including module manifests for
Go's existing module-boundary rules. Other hidden roots and skipped subdirectories remain outside
this slice. Python's native selector test loads its helper dynamically; the maintained
`.corvint/runtime-edges.json` records that real consumer rather than inventing a static import.

The repository already declares `.agent-evidence/` in `.corvint/non-source-paths.json`. Its
classification previously handled only `UNOWNED_DIRTY_PATH`, so a retained `env.sh` instead became
`UNINDEXED_SOURCE_PATH`. An output declaration now outranks a suffix-only claim when no unit owns
the path. The same precedence applies to frontier participation: output is unowned data, not source
of the suffix's language. Actual readers and path-open frontiers still bear; indexed source,
driver inputs, task projections and undeclared source retain their existing treatment. No change
to the AFP-V0-028 allowlist, runtime-readers logic, wire shapes, error text or goldens is needed.

## Evidence

`TestFullplanReplay_AZU_V0_9920_9921` fails before the change for the three reduced replay paths,
on disk and immutable input. It checks the Python helper test, Go command test and transitive
Go helper consumer, isolated zero-unit/non-unknown evidence, and FULL for an undeclared unindexed
Python file. `TestFullplanEvidenceReaders_AZU_V0_9921` checks literal and declared evidence readers
and preserves an indexed Go source owner even under a non-source declaration. Earlier failed
fixture-development runs are retained alongside the final results; no pre-existing assertion was
changed.

The final path-set replay narrows all three cases (total units include declared language commands):

| Merge | Before | After | Go packages before / after |
|---|---|---|---|
| `39c031980` | FULL, 445 | NARROWED, 44 | 443 / 42 |
| `8e2c13aa9` | FULL, 445 | NARROWED, 209 | 443 / 207 |
| `cdeed7ee6` | FULL, 445 | NARROWED, 193 | 443 / 191 |

The indexed Go universe grows from 443 to 447 by admitting the four hidden helper packages.
The single-path Python replay selects `python:.github.macos.select_test` and the `go-macos` CI
job; the Go replay selects `go:github.com/Beamfall/corvint/.github/testconfine/cmd`.
No offending single path retains `UNINDEXED_SOURCE_PATH`. A real evidence-only path still selects
245 reader/driver units and retains 60 frontier diagnostics; those are neither fabricated source
units nor permission to suppress genuine uncertainty. The reduced evidence fixture proves zero
selected units and zero unknowns when those real consumers/frontiers are absent.

Replay harness SHA256: `c83c62c915e6c9b4f894c01e2e4e9b65820bdfade88e7bf6d9571d3ac5bd4432`.
Before JSONL SHA256: `d0a857506b7d866a25b7d50f8cd5e3abaee589e9a9238693827bea4cb32db43b`.
Final JSONL SHA256: `8499841411cb044d1cae35f8fad70bdef8c1ac1ca58e86b086d1d493643e23dd`.

Complete Go and Python plugin package tests passed, as did all six native Python tests,
`make -s spec-requirements-check line-citations-check`, and Windows vet. The required affected
package run passed its functional regressions but failed the existing incremental latency gate:
p95 177.104 ms / 32.326 ms probe = 5.48, above 5.00, with three retained rounds. The unchanged
whole-package retry passed (202.011 s), unchanged: its latency check accepted 4.42 probes
(p95 272.735 ms / 61.762 ms probe), while explicitly skipping the absolute 100 ms budget on
this slow host. Absolute-budget qualification is NOT_OBSERVED. The first failed run remains retained.

Passing affected-package retry log SHA256: `f7b7b0cc85f6f57dbcfb1dc5f1decffbffe6c93ddcc66d84c9f6c9ce6337687e`.
First failed affected-package log SHA256: `ff2e565ddb60beac36fa04ec0c2a5b6698fe9ea32d8be23b57a94629389a3a49`.
Passing Go-plugin log SHA256: `460adef51a777d1ecb5fd5f36c1da23cefe7a10806f23a6e25e07891f0123d23`.
Passing Python-plugin log SHA256: `0360ec84d363fc9f009bebefe4f9d2b5690c6527a2308b81b13c781e4408fd5b`.
Spec/line-check log SHA256: `a44b9f34fc07cac0fc24094ebb1986660617e0aca8b6b33166baf14871762e9c`.


Raw evidence is outside the repository at `/home/russell/lanes/fullplan-evidence/`; only its
measurement digests and supported counts are retained here. The replay harness selects each
merge's first-parent path set against the lane tree, uses that first parent's committed read scopes,
and feeds structural Go attribution and the exact planned Go set into `sessiontest.Build`.
It invokes neither `corvint test` nor a whole-repository selection command. These are path-set
replays against one current tree, not execution of historical tests or hosted CI qualification.

## Limits, workflow follow-up and rollback

Independent review and the host's repository-wide checks are NOT_RUN in this lane by charter.
Native Darwin execution, hosted outcomes, broader AZU-V0 zero-unknown acceptance and comparative
savings are NOT_OBSERVED. The six existing `.github/macos/select_test.py` tests ran on Linux.
The `.github/cishards/test-commands.json` Python `none` assertion predates this change and conflicts
with CI's explicit native Python test; an owner-approved follow-up should reconcile it. Hidden Go
helper packages also need explicit runner inventory entries wherever `./...` is used. This lane
edits no `.github/` files and does not claim those runner changes.

A declared evidence path adds no units itself. An isolated fixture selects zero; the real
repository must still select its evidence readers and computed driver sites. Removing these real
consumers to claim zero would weaken selection soundness.

Rollback reverts the two helper discovery additions, Python runtime edge and declaration precedence,
restoring conservative FULL planning. It does not alter or remove retained evidence or existing tests.

## Friction

1. Directory/JSON/Markdown paths in the initial prechange request refused
   `unsupported-impact-path-suffix`; repeating it before edits with concrete `.go` paths recorded
   all three receipts. The initial refusal stays visible.
2. Installed `corvint` is stale at `d436dff76de1` (54 changed Go paths at the lane base). After the
   runtime edge was declared, its old Python plugin refused `unsupported-affected-graph` because
   `python:.github.macos.select` was absent. A current-tree build is required for final CLI planning.
3. Heavy commands waited for host load and occupied slots; the first whole affected-package run
   failed its calibrated selection-latency gate as recorded above. The host semaphore configuration and
   lock directory were preserved; no override bypassed admission.
4. Line-citation repinning first reported unstaged `select.go`; after staging, the tool repinned
   one `falsifiable-packet-v0.md` citation through its two automatic passes.
