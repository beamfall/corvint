# 2026-11-04 floor: preserve shared effects removed by an edit

Reviewing the base bindings exposed a further V1-2744 soundness gap: head-only reach loses
shared writes the edit removed. For example, a kept test calls `lib.Value`, which formerly set
`lib.Shared = 1`; a second test expects that value. Removing the write makes the second test
fail, but a graph of head alone drops it. This is the same obligation already present for the
older slicer in TSP-V0-006: base bodies count beside head bodies.

AFP-V0-2376 is amended at `a3dfb3140`, with implementation at `eeccc9a9e`. Before narrowing,
the loader type-checks the changed files' base bytes over the current closure, entirely in
memory. Missing, constrained or incompatible source refuses with `BASE_SOURCE_UNPROVED`.
A changed function, method, changed type's method or initialization that could write shared
state at base refuses with `BASE_MAY_WRITE_SHARED_STATE`, including indirect calls and held
callbacks. This deliberately keeps a package whole even when the write remains at head.
Added declarations absent at base need no previous-effect proof. No new requirement ID is used.

## Evidence

- `TestAFPV02376BaseEffectsPreserveRemovedWrites` proves the pure negative control, package
  writes, indirect process starts, incompatible and constrained bases, and unchanged source bytes.
- `TestAFPV02370DryRunNarrowsAnAllowListedDependent` proves the emitted whole-package refusal
  when an edit removes a shared write, alongside the existing narrowing and data-file controls.
- A separate runnable fixture at `cmd/corvint` passes at base; after removing the write,
  `TestReader` fails with `removed write 0`, and the actual dry-run selects the whole package
  naming `lib/lib.go:3 Value` with `BASE_MAY_WRITE_SHARED_STATE`. Both runs use the heavy runner.
- The complete typed-reach package and focused CLI tests pass. Evidence is under
  `/home/russell/.cache/floor-lane/finish/`; the final replays are in its `final/` directory.

## Final measurements

Both tools were rebuilt from `eeccc9a9e`, and the frozen ranges were replayed again. The
wire range `6a2c582ef^..6a2c582ef` still narrows store to 648 of 666 tests. The measured model
planning cost, now including the base proof, is 7.3 seconds. Every one of its 18 excluded tests
and four kept controls ran under the panic-injection oracle: four pass-to-fail controls,
zero missed observers, observed recall 4/4. These individual runs do not prove ordered-suite
independence or recall for every possible edit.

The raw head graphs on `c38c32203` and `d1ccab3b6` still hold 657/676 and 648/666 tests,
respectively. The new base proof refuses both, naming `decodeEntry` and `Config.validate`;
the product already keeps both whole through its earlier path-reader refusals. Their complete
model planning costs are 10.4 and 11.2 seconds. The first range's isolated fault experiment
also ran its 19 head-graph exclusions and four controls, with four observed controls and no
misses; the delivered plan excludes no tests on that range.

The final V1-0643 replay has the same result as the preceding continuation: an unrelated
path selects neither converted package, the floor is 27, and companionrelease and
release-artifact-v0 are selected 47 and 37 times over the 59 nonempty ranges in the frozen
60-merge list. The one empty merge has no dirty-path plan. Replacing only the declarations
with their base versions selects each package on every nonempty range. Retained package
timings give 1808.8 avoided package-seconds, or about 30.1 per listed merge. Timings are
reused unchanged-source evidence, not fresh elapsed-time measurements. The raw files and
drivers retain the exact merge list and separate before/after results.

Both items' measured acceptance criteria remain MET. Final independent review is NOT_RUN here:
the owner reserves it for the orchestrator. Native ticket completion, publication and seal are
also intentionally left to the orchestrator. The external handback retains all remaining
NOT_RUN/NOT_PRODUCED items and numbered friction repros.
