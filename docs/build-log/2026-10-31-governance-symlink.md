# Governing instruction symlinks admitted as their pinned target — V1-1272

Problem: VanillaTwisted tracks `AGENTS.md` as a symlink (mode `120000`, blob `681311eb`) to a
tracked regular `CLAUDE.md` (`100644`, blob `1d694024`). TCP-V0-008 takes the first instruction
candidate, `AGENTS.md`, and its source text is the link text, not instructions. The candidate was
therefore unread, and TCP-V0-008 never substitutes a lower-precedence file for it. Every hook
packet reported `governance:AGENTS.md` unavailable with no reason.

Decision (TCP-V0-270 to TCP-V0-275, experimental under the standing lane auto-accept):

- A symlinked governing candidate stands for its one-hop target.
- The target is resolved lexically against the pinned Git tree, never the working tree.
- It is admitted only when the target is a tracked, in-tree, pinned, readable regular blob.
- The row carries the target's `path` and `blob_hash`, plus the link's `link_path` and
  `link_blob_hash`.
- Every other link is refused with one typed reason in `link_refusals`, and the hook selector
  carries that reason. The refusal reasons are: outside the tree, not in the tree (an untracked
  file and a dangling link look alike here), not a regular file, chain, cycle, is-subject, and
  invalid.
- The no-substitution rule is unchanged.
- TCP-V0-056 screens the target text. A modified working-tree copy of either the target or the
  link downgrades the row as self-modified authority, and the hook selector names that downgrade
  as its `reason`.
- Trees without symlinked candidates keep byte-identical packets.

Rejected alternatives:

- Following the link through the filesystem: this reads unpinned working-tree content.
- Multi-hop resolution: each hop would need its own pin, and a cycle would need a bound.
- Falling back to `CLAUDE.md` when `AGENTS.md` is refused: this violates TCP-V0-008's
  no-substitution rule.

Live evidence (branch build, scratch clone of VanillaTwisted HEAD `584147d8`, read-only on the
owner's checkout apart from `corvint index --if-stale`):

- The base binary (build 34, `dbc17137`) running Codex SessionStart emitted
  `"unavailable":[{"path":"AGENTS.md","section":"governance"}]`.
- The branch build running Codex SessionStart emitted
  `"governance":[{"path":"CLAUDE.md","blob_hash":"1d6940…","link_path":"AGENTS.md","link_blob_hash":"681311…","authority":"project-instructions"}]`.
- The Claude Code adapter drops that row by design (`withoutHostLoadedGovernance`: Claude Code
  loads `CLAUDE.md` itself), so a clean UserPromptSubmit is now correctly silent.
- On the owner's checkout, `CLAUDE.md` is modified locally. There the branch build's
  UserPromptSubmit emits
  `"unavailable":[{"path":"AGENTS.md","reason":"self-modified-authority","section":"governance"}]`,
  the existing TCP-V0-056 screen, now named.

Verification:

- `corvint affected` returned scope UNKNOWN, 158 units, LANGUAGE_FRONTIER=2. Focused tests ran:
  `internal/contextindex` whole (DIRECT_SOURCE_CHANGE) and the cmd/corvint governance and hook
  tests, all passing.
- The analyzer input pin was repinned with `script/analyzer-schema.sh bump`; the schema number
  and the output pin are unchanged.

Out of scope, reported to the orchestrator:

- `query`'s authority start still refuses a symlinked `AGENTS.md`.
- A `task_evidence` row for an explicit link path is still unavailable.
- The other slots do not follow links.

## Review fixes

- P2 deduplication bypassed dirty-link screening: FIXED. `governingRows` now records every link that
  folded into a row under TCP-V0-273 in `contextRow.links`, and `screenRow` screens all of them;
  `TestGoverningSymlinkLaterLinkModifiedStillScreens_TCPV0271` (`pkg/AGENTS.md -> ../CLAUDE.md`
  plus `AGENTS.md -> CLAUDE.md`, only the root link dirty) failed before the fix and passes after.
  TCP-V0-273 now states the rule.
- P2 link-only modification lost its evidence warning: FIXED. The downgrade and
  `withAuthorityWarnings` share `screenRow`, so they cannot disagree; the link-modified case of
  `TestGoverningSymlinkScreenedWhenTargetOrLinkModified_TCPV0271` now asserts the
  `self-modified-authority` warning and showed `warnings:[]` before the fix.
