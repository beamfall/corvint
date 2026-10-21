# 2026-10-20 V1-1974: compaction pin never emitted was reported as dropped (AHI-057)

Decision: a degraded `pre-compact` now prints `corvint-compaction-pin/0 unavailable` (with a
preserve instruction) before its unchanged frame text, and `post-compact` reports
`compaction-pin-not-emitted` when that line is the summary's last pin-shaped line. Every other
summary keeps `AHI-028`: the last pin is verified, and a summary with neither line still reports
`compaction-pin-not-preserved`.

Evidence (root cause, measured 2026-10-20 on the primary checkout's ledger and this project's
Claude Code transcripts; read-only):

- `.corvint/self-observations.jsonl`: 67 hours with `post-compact` `compaction-pin-not-preserved`.
  66 share the hour with a `pre-compact` degradation (59 `dogfood-event-deadline` alone, 3
  `dogfood-event-index-snapshot-stale`, 4 combinations); the remaining hour (2026-10-18T01Z) follows
  a `pre-compact` deadline at 2026-10-18T00Z.
- 829 recorded compaction summaries: 511 contain a pin and every one parses and re-validates under
  `compactionPinPattern` and the path rule; since the ledger began (2026-10-14T15Z) 39 of 173
  summaries carry no pin and all 39 fall in an hour with a `pre-compact` degradation. No compact
  `SessionStart` output carries a pin line (0 of 1648), contrary to the ticket's hypothesis.
- So the verifier reads the right place and detection is sound; the pin was never emitted, because
  `pre-compact` misses its 2 s declared kill on this repository, and the old code could not tell
  that apart from a summary dropping the pin.

Rejected: emitting a fallback pin from `git rev-parse HEAD` when the block read expires (it would
invent dirty-path counts the index did not produce); reading the transcript in `post-compact`
(excluded by the AHI-026..030 non-goals). Not addressed here: making `pre-compact` finish within
its kill, which remains the cause of the missing pins.

Independent review (Codex, one round) found that the unavailable line was ordered only against
pins that matched the full pattern, so a newer pin truncated before `elided=` after an older
unavailable line reported not-emitted. Fixed: the last pin-shaped line (the profile followed by
`revision=` or `unavailable`) is found first and then validated, so a truncated last pin reports
not-preserved and an earlier whole pin no longer stands in for it.

Rollback: revert the commits; a degraded `pre-compact` then prints its frame text alone.
