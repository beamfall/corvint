# Behavior adapter: bounded reconciliation frontier (V1-1089)

Date: 2026-10-21. Base: `5cf75daf0` (`origin/main`). Requirement: `DCP-V1-240`
(`docs/specs/documentation-corpus-v1.md`), accepted under the owner's standing lane auto-accept
(2026-10-11).

## Finding

`BuildBehaviorAdapter` appended one frontier diagnostic per missing outcome per test with no cap.
The library build returned the result, but `corvint docs corpus behavior-adapter` then refused it at
`Encode` (`output bound exceeded`). `--check` skips reconciliation (`DCP-V1-044`) and accepted the
same request, so the two disagreed.

## Decision

The ticket left the bound's shape open. The lane chose a count cap with an explicit count, which
keeps the existing result schema: at most 4096 diagnostics are retained, and the new member
`frontier_omitted` counts the rest. That member is `omitzero`, so every result that fits stays
byte-identical, and no schema version bump is needed. The alternatives were an in-band marker
diagnostic and a new result schema. The marker would hide a number in prose; the schema version
would break every stored previous result for a case that already failed to encode.

The retained entries are the smallest under a total frontier order. That order is the existing sort
key (kind, subject, input, field, detail), extended with revision, digest and correction as
tie-breakers. The retained set therefore does not depend on reconciliation order. While building,
the frontier is compacted each time it reaches twice the bound. After the first compaction, a
diagnostic at or above the largest retained entry is counted without being stored. A previous
result with a negative count is refused. Coverage is computed independently of the frontier and is
unchanged. `TestDCPV1240BoundedFrontierIsTheCanonicalPrefix` proves this with small bounds against
the unbounded frontier.

## Measurement

Repro `behaviorAdapterOutcomeFrontierRequest(t, 4096, 200)`: a 787,469 B request. One run on a quiet
host via a scratch test; live heap was measured after GC with the result held.

| | retained | omitted | live heap growth | build | CLI encode |
|---|---|---|---|---|---|
| base `5cf75daf0` | 820,607 | (none) | 208,980 KiB | 2.36 s | refused, output bound |
| this change | 4,096 | 816,511 | 822 KiB | 0.17 s | accepted |

On the base, `TestDCPV1240BuildBoundsTheFrontier`'s count assertion fails (820,607 retained, against
4,096 expected).

Rollback: revert the commit. Results that carry `frontier_omitted` are then refused as previous
results by the older closed decoder.
