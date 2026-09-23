# Decision 0357 — Freeze the minimum portable proof wire: `cem/0.2`, `ocm/0.1-experimental`, `frontier/0`

Date: 2026-09-22. Status: accepted (ticket V1-0013). Amends
`docs/specs/cem-0.2-canonical-binding.md` (`CEM-CB-025`), `docs/specs/ocm-v0-dogfood.md`
(`OCM-V0-015`) and `docs/specs/change-frontier-v0.md` (`CF-V0-034`). It changes no wire bytes and
no verifier outcome; it fixes the conformance packets that pin them.

## Context

Three portable wires carry the daily proof loop: the CEM (`cem/0.2`, with the `cem/0.1` N-1
reader), the obligation map (`ocm/0.1-experimental`, whose vectors decision 0343 already
freezes), and the change frontier (`frontier/0` and `frontier-error/0`). The CEM 0.2 packet in
`protocol/cem-0.2/` was labelled `candidate-not-frozen`. The OCM and frontier packets pinned
their vectors by producer re-derivation, but nothing pinned the committed bytes of the files
themselves or recorded which hostile repository states each profile covers. A reader could drift
from a frozen vector, or a vector could be rewritten in place, and only a byte comparison against
a regenerated oracle would notice.

## Decision

1. The minimum portable proof wire is frozen as of 2026-09-22: `cem/0.2` with its N-1 `cem/0.1`
   reader, `ocm/0.1-experimental`, and `frontier/0` with `frontier-error/0`. `cem/0.3` is not part
   of the frozen minimum and keeps its experimental status.
2. Each packet pins its vector and fixture files by SHA-256 in its manifest. The CEM 0.2 packet
   already does this (`files`, plus the manifest digest pinned in native and interop tests). The
   OCM and frontier manifests gain `artifactSha256`. The runner checks the exact file set and
   every digest before any case runs, and refuses on drift.
3. Each OCM and frontier manifest carries a `states` list covering, in this order: stable,
   relocated, stale, ambiguous, deleted and unknown. Each state names the cases that pin it. Where
   the real producers cannot reach a state, the frontier entry names the gap in place of a case.
   Every named case must exist. The CEM 0.2 packet covers the same six states through its drift
   cases.
4. A frozen vector's bytes and expected outcome never change. A wire change takes a new exact
   profile identifier with its own packet. Its reader must still read the frozen predecessor's
   vectors (N-1).
5. N-1 reading: the current CEM reader reads every `cem/0.1` `legacyMap` in the CEM 0.2 packet
   with the same acceptance, drift and unknown counts, and without canonical assurance
   (`CEM-CB-025`). The OCM and frontier profiles are the first frozen versions of their wires, so
   they have no own-profile N-1. Their upstream N-1 is `cem/0.1`. The OCM verifier reads it
   (`OCM-V0-006`). The frontier refuses it with `unsupported-frontier-context`.
6. Migration: there is no in-place rewrite of a map or vector. A holder of an older map
   re-prepares it under the current profile and re-cites. Unknown obligations stay unknown, and no
   digest is regenerated to hide a change.

## Consequences

- The freeze does not promote any profile, change intent status, or deliver V1-0010. The CEM 0.2
  daily-loop dependency on V1-0010 remains open.
- Frontier relocated evidence and inherited CEM `evidence-drift` cannot be reached through the
  real producers. `cite` refuses an unstable span (`CEM-CB-005`), and a moved cited span makes the
  evidence file an unbindable hunk. These are recorded as named gaps, not as vectors. The frontier
  deleted state and some of its ambiguous and unknown coverage rely on hand-authored fixtures that
  the runner skips by capability.
- Adding or changing a vector file now requires an intentional manifest digest update in the same
  change.

## Rollback

Set `protocol/cem-0.2/manifest.json` `status` back to `candidate-not-frozen`, and restore the
manifest digest pins in `protocol/cem-0.2/README.md`, `interop/cem01-go/manifest_test.go` and
`internal/cem/workflow/portable_test.go`. Remove the `states` and `artifactSha256` members, their
validators and tests from `conformance/ocm-v0` and `conformance/frontier-v0`. Delete the
`intent-scope-drift` fixture and its operators. Remove `CEM-CB-025`, `OCM-V0-015` and `CF-V0-034`
and the freeze paragraphs.
