# V1-1590 batch: preflight cache, repeat-failure refusal and the run baseline gate

One batch for GitHub #1037 (V1-1594), #1048 (V1-1605) and #1033 (V1-1590), plus V1-1609 and
V1-1611 on the `corvint runs reusable` reader of V1-1578 (#1051). Every requirement below is
agent-drafted and accepted under the owner's standing auto-accept of lane requirements
(2026-10-11). IDs were checked against origin/main and the #1051 branch before numbering.

## V1-1594: preflight result cache (TOL-V0-047..050)

- Opt-in `preflight --cache` instead of a default: an unchanged default keeps every existing
  output byte-identical and keeps the read honest for callers that never asked for replay.
- The key covers what the static checks read (record, ledger, policy, commit) and what the issue
  names even though preflight reads Git objects only: `HEAD`, the tracked diff, untracked files
  and the executable digest. An over-wide key costs a miss; a narrow one would replay a stale
  "ready".
- The cache lives in the user cache directory, never in the repository or `.corvint/`, so
  `preflight` remains a read under product invariant 4 and nothing can mistake an entry for
  evidence. Any keying failure runs the checks (`SKIPPED`), never replays.
- `--cache --deep` is refused: gate commands are not static and depend on the environment.
