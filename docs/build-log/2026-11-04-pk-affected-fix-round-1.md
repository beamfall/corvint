# pk-affected fix round 1

Owner intent: the 2026-11-04 fix-round charter, base
`6c33df33bc5670131482f5263b410cb980580657`, following round-0 head
`4e4f5501f86dbd1cb98019486ead9c3c7ca0b321`. Independent review returned CHANGES_REQUIRED;
this entry records the repairs and corrects round-0 acceptance overclaims. No history is rewritten.

## Decisions and repairs

1. TSP-V0-7303/7311: use json/v2 `omitzero` for the new boolean witness field. False labels
   disappear, and viaLength remains mandatory in full witnesses. The two exact map-string
   expectations and frozen full-range document now include viaLength. An audit of added bool/int
   tags in json/v2 files found no other introduced omitempty scalar. The pre-existing Count tag
   is outside this lane's diff; compact witness structs use the existing v1 canonical encoder.
2. STR-V0-7313: the parity tool requests --full explicitly. Its regression feeds actual captured
   local text and JSON outputs through the subprocess parser; text is refused, full JSON decodes.
   Captures came from a real one-package `example.test/parity` repository, same dirty comment,
   base and declaration for both projections. Remove the generic runCLI output shim; restrict
   runTestPlanCLI's shim to argv[0] == test. JSON-consuming callers now spell --full themselves.
3. STR-V0-7305: TestExecuteCompletionTime_STRV07305 executes an empty selected shard with a fixed
   offset clock, asserts the exact UTC RFC3339Nano field in memory and canonical bytes, reads it
   back, then reads a canonical legacy receipt without the field. It launches no external runner.
4. Drop the duplicate publication implementation and its tests, restore check.go exactly to BASE,
   and remove DWC-V0-7319 from the companion. V1-4573 is superseded by sealed v1fix-a's DWC-V0-929.
   This preserves the earlier EXDEV failure as history; it does not retain two competing fixes.
5. AFP-V0-006 now includes unsupported-affected-temp. AFP-V0-990's changelog explicitly records
   the string-to-object by-path witness wire change under TSP-V0-7311.
6. TSP-V0-7305 / AFP-V0-2487: choose the conservative bounded-I/O option from review. Stat all
   candidates to disclose failures, retain the newest mtime prefix of at most 64 (name tie break),
   then read only those headers. A completedAt outside the prefix cannot revive a discarded file.
   The prefix regression counts probes, including tied mtimes; a second regression saturates
   metadata failures and asserts the 4096-byte diagnostic cap including separators.
7. TSP-V0-7318 / AFP-V0-549: normalize against the arithmetic mean of adjacent calibration probes,
   with the existing nonzero denominator guard. The transition example is now 80ms; the sustained
   240ms regression normalizes to 160ms and must still fail the 100ms bound. No live budget is relaxed.

## Corrected ticket qualifications

- V1-4381 remains PARTIAL: no full-package quiet-host/CI A/B was run. Exactly 209 serial inventory
  rows use the generic runContext/time.Now reason; they are not individual isolation audits.
  Round-0 shuffled subset observations are retained but do not close that acceptance criterion.
- V1-4576 remains PARTIAL: twenty parallel full-suite runs were NOT_RUN. The round-0 focused count20
  invocation timed out after 19 absolute-budget SKIPs and is NOT_QUALIFIED, not twenty passes.
- V1-3880's reuse regression stands under IDX-SNAP-V0-019/115; timing evidence is weak. A fixture's
  cold-versus-hit timings are not a before/after implementation A/B, and no speedup is claimed.
- V1-3728 AC2 is PARTIAL: the AHI-803 sibling attribution test is a fixture. No real retained session
  ledger was counted for post-tool path-outside-project rows. Zero fixture rows do not close AC2.
- V1-3354 is met by supersession: CIN-V0-860 as amended by V1-3480 option B governs the exceptions;
  the old blanket non-language-frontier criterion is not claimed to have passed.

## Verification and limits

Starting make dogfood-change retained bound prechange query and impact receipts; it refused the
missing citation/intent/outcome inputs as expected before final binding. The affected plan selects
cmd/corvint, internal/liveverify/affected, internal/sessiontest, internal/dogfoodflow and tools/str-parity;
scope UNKNOWN and the other selected units/unknowns stay visible in the retained plan.
The new native binary builds through script/heavy.sh. A real full affected fixture emits viaLength:1
and omits anyDirtyPath for its direct-source witness. This is a smoke observation, not a Go test pass.

External sandbox blocker: mkdir -p /var/tmp/pk-affected-fix returns Read-only file system.
The directory is outside the supplied writable roots. The mandated heavy.sh test invocation refuses
unwritable TMPDIR before launching Go. A request for the required directory or an explicitly approved
cache fallback is pending; no fallback is silently substituted. Thus focused test counts, timed
snapshot evidence and CORVINT_UPDATE_GOLDEN=1 regeneration remain NOT_RUN until that dependency is
resolved. The frozen document's viaLength correction is mechanical and unqualified by regeneration.
The make spec-requirements generator completed, but its implicit lrfrepo test initially refused the
unwritable configured /home/russell/bench/rt/.locks. Generation-only --write then succeeded; its Go
obligation check shares the TMPDIR blocker. Ten lane-moved hash-pinned citations were repinned using
the native --repin command, which confirmed unchanged content.

The orchestrator owns the independent review and native ticket writes. No exhaustive gate, Windows
vet (no push), publication, seal, learning evaluation or external platform qualification is run here.
Local CEM binding/checking remains structural evidence only; blocked tests and partial qualifications
must remain visible in the handback and outcome.
