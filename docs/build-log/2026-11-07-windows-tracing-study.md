## 2026-11-07 V1-5117: Windows tracing mechanism study — abstain

<!-- SPDX-License-Identifier: Apache-2.0 -->

Study performed 2026-11-08 for the 2026-11-07 lane charter. Authority: decision 0592 and
`docs/specs/runner-selection-plugins-v0.md` at
`f42df32d12bae40c3bfcc6bb8a48442966e4a71d` (blob
`8b24c426647c60849212a374afbf14c7894b376f`). This is a mechanism decision and evidence record,
not implementation, a new requirement, or Windows qualification.

### Decision and acceptance disposition

**Abstain: no examined mechanism establishes the complete, unprivileged tracing service required
by RSP-V0-021.** Record the Windows backend and its six trace components (`reads`, `listings`,
`lookups`, `writes`, `execs`, `external`) as **`absent`** under RSP-V0-023. This satisfies the
ticket's alternative acceptance path: a recorded absence with evidence and the fallback named.
It does not satisfy the alternative measured-mechanism path. No backend is recommended for
implementation, so the RSP traceability row remains unchanged.

Windows runners, including `dotnet test`, remain FULL when RSP enforcement applies
(RSP-V0-046); a Windows-qualified `untraced` model is the only permitted narrowing exception
(RSP-V0-023/045). This study supplies no such qualification. Before enforcement, the existing
static/FULL plans still govern and remain outside the new spec's soundness claim. Missing trace
components select tests under RSP-V0-013. An experimental capture that starts but loses evidence
must say `incomplete`, never an empty, apparently complete component; an unavailable or disabled
backend says `absent`. Traced/untraced outcome divergence disables tracing under RSP-V0-024.

The host reports `Linux` (`uname -s`). No Windows box or GitHub Actions run was used. Windows
coverage, loss rates, parity and performance are **NOT_OBSERVED**, not zero. Documentary support
and design inferences are separated below; none of the candidate coverage is claimed measured.

### Comparison against the six components

The table describes potential observations, not qualified `present` records. Inferences about
missing completeness guarantees are study conclusions from the linked API boundaries, not claims
that Microsoft promises or forbids every possible future combination.

| Mechanism | File reads, listings, failed lookups, writes | Process tree and execs | External access and completeness conclusion |
|---|---|---|---|
| Kernel ETW file/process providers | FileIo defines create/open, read/write, enumeration, query, rename and set-information events [E1]. Enumeration carries a pattern and file-object correlation [E2]. OperationEnd carries NT status and IRP correlation [E3], allowing a failed-open observation in principle. A complete denominator for all missing-path probes and all write kinds is UNVERIFIED. | Process events carry process/parent identities and image path [E4]. Capture must begin before launch and remain through all descendants; digest and instrumentation identity still need binding. | File/process providers alone do not establish coverage of all network, named-pipe and other IPC contacts. Additional providers and complete attribution would need proof. No loss, name, status or process-correlation gap may be treated as complete [E5]. |
| File-system minifilter | Registered pre/post callbacks expose I/O operations; FLT_PARAMETERS includes create, read, write, directory control and set-information [M1]. Completion status can distinguish failures [M2]. Cached/paging I/O can be skipped by registration flags, so coverage depends on configuration [M3]. | File callbacks alone are not a process-creation/digest tracer; another mechanism would be needed. Paging/cache activity still needs demonstrated attribution to the originating test tree. | A file-system hook alone is no proof of all network/IPC access. Unsupported volumes/operations, unresolved attribution or dropped callback records leave affected components incomplete. Kernel driver deployment independently violates RSP-V0-021. |
| Detours/API hooking | Can instrument selected API calls and their return values [D1]. Hooks for open/read, enumeration, metadata probes and write/rename operations could observe their arguments and failures. Calls not traversing a hooked function are outside that observation: completeness against native calls, mapped access and every runtime is UNVERIFIED. | DLL insertion can occur before application entry, with architecture-specific support [D2]. Every descendant and launch route needs coverage; a single injected root does not prove that. | Network/pipe/IPC functions also require hooks. Inference: bypass or injection failure can be silent unless separately detected; therefore a successful trace log cannot establish complete components for an unrestricted runner. |
| Job objects | Manage groups and aggregate resource accounting, not path-level operation records [J1]. No reads/listings/lookups/write-kind evidence. | Useful lifetime/containment aid, with child-association exceptions and breakaway rules; most completion-port notifications are not guaranteed [J1]. Not sufficient exec identity evidence. | No complete external-contact record. Unsupported trace components remain absent; a proposed event-based composite must treat missing lifecycle coverage as incomplete. |
| ProcMon-style capture | Microsoft documents file-system operations with input/output details and filtering [P1]. Potentially useful as a diagnostic comparison, but the exact failed-lookup/listing corpus and write-kind normalization are UNVERIFIED here. | Includes process/thread activity, process details and a process tree [P1]. Those views do not themselves bind executable digests or prove exhaustive descendant capture. | The cited documentation does not promise every RSP external channel, loss detection or fail-closed completion. Never infer `external=false` from an empty capture. Elevated capture also fails the unprivileged bar [P2]. |

