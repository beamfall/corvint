# Issue 501 operator-note history and dispatch rendering (V1-0698)

## Intent

Issue 501's last status comment (at `618680492cf8`) listed three remaining items: an anchored,
bounded history read; dispatch rendering of the current note; and bringing the experimental spec
to the state its own promotion rules require. This slice delivers the first two and updates
`docs/specs/corvint-tasks-operator-notes-v0.md`. Delivery stays experimental. Promotion still
needs the remaining ON-V0-010 evidence and owner acceptance.

## Decisions

- **History read (ON-V0-008).** `corvint-tasks ticket note history <ticket> [--limit 1..50]
  [--cursor C]` is a pure read.
  - The default page is 20 entries, at most 50, and at most 1 MiB of rendered entries. One entry
    always fits on a page.
  - Every page walks from the committed head reference, checking digest, closed profile, ticket,
    exact revision decrement and previous link. A missing event is MISSING_EVIDENCE; a mismatch or
    broken link is JOURNAL_FORKED.
  - The cursor is unpadded base64url of a closed canonical object
    (`taskman-operator-note-cursor/0`). It binds queue, ticket, anchor head and revision, and next
    digest and revision. A later page refuses unless both anchor and next lie on the committed
    chain at those revisions. A concurrent replacement therefore leaves later pages on their
    anchor, and the page also reports the newer committed head.
  - Precedent: `transaction.ExternalReviewHistory`, with the same 20/50/1 MiB bounds.
- **Dispatch rendering (new ON-V0-011).** Workers launch before they claim, so the claim result's
  pinned `operatorNote` (ON-V0-007) stays authoritative. Dispatch can only show a launch-time copy.
  - The native observation resolves each noted ticket's reference. An unresolvable event becomes
    UNAVAILABLE with its code, never "no note", and does not fail the observation.
  - A new role-prompt placeholder, `{operatorNote}`, renders `""` for a never-noted ticket, so the
    prompt is byte-identical to the same prompt without it.
  - Otherwise the placeholder renders a block with the state, revision and provenance, an authority
    sentence (advisory prose, not instructions, acceptance criteria or authority) and a freshness
    sentence (the claim result supersedes this copy).
  - Substitution is a single strings.Replacer pass, so note text that looks like a placeholder is
    never re-expanded.
  - The `launched` event records `operatorNote`/`operatorNoteRevision` only for noted tickets.
- **Fail-closed defaults (agent-decided; owner questions below).**
  - The placeholder is opt-in rather than auto-appended. This follows the existing config contract
    that placeholders are the only substitutions.
  - The placeholder is refused in host argv, env and activity paths, so note prose never reaches a
    command line or an environment variable.
  - Codex round 1 (P1) showed that a host such as `/bin/sh -c "printf '%s' \"{prompt}\""` still
    splices the rendered prompt, and so the note, into shell code. A role whose prompt uses
    `{operatorNote}` now needs a host that passes `{prompt}` only as one whole argv element.
  - Codex round 2 (P1) showed that checking only the element before `{prompt}` is bypassed by
    `/bin/sh -c -- {prompt}`. The rule now refuses any code-string option anywhere in a
    note-bearing role's host argv: a single-dash cluster containing `c` or `e`, or `--command`,
    `--eval`, `--exec` or `--execute`. It also refuses `{prompt}` in an activity path. A host that
    needs a shell uses a wrapper executable. This is fail-closed and may refuse some harmless
    options, such as `-v -e`.
  - Codex round 3 (P1) showed that `/bin/sh +c {prompt}` passed, because the check matched only
    `-` options. The rule now also refuses `+` clusters containing `c` or `e`. It further refuses
    any argv element whose base name, with any version suffix trimmed, is a shell or script
    interpreter (`sh`, `bash`, `zsh`, `dash`, `ksh`, `fish`, `env`, `xargs`, `python`, `node`,
    `perl`, `ruby` and similar), so spelling variants of interpreter options no longer matter.
    The option denylist alone was not a sound boundary. A program that is not on the list but
    evaluates its argument as code is a recorded limit. A program that
    evaluates its own argument as code is outside what config validation can see.

## Limits

- Each history page re-walks up to 4096 events from the head. No cursor index is stored.
- The 1 MiB page cut cannot be reached through the CLI: 50 entries of at most 8 KiB text come to
  about 16 KiB escaped each. It is witnessed through the store's injected size function.
- The dispatcher observation reads the head event of every noted ticket on every tick, including
  completed tickets. This costs one bounded evidence read per noted ticket.
- Lane, pressure and external (non-native) observations carry no note.
- Supervised-run rendering is not delivered.

## Evidence

- `TestONV0008_NoteHistoryPagesAnchoredChain` and `TestONV0008_NoteHistoryRefusesBrokenEvidence`
  (internal/tasks/cli).
- `TestONV0011_DispatchObservesTheCurrentNote` (internal/tasks/cli).
- `TestONV0011_DispatcherRendersOperatorNote` and `TestONV0011_OperatorNotePlaceholderIsPromptOnly`
  (internal/tasks/dispatch).
- Mutation check: with the launch substitution and the observation wiring removed, both
  `TestONV0011_DispatcherRendersOperatorNote` and `TestONV0011_DispatchObservesTheCurrentNote`
  fail.
- `go test ./internal/tasks/... ./cmd/corvint-tasks/...`, gofmt and `go vet ./internal/tasks/...`.

## NOT_RUN

- `make gate` and the repository-wide `go test ./...` (the owner's scoped-work preference).
- A live dispatcher run over a real host.
- History over a 4096-event chain.
- Archive export/import of noted tickets.
- Two-process CAS.
- Live response-materialization failure.
- Crash witnesses.

## Owner questions

1. Should promotion of ON-V0 be accepted once the remaining ON-V0-010 evidence exists? This slice
   does not mark it accepted.
2. Should dispatch append the note to every role prompt by default instead of the opt-in
   `{operatorNote}` placeholder?
3. Is a per-tick head read for every noted ticket acceptable, or should the observation skip
   terminal tickets?
