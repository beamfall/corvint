# Corvint Tasks create payload V0

Owner: Russell Lewis
Date: 2026-10-25
Intent status: accepted under the owner's standing auto-accept (2026-10-11; V1-2703; GitHub #1812)
Delivery status: experimental

Authoritative inputs: native ticket V1-2703 and GitHub #1812. These requirements amend
`ticket create --template` and the CREATE payload refusal of the
[agent lease contract](corvint-tasks-agent-leases-v0.md), whose Requirements section is at the
256-obligation OCM limit (`maxObligations`), so they live here. They change no stored state,
receipt or accepted wire contract: the CREATE payload stays a closed object.

## Agent digest
- Claim: ticket create --template --skeleton prints a complete CREATE payload; a MALFORMED CREATE names every missing, unknown and invalid key with its shape.
- Status: accepted under the owner's standing auto-accept (2026-10-11; V1-2703; GitHub #1812); experimental; TCT-V0-001 and TCT-V0-002 are implemented with focused CLI tests.
- Exists: the `--skeleton` flag on `ticket create --template` and the combined CREATE diagnosis in the CLI.
- Blocked on: nothing.
- Read next: Requirements; Failure modes; Acceptance evidence.

## User and current state

Human-owned input: GitHub #1812 (filed as V1-2703). `ticket create --template` returned a field
description and a partial payload, so a writer who copied it still met the closed-object codec's
first-failure MALFORMED one key at a time: the nullable keys (`owner`, `milestone`, `dueDate`,
`estimateMinutes`, `supersedes`, `supersededBy`) and `source` had to be guessed in turn. The owner asked for a complete skeleton and one refusal that
lists every problem, without relaxing the closed contract by defaulting omitted keys.

## Requirements

- `TCT-V0-001`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2703) `corvint-tasks ticket create --template --skeleton` MUST print,
  as the only item of an OK envelope, a CREATE payload that holds every closed CREATE key
  (`mutation.PayloadKeys[CREATE]`) and no other: nullable keys as `null`, `body` as the empty string, list keys as `[]`,
  `source` as `{"kind":"NATIVE","sourceQueueId":<this queue>}`, `effects` with coverage
  `INCOMPLETE` and empty lists, and `title` as the empty string the writer must fill. The payload
  MUST be rendered through the CREATE codec's own encoder, so the skeleton with a non-empty title
  is accepted by `ticket create --payload-stdin` unchanged. `--skeleton` MUST be refused MALFORMED
  without `--template`. The command MUST NOT write the state or intent tree.
- `TCT-V0-002`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2703) When the CREATE codec refuses a payload object MALFORMED, the
  CLI MUST return one MALFORMED whose message counts and lists every missing closed key, in key
  order, with its expected shape and skeleton default, then, in key order, every unknown key and every present key whose
  value the codec refuses on its own (checked by substituting that key alone into this queue's
  skeleton) with the codec's error and expected shape, and that names
  `ticket create --template --skeleton`. It MUST NOT accept, default or rewrite any key: the
  refusal and the absence of a write are exactly the codec's. A refusal that is not MALFORMED, or
  a payload that is not an object, MUST be returned unchanged.

## Non-goals

- Defaulting omitted keys on CREATE (the alternative named in #1812): rejected, because the
  closed-object payload contract is accepted and a default would change what a payload digest
  means.
- Diagnosing nested members beyond the codec's error for the top-level key that holds them.
- A skeleton for operations other than CREATE.

## Failure modes

- A skeleton drifts from the codec's key set: prevented by encoding the template through
  `mutation.PayloadValue`; TCT-V0-001's test compares its keys with `PayloadKeys[CREATE]`.
- One bad key hides another: each key is checked alone against the skeleton, so the combined
  message lists all of them.
- The diagnosis loads a different queue than the write: it resolves the same intent root, and on
  any load failure returns the codec's original error.
- An unparseable value (for example a JSON number) cannot be substituted: that key is skipped in
  the diagnosis and the codec's own refusal still stands.

## Acceptance evidence

- `TestTCTV0001_SkeletonIsTheCompletePayload` (`internal/tasks/cli/create_skeleton_test.go`):
  every closed key present, defaults as listed, the state and intent trees untouched, the filled
  skeleton created, and `--skeleton` without `--template` refused.
- `TestTCTV0002_MalformedCreateNamesEveryKey`: the reporter's payload (missing `source`, a bad
  priority, a non-decimal order and an unknown key) gets one MALFORMED naming each of its eight
  problems with its shape and flagging no valid key; a non-object payload keeps the codec's error.

## Rollback

Revert the change. `--skeleton` disappears and a MALFORMED CREATE returns to the codec's first
failure. No stored state, receipt or wire contract depends on either requirement.
