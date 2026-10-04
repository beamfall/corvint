# Tasks user service foundation: issue 500

Human-owned intent: GitHub issue 500, native V1-0697. Intent acceptance is an owner-delegated
decision 2026-10-04 (agent decided), item 1. The owner delegated the pending decisions, and the
agent accepted this intent within stated limits. It is not a direct owner statement. The limits:

- the service is an optional, operator-installed deployment profile;
- the default local product keeps no permanent daemon (agent contract invariant 7);
- the first delivery is the four-file pure foundation plus spec intent and traceability;
- every OS installation, manager operation and runtime qualification stays NOT_RUN and unclaimed.

The governing contract is `docs/specs/corvint-tasks-user-service-v0.md`, SERVICE500-001..011.
It is written as the separately accepted profile that invariant 7 requires. It adds no index
transport or database, so the deployment-neutral index contract is unchanged.

## Delivered

`internal/tasks/service` contains profile.go, render.go, profile_test.go and render_test.go. It
holds closed profile/manifest codecs, launchd/systemd unit rendering, hypothetical install,
uninstall and rollback plans, and launch-fence, legacy-latch, restart-debt and resume models. It
imports only the Tasks wire package, nothing imports it, and it performs no filesystem, process,
manager or queue operation.

The four files are byte-identical to the reviewed repaired source from the prior preparation.
Their SHA-256 values match its manifest:

| File | SHA-256 |
|---|---|
| profile.go | e39805005d8c1d72d90f606813495925a810eb7074d3486098fb5852a9cb9ba2 |
| profile_test.go | 0ffec4bf22ea2838c76984acfe7382245627255e4fd8db0642445be753a54225 |
| render.go | 43b1e1f35d5597e8e9fc53d147d0eb6dc86d093526da9f6bae3315f5451a8e84 |
| render_test.go | 02eaf2d7615693cba71790c854e32e1685667ca553bbaabf616b5405069c4337 |

That preparation's plan review passed after its H1/M1/M2 findings closed. Its source review also
passed after F1 (resume replay preserves later failure debt), F2 (uninstall needs the exact
current manifest) and F3 (rollback cleans only the published subset under same-lineage
restoration) closed. Those reviews ran against base 9770fc6334a08f9d1e00508c396d62bae2563446.
The package directory is new on current main, so the bytes apply unchanged.

## Evidence and limits

The focused service package tests, a race run and vet passed on current main. The spec registry,
requirement generation, definition and traceability checks are part of the change evidence.
Exhaustive `make gate` and repository-wide tests are NOT_RUN under the scoped owner instruction.

The following are all NOT_RUN and unclaimed: CLI routes, native durable writers, operation
journal recovery, OpenControlled fencing, #464 helper ownership, launchd/systemd installation and
manager operations, login/boot scope, and Darwin/Linux runtime qualification. Model and renderer
results do not qualify a service. Native ticket completion remains open.

## Rollback

Revert the commit. The package has no callers and the spec adds intent and catalog rows only, so
reverting cannot unregister a unit or affect a running process.
