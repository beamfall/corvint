# 2026-10-07: shared dispatch prompt fragments (V1-0954)

## Intent

Issue 656 (ticket V1-0954): a 21-role `taskman-dispatch/0` configuration repeated a 6 to 8 KB rules
block in every role `prompt`, grew past the 256 KiB `MaxConfig` and was refused by `dispatch status`.
The change adds proposed CAL-V0-175 to CAL-V0-178 to
`docs/specs/corvint-tasks-agent-leases-v0.md`, pending owner acceptance.

## Decisions

- Syntax: a top-level `prompts` map plus a role `prompt` that may be an array of literal strings and
  `{"fragment": NAME}` objects. A `{fragment:name}` placeholder inside strings was rejected: today's
  placeholder pattern `\{[A-Za-z]+\}` does not match it, so existing prompts containing such text are
  accepted literally and would silently change meaning. An `"@name"` string element was rejected
  because it is ambiguous with literal text. String prompts are untouched, so configurations without
  fragments decode exactly as before.
- Expansion runs inside `DecodeConfig`, before `validate`, and then drops the map and parts. Every
  role check (size, placeholders, `{operatorNote}` host safety, `{model}`, `{effort}`) sees only the
  expanded prompt, and every consumer (dispatcher start and idle reload, `dispatch status` and
  `unpark`, service run and lifecycle) uses that decoder (grep of `DecodeConfig`). The decoded
  configuration equals the inline one, so it also marshals back as an inline configuration.
- Nesting is impossible by construction: a fragment value must be a JSON string, and strings carry
  no reference syntax. The fragment map decoder refuses repeated names (encoding/json would keep
  the last) and names the fragment in every refusal.
- An unreferenced fragment is refused (fail closed): it costs nothing to remove, and it usually
  means a role dropped its shared rules or a reference was renamed. Owner question: keep this, or
  admit unused fragments to ease staged edits?
- Refusals use the existing configuration refusal (usage error at start, `configFile` INVALID in
  status, recorded reload refusal); no new result code. The dispatcher has no separate prompt
  digest; the configuration digest stays the SHA-256 of the file bytes.
- Bounds: 32 fragments, 1..65536 bytes each, 1..64 parts per role, expanded prompt 1..65536 bytes
  (checked while joining, so expansion memory stays bounded).

## Evidence

- Pre-change context: `corvint affected --base 8af2bf62` and
  `corvint --root $PWD context --task "dispatch config shared prompt fragments ..." --subject
  internal/tasks/dispatch/config.go` (it surfaced `internal/tasks/cli/dispatch.go` and the leases
  spec). CEM binding is left to the integrating batch.
- `TestCALV0175_FragmentPromptEqualsInline` builds 21 roles sharing a 12.5 KiB block: inline it is
  over 256 KiB and refused; with a fragment it is under 64 KiB and decodes to the inline Config.
- Focused tests: `go test ./internal/tasks/dispatch/` and the `CALV0176|CALV0127` CLI tests pass.

## Limits and NOT_RUN

- `make gate` and the full `go test ./...`: NOT_RUN (lane policy).
- A live dispatcher run with an adopter-sized configuration: NOT_RUN.

## Independent review

- Codex round 1 (gpt-6-astra, read-only): three findings, all accepted and repaired with
  regressions that fail on the round-1 source. P2: a repeated `prompt` member must decode per
  occurrence as the string field did (`null` keeps the earlier text; an earlier bad type still
  fails). P2: the 32-role bound now refuses before expansion allocates. P3: the expanded-size
  refusal names the part or fragment that passes the limit.
- Codex round 2: round-1 repairs confirmed; one P2 accepted: `Role.UnmarshalJSON` now starts from
  the existing element, as the default decoder does, so a repeated top-level `roles` member keeps
  earlier fields (regression in `TestCALV0176_ConfigWithoutFragmentsUnchanged`, fails without it).
  Repeated top-level members are themselves an older looseness of the closed decoder, left as is.
- Codex round 3: no blocking findings and no remaining P0-P3 findings at 75f54b15.
- After the repairs: `go test -count=1 ./internal/tasks/dispatch/ ./internal/tasks/cli/
  ./internal/tasks/service/` passes; local doc gates and use-case receipts pass.
