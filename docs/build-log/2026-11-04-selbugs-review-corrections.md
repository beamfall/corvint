# Selection correctness: required checks and fail-closed inputs

Owner intent: correct all three findings from the independent review of `3608d92a4`,
keeping the original lane base `6f12f4704235dc8f9bea19fc12995bfb68c746c3` and requirement IDs.
The review verdict was CHANGES_REQUIRED. Its two P1 findings and one P2 finding have
regression controls that fail against that head. This record describes local evidence;
the orchestrator owns the next independent review and integration.

| Finding | Requirement | Change and proving evidence |
|---|---|---|
| P1: shell job failures bypass the required check | CIN-V0-2320, V1-3489 | The required job depends on the shell job and checks its result before any early return. Workflow tests execute its actual shell for PR, merge-group and push events, with successful, failed, cancelled, skipped and missing results, including batched early returns. Only pushes admit the intentional skip. The regression fails against the reviewed workflow and passes after the fix. |
| P1: a same-line Darwin declaration is omitted | ASD-V0-2403, V1-4245 | Go parsing and canonical formatting expose every same-line declaration before scanning. Parse/tool failures, block comments, remaining semicolons and unscannable identifiers keep the whole command local. The offload fixture injects a failing second same-line test: the reviewed code returns zero with a regex omitting it; the fixed code returns the injected status 6. Existing split, whole-local, shared-helper, split-header and local-failure controls pass, together with malformed/commented/semicolon/Unicode syntax controls. |
| P2: incomplete receipts narrow to no steps | AFP-V0-2401, V1-3489 | Missing or null arrays, malformed members and missing or invalid selected identifiers take FULL. Unknown entries require reason and detail. Complete empty arrays still narrow. The new receipt matrix fails against the reviewed selector and passes after the fix. |

Each fix includes its governing spec amendment in the same commit. The CI fix is a separate
commit from the local wrapper and selector fixes. No new requirement IDs were allocated.

Focused commands, all passing after repair:

```sh
HEAVY_WHOLE_REASON='selbugs lane' bash script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./tools/shell-test-select/
HEAVY_WHOLE_REASON='selbugs lane' bash script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./tools/ci-reuse-plan/
make spec-requirements
make line-citations-check
make windows-vet
```

Darwin regression commands and negative-control outputs are retained in
`/home/russell/.cache/selbugs-review/`. The private runner preserves the suite's shared setup
and selects only offload group 9; it changes only the numeric group predicate, avoiding the
unrelated suite's background/concurrency cases. Its before copy uses the reviewed wrapper;
its after copy uses the delivered wrapper. Go and SSH are fixture stubs; this is not native
Darwin/WSL qualification. The injected failure models a test failing only if the local regex
selects it, or the entire package stays local.

The first complete documentation-gate attempt caught fixture function names in the spec's
traceability table: they were interpreted as repository test names. The table now describes
the fixture without naming those functions; the focused traceability gate passes. The first
CI package run also caught an older exact dependency assertion, which was updated with the
new dependency; the complete package then passed. Original before/after range measurements
remain in `2026-11-03-selbugs-selection-correctness.md`; no new performance claim is made.

Independent review, hosted CI execution, native Darwin execution, publication, ticket writes
and sealing are NOT_RUN or NOT_PRODUCED under this round's instructions and host limits.
The default finish command always seals, so the documented per-step binding/rebinding/check
sequence is required to honor the explicit stop before seal. Final check outcomes, commands
and the complete cumulative friction list are retained in the lane handback.
