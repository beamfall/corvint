# Decision 0254 — lease renew never shortens a lease

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`scopelease.Renew` set the expiry to now plus `D`, so `lease renew --ttl 1s` on a lease with an
hour left moved its expiry earlier. `SCL-V0-005` says renew MUST "extend" the holder's lease, and
the truth boundary makes a live lease the coordination hint other sessions refuse against, so a
renewal that silently shortens a claim contradicts the requirement's word.
`TestRenewNeverShortensALease` reproduced it through `Renew` on a temporary root.

The call: renew extends. The renewed expiry is the later of now plus `D` and the recorded expiry;
`D` keeps its 24-hour bound. A renewal whose `D` ends earlier than the recorded expiry succeeds and
reports the recorded expiry unchanged. `SCL-V0-005` is amended to say so.

Rejected: documenting that renew sets the expiry. That would let a holder shorten its lease, a job
`release` already does explicitly, and it would contradict the requirement text rather than
clarify it. No command, wire member, exit code, or lease document field changes.

Rollback: revert the commit. That restores now-plus-`D` renewal, the unamended `SCL-V0-005`, and
the `docs/agent-memory/ideas.md` item.
