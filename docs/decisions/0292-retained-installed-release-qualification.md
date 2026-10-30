# 0292: Retained installed release qualification

Date: 2026-09-13
Status: proposed technical detail, owner review pending

PUB-V0-016's final qualification consumes the already retained companion bundle. A shared
verifier admits exactly its archive, checksum and smoke report, decodes closed bounded archives,
checks the nine component and five artifact references, and extracts only into fresh owned
scratch. The qualifying helpers, fixtures and dependency lockfiles come from that verified Corvint
source archive. The expected Corvint commit and tree are explicit frozen inputs, and the wrapper
builds the checker only from that clean revision with Go 1.27.0 and verifies its embedded VCS data.

The opt-in `public-release-check` runs the reviewed installed VSIX lifecycle normally and under
interruption, the foreground documentation watch through real MCP discovery/draft/consume, and an
idempotent planning seed rendered through the real console in pinned Playwright/Chrome. Every
child uses the shared bounded process-group runner, while the outer wrapper observes PID/start
identities so separately grouped descendants are also removed and verified absent. Per-stage output
is 1 MiB, aggregate captured output is 8 MiB, and the outer command is bounded by its caller. A PASS result is linked into a
previously absent path only after scratch cleanup and a final retained-bundle recheck. It records
digests, source identities, the bounded installed-host and roadmap results, and the five original
bounded automatic-docs/MCP JSON documents without private scratch paths. Source, bundle, cache,
scratch and result overlaps are refused before creating output, and pinned runtime bytes are
rechecked around every child invocation.

This command builds no release artifact, changes no repository or host configuration, grants no queue
execution authority, and performs no publication or visibility action. The actual installed run
remains `NOT_RUN` until the integrated source and retained artifacts are frozen. Rollback removes
this opt-in coordinator; the companion build and default native product are unchanged.
