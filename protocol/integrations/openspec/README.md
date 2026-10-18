<!-- SPDX-License-Identifier: Apache-2.0 -->
# Corvint integration for OpenSpec (experimental)

This directory is the `STA-V0-009` package in `docs/specs/spec-toolchain-adapters-v0.md`. OpenSpec
has no hooks, so the integration is advisory text the agent reads. Supported OpenSpec releases are
only those listed in `interop/spec-toolchains/versions.json` (`STA-V0-001`).

| File | Use |
|---|---|
| `config.yaml` | Snippet to merge into `openspec/config.yaml`: `context`, `rules.tasks` and `operations.apply.guidance` telling the agent to run `corvint context` before apply and `corvint review` after it |
| `skills/corvint-openspec/SKILL.md` | Agent skill with the same two read-only steps; copy it into the agent's skills directory |

Every step only reads and reports Corvint output, including each `unknown` and `NOT_PRODUCED`. None
blocks the workflow, edits a file or commits. `corvint review` refuses a dirty worktree; the steps
report that refusal instead of committing. Remove the integration by deleting the merged keys from
`openspec/config.yaml` and the copied skill.

OpenSpec injects `rules` only into the matching artifact's instructions, and in 1.14.3 `apply`
reads `operations.apply.guidance` and `context`, not `rules`. The snippet therefore carries the two
steps in all three places: `rules.tasks` puts them into the task list `apply` works through.

## Reading OpenSpec specs as OCM intent

Corvint reads OpenSpec Markdown as declared intent only with an explicit form; it never infers the
form from the path. OpenSpec requirements have no IDs, so each obligation ID is `OSR-` plus the
first 12 uppercase hex digits of SHA-256 over the requirement name, and the envelope member
`nativeNames` maps it back to the case-sensitive name (`STA-V0-002` to `STA-V0-004`).

A main spec (current truth), with the `openspec-spec` form:

```sh
corvint ocm prepare --target "$TARGET" --expected-base "$BASE" \
  --intent openspec/specs/<capability>/spec.md --intent-form openspec-spec \
  --cem .corvint/change.cem.json --map .corvint/change.ocm.json
```

One change delta file, with the `openspec-delta` form. ADDED and MODIFIED requirements are
obligations; REMOVED and RENAMED entries are listed in `excludedRequirements`:

```sh
corvint ocm prepare --target "$TARGET" --expected-base "$BASE" \
  --intent openspec/changes/<id>/specs/<capability>/spec.md --intent-form openspec-delta \
  --cem .corvint/change.cem.json --map .corvint/change.ocm.json
```

`TARGET` and `BASE` are full 40-hex commit IDs. Then read the map with `corvint ocm status --map
.corvint/change.ocm.json --cem .corvint/change.cem.json --expected-base "$BASE" --target "$TARGET"`.
The two forms ship in a separate lane of the same spec; until they land, the installed binary
refuses the form names.
