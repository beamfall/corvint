# macOS tracing mechanism decision

Requirements: RSP-V0-021, RSP-V0-022 (macOS analogues), RSP-V0-023.
Ticket: V1-5114 (T12). Decision: keep macOS trace components absent.

The accepted spec explicitly allows absence until an OS backend qualifies. This
slice supplies a Darwin backend behind the existing `proctrace.Backend`; it does
not qualify a tracer or enable runner selection. No mechanism studied below
meets the lane's unprivileged, unsigned, whole-process-tree constraints on the
retained evidence. This is not a proof that no future mechanism can qualify.

## Mechanism study

These are documented capabilities and limitations, not measured event coverage.
The study preceded implementation. No tracer, fixture, privileged operation,
entitlement request, SIP change, injected library or background service was run.
The local host reports macOS 26.6.2 (25G83), arm64, UID 501, Go go1.27.3.

| Option | Reads | Listings | Failed lookups | Writes | Execs | Decision |
|---|---|---|---|---|---|---|
| DTrace / dtruss | Potential syscall probes | Potential directory syscall probes | Potential syscall-return probes | Potential syscall probes | Potential exec/fork probes | Ineligible: installed `dtruss(1m)` requires root; SIP also prevents tracing protected processes. No completeness claim for any column. |
| fs_usage / kernel tracing | Filesystem syscall display | Potential filesystem syscall display | Potential syscall error display | Filesystem syscall display | Not qualified as a tree-wide exec collector | Ineligible: installed `fs_usage(1)` requires root; no supported unprivileged path established. |
| Endpoint Security, including descendant-scoped clients | Event coverage not qualified | Event coverage not qualified | Event coverage not qualified | Event coverage not qualified | Exec notifications documented | Ineligible: descendant-scoped clients remove root/TCC requirements but still require the Apple-granted Endpoint Security entitlement. Do not assume rootless means entitlement-free. |
| FSEvents | No read observation | No enumeration observation | No failed-probe observation | Change notifications; not exact write kinds or process attribution | No exec observation | Unprivileged notifications cannot establish the required dependency trace. |
| kqueue vnode / process filters | No file-read observation from vnode modification events | No directory-enumeration observation | Cannot watch a nonexistent vnode to observe failed probes | Watched-vnode changes, not complete write kinds | Process filters can notify exec/fork/exit | Unprivileged but lacks required file observations; process notifications do not recover them. |
| DYLD interposition / loader diagnostics | Only intercepted calls / loader activity | Only intercepted calls | Only intercepted calls / loader searches | Only intercepted calls | Only intercepted launch paths | Cannot certify direct syscalls or the whole tree; protected launches purge DYLD variables and hardened executables restrict injection. A cooperating process is not an independent trace authority. |
| ptrace / Mach debugger | Would require instruction-level reconstruction | Would require instruction-level reconstruction | Would require instruction-level reconstruction | Would require instruction-level reconstruction | Debugger can follow controlled execution; tree completeness unqualified | Installed public ptrace requests expose stepping/attachment, not the Linux syscall-stop/filter interface. No complete unprivileged tree collector established; protected-process attachment is restricted. |
| sandbox-exec logging / denial policy | Denials are not a complete successful-read stream | No complete successful-listing stream established | No complete failed-lookup stream established | Denial policy can alter outcomes | No complete exec-observation stream established | Containment is not complete observation; installed command is deprecated. Do not substitute sandbox denials for a trace. |

Original evidence, consulted for this decision:

- Installed Apple manual pages `/usr/share/man/man1/dtruss.1m` (DESCRIPTION,
  root-only), `/usr/share/man/man1/fs_usage.1` (DESCRIPTION, root-only),
  `/usr/share/man/man1/sandbox-exec.1` (DESCRIPTION, deprecated containment).
- Installed Command Line Tools SDK `usr/include/sys/ptrace.h` (public request
  definitions): no Linux-style syscall tracing request. This does not rule out
  every possible debugger-based implementation; none is qualified here.
