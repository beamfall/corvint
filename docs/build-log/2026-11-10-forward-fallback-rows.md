# V1-5216: a no-plan FULL fallback selection is a complete forward row

AFP-V0-047 amended under the owner's standing lane auto-accept (2026-10-11); base
`7bf7ffe8de371f29d4a3e4b0c03857732e4c8d45`. The `.github/forward-rows/row.jq` change is on
branch `claude/forward-fallback-rows-5216`; these tests, the spec text and this entry are on
`claude/forward-fallback-rows-5216-docs`. The tests that pin the new class pass only with both.

PR 163 made the shadow `corvint test --dry-run` fail closed: when the affected graph cannot be
compiled it builds no plan and selects the declared whole Go suite (`scope` `FULL`, `reason`
`no plan: <code>: <message>`, the single unit `declared:go`). Its `selection.json` therefore had an
empty `planDigest` and, because `selection.jq` lists only `go:` units, an empty `selected`. The
row classifier required a `sha256:` plan digest on every shard, so each such run became a
`ci-forward-INCOMPLETE` row. Three of them in the latest 20 trip the AFP-V0-030 kill switch, and
`accept.sh` then decided `FULL` for every pull request.

Decision: `row.jq` admits an empty plan digest only for the recognisable fallback (scope `FULL`, a
string reason beginning `no plan: `, `planDigest` exactly empty). Every other shape check is
unchanged, so a null, malformed or empty plan digest on any other selection still makes the row
INCOMPLETE. The row is complete only when every selection shard is that fallback; a mix of
fallback and planned shards is INCOMPLETE. A complete fallback row takes S as every package with
an outcome (less any AFP-V0-099 narrowed-out package), so it is CLEAN or FLAKY and never a false
MISS. The miss rule and the AFP-V0-029/030 thresholds are unchanged.

Rerun double count: not reproduced. `bar.jq` keeps one row per pull request and tested commit
(the newest), so reruns of one pull request and commit count once. Several rows of one pull request
come from its head being tested against several bases, each a different merge commit, which
AFP-V0-047 counts as separate rows by design.

Evidence:

- `TestAFPV0047NoPlanFullFallbackIsComplete`: eight fallback shards give a CLEAN row over every
  outcome package; a failed queue package with a failed rerun is selected, not missed.
- `TestAFPV0047OtherPlanlessSelectionsStayIncomplete`: an empty plan digest on a NARROWED
  selection, a FULL selection with another reason or none, a null or malformed fallback plan
  digest, and a fallback scoped NARROWED all stay INCOMPLETE; seven fallback shards with one
  planned shard are INCOMPLETE with `selection shards mix a no-plan FULL fallback with a plan`.
- `TestAFPV0030RerunsOfOneRowCountOnce`: three uploads of one INCOMPLETE row plus another count
  as 2 and leave the switch clear; a third distinct INCOMPLETE row trips it.
- Without the `row.jq` change, the first test and the mixed case fail; the rest pass.

Not done: the three existing pr163 rows stay in the window until newer rows displace them.
Listing them in `infra-reclassified.txt` needs a recorded owner decision (AFP-V0-463). Hosted rows
are `NOT_OBSERVED`. The `.github` change needs the owner's `ci-control-plane` review.

Rollback: revert the `row.jq` commit; fallback runs then classify INCOMPLETE again.