### Privilege, CI feasibility and overhead

GitHub documents hosted Windows VMs as administrator sessions with UAC disabled [C1]. Thus an
experiment that runs on `windows-latest` is not evidence of unprivileged operation. Actual image
version, architecture, security policy, installed tools and job constraints were NOT_OBSERVED;
the mutable runner label is not a qualified execution identity (RSP-V0-006/053).

| Mechanism | Privilege and signing | GitHub `windows-latest` feasibility | Overhead assessment |
|---|---|---|---|
| Kernel ETW | Microsoft limits NT Kernel Logger control to administrators or LocalSystem [E6]. General Performance Log Users permissions do not remove that specific restriction. No custom driver signing for the built-in providers. Other provider/session permission combinations are UNVERIFIED. | An elevated diagnostic experiment is plausible given [C1], but was not run and would not meet RSP-V0-021. | Buffers consume memory; overload can discard events [E5]. CPU, wall time and trace volume are NOT_OBSERVED. No low-overhead completeness claim. |
| Minifilter | FilterLoad requires SeLoadDriverPrivilege [M4]; kernel drivers require signing under Windows policy [M5]. No assumption that test signing is usable on hosted images. | Requires a suitable signed driver, installation/load permission and validation against the actual image. Administrator access alone proves none of these; feasibility is UNVERIFIED. | Per-operation callbacks and transport create work (inference from [M1–M3]); latency, CPU and memory cost are NOT_OBSERVED. |
| Detours | User-mode library and DLL insertion [D1/D2], no kernel-driver signing. Same-user child creation uses the caller's security context [D3]. Unprivileged operation for an ordinary owned process is plausible, not qualified for every child/policy. | Plausible controlled experiment with matching DLL architectures; injection/mitigation compatibility is UNVERIFIED. A native DLL also needs a separate decision against the default native-Go product boundary. | Function redirection, instrumentation and transport add work (inference from [D1]); per-runner cost and outcome parity are NOT_OBSERVED. |
| Job objects | Windows process-management API with access-controlled handles [J1], no custom driver. Feasible for owned processes subject to assignment rights. | Plausible helper; nested-job and child-launch compatibility must be tested. It cannot replace tracing. | Accounting/notification cost is NOT_OBSERVED. Less information is not equivalent to a cheaper complete tracer. |
| ProcMon-style capture | Microsoft instructs running capture elevated [P2]. No local driver was installed or inspected: its exact driver/signature identity is NOT_OBSERVED. A custom driver-backed replacement inherits [M4/M5]. | Plausible elevated diagnostic, with pinned binary/license and policy checks still unverified. Not an approved bundled dependency or unprivileged service. | Stack capture and logging are documented features [P1]; their cost and capture volume are NOT_OBSERVED. Configure/bound them before any future measurement. |

### Cross-cutting fail-closed obligations

These are applications of the existing requirements, not amendments or new wire/error fields:

- RSP-V0-006/021: establish each component for the entire tree, including short-lived children,
  asynchronous operations and termination/drain. PID reuse, a late start, unknown schema,
  lost events, missing completion or an unobserved child prevents a completeness claim. ETW loss
  counters are necessary checks, not proof that the chosen providers cover every operation [E5].
- RSP-V0-007: retain both lexical and resolved paths and case-fold on case-insensitive volumes.
  Windows offers opened/normalized handle paths [N1], but a failed open provides no successful
  handle to resolve. Reparse points, relative paths and name races therefore need explicit
  qualification; matching a display string to a Git path is insufficient (inference).
- Preserve operation order and successful write disposition. Drop a subsequent read only for the
  exact proved no-prior-content cases in RSP-V0-007. Append, positioned overwrite and truncation
  keep read edges; a generic write event is not proof of a safe replacement. Bind repository
  reads and executables to immutable content, not just event names.
- RSP-V0-049: resolve declared sibling-repository reads into the correct identity and pinned
  diff. Outside reads not in the repository/sibling/toolchain/cache/temporary classes make
  `reads` incomplete. Outside-path class counts do not grant selection authority.
- RSP-V0-054 and RSP-V0-009: build activity missing from the build graph reaches every test of
  that build; unattributable concurrent test activity reaches every test in the tree. Do not
  discard it as setup noise. External contacts include loopback and pre-existing local processes,
  not just internet traffic; observed contact selects the test, and unknown coverage is not false.
- RSP-V0-022/024 and Failure modes: caps, untraceable activity and evidence gaps widen selection.
  Qualification must compare actual outcomes traced/untraced and retain per-runner overhead.
  A hybrid of incomplete observations is not complete without a proved combined denominator.

