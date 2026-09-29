# LSP editor qualification tooling V0

- Owner: Russell Lewis
- Date: 2026-09-29
- Intent status: proposed experimental tooling; owner selected VS Code and Neovim
- Delivery status: experimental; UNQUALIFIED
- Authoritative inputs: owner scope selection retained in native V1-0478; proposed LQP-V0-011/012/017 contract in PR #360; AGENTS.md

## Agent digest
- Claim: Actual editor lifecycle, definition, Git-bound context and edit-cancellation development probes retain wire evidence without promotion claims.
- Status: proposed experimental tooling; owner selected VS Code and Neovim; experimental; UNQUALIFIED
- Exists: `script/qualify-lsp-editors.py` and private disposable client assets under `tools/lsp-editors/`.
- Blocked on: exact companion integration, full real-client conformance, protected baseline, human acceptance of numerical floors.
- Read next: Profile, requirements, evidence and rollback.

## User job and profile

A developer must distinguish successful protocol startup from a qualified editor product. This
optional Python/Node/Lua tooling is outside the default native-Go product. It never installs a
server or an editor, modifies operator editor configuration, reads sealed holdouts or claims
semantic adequacy. The server executable and arguments are explicit operator inputs.

The first development tuples are macOS arm64, Go, VS Code 1.137.0 commit
`645f29cc3176500b4b5762ba887cf2a7f0ffdf2c` with Microsoft's `vscode-languageclient` 10.1.2,
and Neovim v0.12.5 with its builtin client. The companion build/digest and actual negotiated
encoding/capabilities must be recorded per run. A single disposable root is the only exercised
layout. These tuples remain **UNQUALIFIED**. Go/gopls 0.23.0 is the separate first agent profile,
not an inherited editor capability. Non-Go languages and other client builds are unsupported by
this probe profile. Session timeout is operator-selected 1–120 s (default 20 s); wire messages
are bounded to 1 MiB, headers 8 KiB, retained transcript 256 KiB. These development bounds are
not frozen product resource or promotion floors.

## Requirements

- `LEQ-V0-001`: The harness MUST use the actual selected editor and explicit server executable, report their identities and client observations, require an untruncated, ordered initialize/result/initialized/shutdown/result/exit exchange with root, encoding, configured server digest and proxy group retirement for successful exit, retain nonzero failure outcomes, and mark every tuple UNQUALIFIED.
- `LEQ-V0-002`: Editor state MUST be disposable and isolated from user configuration. Owned server groups MUST be retired on EOF or interruption. INT/TERM, exceptions and malformed evidence MUST still retire owned processes and preserve a report or original cleanup hold. Cleanup gaps MUST be reported and MUST prevent a successful probe result; an ordinary successful lifecycle MUST NOT imply crash/interruption conformance.
- `LEQ-V0-004`: Semantic development mode MUST bind one actual unsaved Unicode definition to its exact negotiated position encoding and wire transcript, verify disk remains unchanged, and preserve UNQUALIFIED status and all unrun cases.
- `LEQ-V0-005`: Context development probes MUST verify exact ordered current-overlay wire binding, immutable Core packet evidence against the fixed fixture Git objects and bytes, unchanged fixture disk/repository, and separate non-Git overlay identity. ABSTAINED MUST remain visible without passing the expected governance fixture; no qualification or authority may be inferred from the mode flag.
- `LEQ-V0-003`: Unicode, rapid edits, multiple roots, stale responses, provider crashes, descendant interruption, semantic outcomes and resource distributions MUST remain NOT_RUN until separately witnessed against the exact tuple. No method may be qualified by its mere presence in initialize.

- `LEQ-V0-006`: Freshness development mode MUST inspect the complete transcript for one actual edit-driven cancellation between a rapid request and its reply, then validate a healthy retry against the newest unsaved overlay and strictly increasing successful captures in one session. Scheduling misses MUST remain NOT_WITNESSED with nonzero exit; malformed or incomplete evidence MUST fail. No artificial delay, explicit cancellation or competing document/request transition may establish the witness. All tuples remain UNQUALIFIED.

The optional `--semantic-development` mode exercises one exact development witness in a private Go
1.27.1 module. Disk contains `package p` and `var disk int`; the actual unsaved buffer contains
`package p`, `/*😀*/ var x int`, and `var y = x`. Its definition request uses line 2 character 8;
the target must be line 1 character 13 in UTF-8, 11 in UTF-16, or 10 in UTF-32. The harness checks
actual didOpen/full-text didChange/request/result bytes, the client unsaved-buffer flag and unchanged
disk. Explicit `{workspace}` server arguments resolve to this private root. It neither negotiates
nor infers wider method support. Broader Unicode, edit freshness, outcomes and qualification remain
NOT_RUN and every tuple UNQUALIFIED.

