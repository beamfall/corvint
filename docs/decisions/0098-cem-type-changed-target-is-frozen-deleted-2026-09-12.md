# Decision 0098 — a same-path type change is `deleted` in frozen cem/0.1 drift

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls
and record them (orchestration thread, 2026-09-12).

`internal/cem/verify/verify.go` `driftItem` reports `deleted` when the evidence path still exists at
the target but its entry, with a different OID, is a symlink, directory, or gitlink. The backlog
asked whether that asserts a removal the lookup disproved, and a held branch (`434e5157`,
`wave4-cemtype-20260912`) added a sixth status, `type-changed`. A companion spec gap asked whether an
identical-OID symlink may be `stable`.

The owner call: keep the five frozen statuses and the current Go behavior, and make the reading
spec-owned as `CEM-PILOT-019`. Branch `434e5157` is not merged.

- `type-changed` is a drift-rule change. `interop/cem-0.1/ALGORITHMS.md:149-153@a4df8285` closes same-path
  drift to `stable`, `relocated`, `stale`, `ambiguous`, and `deleted`; `interop/cem-0.1/runner.py`
  and `tools/cem-interop-runner/packet.go:283@f6458964` reject any other value; `CEM-CB-003` keeps
  `cem/0.2` drift byte-compatible with `cem/0.1`; and `interop/cem-0.1/IMPLEMENTATIONS.md` says a
  changed drift rule creates a new profile rather than rewriting 0.1 evidence. A versioned extension
  would need a new profile and its own vectors for a case that changes no acceptance outcome.
- The frozen text is silent on the target entry kind. It names a missing path and an identical blob
  OID, then searches "the entire target blob". The base rule (`ALGORITHMS.md:123-124`) admits only
  regular-file blobs as evidence content. The owner reading: OID identity is decided first and is
  `stable` for any mode (content identity needs no search, and both Go implementations already agree,
  `TestDriftSymlinkSameOIDIsStable` and `interop/cem01-go` `TestTargetSymlinkWithIdenticalBlobIsStable`);
  a changed entry is searched only when it is a regular-file blob, and otherwise has no file content
  and is `deleted`. Searching a symlink's link text would let a link such as `../evidence` report
  `relocated` and accept evidence that is not a file, the invented certainty invariant 2 forbids.
- No reason member is added. There is no drift-row reason field, and a new member is the wire
  addition decision 0091 removed under `GPK-V0-003`. The limitation is documented instead in the
  `cem-pilot-kit.md` failure and trust model: `deleted` does not assert removal.
- No interop artifact changes. Every `interop/cem-0.1` manifest vector, `ALGORITHMS.md`, and the
  runners stay byte-identical; `cem verify` output is unchanged for every input.

Known gap: the Corvint-authored probe `interop/cem01-go/cem.go:1031@0624fff8` skips only non-blob object types,
so it searches a changed symlink and can differ from Corvint there. No vector exercises it, and
`CEM-GO-001` limits the probe to the public kit, which this decision does not amend. The errata
sentence, a drift vector, and the probe repair are filed together in `docs/agent-memory/ideas.md`.
The Python oracle, which raised `unsupported-tree-mode` here, was retired by decision 0088, so no
`GPK-V0-033` parity case is open.

Rollback: revert this decision's commit, which removes `CEM-PILOT-019`, its trust-model sentence and
test, and restores the two backlog entries; no production code or wire bytes change.

## Amendment 2026-09-12 (same day): the kit states the rule as erratum 1, and the probe follows it

The known gap above is closed in the public kit rather than left as a Corvint-only reading. Call: the
target-kind sentence is a clarification of existing cem/0.1 semantics, not a behaviour change, so it
lands as a kit erratum and creates no `cem/0.2` profile.

- It is a clarification. The base rule (`interop/cem-0.1/ALGORITHMS.md:123-124@1c0515d4`) admits only
  `100644`/`100755` blobs as evidence content and drift compares that content; Corvint already reports
  `deleted` (`CEM-PILOT-019`); the retired Python oracle also refused to search, raising
  `unsupported-tree-mode`; and no prior vector's fixture bytes or expected record change. The only
  reader it moves is the Corvint-authored probe, which could accept by searching link text, the
  outcome the base rule excludes.
- It may land in 0.1. `interop/cem-0.1/IMPLEMENTATIONS.md` freezes the manifest once an external
  implementation starts, and none has (P1, C1, C2 unclaimed). The erratum is recorded there with the
  old and new manifest SHA-256, so 0.1 evidence is not rewritten silently.
- Set aside: a `cem/0.2` profile, which would duplicate the whole suite for one clarifying sentence;
  an out-of-manifest errata vector, which neither runner would execute; and leaving the historical
  `runner.py` pinned to the old digest, which would ship a kit runner that refuses its own manifest.
  `CEM-EXT`'s immutable-bytes clause is amended for this erratum: `runner.py` changes only its
  manifest digest, artifact count, and matrix pins, and receipts made before the erratum describe
  only the earlier packet.

This supersedes the "No interop artifact changes" bullet. Changes: the sentence in `ALGORITHMS.md`
same-path drift; drift vector `deleted-symlink-changed` (`targets/symlink-changed.patch` turns
`docs/rule.txt` into a symlink whose link text is exactly the evidence span, so a probe that searches
it reports `relocated`); manifest SHA-256 `2655258e…` with 51 artifacts and a 7/19/6 matrix, pinned in
`tools/cem-interop-runner`, `interop/cem-0.1/runner.py`, `observation.schema.json`, `CEM-EXT-002..004`/`008`, and `CEM-GO-004`;
and `interop/cem01-go` `computeDrift` searching only regular-file blobs. Existing vector files are
byte-identical; the manifest gains one drift row, one digest entry, and the two JSON separators they
need.

Rollback: revert the amendment commit; the kit, runner pins, schema, specs, and probe return to the
7/19/5 packet with manifest SHA-256 `324f1588…`.
