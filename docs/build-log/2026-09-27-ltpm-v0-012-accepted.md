## 2026-09-27 V1-0144 LTPM-V0-012: owner accepts the stranded-trace retirement path

`LTPM-V0-012` merged in beamfall/corvint#286 as a proposal, implemented experimentally and awaiting
owner acceptance. It lets `corvint migrate-traces` quarantine a trace named for a commit that an amend
or rebase made unreachable. The owner accepted this retirement path on 2026-09-27, instead of keeping
the refusal and closing V1-0144.

The spec now records the requirement as accepted and implemented experimentally. Its test and
traceability rows drop the "proposed" label. No code, test or wire contract changes.

Rollback: revert this change, which returns `LTPM-V0-012` to proposed.
