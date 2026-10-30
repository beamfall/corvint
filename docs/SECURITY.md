# Operational security and support

The repository's [security policy](../SECURITY.md) owns vulnerability reporting and the support
window (`SOP-V0-010`): private GitHub vulnerability reporting, acknowledgment within seven days, and
for every 0.x version security fixes for the latest published release only. A release leaves the
window when its successor is published; a withdrawn release leaves it immediately. This note adds
no promise beyond that. On 2026-09-23 the GitHub API reported private vulnerability reporting
enabled for the repository; no live report has been sent through it.

Supported upgrade path: the lifecycle check (`SOP-V0-003`) qualifies an upgrade from the previous
published release (N-1) into a new store and rollback to it; other ranges are not qualified. The
1.0 support duration, end-of-support notice period and maintainer escalation fallback are owner
decisions for V1-0021 and are not implied by the 0.x window. The [changelog](RELEASE-NOTES.md) is
the release history and the [release runbook](RELEASE-RUNBOOK.md) the reproducible procedure.

The default Core is one native Go binary with no account, permanent daemon or external database.
Installation uses the existing verified candidate and version/platform store. It neither installs
services nor changes credentials, global trust, shell selectors or another installation. Candidate
checksums are integrity evidence; the alpha's unsigned artifacts are not authenticated by them.
Execute only a candidate whose provenance you trust. A successful version probe is not a sandbox
for an arbitrary malicious executable.

The candidate verifier admits 15 files, five directories including the root, at most 512 MiB per
file and 1 GiB total input, using bounded directory reads. Decompression and copies use additional
memory; this is not a 1 GiB RSS cap or a hard whole-verifier time limit. Both version probes have
30-second execution limits and one-second shutdown allowances, retain at most 4 KiB per stream,
and suppress their output in error messages. Owned process-group cleanup is required. Deliberate
process-group escape, concurrent adversarial filesystem replacement and power-loss recovery remain
unqualified. The installation store requires exclusive trusted-local ownership.

Before sharing diagnostics, inspect them for repository paths, source contents and secrets. The
existing secret-screening regressions cover traces and product evidence; suppression of candidate
probe output adds a narrower boundary, not a guarantee that arbitrary diagnostics are secret-free.

Governing instruction files are an injection path. Corvint gives `AGENTS.md`, `CLAUDE.md` and
the other governing files its packet reserves the project's top authority, so a poisoned file would
inherit it. Two published attacks show the path: Pillar Security's "Rules File Backdoor" hid agent
instructions in invisible Unicode inside rule files that review shows as clean, and Backslash
Security showed an `AGENTS.md` injection that made OpenAI Codex CLI stage local credentials for
exfiltration before the user's task. `corvint context` therefore downgrades, with a named
`hidden-unicode` warning, a reserved row whose pinned text holds a zero-width (invisible operators
included), bidi-control or tag code point, and with `self-modified-authority` one the working tree
modifies; `cem status` reports a governing file the change itself edits as
`selfModifiedAuthority`, and `cem cite` refuses to cite it as authority for that change. The exact
code points, limits and non-goals are in
[Task-Context Packet V0](specs/task-context-packet-v0.md) ("Authority screen for reserved rows",
TCP-V0-055..058) and [CEM stable 1.0](specs/cem-stable-v1.md) (`CEM-V1-114`, `CEM-V1-270`). The
screen is not a general Unicode policy: homoglyphs, variation selectors and ordinary rows are not
screened, and absence of a warning is not evidence that a file is benign.

Release-blocking regressions for hostile repositories, paths, symlinks, case folds, bounded output,
memory, time, interruption cleanup and secret screening are the matrix in
[Stable operations V0](specs/stable-operations-v0.md), run by `make hostile-regressions-check`.
Whole-process resident memory is NOT_COVERED there. Lifecycle runs are recorded per host; linux amd64
is NOT_RUN. No artifact is promoted or published by this work.
