# Named environment pools for external-agent leases

Issue 342 adds optional named pools to native Tasks policy and claims. Allocation is one journal
transaction with the existing attempt and source reservation. Physical reuse has a separate boundary:
release, completion and reap quarantine the allocation; exact-allocation operator confirmation is
required, after configured cleanup succeeds. That confirmation is an attestation, not a process proof.
The accepted contract is CAL-V0-028..034 in `corvint-tasks-agent-leases-v0.md`.

Independent review caught incomplete allocation tuple checks, occupied-member removal, missing
no-health configuration validation, a cancellation watcher race, preview over-allocation and old-claim
replay borrowing a successor allocation. Repairs compare complete bindings, retain receipt-bound
replay output, consume preview capacity and join cancellation before final group observation.

Focused regressions and actual native disposable queues exercised concurrent distinct allocation,
reserved review capacity, required-pool refusal, pure occupancy, quarantine/confirmation/reuse,
health failure skip/pass, cleanup prerequisite and receipt audit. Process qualification waits for a
parent, child and grandchild: normal parent exit leaves a residual group for runner cleanup; timeout
and SIGTERM leave no live descendants. SIGKILL preserves pending ownership; replay never reruns the
probe and explicit orphan recovery quarantines. The harness cleans only still-matching owned process
identities. Initial sandbox process observation refused `/bin/ps`; qualified runs used explicit host
process access. This is a trusted process-group boundary, not containment of hostile detached processes.

Source remains uncommitted at builder handoff. Final frozen enrollment, independent completion review,
native ticket completion and CEM closeout belong to the owner/reviewer; no full repository gate or
publication is claimed here. Older readers cannot consume the new optional records/projection; rollback
requires a recorded safe migration after occupancy is resolved.
