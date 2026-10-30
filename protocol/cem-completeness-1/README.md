# CEM completeness profile, version 1

SPDX-License-Identifier: Apache-2.0

`cem-completeness/1` is a verifier profile for `cem/1.0` stable maps. It is versioned apart from
the stable reference-integrity verifier and does not change it. Reference integrity proves that
every cited receipt, plan and artifact exists and matches its digest. It cannot prove that the map
cites every run that happened: a self-consistent map can omit a failed retry or a narrowed
selection and still be accepted. This profile compares the map with an independently supplied run
inventory and refuses, with explicit missing entries, when the map leaves something out.

The governing contract is `docs/specs/cem-completeness-profile-v0.md` (`CCP-V0-*`). This file is
the portable description an independent reader needs.

## Inputs

| Input | Meaning |
| --- | --- |
| map | The `cem/1.0` stable map bytes. |
| artifact root | Directory that holds the map's declared artifacts; each is read and digest-checked. |
| inventory | A `cem-run-inventory/1` document, or absent. |
| map producer | Identity the caller knows produced the map. May be empty. |
| target | Target revision the caller verifies the map against. May be empty. |

The map producer, the inventory producer and the target are caller-asserted. The profile does not
authenticate them, and every result reports `authentication: NOT_OBSERVED`.

## Run inventory, `cem-run-inventory/1`

A single strict JSON object (valid UTF-8, no duplicate member, no unknown member, no trailing
data, at most 4 MiB, at most 4096 required tests plus runs):

```json
{
  "profile": "cem-run-inventory/1",
  "producer": "ci:independent-run-inventory",
  "baseRevision": "<40 or 64 lowercase hex>",
  "targetRevision": "<40 or 64 lowercase hex>",
  "requiredTests": [{"runner": "go-test", "project": "example.com/greet", "id": "example.com/greet::TestGreetName"}],
  "runs": [{
    "runner": "go-test", "project": "example.com/greet", "attempt": 1,
    "receiptSha256": "<64 lowercase hex>",
    "inputFiles": {"greet.go": "<64 lowercase hex>"},
    "outcomes": [{"id": "example.com/greet::TestGreetName", "state": "FAILED"}]
  }]
}
```

`requiredTests` and `runs` are present arrays (they may be empty). Required tests are unique and
non-empty. Every run has `attempt >= 1`, a receipt digest unique in the inventory, a
`(runner, project, attempt)` key unique in the inventory, a present `inputFiles` object, and at
least one outcome. Outcome ids are unique within a run, and `state` is one of `PASSED`, `FAILED`,
`SKIPPED`, `FLAKY`, `TIMED_OUT`, `INTERRUPTED`, `UNKNOWN` or `NOT_REPORTED`. Every state other than
`PASSED` is non-passing. Retries are separate runs with increasing `attempt`. Any violation is
`inventory-invalid`.

The inventory never states its own independence. The verifier derives it: `NOT_OBSERVED` when the
caller supplied no map producer, `SELF_AUTHORED` when `producer` equals the map producer, and
`INDEPENDENT` otherwise.

## Runner adapter

Each `runnerReceipts` row of the map is read as follows.

- A row whose `profile` is not `corvint-test-runner-receipt/0` is opaque and is never read as a
  runner receipt.
- Otherwise the row's plan must decode with `profile: corvint-test-runner-plan/0` and the receipt
  with `profile: corvint-test-runner-receipt/0`, and `request.runner` must equal
  `observation.runner`. If not, the row is not readable.
- A readable row yields: runner and project (`request`), the selection (`request.expectedTests`),
  input file digests (`request.inputFiles`), terminal outcomes (`observation.tests[].id/state`),
  and completeness (`observation.complete`).

## Judgement

Entries go into `missing` (each one refuses) or `notObserved` (each one prevents a PASS).

| Reason | List | When |
| --- | --- | --- |
| `inventory-not-supplied` | notObserved | No inventory. Nothing else is checked. |
| `inventory-invalid` | missing | The inventory fails the rules above. Nothing else is checked. |
| `map-invalid` | missing | The map is not a well-formed `cem/1.0` stable map. |
| `artifact-unavailable` | missing | A declared artifact cannot be read or does not match its digest. |
| `source-identity-mismatch` | missing | `source: baseRevision` or `targetRevision` differs from the map base or the supplied target; `source: inputFiles:<path>` differs between an inventory run and its cited plan. |
| `target-not-supplied` | notObserved | No target was supplied. |
| `receipt-opaque` | notObserved | A cited row is opaque. |
| `receipt-not-readable` | notObserved | A cited labelled row is not readable. |
| `receipt-incomplete` | notObserved | A readable cited receipt has `complete: false`. |
| `failed-run-omitted` | missing | An inventory run with a non-passing outcome is not cited; one entry per non-passing outcome, with `test`, `attempt` and `terminalOutcome`. |
| `receipt-missing` | missing | An inventory run whose outcomes all passed is not cited (run entry with `attempt`), or a required test is selected by a readable cited plan but recorded by no readable cited receipt (test entry). |
| `run-identity-mismatch` | missing | A cited readable run's runner or project differs from its inventory run. |
| `terminal-outcome-mismatch` | missing | An outcome differs, is missing or is extra; `terminalOutcome` is the inventory state (empty if the inventory has none). |
| `run-not-in-inventory` | missing | The map cites a run the inventory does not list. |
| `selection-narrowed` | missing | No readable cited plan for the required test's runner and project selects it. |
| `selection-not-observed` | notObserved | A required test is not both selected and recorded, and some cited row is opaque or not readable, so narrowing cannot be decided. |
| `inventory-self-authored` | notObserved | The inventory producer is the map producer. |
| `inventory-independence-not-observed` | notObserved | No map producer was supplied, or the inventory is invalid. |

An inventory run is matched to a cited row by receipt digest. Required tests are matched to
readable cited runs by `(runner, project)`.

## Result, `cem-completeness-result/1`

```json
{"profile": "cem-completeness-result/1", "completenessProfile": "cem-completeness/1",
 "outcome": "PASS|REFUSE|NOT_OBSERVED", "mapSha256": "<sha256 of the map bytes>",
 "referenceIntegrity": "NOT_EVALUATED",
 "inventory": null | {"sha256", "producer", "mapProducer", "independence", "authentication"},
 "missing": [], "notObserved": []}
```

The outcome is `REFUSE` if `missing` is non-empty, else `NOT_OBSERVED` if `notObserved` is
non-empty, else `PASS`. So a missing inventory is never PASS, and a self-authored inventory can
refute a map but never confirm it. Entries carry only `reason`, `runner`, `project`, `test`,
`receiptSha256`, `attempt`, `terminalOutcome` and `source`, with empty members omitted, and are
sorted by those members in the order reason, runner, project, test, receiptSha256, source,
terminalOutcome, then attempt (byte order for strings, numeric for attempt).

`referenceIntegrity` is always `NOT_EVALUATED`. A completeness PASS means only that the map is
complete against the inventory. Use it together with a stable `ACCEPT` over the same `mapSha256`.

## Vectors

`VECTORS.json` lists each vector's map, inventory (or `null`), map producer, target and the exact
expected outcome, independence and entries. Paths are relative to this directory, which is also
the artifact root. `withheld/` holds a synthetic receipt that is named by the `receipt-missing`
inventory and cited by no map. The native verifier (`internal/cem/verify`) and the portable reader
(`interop/cem01-go`) both run every vector.
