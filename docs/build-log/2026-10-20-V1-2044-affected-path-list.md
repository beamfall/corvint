# V1-2044: `corvint affected --path` plans a listed change at HEAD (GitHub #1301)

Date: 2026-10-20. Ticket: V1-2044. Base: origin/main da15cd7dd.

## Decision

Issue #1301 offered two options: a path-list input to `affected`, or a closure-depth member on
`impact` receipts. This change takes the first:

- `affected` already walks reverse dependencies transitively.
- It already has the scope and unknown vocabulary for a plan it cannot close.
- Adding an optional member to `affected` is compatible under CCF-V1-006.

The `impact` receipt is pinned byte for byte by `conformance/cli-parity-v0/manifest.json`, so
any new member there is a breaking change that needs its own decision. Instead, the `impact` help
and `docs/CAPABILITIES.md` now state:

- reverse imports are one hop;
- a `READY` receipt is not a closed test selection;
- `affected --path` plans one.

`--path` requires a clean worktree. The graph is built from the worktree, so this is the only way
the plan describes HEAD without copying every blob into scratch as `--snapshot` does. It refuses
with the existing `unsupported-affected-mixed-worktree` code.

Requirements `AFP-V0-094..096` were added to `docs/specs/affected-plan-v0.md`. They are accepted
under the owner's standing auto-accept of lane requirements (2026-10-11). The numbering leaves a
gap above `AFP-V0-088`, which open PR #1314 uses.

## Evidence

- `TestAFPV0094PathListSelectsTwoHopImportersAtHead`:
  - over the fixture `core <- mid <- leaf` with `mid`'s tests removed, `--path core/core.go`
    selects `leaf` with witness `core, mid, leaf`;
  - for one committed edit, the `plan` member is byte-identical to the `--base` plan, in both the
    compact and `--full` forms.
- `TestAFPV0095PathListKeepsUnknownScope`: an unowned listed path keeps `UNKNOWN`.
- `TestAFPV0094PathListRefusals` and `TestAFPV0096HelpStatesImpactClosureDepth` cover the
  refusals and the help text.

## Not done

- No machine-readable closure-depth member on `impact` receipts. That needs a CCF-V1-006
  divergence decision.
- No `--path` for the Playwright, provider or snapshot profiles.

## Review fixes (Codex, PR #1316)

- An ignored file HEAD does not track was not dirty but was read by the disk graph, so `--path`
  could report `BOUNDED` at HEAD over it. `--path` now refuses `unsupported-affected-mixed-worktree`
  when the list names, or any graph unit source or test is, an ignored untracked path
  (`affected.IgnoredPaths`).
- `--path --full` consumed the next option as its value. The separated form now refuses a value
  that begins with `-`; `--path=VALUE` still admits such a name.
- Each fix has a regression test that failed before it.
