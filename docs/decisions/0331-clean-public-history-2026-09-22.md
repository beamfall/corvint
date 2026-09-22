# Decision 0331: clean public history with a private archive

- Date: 2026-09-22
- Owner: Russell Lewis
- Status: accepted publication direction; execution requires the checks below
- Authority: after being told that old license grants survive a history rewrite and that the
  repository is already public, the owner instructed retaining the history privately and
  publishing a clean snapshot.

## Decision

Prepare a parentless public root from the current public main tree, with the reviewed commercial
licensing policy in [decision 0330](0330-preserve-commercial-licensing-2026-09-22.md). This is a
new replacement plan under [decision 0002](0002-future-publication-transition.md), not reuse of
that decision's completed first-publication authorization. Preserve newer public source changes;
the older licensing branch is not the publication base.

Keep a private, verified mirror of published refs and objects, a local history bundle, working-file
backups, and release metadata and assets before replacing any public ref. Preserve existing local
worktrees. Replace public main only with a lease pinned to its observed object ID. Do not proceed
if the remote changes during preparation.

The existing prereleases, tags, and additional branches require a separate recorded owner choice
before withdrawal. Retaining one can retain old reachable history; resetting main alone must be
reported as such. This decision creates no replacement binary release, package, or announcement.
Preserve the exact reviewed license texts, path boundary, and third-party notices.

## Verification and limits

The candidate must contain exactly one commit with no parent and no imported predecessor Git
objects, refs, replacements, grafts, submodules, or object alternates. Compare every candidate path
against the recorded public base: only the reviewed licensing files, their legal-file digests,
the decision/index/build-log updates, and this publication plan may differ. Run the full source
gate on the clean, fixed candidate, retain its exact commit/tree receipt privately, and have an
independent reviewer inspect the candidate and replacement procedure. Verify the published tree
and main commit count from a fresh clone after the guarded push.

The Git-derived build count restarts at 1. The source version stays unchanged; this does not
requalify or republish any previous binary archive. A later binary release needs its own exact
revision qualification and version decision.

Historical documentation and sealed CEM records remain historical records. Their pre-snapshot
commit IDs can require the private archive and are not resolvable evidence in the new public clone.
Do not represent those records as validation of the new root. New indexes, repository-identity
bindings, change maps, and checks must use the new history. Existing branches must be rebased or
their changes reapplied onto that history before those branches are republished or merged into
replacement main; merging an old lineage would bring the old graph back.

The manual legal-policy decision was independently reviewed, but executable OCM closure refused
it with `invalid-requirements-section`. Keep that limitation visible: publishing the policy does
not invent executable proof of ownership, consent, or enforceability. The full source gate and
manual publication review remain separate checks; neither makes that OCM result pass.

Changing refs does not revoke licenses, alter copies already obtained, or guarantee removal from
GitHub pull-request references, caches, or third-party clones. The private archive preserves the
original rights and provenance record.

## Rollback

Before publication, leave the public repository untouched. If publication or post-push verification
fails, retain the complete private archive and exact remote-ref inventory for a guarded restoration;
never overwrite newer remote work. Withdrawn release assets retain their original bytes privately,
although republishing them cannot promise the same GitHub IDs or download history. Do not delete
the private archive after a successful reset.
