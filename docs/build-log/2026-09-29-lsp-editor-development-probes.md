# LSP editor development probes, 2026-09-29

## Scope and disposition

V1-0478 and V1-0483 remain open. The owner selected Go/gopls agents and real VS Code/Neovim
editors. This isolated tooling slice implements the proposed LEQ-V0 development profile, not
accepted semantic support or promotion. Native task records were read only. No sealed holdout,
user editor configuration, default Core dependency or unrelated primary worktree was changed.

Base: public `origin/main` `75f06f9020a38af8a775a35479b4b16cc69b8e3c`, branch
`codex/lsp-editor-qualification`. The practical earliest proof was the real client handshake with
an explicit candidate executable; each missing semantic/outcome case remains NOT_RUN. Routine
tool setup/profile work used the inherited Sol/low task routing. Live account weekly usage was
87% at entry and 92% at the later checkpoint; no reset credit was redeemed. Billed task tokens,
cache usage and whole-task cost are NOT_OBSERVED; no savings claim is made.

## Actual clients and provenance

Installed VS Code: 1.137.0, arm64, commit
`645f29cc3176500b4b5762ba887cf2a7f0ffdf2c`, observed with `/opt/homebrew/bin/code --version`.
The disposable development extension uses Microsoft's `vscode-languageclient` **10.1.2**, not a
handwritten mock client. Exact package and transitive dependencies are retained in
`tools/lsp-editors/vscode-client/package-lock.json`; npm integrity for the client tarball is
`sha512-cOv5SMvtAhfVeo5m2r9L2rVHAD0J5CFOsmvH1WgNmPaI61VYjJNc3XUDLOKeFSDtWoAXIEoK6KetuBEL/3hErg==`.
Node/npm are qualification tooling only and were installed under `/tmp`.

Neovim was absent from PATH. Official pinned v0.12.5 macOS arm64 archive was downloaded from
`https://github.com/neovim/neovim/releases/download/v0.12.5/nvim-macos-arm64.tar.gz` into
`/tmp/corvint-lsp-editor-tools`, without a system installation. Its SHA256
`65fb000099e47ca1b762584c484cc833f40e30851a0ec450d4174e16317c1f9b` matches the official GitHub
release API asset digest retained in `/tmp/lsp-neovim-release.json`. Running its executable reports
NVIM v0.12.5, Release, LuaJIT 2.1.1774638290. The probe invokes the actual builtin LSP client.

Current official API documentation was fetched with Context7: `/neovim/neovim` documented
`vim.lsp.start`, `on_init`, `on_exit`, `Client:stop`; `/microsoft/vscode-languageserver-node`
documented awaited `LanguageClient.start/stop`. The initial library lookup for
`vscode-languageclient` returned an unrelated Swift library; that result was rejected and the
exact Microsoft project ID was used. No library behavior was inferred from the wrong result.

## Development observations and retained failures

Server: `/tmp/corvint-lsp-v10481-20260929 --experimental`, SHA256
`24fda1ef820a0ab3ecbd55527a25013b313bfc3de24c0535de9e703d68b9c511`. At execution this binary was
supplied from the separate dirty stdio candidate above the public base. Its immutable source
binding is NOT_OBSERVED here; the integrating owner must bind its final source before qualification.

Both actual clients observed initialize, initialized, shutdown, shutdown result and exit, with
zero standard semantic methods advertised. Neovim negotiated UTF-8; VS Code negotiated UTF-16.
Each final tooling report contains seven raw wire records, exit 0 and no remaining owned editor
processes. This is **LIFECYCLE_OBSERVED / UNQUALIFIED**, not conformance to unrun methods.

- `/tmp/lsp-editor-neovim-final-tooling.json`: SHA256 `02b23181b23d0a641241fb866efbf6435f791d6508cfe9d5afb87b14619d52ae`.
- `/tmp/lsp-editor-vscode-final-tooling.json`: SHA256 `87c0b23c71e28ef40d33e2768e8e7d15b9dd0d536b09218c51929e7f9c97fb43`.

Those runs used the final transport/cleanup code before a later identity-report-only addition;
that addition adds the CLI version output and installed VS Code application executable digest.
It does not change the wire transport or client startup. The report includes private disposable
paths; it is local raw evidence and needs publication review.

Early upstream gopls-only smoke exposed a proxy shutdown wait and a Neovim fast-event callback
error; those failures remain retained and qualify no Corvint behavior. The proxy now ends when
its server exits, and Neovim schedules report writing on the main loop. A focused interruption
probe exposed signal-handler reentry into `Popen.wait`; the repair sends signals without taking
that wait lock and bounds forced group retirement. The exact owned stuck proxy was retired.
The final regression exercises a fragmented 100 KiB frame, EOF, and interruption of a server and
child that both ignore SIGTERM; the configured process group retires.

The first real VS Code run completed the wire lifecycle but left its isolated macOS application
and helpers after closing the window. A socket in that private user-data directory also exposed
an invalid copy-based cleanup hold. The final extension quits its isolated application, the harness
retains the original temporary directory on cleanup failure, and cleanup follows exact
private-directory process seeds plus their witnessed PID/PPID descendants. Every signal checks
that the current command still matches the captured command. The earlier owned application,
helpers and respawned crash reporter were retired by exact verified PID; no broad editor kill
was used. Private earlier client result and wire survived in
`/tmp/lsp-editor-vscode-candidate.cleanup-hold`.

## Focused verification and uncertainty

Actual Corvint query and impact pre-context receipts are retained under
`/tmp/corvint-lsp-editor-evidence-20260929` with a SHA256 manifest; original raw paths remain.
The first impact invocation used an invalid concatenated base and refused
`unsupported-impact-range`; the corrected immutable base receipt was retained. `affected --base`
ran before checks. Its generic full-gate advice is not a policy override: owner scoped-issue
preference forbids starting `make gate` here. Full gate remains NOT_RUN.

Focused checks passed: optional harness self-check (transparent frames, EOF, interrupted owned
server/descendant retirement); Python compilation; Node extension syntax; requirement-index
byte comparison. Real-client lifecycle checks passed as above. All Unicode-position, rapid-edit,
multi-root, stale-response, crash, editor-interruption, semantic outcome, held-out and resource
measurements remain NOT_RUN. Numerical precision/recall/latency/resource/outcome floors remain
proposed until a frozen baseline and owner acceptance. No tuple is promoted.

`make dogfood-change` ran at change start and retained its incomplete result: no committed diff,
missing declared intent and outcome inputs. Enrollment with an empty intent list refused
`plan-bound-exceeded`; the new proposed spec is absent from the immutable base. That refusal is
retained rather than substituting unrelated accepted authority. Final CEM binding, clean target
checks, seal, independent review and native completion are NOT_PRODUCED in this substream and
remain with the integrating owner. No push, merge, seal or native task mutation occurred.

## Rollback and handoff

Remove the optional harness/client assets and disable the experimental companion; the default
native-Go product and user configuration are unchanged. Retain failed evidence and any unresolved
cleanup hold. The root coordinator owns independent review and integration. Prerequisites and
example invocations are in `tools/lsp-editors/README.md`.
