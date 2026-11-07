<!-- SPDX-License-Identifier: Apache-2.0 -->
# mbapproved: two typed-reach soundness findings from the batch review

The independent review of the mbapproved batch (entry `2026-11-06-mbapproved`) returned
CHANGES_REQUIRED with two P1 findings in the floor lane's V1-2744 typed-reach slicer. In both, a
narrowed package drops a test that the change affects. AFP-V0-2376 is amended at `babfbfc50` and
uses no new requirement ID; each fix is its own commit with its regression.

1. `BaseEffects` rooted only changed functions, methods and the methods of changed types. Code a
   changed package variable holds was never checked at base. Take `var Hook = func() { State = 1 }`
   edited to an empty literal: head reach keeps the test calling `Hook` and drops the test reading
   `State`, which then fails at head. `b5b0e0801` adds the held node of every changed variable,
   matched by object or by changed package, as a base root, so a removed write in a held function
   literal, function value or method value refuses with `BASE_MAY_WRITE_SHARED_STATE`. A held
   literal that calls was already an initialization change in `DiffFile` and still is.
2. In `fileDecls`, every blank declaration shared the key `_`, so the last one hid every earlier
   one. Editing the first `var _ = setup(1)` without moving a position therefore vanished from
   `DiffFile`. An ordinary edit in the same change then kept the declaration set non-empty, and the
   package was narrowed past the changed initialization. `26882a498` keys every blank function,
   type or value by its ordinal among the file's blanks, each with its own initialization status,
   as `init` functions already were.

## Evidence

- `TestAFPV02376BaseEffectsSeeHeldCallbacks`: before `b5b0e0801`, the function-literal,
  function-value and method-value cases failed with `base effects "", want writes=true`, after head
  reach had dropped `TestBState`. The pure-callback control and the calling-literal
  initialization case passed. All five pass after the fix.
- `TestAFPV02376BlankInitializersKeepOwnKeys`: before `26882a498`, the first, middle and
  last-before-a-blank-constant edits failed with `blank initializer edit hidden: {Names:[Pure]
  AlwaysRun:}`. After the fix each keeps the package whole with `ALWAYS_RUN_REACHES_CHANGE`, and
  the ordinary edit alone still narrows to `TestPure`.

## Verification

`corvint affected --base 4052b854...` selects `internal/liveverify/affected/golang/typedreach`
(whole, `DIRECT_SOURCE_CHANGE`) and `cmd/corvint` (whole). It lists the third importer,
`tools/typed-reach-measure`, as `NO_SELECTABLE_TEST`. `corvint test --dry-run` plans FULL scope,
so both packages run whole. Each ran under `script/heavy.sh`, one package per command, and passed:
typedreach whole, and `cmd/corvint` complete in `script/go-test-shard.sh` shards 1 to 8 (1493
inventoried names). `tools/typed-reach-measure` builds and vets, and `make windows-vet` passes.

## Rollback

Revert `26882a498` and `b5b0e0801`, then `babfbfc50`. The slicer then returns to the batch head's
behaviour with both gaps open, so instead set `typedReachPackages` empty to stop narrowing.
