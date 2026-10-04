# Tasks dispatcher: model-escalation ladder

Owner request [issue 499](https://github.com/beamfall/corvint/issues/499) (ticket V1-0696) asked the
dispatcher to move a ticket to a stronger model after consecutive sessions without progress. The
amendment lives in S11 of `docs/specs/corvint-tasks-agent-leases-v0.md` (CAL-V0-052, 054, 055, 057
and 058); no requirement ID was added.

Decisions:

- The configuration follows the issue's schema. A match role names a base `model`, an `escalate`
  list of `{after, model, cap, notes}` tiers, and `deescalateOnProgress` (default true). The host
  renders the model through a new `{model}` placeholder. A model on a host whose argv and env never
  render `{model}`, and a host whose argv, env or `activityPaths` render `{model}` serving a
  model-less role, are refused at load: the setting cannot be silently ignored.
- The streak is per ticket and counts consecutive finished sessions whose durable fingerprint did
  not change (the existing CAL-V0-057 progress test). Progress resets it: a finished session with
  progress, or a known fingerprint change of a backed-off key with no worker between sessions,
  including a newly admitted declared progress token that unparks it. Each escalating role
  remembers the tier it last launched at, so `deescalateOnProgress: false` keeps that tier after
  progress. Only an operator unpark clears backoff without resetting the streak.
- `parkAfter` is not overridden. A tier whose `after` is at least `parkAfter` is reached only by a
  launch after an operator unpark, because a state change that unparks the key resets the streak.
- A tier `cap` counts running and newly assigned workers of that role at that tier. A candidate at a
  full tier waits; it is never launched on a weaker model.
- The ledger gains a closed `escalation` member and per-worker `tier`/`model`, all omitted when no
  ladder is configured, so a ladder-free ledger is byte-identical to the pre-ladder encoding
  (`TestCALV0057_NoLadderKeepsLegacyLedger` compares against bytes written by the 4b10a021 encoder).
  Load validation bounds
  them (tiers 1..8, at most 32 role records per ticket). Records for tickets that leave the
  observation are pruned.
- A launch that raises a role's tier emits `escalated` after the ledger save, carrying from/to tier,
  both models, the streak and the tier notes. `launched` carries tier and model. `dispatch status`
  shows each worker's tier and model and, per ticket, the streak and the next launch tier per role.

Prior Codex work:

- The leaf `/private/tmp/corvint-499-leaf-20261004` (at 21191633, untracked `tier.go` and
  `tier_test.go`, byte-identical to the preparation `SOURCE.patch`) was read, not modified, and
  superseded. It used a different host per tier instead of the issue's model strings, a session
  dedup book with baselines, reserved pending events, counted unknown busy workers against every
  tier cap, and raised the park threshold for high tiers (`tierParkAfter`). The last is new
  normative intent and was not adopted.
- Its validation choices (strictly increasing `after`, tier cap within the role cap, refusal on
  lane roles, deescalate default true, tier caps counting busy and new workers) agree with this
  implementation and served as cross-checks; no code was copied. The preparation plan in
  `/private/tmp/corvint-499-preparation-20261004` was used as orientation only.

Independent review (PASS-WITH-FINDINGS, no HIGH) found that progress between sessions did not
reset the streak: a ticket parked on its top tier and then moved by the owner or another agent
relaunched on that tier. The fix above treats that change as progress; the review's `{model}`
`activityPaths` gap and the byte-identity witness are fixed with tests.

Known limits (accepted, no code change):

- A candidate held at a full tier does not take its key, so a lower-priority role matching the same
  ticket can launch on it with a weaker model; "never on a weaker model" holds per role.
- Tier caps are per role, not per model: two laddered roles escalating to the same scarce model
  have separate caps.
- `dispatch status` lists every laddered role for every ticket, including roles whose match never
  selects that ticket.

Rollback: a pre-ladder binary decodes the ledger with unknown fields disallowed, so it refuses a
ledger that holds `escalation` or a worker `tier`/`model`, and refuses a configuration with `model`,
`escalate` or `deescalateOnProgress`. Before downgrading, remove those settings and every `{model}`
placeholder from the configuration, run the current binary once so it drops the `escalation` record,
let laddered workers finish, and confirm `state.json` carries no `escalation`, `tier` or `model`
member. Then revert the change and install the older binary.

Open owner question (parkAfter left unchanged by coordinator direction; no new normative intent):
should a top tier with `after >= parkAfter` be reachable without an operator unpark, by raising or
suspending `parkAfter` for laddered tickets?

NOT_PRODUCED: live qualification against a real model host; the typed top-tier NEEDS-OPERATOR
request (blocked on [issue 502](https://github.com/beamfall/corvint/issues/502)); native completion
of V1-0696.
