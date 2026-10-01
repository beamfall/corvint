# Post-merge metrics V0

Owner: Russell Lewis
Date: 2026-10-01
Intent status: proposed
Delivery status: experimental

Status: experimental optional Darwin/Linux companion; human-owned feature intent is GitHub issue #396
(native V1-0543). This technical contract proposes implementation details, not accepted automation
policy. Decisions 0081 and 0373 retain owner authority. No merge permission is emitted.

## Agent digest
- Claim: A local companion reports per-class post-merge evidence from stored records and immutable Git, with recommendations that grant no merge authority.
- Status: proposed/experimental optional Darwin/Linux companion.
- Exists: internal/postmergemetrics; cmd/corvint-postmerge-metrics; focused reducer/Git/CLI fixtures.
- Blocked on: independent runtime/platform qualification and owner automation-policy promotion; current integration evidence is separate.
- Read next: Requirements; Wire and reducer contract; Git observation and operational limits.

## Requirements

- `PMM-V0-001`: Validate closed JSON policy and history envelopes, duplicate keys, trailing values,
  depth and all stated bounds. Reject malformed facts, unknown classes, duplicate identities,
  dangling event targets, timestamps outside UTC, and events earlier than their target run.
- `PMM-V0-002`: Retain each stage outcome and duration, follow-up creation latency or explicit no-op
  or unknown, and immutable generated-request commit pairs. Measure net textual additions/deletions
  and changed files from the bot's last commit to the approved commit, including measured zero.
  Never label the proxy authenticated human authorship.
- `PMM-V0-003`: Report per-class window cohort totals, distinct corrected generated-run numerator
  and generated-run denominator, separate event kind counts and net edit counts. Compare integer
  basis-point thresholds without rounded arithmetic, retaining unavailable evidence as unknown.
- `PMM-V0-004`: Process full retained eligible run/event chronology before the exclusive as-of cutoff.
  A revert resets cooldown to N at event time; next N class runs are review-required. A late revert
  must not consume cooldown on runs preceding it. Equal-time events precede runs; stable IDs break
  ties. Repeated reverts reset cooldown; old targets and events outside the cohort still affect it.
- `PMM-V0-005`: Validate contiguous per-class sequence inventories starting at 1 and independent run
  and event watermarks through the report cutoff before considering history complete. Missing,
  partial, unknown or insufficient history prevents eligibility. Assertions remain caller provenance.
- `PMM-V0-006`: Produce byte-identical reports from the same validated policy, records, immutable Git
  objects, Git executable version and window; include canonical policy/history digests, provenance limits and measurements.
  Recommend review, demoted or eligible-for-owner-consideration only; never change merge policy.
- `PMM-V0-007`: Expose `corvint-postmerge-metrics report --policy FILE --records FILE --from UTC
  --until UTC` as an optional standalone binary. Fail closed with fixed typed errors; bounded input,
  total deadline, Git output/time and owned-process cleanup. Do not register a Core CLI/MCP command.

## Wire and reducer contract

All fields are required except pointers explicitly described as nullable. Profiles are
`postmerge-policy/0`, `postmerge-history/0`, `postmerge-report/0`. Unknown or duplicate object fields,
trailing JSON, depth >32, policy >1MiB and combined history >16MiB refuse. At most 64 classes,
10000 runs, 50000 events. IDs/class/stage names are ASCII letters/digits/underscore/hyphen (1..128
bytes); at most 32 uniquely named stages per run. Durations are nullable integer milliseconds,
0..31536000000; unknown status requires null and measured statuses require a value. Policy classes
have `id`, `maxCorrectionBP` (0..10000), `minSamples` (1..10000), `demoteRuns` (0..10000).

Policy has `profile,classes`. History has `profile,classes,runs,events`. Each class history has
`class,lastSequence,runsComplete,eventsComplete,runsThrough,eventsThrough`; booleans are explicit
caller assertions. Both watermarks are real UTC instants. Runs have `id,class,sequence,at,outcome,
stages,followup,git`; outcome is generated/no-change/failed/unknown. Stages have `name,outcome,
durationMs`; outcome is passed/failed/skipped/unknown. Followup has `status,durationMs`; status is
created/no-op/unknown (no-op duration exactly 0). Git is nullable, otherwise `{root,bot,approved}`;
full lowercase 40-hex commits or 64-hex SHA256 commits, same format, absolute local root. Generated
runs require Git evidence for eligibility; non-generated runs may not carry it. Events have
`id,runID,at,kind`; kinds revert/correction/invalid-finding/containment. Revert/correction require a
generated target. Event and run IDs are unique within their respective namespaces.

