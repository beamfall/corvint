# Pi capability contract

Native ticket: V1-0506. Base: `cdd2e31fffd732419eb9ade531ccf289259f3243`.

Most of V1-0506 already landed through PR #384: the numbered Pi workflow spec, the closed Core and
Tasks read facade, bounded argv-only process ownership, identity and generation discard, and the
LCP-V0-008/LCP-V0-009 amendment admitting the exact Pi 0.99.1 / adapter 0.3.2 tuple. Two pieces
that the dependent tickets (V1-0507 Tasks workflow, V1-0509 recovery, V1-0511 qualification) need
were missing: a closed, checkable capability inventory, and an actual-native witness that a named
task reaches selected tests without any repository or task-store write.

Decision: the inventory lives in `pi-workflow-v0.md` as the `corvint-pi-capabilities/0` JSON block
(PWV-V0-011), not in the shipped package. Shipping it would require an adapter version bump under
`script/check-host-package-versions.sh`, and AHI-024 plus the LCP tuple admit only 0.3.2, so a bump
would invalidate the exact qualified tuple for no runtime change. A later version may ship the
inventory as a declaration. Other host adapters declare only `unavailableCapabilities`; Pi keeps
that list and the inventory's unavailable set must equal it. Tasks write admission stays per call
against the installed native `help` result (`capability-unavailable` before any mutation argv);
operations outside the inventory (`complete-manual`, `ticket create/refine`, `init`, `index`)
refuse with `unsupported-mutation` and no native call. Deferred routes name their owning ticket.

`integrations/pi/capabilities.test.mjs` registers the same modules as `index.ts` against a recording
host and compares tools, commands, lifecycle events, Core/Tasks operation sets and the unavailable
set with the inventory, then checks per-call write negotiation. `integrations/pi/core-host.test.mjs`
(PWV-V0-012) runs actual native binaries on a temporary repository: `context` returns the governing
`AGENTS.md` and the named source pinned to their `HEAD` blobs, `impact` names the same-package test,
and `affected --base` selects it as `PLAN_ONLY` while retaining the `NO_REPOSITORY_GATE_DECLARED`
unknown. Tasks `help` and `queue` run in a repository without a store. A whole-tree content
snapshot, including the Git directory, is byte-identical afterwards.

`core-host.test.mjs` is a manual qualification test. It is excluded from `make host-adapter-test`,
CI and every gate because it needs installed native binaries; run it by hand with
`node --test integrations/pi/core-host.test.mjs`, using `corvint` and `corvint-tasks` from `PATH` or
`CORVINT_BIN` and `CORVINT_TASKS_BIN` set to their paths. `capabilities.test.mjs` runs in
`host-adapter-test` and also reads the shipped `writes` set from `tasks.js`, so a write added there
without amending the inventory fails.

Evidence: both native runs passed against installed Core 1.0.0-rc.1 build 163 with Tasks build 202,
and against Core built from the base source. Live Pi host qualification was not run: no
host-observable behavior changed and the package is unchanged (NOT_RUN, not implied). The
repository-wide `make gate` was NOT_RUN under the owner's scoped-issue preference.

Rollback: revert this change. The package, tuple and native behavior are untouched.
