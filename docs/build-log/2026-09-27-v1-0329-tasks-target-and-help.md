## 2026-09-27 V1-0329: corvint-tasks mutations take the local ticket ID, and each verb's help names its payload

The pre-1.0 panel addendum found, and use on 2026-09-25 confirmed, two gaps in the corvint-tasks
mutation verbs:

- `ticket <mutation> --target V1-0263` was refused with `/targetId: ticket ID must be
  ticket:<authority>:<queue>:<local>`, although `ticket show V1-0263` resolves the same local ID.
- No command named a mutation's payload keys. An agent learned them only from successive
  refusals.

Decision: one function, `qualifyTicket`, expands a local token against the store's queue. Reads and
mutations both call it, so they accept the same forms. The envelope still records the full ticket
ID, so the §3.3 wire contract, the request digest and replay are unchanged. `ticket <mutation>
--help` returns the verb's operation, its sorted payload keys and its flags. The keys come from
`mutation.PayloadKeys`, the table `decodePayload` now enforces, so help cannot drift from the
refusal. The top-level help names the per-verb form.

No spec in `docs/specs` owns the mutation CLI: the task-store contract is still being recovered
(V1-0310), and CTS-V0 covers only store initialization. This entry records the change until that
contract exists.

Evidence: `TestMutationTargetTakesTheLocalIDAndHelpNamesThePayload` prioritizes a ticket by its
local ID and reads `payloadKeys` from `ticket prioritize --help`. The existing mutation, init and
retry tests in `internal/tasks/cli` and `internal/tasks/mutation` still pass.

Rollback: revert the change. `--target` again needs the full ticket ID, and help lists no payload
keys.