Times parse as RFC3339Nano with literal Z; reports normalize them to UTC RFC3339Nano. `from < until`;
window cohort is runs in `[from,until)` and event knowledge is strictly before `until`. Supplied
future rows are validated but excluded. Sequences uniquely enumerate each class from 1 through
lastSequence and must strictly increase in chronological `(at,id)` order. Missing sequence/prefix
or metadata is incomplete evidence, not an invented clean history. Duplicates/order contradictions
are malformed. Metadata watermark before cutoff or before any supplied row (including future rows) is incomplete.
Both inventories must assert completeness; no collector authentication is claimed.

Denominator is generated cohort runs; numerator is distinct such runs having >=1 correction or
revert before until. Event counts refer to event times in the window, including old targets. Total,
no-change, failed, unknown and demoted cohort counts remain separate. Threshold equality passes:
`corrected*10000 <= maxCorrectionBP*generated`; zero denominator cannot pass minSamples. Failed or
unknown cohort outcomes, missing stages or any failed/unknown stage, unknown followup, missing/binary/gitlink Git line evidence,
invalid-finding/containment window events, incomplete history, low samples or excess correction rate
force review. Any active cooldown at until recommends demoted first. A revert sets cooldown N;
each subsequent class run (any outcome) consumes one and is marked demoted. A reverted cohort run
keeps the class review-required (`reverted-cohort`) after cooldown reaches zero; demoted count is the union, avoiding double counting. Eligibility at as-of
is distinct from historical per-run demotion. Counts are evidence; recommendations grant no authority.

## Git observation and operational limits

Use direct argv Git under sanitized environment, with no replacement objects, lazy fetch, prompts,
external diff or textconv. Pin diff algorithm to myers and disable rename recognition: a rename is
explicitly delete+add (two changed paths); do not claim logical rename units. `--numstat -z` permits
tabs/newlines in valid UTF8 paths via JSON escaping; reject invalid UTF8, absolute, dot-segment or
>512-byte paths. Binary rows retain binaryFiles and null aggregate textual additions/deletions;
textual partial counts may be retained separately. Gitlink deltas retain gitlinkFiles and unknown
line totals; their synthetic numstat pointer lines are excluded from textual counts. Full commits and trees bind the measurement;
bot must be ancestor of approved. Dirty worktree files do not enter immutable blob measurement.
Pin committed attributes to approved revision; ignore global/system attributes/config and refuse
local info/attributes because it can change binary classification. Named attribute diff drivers are refused because mutable driver binary settings can affect counts.
The Git executable version is recorded; cross-version byte equivalence is not claimed. No filesystem mutations, optional Git locks, object fetching or hooks are requested.

Each Git command has 10s execution and 4MiB stdout/64KiB stderr limits, owned process-group cleanup;
bounded descendant observation does not claim full containment or visibility of a fast detach.
The whole CLI has a 120s context deadline. Cache at most 10000 distinct root/commit pairs for one
report. No shell. Commit resolution, ancestor and tree validation failures return fixed errors.
Report counts cannot authenticate who edited the commits or prove an external event inventory.

## Acceptance evidence and non-goals

Meaningful reducer fixtures cover late/repeated/out-of-window reverts, cutoff/tie boundaries,
per-class independence, missing prefix/middle/watermarks, exact thresholds, insufficient samples,
unknown evidence and canonical reproducibility. Disposable real Git repositories cover add/delete/
modify, pure and edited rename, binary, zero delta, odd paths, nonancestor, configuration isolation,
output bounds and cancellation. CLI fixtures exercise closed input and deterministic stdout.
External integrations, automatic record collection, authentication, actual merge actions, promotion,
and a hosted dashboard are out of scope. No repository-wide gate is claimed.

## Failure modes and rollback

Malformed input and unavailable immutable Git evidence fail closed with no report. Valid incomplete
history yields review with reasons. Provenance assertions never become authority. Rollback removes
the optional binary/package/spec; no persistent data migration or runtime policy mutation occurs.
