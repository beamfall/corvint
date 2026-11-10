# Design: `tasks-policy-v15`

## Decisions

- Content. Version 15 is version 14 with `policyVersion` "15", `cemRequired` and `ocmRequired`
  removed or set false, and the gate entries `cem-verdict` and `context-focused` removed. A JSON
  diff of version 14 and version 15 shows only those members.
- Order. (1) M0 removal lands on `origin/main`. (2) Check that no release object (`corvint-tasks
  release show` for each release) and no OPEN or CLAIMED ticket references a removed gate. (3) Save
  the version 14 bytes and SHA-256 under the build-log entry. (4) Run `corvint-tasks policy update`
  against the store. (5) Open one carry PR that changes only `.taskman/policy.json` to the bytes the
  store now holds.
- In-flight attempts. Attempts admitted under version 14 are not migrated. If the store refuses
  their submit with `STALE_POLICY`, the attempt is resubmitted under version 15.
- Not decided here (Open): whether `context-vet` is kept as is or retargeted after the cut; whether
  the CEM entries in `reservations.sharedPaths` are removed in version 15 or later.

## Failure modes

- A release or ticket still names `cem-verdict` or `context-focused`: the update stops at step 2 and
  the reference is fixed first.
- The store update succeeds but the carry PR is not merged: the tracked file and the store differ;
  the carry PR is the only open action and is tracked by the ticket.
- The carry PR contains other changes: refused in review; the policy carry stays alone.
- Policy updated before M0 lands: attempts would no longer need a CEM while CEM commands still run.
  Step 1 prevents this.

## Acceptance evidence

- The JSON diff of version 14 and version 15 showing only the listed members.
- The empty reference check output from step 2.
- `corvint-tasks policy show` (or the store's policy read) reporting version 15.
- The merged carry PR whose `.taskman/policy.json` bytes match the store.
- One attempt submitted and completed under version 15 without a CEM.

## Rollback

- Restore from the retained version 14 bytes, checking their SHA-256, through `corvint-tasks policy
  update`, and revert the carry PR.
- If the store refuses a lower `policyVersion` (not verified), publish version 16 with the version 14
  content instead.
