# Decision 0364: Compact evidence summaries with exact digest-pinned drill-down

Date: 2026-09-23. Status: proposed (experimental delivery; ticket V1-0023).

This decision:

- adds `ESV-V0-008`, `ESV-V0-009`, `ESV-V0-010` and `TCP-V0-024`;
- amends `ESV-V0-005`;
- changes no accepted wire and no `protocol/**` wire, and adds no root verb.

## Context

A `context` packet can hold up to 50 result rows, each with a full evidence row, action and summary.
An agent that needs only the answer's shape still pays for every row. It then re-reads source files
with ordinary tools, whose bytes may differ from the pinned revision the packet cited.

The existing source-view consumer (`adapter source-view`) expands one evidence row, but it needs
a saved packet file, a commit and a selector.

`ESV-V0-005` also stayed open. The manifest froze digests of the deleted Python bundle, and no
native source digest existed.

## Decision

1. `corvint context ... --summary [--summary-bytes N]` projects the exact default stdout of the same
   invocation into at most N bytes, including the LF (1024..65536, default 8192).
   - Every top-level member except `results` is kept verbatim, so coverage, critical rows,
     critical omissions, unexamined scope, answerability and the subject gap survive unchanged.
   - Rows keep packet order, identity, authority, trust, confidence, reason, line, `blob_hash`
     and `evidence_gap`, plus a handle.
   - The `summary` member records the full packet's sha256 and byte count, the row totals, an
     explicit `evidence_complete: "UNKNOWN"`, and a continuation route: rerun without `--summary`;
     `results[results_shown:]` are the omitted rows.
   - A budget that cannot hold every row through the last critical row refuses with
     `summary-budget` and never drops a critical row.
2. A handle is `cv1:TREE:BLOB:RANGE:PATH`.
   - TREE is the packet's full revision tree OID, which pins the handle.
   - BLOB may be abbreviated to at least 7 hex digits.
   - RANGE is `all` or `START-END`.
3. `corvint context --expand HANDLE [--max-bytes N]` returns only the selected lines of that blob,
   read from Git objects.
   - The read bytes must recompute the blob object ID, which is the native source digest.
   - The output reports the whole-blob sha256, the selection sha256 and exact offsets.
   - The handle refuses as `stale-handle` when HEAD's tree is not TREE, even when the blob still
     exists.
   - An abbreviated BLOB that names more than one object anywhere in the object database refuses
     as `ambiguous-handle`. It is not disambiguated through the current tree, because that would
     substitute current content for an under-specified pin.
   - Invalid grammar, hostile paths (traversal, absolute, leading dash, control or non-UTF-8
     bytes), oversize input and overflowing or reversed ranges refuse as `invalid-handle` before
     any Git read.
   - Every refusal prints nothing on stdout.
4. `ESV-V0-005` is resolved. `currentState.nativeSourceDigests` in
   `benchmarks/selfuse-batch/source-views-manifest.json` freezes the sha256 of
   `cmd/corvint/source_handoff.go` and `cmd/corvint/context_summary.go`, and
   `TestSourceViewNativeSourceDigestsAreFrozen` recomputes them. `cmd/corvint/host_adapter.go` is
   excluded because it carries unrelated adapter surfaces.
5. Both views stay experimental.
   - The matched complete-task trial is preregistered in `benchmarks/evidence-summary-trial-v0.json`
     as NOT_OBSERVED. It compares current packets against summary plus expansion.
   - Until that trial runs and the owner accepts it, V1-0023's release delivery claim is not
     satisfied, and no token, cost, latency or quality benefit is claimed.

## Alternatives set aside

- A `--view` enum was rejected. `--summary` is a boolean opt-in, which leaves the default argument
  grammar untouched.
- A new root verb was rejected, because of the PR #80 maturity-label rule.
- Changing `query` was rejected. Its wire is held byte-exact by GPK-V0-002.
- Resolving an abbreviated blob through the pinned tree was rejected, for the reason in point 3.
- Wrapping expand output in the AHI-004 envelope was rejected. Plain `context` stdout is not
  enveloped, and `selection.text` already round-trips exact UTF-8.

## Consequences and rollback

- Without the flags, `context` prints the base bytes. `TestContextDefaultWireIsTheGolden` compares
  them with a golden captured from the base binary (`1894b9e5`).
- Both views are read commands. `TestContextSummaryAndExpandAreReadOnly` checks that they leave no
  repository changes and no `.corvint/` directory.
- Rollback deletes `cmd/corvint/context_summary.go`, its test and golden, and the view flags, check
  and help paragraph in `cmd/corvint/taskcontext.go`. It also drops the file from the manifest.
- There is no persisted state or migration, and saved handles stop resolving.