### Reopening and rollback

No implementation stages are approved. Reopen only with a proposed mechanism compatible with
RSP-V0-020/021, or a separately accepted change to those boundaries. Before recommending it,
collect on a pinned Windows image under a genuinely non-admin token: ordinary and missing-path
opens/metadata probes, empty and populated directory enumeration, cached/mapped reads, all five
write kinds and subsequent reads, rename/reparse/sibling cases, short-lived descendants, and
network/loopback/pipe/other IPC to pre-existing processes. Include loss/cap/denial and bypass
negative controls; require incomplete/absent fallback where appropriate. Then demonstrate
per-test attribution, traced/untraced outcome parity and measured wall-time/CPU/memory/volume
cost on each runner. Privileged diagnostics may be comparison evidence only, not the admitted
backend. This is a proposed reopening experiment, not measured acceptance evidence.

Rollback of this study is a superseding build-log entry under decision 0423. It changes no code,
requirements, frozen output or runtime behavior; the existing FULL/absent fallback needs no rollback.

### Primary sources and evidence limits

Sources accessed 2026-11-08. Microsoft documentation supplies the platform facts; Microsoft-owned
Detours documentation supplies insertion details. GitHub's own documentation supplies its runner
policy. No secondary-source performance estimates are used.

- [E1: FileIo event types](https://learn.microsoft.com/en-us/windows/win32/etw/fileio).
- [E2: FileIo_DirEnum](https://learn.microsoft.com/en-us/windows/win32/etw/fileio-direnum).
- [E3: FileIo_OpEnd](https://learn.microsoft.com/en-us/windows/win32/etw/fileio-opend).
- [E4: Process_TypeGroup1](https://learn.microsoft.com/en-us/windows/win32/etw/process-typegroup1).
- [E5: EVENT_TRACE_PROPERTIES, buffer bounds and loss counters](https://learn.microsoft.com/en-us/windows/win32/api/evntrace/ns-evntrace-event_trace_properties).
- [E6: StartTraceW, ERROR_ACCESS_DENIED](https://learn.microsoft.com/en-us/windows/win32/api/evntrace/nf-evntrace-starttracew).
- [M1: FLT_PARAMETERS](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/fltkernel/ns-fltkernel-_flt_parameters).
- [M2: Post-operation callback](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/fltkernel/nc-fltkernel-pflt_post_operation_callback).
- [M3: FLT_OPERATION_REGISTRATION](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/fltkernel/ns-fltkernel-_flt_operation_registration).
- [M4: FilterLoad privilege](https://learn.microsoft.com/en-us/windows/win32/api/fltuser/nf-fltuser-filterload).
- [M5: Driver signing](https://learn.microsoft.com/en-us/windows-hardware/drivers/install/driver-signing).
- [D1: Microsoft Research, Detours](https://www.microsoft.com/en-us/research/project/detours/).
- [D2: Microsoft Detours, DetourCreateProcessWithDlls](https://github.com/microsoft/Detours/wiki/DetourCreateProcessWithDlls).
- [D3: CreateProcessW](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-createprocessw).
- [J1: Job objects, accounting, breakaway and notification limits](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects).
- [P1: Process Monitor capabilities](https://learn.microsoft.com/en-us/sysinternals/downloads/procmon).
- [P2: Microsoft capture procedure, elevated permissions](https://learn.microsoft.com/en-us/troubleshoot/windows-client/shell-experience/troubleshoot-apps-start-failure-use-process-monitor).
- [C1: GitHub-hosted runner administrative privileges](https://docs.github.com/en/actions/reference/runners/github-hosted-runners#administrative-privileges).
- [N1: GetFinalPathNameByHandleW](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getfinalpathnamebyhandlew).

### Local verification and self-use scope

Corvint context, query and path impact were used before the study edit, at the spec commit above.
Context found the two requested RSP terms with no critical missing source, but omitted five
results; query omitted four ranked results. Impact retained unknown reverse-import resolution
for the document. Their mixed-worktree notice refers to the initial generated CEM, not source
changes. These are guidance receipts, not Windows evidence or retroactively bound base receipts.
The initial `make dogfood-change` against the instructed outer base
`8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734` returned missing citation/intent/outcome inputs and
`coordination-time-impact NOT_PRODUCED unsupported-impact-range`; its prechange receipt slots
were NOT_OBSERVED. That base includes the inherited spec commits; the study alone is record-only
relative to the charter's `f42df32d12bae40c3bfcc6bb8a48442966e4a71d` baseline.

Required local checks are `make -s spec-claims-check spec-status-check line-citations-check`.
Exact command results and final CEM binding are retained in the lane handback. No Go package is
changed; Go package tests, Windows live qualification, frozen selection evaluations and broad
repository gates are NOT_RUN for this documentary abstention. The orchestrator owns independent
review and ticket completion; this lane does not write the task store, seal or publish.
