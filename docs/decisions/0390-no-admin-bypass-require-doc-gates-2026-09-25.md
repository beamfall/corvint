# Decision 0390 — Require doc-gates and replace the admin bypass with a per-SHA admin status

Date: 2026-09-25. Status: accepted by the owner on 2026-09-25 ("require doc-gates and block admin
merges while checks run"). Amends decision 0320 step 2 and `AFP-V0-016`'s ruleset sentence; the
`ci-control-plane.yml` logic is unchanged.

## Context

V1-0268 found pull requests merged through the ruleset's admin bypass before their own
`go-product` check finished: PR #186 merged at 11:52:30Z, and its `go-product` run completed at
12:31:43Z with `failure`. Decision 0320 made the repository admin role the only bypass actor, in
pull-request mode, so a `.github/` change could merge by an explicit, logged bypass. The same
bypass also let any PR merge while its checks were still running. The `doc-gates` job in `ci.yml`
(PR #212, V1-0244) runs the documentation gate checks, but the ruleset does not require it.

A GitHub ruleset bypass skips every rule of the ruleset. It has no mode that allows a bypass
only after the required checks complete. So "block admin merges while checks run" cannot be
had while a bypass actor exists.

## Decision

1. The `main` ruleset requires a pull request and the `go-product`, `ci-control-plane` and
   `doc-gates` checks. It has no bypass actor.
2. `ci-control-plane.yml` still posts `failure` on the head of a PR that changes `.github/`.
   The owner consents to such a change by posting a `ci-control-plane` `success` commit status
   on the exact reviewed head SHA, as a repository admin:

   ```sh
   gh api -X POST repos/beamfall/corvint/statuses/<head sha> -f state=success \
     -f context=ci-control-plane -f description="admin reviewed .github change at <sha>"
   ```

   GitHub uses the latest status per context on a commit. So the consent binds to one SHA, and
   the status API logs its creator. `go-product` and `doc-gates` stay binding, so no merge can
   happen while those checks run. A later push to the PR gets a fresh `failure` from the
   workflow on the new head and needs fresh consent. If the workflow runs again on the same SHA
   after the consent (a CI rerun), its `failure` becomes the latest status, and the consent must
   be posted again.
3. The ruleset change is applied only after `doc-gates` is on `main` (PR #212). Before that, no
   PR could report `doc-gates`, and every PR would block.
4. The threat model of decision 0320 step 3 is unchanged: posting any status on the base
   repository needs write access. This decision narrows what an admin can do in one step: an
   admin can no longer merge a PR whose `go-product` or `doc-gates` check is pending or failed.

## Rollback

Restore the bypass actor (`RepositoryRole` 5, `pull_request` mode) and drop `doc-gates` from the
required checks through the ruleset API. Revert the `AFP-V0-016` text and the
`ci-control-plane.yml` comment and description to the decision 0320 wording. No source, pin, or
other gate changes.
