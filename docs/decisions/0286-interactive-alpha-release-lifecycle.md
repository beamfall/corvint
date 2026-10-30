# 0286 — Interactive alpha release lifecycle

Date: 2026-09-13

Owner direction: complete the integrated release and repository cleanup, retaining private
repository visibility for the owner. This implements the existing PUB-V0-006 scope and the
owner's automatic unit/E2E and VS Code feedback requirement.

Use the editor save event to rerun the full explicitly configured JS unit/E2E suite, preserving
one-shot terminal results only after verified group cleanup. The Go foreground session retains
its existing watched-input and non-policy preview boundary. VSC-V0-071..076 and PUB-V0-016
freeze the narrow interactive alpha profile. The independent lifecycle plan review passed after
requiring cleanup on normal exit and preserving the provider's newline-optional JSON framing.

Qualification requires actual pinned runners, isolated editor, retained evidence and MCP
agreement. Source implementation and controller stubs are not installed qualification.
Rollback disables live tests and reverts this adapter slice; core CLI behavior is unchanged.
