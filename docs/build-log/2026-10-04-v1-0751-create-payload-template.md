## 2026-10-04 V1-0751: `ticket create --template` prints a fill-in CREATE payload

Human-owned intent: ticket V1-0751 (FEATURE P3). `ticket create --help` named the 20 CREATE keys and
the canonical rules, but value types, the nested `effects` and `source` shapes, null-able keys, the
`order` string and the queue's `sourceQueueId` were discoverable only by refusal. An agent should
be able to print a valid payload, edit three fields and submit.

Requirement: `CAL-V0-073` in `docs/specs/corvint-tasks-agent-leases-v0.md` (V1-0751 amendment).
The first draft used `CAL-V0-069`, which main reserves for issue 494's coordinated unlanded work
(PR 557); the merge with main renumbered it to the first ID free on main and every open PR branch.

### Change

- `corvint-tasks ticket create --template` returns one `taskman-command-result/0` item with
  `payload` (canonical CREATE object for this queue), `payloadCanonical` (its exact bytes),
  `fill` (`acceptanceCriteria`, `body`, `title`), `fields` (type, `nullable`, and `values` for a
  closed scalar enum, `elementValues` for array members or `current` for the open queue id, per
  dotted path, including nested `effects`, `source`, `effects.resources[]`,
  `dependencies[]` and the optional `localToken`, `requiredRoles`, `requiresPool`),
  `nullableKeys`, `optionalKeys`, `usage` and `note`.
- The payload is a `mutation.CreatePayload` rendered by `mutation.PayloadValue`, the CREATE codec's
  encoder. Enum values come from the `ticket` vocabularies the
  decoder enforces; `requiredGates` and `dependencies[].gateId` values come from the current policy
  gates, `requiresPool` from its pools, and `source.sourceQueueId` from `queue.json`. The stage-role
  enum moved from a literal inside `ticket.ReadStageRoles` to `ticket.StageRoles`/`ticket.Stages`
  with unchanged behavior.
- Defaults: `FEATURE`, `P2`, order `"0"`, `AUTONOMOUS`, `effects.coverage` `INCOMPLETE` (empty
  touch paths do not qualify effects), `source.kind` `NATIVE`. The title is empty, so an unedited
  template refuses; an empty `acceptanceCriteria` would leave the ticket `DRAFT`.
- The command reads only the intent store (`intent.Load`), never stdin, the journal or the lock, and
  works before `init`. `--template` is refused on other verbs and when any other flag is passed,
  detected by presence (so `--role OWNER` and `--payload ""` refuse too).
  `ticket create --help` lists `--template` and the top-level help names it.
- `readPayload` and the wire parser are untouched (V1-0750 owns payload canonicalization).

### Non-goals

Templates for other verbs, interactive prompting, defaults from history, any change to CREATE
validation, canonicalization or wire profiles.

### Failure modes

- Drift guard (review finding): `fields` keys must equal the emitted payload's key paths (with one
  dependency and one resource element populated) plus `optionalKeys`; every payload path marked
  nullable is submitted as null and must be accepted, every other path must be refused. Writing it
  found that `dependencies[].gateId` is nullable only with obligation `COMPLETED`; its note now
  says so. Optional-key nullability and free-text `type` strings are not probed.
- A changed enum vocabulary is read from the `ticket` variables at run time; the test checks the
  listed vocabularies.
- Agents that resubmit the template unedited get a refusal (empty title), not a junk ticket.

### Evidence

- `GOTOOLCHAIN=local go test -count=1 -run CALV0073 -v ./internal/tasks/cli/`: four tests PASS. The
  first feeds the printed template, with title, body and acceptanceCriteria filled, back through
  `ticket create --payload-stdin` on a fixture store and sees an OPEN ticket; it also proves the
  store and intent trees byte-identical, no `taskman.lock`, and no stdin read (panicking reader).
- `GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/tasks/... ./cmd/corvint-tasks/...`
  and `go vet` on the touched packages: see the change's report.
- Independent review, `make gate` and dogfood seal: NOT_RUN (scoped issue work).

### Rollback

Revert the change. No store, journal, wire or policy state depends on `--template`.
