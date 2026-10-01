# Governing authority screen: hidden Unicode and self-modified authority (V1-0414)

Base: `b8fe75d46f77054aaba25f52487bb873b49eee5a`. Ticket: V1-0414. Owning intents: TCP-V0-055..058
(`docs/specs/task-context-packet-v0.md`, experimental) and CEM-CB-026
(`docs/specs/cem-0.2-canonical-binding.md`, experimental).

## Threat

`corvint context` gives governing instructions, task-named specs and the paths the instructions
route the project's top authority. A poisoned instruction file inherits that authority. Two
published attacks show the path: the Rules File Backdoor hid agent instructions in invisible
Unicode inside rule files that review shows as clean
([pillar.security](https://www.pillar.security/blog/new-vulnerability-in-github-copilot-and-cursor-how-hackers-can-weaponize-code-agents)),
and an `AGENTS.md` injection made OpenAI Codex CLI stage local credentials before the user's task
([backslash.security](https://www.backslash.security/blog/openai-codex-injection-in-agents-md-exfiltrating-credentials)).
A second path is an agent editing the governing file of its own change.

## Decisions

- Screen every reserved row (governing, spec-mentioned, instruction-routed) over its pinned
  bounded text for the exact sets in TCP-V0-055: zero-width U+200B..U+200D, U+2060, and U+FEFF
  except at byte 0; bidi controls U+202A..U+202E, U+2066..U+2069, U+200E, U+200F, U+061C; tags
  U+E0000..U+E007F. A leading BOM is exempt, because editors write it and it hides nothing.
- Downgrade instead of dropping. The row stays in the reserved block, so truncation keeps it,
  with a named `warnings` entry. It moves after every clean reserved row and gets authority
  `downgraded-authority` (trust `repository-content`), score 0 and an inspect-not-follow action.
  It never satisfies `governance` or `critical`, is named with its warnings in
  `governance_refused`, routes no instruction paths, and the dogfood prompt profile does not
  record it as governance.
- The packet's "diff under review" is the working tree against the indexed revision
  (`Index.DirtyPaths`). No `--base` option was added. The committed range is reported by
  `cem status` as `selfModifiedAuthority` (CEM-CB-026): governing files the map's patch changes,
  the hunks that change them, and every basis that cites them.
- `cem status` reports this condition and does not block on it. Making the map `incomplete`
  would turn existing ready maps not-ready, which breaks the additive-wire rule, and that policy
  is the owner's to accept. The member is omitted when empty, so the core-freeze `cem-status`
  golden and the `cem-status-bound` oracle case are unchanged.
- An unreadable reserved path is downgraded as `hidden-unicode-unscreened`, not cleared by
  default (invariant 2). The governing generator already skips unreadable files, so in practice
  this guards the other relations.
- Edits to cited files were made in place (`taskcontext.go`, `trust.go`,
  `local_completion_context.go`), and new code lives in new files, so no line citation moved. The
  analyzer audit digest was re-pinned for a consumer-only change, following the precedent of
  `1c7a897a`.

## Owner follow-ups

- CCF-V1-005 freezes `coverage.governance_refused`. Its text says the member's relation and trust
  are NOT_PRODUCED because no generator writes a row, and that CCF-V1-006 decides by review. A
  generator now writes rows (`repository-content`, relations `governing`, `spec-mentioned`,
  `instruction-routed`). The accepted CCF text was not edited, so this needs owner review.
- There is no allowlist for legitimate joiners (emoji ZWJ, ZWNJ in some scripts). Such a governing
  file is downgraded.
- `AGENTS.override.md` is outside `documentKind`'s instruction rule, so it is outside both screens.

## Evidence

Focused tests and doc checks are listed in the PR for this branch. `make gate` and the full
`./...` run are `NOT_RUN`, per the owner's scoped-issue preference.
