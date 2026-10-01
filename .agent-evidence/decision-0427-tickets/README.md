# Decision 0427 ticket payloads

Five `corvint-tasks ticket create` payloads for the owner-commissioned work named in
`docs/decisions/0427-close-the-three-unproven-readme-rows-2026-10-01.md`. They were not created
natively from the authoring session: that clone carried the tracked `.taskman/` export but not the
`.git/taskman` journal, and `docs/TASKS-EXTERNAL-AGENTS.md` forbids `init` over populated records.
Create each from the primary checkout, in this order, and record the assigned IDs in the decision:

```sh
for f in .agent-evidence/decision-0427-tickets/[1-5]-*.json; do
  corvint-tasks ticket create --request-id "decision-0427-$(basename "$f" .json)" --payload-stdin < "$f"
done
```

The payloads are canonical JSON as `ticket create --help` requires. Nothing here is a ticket until
the native writer commits it.