- [Apple runtime protections](https://developer.apple.com/library/archive/documentation/Security/Conceptual/System_Integrity_Protection_Guide/RuntimeProtections/RuntimeProtections.html):
  protected-process attachment and DYLD restrictions.
- [Apple Endpoint Security entitlement](https://developer.apple.com/documentation/bundleresources/entitlements/com.apple.developer.endpoint-security.client):
  entitlement requires an Apple request.
- [Apple descendant-scoped client](https://developer.apple.com/documentation/endpointsecurity/es_new_descendants_client%28_%3A_%3A%29):
  root/TCC exemption retains that entitlement requirement.
- [Apple file system events](https://developer.apple.com/documentation/coreservices/file_system_events):
  modification notifications, not dependency reads.
- [Apple kernel queues guide](https://developer.apple.com/library/archive/documentation/Darwin/Conceptual/FSEvents_ProgGuide/KernelQueues/KernelQueues.html):
  vnode modifications and process notification limits.
- [Apple DYLD entitlement](https://developer.apple.com/documentation/bundleresources/entitlements/com.apple.security.cs.allow-dyld-environment-variables):
  hardened runtime injection restrictions.

Retained manual/header bytes are outside the repository under
`/Users/russelllewis/lanes/rspt12-evidence/`, with host metadata in
`mechanism-host.json`. SHA-256 digests of the exact local sources:

| Source basename | Bytes | SHA-256 |
|---|---:|---|
| dtruss.1m | 2329 | `107ecdf1a98de716a28bff409c379bb809271974c42083268fb39d50d3c46c25` |
| fs_usage.1 | 5174 | `516b37f33b209156bb6ebffef63aa8f14c7c5b5c21535e09340ebb499dc8f97a` |
| sandbox-exec.1 | 1102 | `0d264f72dc6a8e0db5474840f33b512542c469beb109ed146c17055da287336b` |
| ptrace.h | 4821 | `35244257713265037ac448b342c70466ee7b227900aef4be827343dcf99ca1bc` |

## Result and safety boundary

`New().Trace` on Darwin returns backend `absent`, retaining the existing
`unsupported-platform` reason and adding `darwin-mechanism-unqualified` to all six
components: reads, listings, lookups, writes, execs and external contact. Every
component is absent with no entries. The existing outcome stays unstarted with
exit code -1; no command is launched and no streams are consumed. Empty or
malformed requests, missing/cancelled contexts, unsupported executables and caps
cannot produce a present component. This is absence, not a successful empty
trace or an executed test result. Execution remains the caller's responsibility.

XCTest/XCUITest gain no trace-based classes. They remain coverage-only candidates
under an independently OS-qualified untraced model; this slice enables no
narrowing and does not change their absent selection-piece declarations. Windows
and other unsupported platforms retain the prior backend. Linux is unchanged.
No wire shapes, public errors, exit codes, stderr, goldens or frozen outputs change.

Rollback is reverting this slice: Darwin returns through the generic unsupported
backend again. There is no persisted state, evidence migration or helper to retire.

## Acceptance and retained limits

- Positive/negative unit fixtures for each scoped ID are in
  `internal/proctrace/backend_darwin_test.go`; the traceability rows name them.
  They test the specified absence path, not a qualified positive tracing path.
- **NOT_RUN:** all Go tests, real fixture coverage (including listing and failed
  lookup coverage), traced/untraced parity, overhead and runner qualification.
  The build-only lane explicitly defers execution to integration. No trial
  denominator or observed coverage percentage exists; absent is the safe result.
- **NOT_PRODUCED:** qualified macOS tracing mechanism, tracing record and narrowing
  evidence. The studied options are ineligible or incomplete under the stated
  constraints. Independent review and final dogfood binding belong to integration.
- Native Darwin `go vet ./internal/proctrace` passed through `script/heavy.sh`
  with the supplied Mac configuration and empty remote host. Formatting, the
  static spec generator/readme/worktree checks and `make -s line-citations-check`
  passed. Regeneration changed no generated spec files; repinning moved nothing.
  The full `make spec-requirements` and `make spec-requirements-check` targets
  invoke Go tests, so only their static script portions ran under build-only
  instructions. Test-backed gate results remain NOT_RUN.
- Corvint context, query and path-impact were used before editing, and affected
  planning is retained outside tracked source under
  `/Users/russelllewis/lanes/rspt12-evidence/`. Context omitted paired tests and
  eight spec-cited sources; the relevant proctrace sources/tests were read directly.
  Non-Go range omissions and other receipt uncertainty remain limitations.
- The requested startup `dogfood-change` ran against
  `8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734` and refused missing citation/intent/
  verification inputs. The charter's integration base is
  `80274abee8da1afdf4e193751b6f1f71231e4581`; both are retained, not substituted.
  No bind, seal, review model or Tasks write is performed by this build-only lane.
