# Pi native workflow candidate

Owner direction: 2026-09-29 “do it. parallel”, after Pi integration research and the
owner's update to Pi 0.99.1. The owner separately approved isolated build work after
the native Tasks claim refused RESOURCE_COLLISION. No foreign reservation, queue
policy, merge or native completion was changed by that exception.

## Contract and result

Experimental ordinary extension tuple: Pi 0.99.1 / adapter 0.3.1 on Darwin arm64.
The candidate adds bounded native Core reads, explicit Tasks operator commands,
a read-only evidence cockpit, immutable source expansion and an explicitly enrolled
local-policy bridge. The bridge can request one idle remediation and then releases
visibly unresolved. Native receipts, refusals and uncertainty remain visible.
Legacy Stop keeps false continuation; protected Pi and Frontier authority are absent.

Intent is PWV-V0-001..010, AHI-020/024/025 and LCP-V0-008/009. The new PWV spec
records accepted human direction and experimental technical delivery. The final
CEM/OCM uses existing base intents; PWV did not exist at the base and cannot be
retrospectively claimed as a base OCM intent.

## Qualification and independent review

Source review: independent Astra/medium reviewer, clean f515ffd360264a405953da3b149253a31adb47b3,
base 8691eb3c88c3d31e01fe2f2d414e207736d83261. Gate B is PARTIAL, with no further
source repair requested after the two bounded Tasks repair cycles. Fixed findings:
refusal replay presented as success; incomplete signal ownership; asynchronous
cockpit identity and view-navigation races; valid mixed-worktree receipts rejected;
boundary entries replaced; workflow identity and actionable remediation missing.
An uncertain gate retry now refuses before redispatch and retains reconciliation.

Final tuple checks: 79 unit checks passed, zero failed, one real-expiry opt-in skipped;
9 exact native-host checks passed; scoped Go Pi/dogfood/Claude lifecycle checks
passed; spec, requirement-definition and package-version checks passed, including
11 package-gate fixture cases. The independently retained native Tasks matrix passed
4/4, including actual five-minute lease expiry and interruption of a live gate plus
its descendant. The final package gate includes all six new shipped Pi modules.
The source package and native adapter advanced together to 0.3.1 after the gate
correctly rejected a stale intermediate version. Failed intermediate runs remain
retained, rather than replaced by passing output.

Real host evidence covers install/update/disable/remove, text/JSON/RPC/TUI, trust
refusal, startup SIGINT/SIGTERM, descendant retirement, fork, tree navigation,
default manual compaction, one automatic retry and overlapping native tools.
Enrolled workflow evidence covers one remediation, recursive release, errors,
aborts, queued messages, mixed worktrees, earlier boundary entries and nested cwd.
Automatic recovery packets remain ephemeral in session storage.

Evidence directory: /private/tmp/corvint-pi-wave. Final logs: units-final-031.log,
native-final-031.log, go-final-031.log, version-spec-final.log. Additional source
fixtures and evidence: tasks/qualification-final/, host/qualification-final/,
workflow/, cockpit/. The entire repository make gate is NOT_RUN under scoped
issue policy; affected-final.json retains selection uncertainty, including dynamic
TypeScript/runtime behavior that static affected selection cannot establish.

## Retained limits and open work

- Legacy packet cache still keys cwd/session, rather than canonical Git-directory
  identity. Native immutable selector validation remains; same-path repository
  rebinding of that legacy cache is unqualified (PWV-V0-004 PARTIAL).
- Initial claim expectedRevision is preflight-only: native build202 has no atomic
  requested-ticket-revision CAS and may admit newer acceptance intent. Inspect
  the actual native attempt receipt before further work.
- Native gate run executes before receipt replay lookup. An uncertain pending
  gate cannot safely resume automatically; explicit reconciliation remains open.
- Full real-PTY resize/theme/control-character and combined active-workflow/attempt
  recovery matrices remain incomplete. Automatic overflow compaction, all retry
  families, Linux, Windows and protected exact-image campaigns are unqualified.
- V1-0510 is dependency-blocked: current native supervisor capsules admit only
  taskman-codex-supervisor/0. No Pi participant or substituted Codex capsule exists.
- V1-0511 comparative campaign is NOT_RUN. Provider tokens, cache, billed cost and
  superiority over OpenCode are NOT_OBSERVED. Current comparison baseline is
  installed OpenCode 2.0.18 and repository plugin 0.7.5, not the older research
  plugin 0.7.3. The proposed comparison target awaits the owner's preference.
- V1-0506/0447/0507/0508/0509 remain PARTIAL; V1-0510/0511 remain MISSING.
  The native queue has no admitted Pi lease. Landing and the native completion
  write remain open; no ticket is completed by this source candidate or CEM seal.

## Tool freshness and rollback

Installed optional corvint-update checked official channels at closeout: Core
1.0.0-rc.1 build163 / v1.0.0-rc.1; Tasks developer build202 /
tasks-dev-20260929.2. Both match the previously verified official binary hashes.
Updater reports release evidence only, publisher identity NOT_VERIFIED, atomic
visibility without a power-loss durability guarantee. No official update was needed.
The Pi candidate is a separately named local source artifact, never an official
Core release, and does not replace the installed official binaries.

Installation retains a versioned artifact manifest, matching licenses and the
previous Pi settings for rollback. Remove the candidate package registration and
restore the previous registration to roll back; retain task receipts and pending
operation metadata. No store or active foreign process is erased. Candidate
installation and final clean evidence sealing are separate integration closeout
steps, recorded by their generated artifact receipts.
