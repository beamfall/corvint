# 2026-11-04 floor: runner source, positions and shared-state soundness

The review of `44a62f847da872a8bff7682dfb2159f51c67913d` found four P1 gaps in
V1-2744's experimental typed test narrowing. This round retains the inherited work and
amends the existing AFP-V0-2371, AFP-V0-2374 and AFP-V0-2376 requirements with the fixes
and regression tests in the same commit. No requirement IDs change.

## Decisions

- **AFP-V0-2371, runner configuration:** source discovery enables cgo for module packages
  so their cgo source triggers refusal. Any included or ignored module Go file with a build
  constraint refuses narrowing with UNMODELLED_SOURCE and the source path, rather than
  guessing which runner tags apply. Internal tests, external tests, linked dependencies,
  race, cgo and custom tags are covered. Legacy constraint whitespace uses Go's constraint
  recognizer, also for the base overlay. Standard-library loading still uses source-only
  signatures without generated cgo declarations; its existing effect model governs calls.
- **AFP-V0-2376, moved positions:** declaration comparison also fingerprints syntax-node
  file, line and column positions, including adjusted line directives. Growing A therefore
  changes the set to include an otherwise unchanged B that moved. Byte offsets alone do not
  seed changes. Dependent-package callers of B are retained.
- **AFP-V0-2374, logger receivers:** Logger.SetPrefix, SetFlags and SetOutput may write the
  default logger unless receiver ownership is proved. Calls, bound method values and method
  expressions carry the write even when the receiver has no package-variable root.
- **AFP-V0-2374, signal cleanup:** remove the rule that any Stop in a body cancels every
  Notify write. Channel identity and restoration on every exit are unproved, so retain the
  write, including with a deferred Stop. Mismatched channels, conditional cleanup and early
  returns now retain later disposition readers.

This intentionally gives up precision. In particular, real tasks/store sources carry build
constraints, so the old 648/666 narrowed-store measurement is historical evidence for an older
model, not a current delivered narrowing claim. A replay against the retained d1ccab3b6 range
now refuses at gate_process_unix.go with runner configuration parity unproved. Synthetic
allow-listed packages without constraints still narrow. Matching runner configurations before
admitting constrained packages is future precision work, not an assumption in this change.

## Discriminating evidence

The new regressions are TestAFPV02371RunnerConfigurationRefusesHiddenSource,
TestAFPV02376MovedPositionsKeepDependentReaders,
TestAFPV02374LoggerReceiverSettersKeepOrderedReaders and
TestAFPV02374SignalCleanupCannotEraseWrites. Each finding has an observed failing negative
control against the inherited implementation. The signal control restores only the old graph
rule for that run, then restores the delivered source. Variadic signal arguments isolate this
rule from the separate opaque-interface conservatism. No nested reviewer was invoked.

The complete typedreach package passes through script/heavy.sh. Focused command integration
also passes. In addition, six executable fixture repositories use the actual product dry-run:

| Edit observed by a test | Base suite | Changed full suite | Generated selection |
|---|---|---|---|
| race-only dependency caller, CGO_ENABLED=1 and -race | PASS | FAIL TestHidden | whole; same failure |
| unchanged B's runtime.Caller line after A grows | PASS | FAIL TestB | A and B; same failure |
| default Logger.SetPrefix receiver mutation | PASS | FAIL TestBReader | writer and reader; same failure |
| Notify and Stop on different channels | PASS | FAIL TestBReader | writer and reader; same failure |
| conditional Stop | PASS | FAIL TestBReader | writer and reader; same failure |
| return before Stop | PASS | FAIL TestBReader | writer and reader; same failure |

All Go test commands run through script/heavy.sh, one package per command. The five narrowed
fixtures drop an unrelated control, retaining 2/3 tests; the race fixture keeps the package
whole. This is six observed fault cases with zero omitted failing observers, not universal
recall. The logger and signal proofs run the ordered suites together, not tests in isolation.

Raw manifests, commands, test outputs, the fixture driver and summaries are retained under
`/home/russell/.cache/floor-lane/review4/`. The external floor.handback.md records the final
checks, head, dogfood outcome and complete inherited friction list. Independent re-review is
reserved for the orchestrator. Publication, seal and native ticket writes are intentionally
not run. V1-0643's fixture code and retained savings evidence are unchanged in this round.

Rollback remains the spec's existing removal of the typed-reach allow-list to restore whole
packages. No production promotion or expanded capability claim is made.
