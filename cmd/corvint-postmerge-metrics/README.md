# Post-merge metrics companion (experimental)

Build this optional Darwin/Linux companion from the repository root:

```sh
go build -o /tmp/corvint-postmerge-metrics ./cmd/corvint-postmerge-metrics
/tmp/corvint-postmerge-metrics report --policy policy.json --records records.json \
  --from 2026-09-01T00:00:00Z --until 2026-10-01T00:00:00Z
```

It reads local files and immutable Git objects, writes a JSON report to stdout, and grants no merge
authority. It does not collect external service events or change repository policy. Failure writes
a fixed JSON error to stderr and exits nonzero. Git 2.54 was exercised on Darwin; other versions
must support the pinned-attribute options and remain unqualified until tested.

A policy defines content classes and their correction threshold in integer basis points (100 = 1%),
minimum generated-run sample count, and runs to demote after a revert:

```json
{"profile":"postmerge-policy/0","classes":[{"id":"flow-prose","maxCorrectionBP":100,"minSamples":20,"demoteRuns":3}]}
```

Records use the closed `postmerge-history/0` envelope below. Replace `root`, `bot`, and `approved`
with the absolute repository root and full immutable commit OIDs; the bot commit must be an
ancestor of the approved commit. These sample OID placeholders deliberately fail validation.

```json
{
  "profile": "postmerge-history/0",
  "classes": [{
    "class": "flow-prose", "lastSequence": 1,
    "runsComplete": false, "eventsComplete": false,
    "runsThrough": "2026-10-01T00:00:00Z",
    "eventsThrough": "2026-10-01T00:00:00Z"
  }],
  "runs": [{
    "id": "run-1", "class": "flow-prose", "sequence": 1,
    "at": "2026-09-02T12:00:00Z", "outcome": "generated",
    "stages": [{"name": "author", "outcome": "passed", "durationMs": 1200}],
    "followup": {"status": "created", "durationMs": 800},
    "git": {"root": "/absolute/repository", "bot": "BOT_FULL_COMMIT_OID", "approved": "APPROVED_FULL_COMMIT_OID"}
  }],
  "events": [{"id": "correction-1", "runID": "run-1", "at": "2026-09-03T12:00:00Z", "kind": "correction"}]
}
```

Set completeness true only when the retained run and event inventories are complete through their
respective watermarks. Preserve the entire class sequence from 1, including runs before the report
window: old events can affect current cooldown. Missing sequences, incomplete inventories, missing
measurements, unknown stages/follow-up or insufficient samples keep the recommendation in review.
The tool validates these assertions' consistency; it does not authenticate the collector.

The cohort is runs in `[from,until)` and only events before `until` are known. Correction rate counts
distinct corrected/reverted generated cohort runs divided by generated cohort runs. No-change,
failed and unknown runs remain separate; zero generated samples cannot establish eligibility.
A revert resets cooldown at event time. An in-window reverted target still requires review after
cooldown expires. `eligible-for-owner-consideration` is evidence for an owner decision, never approval.

Net added/deleted lines and changed paths measure the bot-to-approved delta, not authenticated human
authorship. Renames count as delete plus add. Binary/gitlink totals are unknown; partial text counts
and unknown-run counts remain visible. Local `info/attributes` and named attribute diff drivers
refuse because mutable settings can change counts. Ordinary dirty worktree files do not enter the
measurement. Reports include pinned commits/trees, Git version, canonical input digests and provenance
limits. Reuse the same stored inputs, immutable objects, Git version and window for reproducibility.

See [the proposed contract](../../docs/specs/postmerge-metrics-v0.md) for every field, bound and rule.
Run the focused fixtures with:

```sh
go test -count=1 -timeout 30m ./internal/postmergemetrics ./cmd/corvint-postmerge-metrics
```
