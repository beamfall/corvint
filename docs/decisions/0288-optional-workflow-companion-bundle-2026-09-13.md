# Decision 0288 — Ship optional workflow tools through the companion pipeline

Date: 2026-09-13. Status: accepted. Authority: owner-directed public-release execution.

The macOS arm64 companion bundle adds five separately installable workflow binaries, the VS Code
VSIX, and the four raw host-package trees. The existing exact-export, independent-build,
deterministic-archive, checksum, notice, installed-smoke, fail-before-retain pipeline remains the
only builder. Corvint binaries share one exact Corvint source archive and notice group; `atm` uses one
task-manager source archive and notice group.

The default release remains the independent one-binary Corvint Core archive. Every host package and
the VSIX remains `FALLBACK`; packaging does not install a host package, qualify an editor, bundle
OpenCode's dependency or host, promote Go preview evidence, enable a runtime, or make a network
service. `PUB-V0-016` separately qualifies the installed VSIX/providers/MCP against the retained
bundle bytes.

`vsce` emits variable ZIP metadata even when two compiled member sets agree. The pipeline therefore
decodes both independently built VSIX files against one closed member allowlist, requires every
member byte to agree, and deterministically rebuilds the ZIP twice from those sets. It never
canonicalizes a semantic/member disagreement.

The manifest records exact Node, npm, TypeScript and vsce versions, their executable digests, and
the exported lockfile digest. A separate notice inventories the lock-pinned npm build dependencies
and licenses; none is a VSIX runtime dependency or shipped `node_modules` content. Successful smoke
evidence is retained atomically beside the archive and checksum, bound to the archive and invoked
component/source identities without putting a self-referential record inside the archive. The shell
gate owns the foreground Go runner's process group and terminates and waits it on interruption.

Rollback removes these optional members and restores the four-binary companion manifest. It does
not change the Core archive, repository data, task-store format, publication state, or visibility.
