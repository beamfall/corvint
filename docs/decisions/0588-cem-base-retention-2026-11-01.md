# Decision 0588: committed CEM bases must be retained or labelled

- Date: 2026-11-01
- Status: accepted under the owner's standing lane auto-accept (2026-10-11); V1-3758
- Authority: the 2026-11-01 CEM format audit (V1-3758) and decisions 0331 and 0587.

## Context

A Git-referencing CEM is verifiable only while its base and target objects exist. After a branch is
deleted and Git collects garbage, a base no retained ref reaches is gone, and the map's verdict
becomes `UNSUPPORTED` / `repository-object-unavailable` (decision 0535). Measured on 2026-11-01 at
`8daec98ce4349c1583186692726d2c3a782ac647`: of the 111 maps under `.corvint/changes/`, 17 have a
base that `origin/main` does not reach, and 14 have a base that is not an ancestor of the target.

The 17 fall in three classes:

- 11 **archive-lineage** maps (`06cf00ac`, `3807936c`, `56acf444`, `57f4c2d9`, `65d29e1e`,
  `8668f77b`, `89561333`, `8d7b4418`, `e9eec841`, `ebda9950`, `ec75c04d`): base and target share
  no merge base with `origin/main`. They are the history-repository CEMs decision 0587 kept for the
  use-case receipts and the affected-plan replay tests; their commits belong to
  `beamfall/corvint-history` (final commit `1c4d8793cf71cbb01b8509b68a6cb1502a40a13e`). On this
  host they are reachable only from local branches.
- 4 **rebased-lane-base** maps (`39dce5a5`, `959798c4`, `c81de0ca`, `f6097dc2`): the target is in
  `origin/main`, but the base is a pre-rebase commit that no pushed ref holds.
- 2 **base-unavailable** maps (`0924835e`, `f4876787`): the base object is already absent here;
  both consumers answer `UNSUPPORTED` / `repository-object-unavailable` for them today.

Of the 14 maps whose base is not an ancestor of the target, 6 are the rebased-lane-base and
base-unavailable maps above; the other 8 (`24e42970`, `435bdf35`, `4b662635`, `5647ed06`,
`6e690536`, `6ec70197`, `75267705`, `d77eed5d`) have base and target both in `origin/main`, stay
verifiable, and need no label.

## Decision

1. Sealed maps are not re-anchored. Rewriting `baseRevision` changes the sealed bytes, so the map
   would no longer match the excluded-artifact sidecar of its own target and would become a
   different, unsealed claim.
2. Each of the 17 is labelled in `.corvint/cem-lineage.tsv` (`corvint-cem-lineage/1`: target,
   base, label, retained ref). An archive-lineage row names the archive commit as its retained
   ref; a rebased-lane-base or base-unavailable row names `none`. A label is a disclosure: it never
   makes a map verify, and an unavailable base still yields `UNSUPPORTED`, never an invented pass.
3. `make cem-base-retention-check` (`script/check-cem-base-retained.sh`, a `make gate` step) refuses
   a tracked map whose base no retained ref (default `refs/remotes/origin/main`) reaches unless the
   register labels exactly that base, and refuses a stale or mismatched register row or an unknown
   label. A new map bound by `make dogfood-change BASE=<origin/main sha>` passes by construction.
   Without a resolvable retained ref the check prints that it was skipped (`NOT_RUN`).

## Limits

- The archive-lineage rows' reachability from `1c4d8793` was not verified: the archive repository
  is not readable from the host that made this record. That verification needs archive access.
- The 4 rebased-lane bases and the 2 missing bases have no retained ref. Retaining the 4 needs a
  pushed archive ref holding them; the 2 missing bases are lost to this repository. Neither is
  invented here.

## Rollback

Delete `.corvint/cem-lineage.tsv`, the two scripts and their Makefile targets and gate steps. No map
changes.
