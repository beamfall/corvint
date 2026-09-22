# Decision 0084 — the snapshot-present corpus binds `GPK-V0-017(a)` index-building events and `(b)`

Date: 2026-09-11. Status: accepted. Authority: delegate call under the repository owner's standing
2026-09-04 instruction "I want you to make the calls for me" (decision 0049 records the same
authority); the owner may reverse it by restoring the previous `GPK-V0-017` text.

## What is decided

1. `GPK-V0-017` is amended by a dated edit: `beamfall-snapshot-present` in
   `conformance/perf-v0/manifest.json` is the binding corpus for (a)'s index-building non-query
   events (`file-change`, compact `session-start`) and for (b) query events. The cold `beamfall`
   corpus is still measured and reported beside every binding figure; it is no longer a threshold.
2. The binding condition decisions 0037 item 1 and 0048 item 1 set — "an automatic index refresh
   ships and is dogfooded" — is re-based to "a supported index refresh ships and is dogfooded".
   Both records are otherwise unchanged; their cold-corpus wording is superseded only on this point.
3. Nothing is waived. A formal result that measures only the cold corpus reports (a)'s
   index-building rows and (b) as `NOT_RUN`. The 2026-08-31 packet-5 preregistration
   (`conformance/perf-v0/packet-5-manifest.json`) carries no snapshot-present task, so the
   `script/release-checklist` packet-5 row stays `NOT_RUN` until a retry preregistration adds the
   six mirrored tasks and a formal run completes.

## Why

The condition as written can no longer be met: the automatic refresh decision 0049 item 3 shipped
was withdrawn on 2026-09-08 by owner instruction, and `IDX-SNAP-V0-012` now forbids the hooks to
start a persistent refresh (`docs/specs/index-snapshot-v0.md`, `docs/BUILD-LOG.md` entry
"2026-09-08 — Claude native completion and bounded prompt context"). Decision 0037's reason for
keeping the cold corpus binding was that no code path built an index for a user, so a warm figure
would describe a path nobody experienced. That no longer holds: the supported refresh is the
operator's explicit supervised `corvint index --if-stale` (`IDX-SNAP-V0-011`), the Claude Code
plugin's own prompt guidance instructs the operator to run it after every runtime or committed-tree
change (`integrations/claude-code/plugins/corvint/scripts/corvint_hook.py`), and it is dogfooded: the
fresh probe answers in 60 ms with `mutates:false` and leaves `.corvint/index/` byte-identical
(BUILD-LOG "Decisions 0049 and 0050, and what wave 18 landed against them"), and an explicit
0.928 s build followed by 0.342/0.325/0.323 s native prompts is recorded in the 2026-09-06 native
adapter entry. The harness's `"setup": [["index"]]` produces exactly the state that refresh leaves,
so a snapshot-present measurement is the path a compliant operator experiences, and the cold figure
beside it keeps the cost of skipping the refresh visible.

## What this record does not do

It does not run the packet-5 protocol, write the retry preregistration, change
`script/release-checklist`, or change any harness code; the checklist's own binding defect is filed
separately in `docs/agent-memory/fixes.md`.

Rollback: restore the previous `GPK-V0-017` text, the `conformance/perf-v0/README.md` corpus
paragraph and the `docs/RELEASE-NOTES.md` limitation line, and regenerate
`docs/specs/REQUIREMENTS.tsv`.
