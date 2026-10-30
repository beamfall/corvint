# Changelog

All notable changes to the Corvint VS Code extension are recorded here.

## 0.1.0 — Unreleased developer preview

- Added trust-gated, root-bound local Corvint CLI query and impact projection.
- Added an explicit, no-fallback MCP `2026-07-28` stdio client profile for the experimental local
  `corvint-mcp` server contract; runtime conformance remains `NOT_RUN`.
- Added Evidence, Impact, and Why Tree Views, status and commands, bounded diagnostics and
  decorations. The Test Observation import command is not offered until a shared Corvint go-live
  verifier is linked (V1-0438).
- Added executable pinning and drift detection, bounded process/JSON handling, deterministic
  degraded states, cancellation, memory-only state, and lifecycle cleanup.

This version is `FALLBACK`, is not published, and has no promoted compatibility tuple.
