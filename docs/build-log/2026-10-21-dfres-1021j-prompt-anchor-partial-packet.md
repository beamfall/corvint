# Prompt context keeps the first 32 anchors instead of refusing

Lane dfres-1021j, ticket V1-2276. Requirement `LCP-V0-035` in
`docs/specs/local-completion-policy-v0.md`, accepted under the owner's standing auto-accept
(2026-10-11). `LCP-V0-034` belongs to open PR #1453 and is not reused.

## Problem

`DogfoodPromptContext` refused any prompt whose derived explicit anchors exceeded 32
(`unsupported-dogfood-context-bounds`), so a long prompt lost its whole context packet; PR #1453
only renames the hook-side code to `dogfood-event-prompt-bounds`.

## Decisions

- `addAnchors` keeps the first 32 anchors in a fixed order (requirement IDs, mentions, paths,
  identifiers, each already sorted) and resolves only those.
- The dropped anchors are raw task terms, which `LCP-V0-011` keeps out of the packet, so the
  packet names them by kind and count (`dropped_anchors`, `dropped_anchor_kinds`) rather than by
  text. This is a deliberate narrowing of the ticket's "names the dropped anchors".
- The two fields appear only when anchors were dropped, so every packet within the bound keeps its
  bytes (no golden or host projection changes).
- An otherwise resolved prompt reports `unresolved`/`anchor-bound-exceeded`; earlier reasons keep
  precedence.
- `unsupported-dogfood-context-bounds` stays for the 128-candidate definition bound; the byte
  bound stays `invalid-dogfood-context`.

## Evidence

- `TestDogfoodPromptLCPV0035KeepsFirstAnchorsInOrder` fails before the fix (the refusal) and
  passes after; `TestDogfoodPrompt*` in `internal/contextindex` pass.

## Not delivered in this lane

- V1-2282 is in flight in PR #1451 (`DCW-V0-115`: the prepare and bind passes no longer run the
  recorder, so the by-design `record-index-failed` rows of the uncommitted sidecar disappear).
- V1-2281 is skipped: `benchmarks/untouched-repository-v1/preregistration.json` is sealed with
  run-001 recorded, and its invalidation rule makes any `harness.py` change a new numbered
  preregistration, which is an evaluation-freeze decision for the owner.

## Rollback

Revert the change: `addAnchors` calls `failBounds` again past 32 anchors and the two resolution
fields disappear. No state or format migrates.
