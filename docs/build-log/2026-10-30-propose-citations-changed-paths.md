# propose-citations: no changed paths, no map refusal, added-file anchors

Date: 2026-10-30. Tickets: V1-3144, V1-3204, V1-3209. Requirements: `DCW-V0-393`, `DCW-V0-394`,
`DCW-V0-395` in `docs/specs/daily-change-evidence-workflow-v0.md`.

## V1-3144: a draft cites only untouched files

`docs/LANE-ORCHESTRATION.md` section 3 says a citation plan cites only files the diff does not
modify. The draft broke that rule in two ways. It ranked changed files last but still cited them.
It also fell back to a nearby line of the hunk's own file. Decision: the draft collects every path
`BASE..HEAD` adds, modifies or deletes and never cites one. That covers anchor candidates, the
sealed-contract row when `docs/DOGFOOD.md` itself changes, and the same-file fallback, which is
removed. A hunk with no anchor in an untouched file gets an `unknown` `insufficient-evidence` row
that names the missing anchor. The check's anchor tiers are unchanged. The `!p.basenames` guard
keeps the draft-only filter out of `unanchorableHunks`.

Trade-off accepted: drafts now carry more `unknown` rows. A row that cites the change's own text
is wrong evidence anyway, and the reviewer must replace it. An explicit unknown makes that work
visible instead of hiding it behind a plausible span.

## V1-3204: an unusable map no longer refuses

A missing map already drafted the `DCW-V0-350` hunk-ID template. Only a map that exists but is
unreadable, or that records another base, refused `map-unavailable` or `map-base-mismatch`. This
forced an extra `dogfood change` before the draft. Decision: both cases print a NOTE and draft the
same template from the canonical patch, labelled `map=UNREADABLE` or `map=OTHER_BASE`. Hunk-ID keys
are valid under any map the bind later prepares.

The bind regenerates an outdated map but does not replace an invalid one, so the unreadable NOTE
tells the lane to remove or restore that map first. Rejected: running a bounded `cem prepare` from
the draft. That would make a read-only command write `.corvint/change.cem.json`, and the
canonical patch already gives the same hunk IDs. Retained limit: the exact cause in the V1-3177
lane was not reproduced. The transcript shows only the refusal line. A stale binary or a leftover
invalid map are both possible, and both now draft.

## V1-3209: anchoring hunks of added files

A hunk in a file absent at `BASE` has no BASE line of its own. Options considered:

1. Anchor on the file's own HEAD text. Rejected: it is circular, since the change would author its
   own evidence. It violates `DCW-V0-393`, and evidence must be pinned to `BASE` (product
   invariant 1).
2. Exempt added files from the anchor tiers. Rejected: a change could move code into a new file to
   escape the gate.
3. Keep the existing rules. Chosen. An added-file hunk is anchored only by a `CEM-V1-081` BASE
   anchor in an untouched file: a line that names its path or a basename of at least 6 bytes, or
   that defines a requirement ID the hunk adds. Without one, it reaches the verdict gate through a
   cite row plus a passed runner receipt (`CEM-V1-606`, `CEM-V1-607`). The check's below-anchored
   NOTE already names that route.

The only change is the draft's unknown note for such a hunk. It now says the file is absent at
BASE and names the runner receipt. No gate, anchor or check behaviour changes, so no follow-up
ticket is needed.