The optional `--context-development` mode is a separate one-request probe of the closed namespaced
`corvint/context` request. It builds a tiny tracked Go fixture with root/nested AGENTS, numbered
SPEC and test using fixed offline Git commands, never client-supplied shell or source commands.
Actual clients change the unsaved buffer and request `{textDocument:{uri},task,limit:20}`.
The exact experimental marker is `corvintContext:{method:"corvint/context",schema:"corvint-editor-context/0"}`.
The result's intact Core bridge envelope must remain separate from the opaque session/capture,
URI/version/SHA256 overlay observation. Validator evidence joins packet commit/tree/blob identities
to actual Git objects and expected fixture bytes, and requires the nested governance/spec/test
witnesses. ABSTAINED is preserved as an observed response and fails the expected fixture proof.
Capture IDs are decimal strings matching ASCII `[1-9][0-9]*`, bounded by uint64
`18446744073709551615`; JSON numbers, zero, leading zeros and overflow are refused. One capture
checks format and bounds; whole-session monotonicity remains NOT_OBSERVED. Every claimed packet
path/blob and any line/span/content must match the fixture Git bytes, including additional tracked
paths. Initial disk text, one exact full replacement and no close/reopen before reply bind the overlay.
A clean Core dirty-path digest binds canonical JSON `[]`, not empty bytes. Exact
`receipt.coverage.critical` governing selectors are selection metadata; each must name a known
fixture governance path with independently bound evidence. Missing critical selectors, arbitrary
path dictionaries and unbound actual evidence fail. Private raw paths stay local; the publishable development summary uses repository-relative paths.
All tuples remain UNQUALIFIED and actual context clients remain NOT_RUN until reviewed server execution.

Cleanup signals only this harness's unreaped `Popen` process group. Indirect descendants are
observation-only; a changed command or zombie state cannot imply retirement. Their disappearance
must be observed within a fixed five-second monotonic stage, with inventories and sleeps bounded
by remaining time and polls at most 0.1 seconds. Sampled start/state/command describes lineage;
it grants no atomic PID-generation or signal authority. Ambiguous lineage or unavailable inventory
produces UNKNOWN and preserves the cleanup hold. Before the one-second pre-signal inventory, cached `Popen.returncode is None` pins the unreaped
owner without polling under the single-thread/no-other-reaper invariant. Its valid census row is
required; capture and descendant expansion precede any potentially reaping call. Already-reaped
numeric PIDs never seed ownership. This inventory retains initially attributable descendants across parent exit
and command changes; unavailable setup remains UNKNOWN even after direct retirement. Direct TERM/KILL communicate stages each have
three-second budgets before observation; the 1+3+3+5-second stage composition is not a universal five-second total.
A surviving indirect child remains a failure. These bounds do not qualify editor interruption.

The mutually exclusive `--context-freshness-development` mode uses the same tracked fixture and
three direct requests: READY baseline A, immediate rapid request A plus real unsaved full replacement
B, then a newest-overlay retry after the rapid reply. Its whole-transcript state machine requires
three unique typed IDs, exact request/edit/reply ordering, full replacements, closed responses,
unchanged disk/Git/Core bindings and increasing successful uint64 captures in the same session.
For the frozen server the expected edit-driven response is exactly `-32800`, `Request cancelled`,
`data:{reason:"CANCELLED"}`. `-32801/CONTENT_CHANGED` is separately observed stale rejection,
not a cancellation pass. Competing edits, open/close, explicit cancellation, shutdown and semantic
requests invalidate the interval. Frontend pending-at-edit observation is necessary but insufficient;
the raw wire and admitted server error establish the bounded interpretation, not internal worker timing.
A response-before-edit or edit-before-request scheduling miss remains NOT_WITNESSED/nonzero even
with a healthy retry. There is one attempt per client, no barrier, repeat-until-pass or output delay.
Broader freshness distributions and qualification remain NOT_RUN; actual new probes remain NOT_RUN
until exact reviewed server/tool execution. Core's immutable packet remains separate from both overlays.

## Failure modes and evidence

Unavailable executables or unpinned language-client modules refuse before startup. Client timeout,
server failure, missing client result or remaining owned editor processes produce a failing exit.
The report contains a private bounded raw transcript: disposable machine paths are local evidence,
not public diagnostics. Review before publication. Transcript truncation prevents a completeness
claim. A development startup pass establishes only the recorded handshake and shutdown. Exact private
editor process seeds and their witnessed PID/PPID descendants are observed after the disposable
instance quits. Signals target only the unreaped owned Popen group; indirect descendants are observation-only. Any remaining
process preserves the original temporary directory as an explicit cleanup hold.

`--self-check` validates asset presence, transparent fragmented wire frames, EOF and
interruption retirement of a signal-resistant server group, outer harness INT/TERM and malformed/missing evidence; it does not test editor interoperability. Python compilation is a tooling
syntax check. Actual-client reports belong outside tracked source; their exact digests, commands
and observed outcomes are retained in the build log. Full LQP conformance, three-arm semantic
outcomes, cold/warm p50/p95 and resource floors remain NOT_RUN. Floors require frozen baseline and
owner acceptance before promotion. Independent review remains required.

## Rollback

Remove the optional harness and client assets, and disable the separately invoked experimental
companion. No default Core behavior, editor installation, user configuration or persistent evidence
migration changes. Temporary tooling acquisition under `/tmp` can be removed after retained raw
reports are copied; preserve any cleanup hold until its exact processes are retired.
