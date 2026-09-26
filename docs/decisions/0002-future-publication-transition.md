# Decision 0002: future public publication transition

- Status: accepted and executed (2026-08-29)
- Owner: Russell Lewis
- Date: 2026-08-26

## Context

Before this decision, the private Corvint checkout was governed by a source-available product licence
with plain Apache-2.0 for the interoperability layer. The owner has directed that the
public `beamfall/corvint` publication use AGPL terms for the product, retain Apache-2.0 for protocol,
specification, conformance, examples, and interoperability paths, and expose no predecessor history.

## Decision

The public repository is created as exactly one parentless root commit from the final approved tree.
Product paths are AGPL-3.0; the explicitly enumerated protocol/spec/conformance/interop paths remain
Apache-2.0. The root commit has no parent, its subject/body and reachable history contain no
predecessor-project name, and no prior private commit graph is pushed.

The Apache-2.0 boundary is exactly:

- `protocol/**` for future vendor-neutral protocol/spec/schema material;
- `conformance/**`;
- `interop/**`;
- `examples/**`;
- `docs/CHANGE-EVIDENCE-MAP.md`;
- `docs/CEM-CI.md`;
- `docs/cem-0.1.schema.json`;
- `docs/cem-0.2.schema.json`;
- `docs/lrf-0.schema.json`;
- `docs/tcq-0.schema.json`.

Every other path is AGPL-3.0 unless a file has a separately owner-approved notice. A future Apache
protocol/spec file must enter `protocol/**`; adding an exception requires an explicit amendment to
this decision before publication.

The predecessor-name denylist and any sensitive provenance mapping remain controlled owner inputs;
they are checked without being embedded into the public tree or commit message. Public provenance
describes the new root and current authored artifacts without importing private predecessor history.

## Transition gate

The transition is forbidden until all of the following are simultaneously true:

1. the accepted analyzer-capability registry, lock, protocol, containment, exact tuple, and rollback
   gates pass, and every required native/plugin lane has a fresh exact `P0=0/P1=0/P2=0` review and is
   integrated;
2. bounded full Beamfall polyglot shadow dogfood, final exact readiness audit, and one serialized full
   gate pass with retained receipts;
3. the final tree contains the owner-approved AGPL product text, Apache path boundary, notices,
   dependency licenses, provenance, and generated-artifact policy, each independently reviewed;
4. a locally constructed candidate has exactly one root and one commit, no parent header, no hidden
   refs/replace/grafts/submodules importing old history, and byte-for-byte matches the approved tree;
5. the controlled predecessor-name scan passes across commit subject/body, refs, reachable objects,
   release metadata, and generated provenance;
6. the destination remote, branch protection, and exact candidate object ID are re-verified
   immediately before the authorized push.

The push creates no tag, release, package, image, announcement, or deployment. Any release remains a
separate outward action.

## Rollback

Before push, discard the candidate root and keep the private checkout/remote unchanged. After push
but before any release, correction uses an explicit owner-approved replacement/publication plan; the
public history is never silently rewritten by this decision.

## Execution record (2026-08-29)

The owner directed execution. `LICENSE` now carries AGPL-3.0-or-later, `LICENSE-APACHE-2.0` carries
the Apache text for the enumerated interoperability boundary, and `LICENSING.md`, `PROVENANCE.md`,
`README.md`, `AGENTS.md`, `ROADMAP.md`, the packaging metadata, and the release-artifact checker
were updated to match. The public repository was rebuilt as one parentless root commit from this
tree.

Gate items 1 and 2 (analyzer-capability lanes, bounded polyglot shadow dogfood, serialized full gate
with retained receipts) were **not** satisfied at execution time. The owner waived them explicitly,
having found the repository already public under the superseded terms; correcting the published
licence took precedence over the readiness sequence. Items 3-6 were satisfied. No tag, release,
package, image, announcement, or deployment accompanied the correction.

## Amendment (2026-09-26): frontier file exception

The owner approved one file-specific exception (decision 0422, answer A5; V1-0379). The files
`cmd/corvint/frontier.go`, `cmd/corvint/frontier_adapters.go` and `cmd/corvint/frontier_test.go` keep
their Apache-2.0 notices. They are the only Corvint source files outside the boundary above that
carry a separately owner-approved notice, and `LICENSING.md` names them.

The files were already public with these notices when the exception was approved. This amendment
records that approval after publication; it changes no notice and relicenses nothing. The boundary
list above is unchanged, and it is not extended to `cmd/**`.

Rollback: revert this amendment and the `LICENSING.md` section together. Rights already granted
under Apache-2.0 stay granted.
