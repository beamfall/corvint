# 2026-10-16 V1-0493: dogfood enrollment status after the evidence seal

Lane V1-0493 (spike). Base `b1ee552b3bebeaacf8f54e62f7f67572f231848e` (origin/main). This
entry records evidence and a semantics reading only. Behavior is unchanged, and no requirement
is added or amended.

## Question

An enrolled local-completion workflow (`docs/specs/local-completion-policy-v0.md`) finishes
satisfied on the bind commit. `docs/DOGFOOD.md` §4 then requires the seal, a rename-only commit
that moves `.corvint/change.cem.json` to `.corvint/changes/<bind>.cem.json`. After the seal,
`finish` reports `evidence-bindings-required` and `public-evidence-command-failed`. The question
is which state the enrollment is in after an accepted seal that leaves source proof unchanged.

## Reproduction

The scratch clone was branched from BASE with a scratch identity. The `corvint` binary was built
from that clone. The plan froze BASE, one intent (`docs/specs/local-completion-policy-v0.md`) and
one check `noop` with argv `/usr/bin/true`, timeout 60 s and `allowCemSidecarOnlyReuse: false`.
The plan digest is `2ebc7a89…`. The scratch implementation commit adds one build-log file. The
bind commit is `9f98b0e1`, and the seal commit `09a25a08` is
`R100 .corvint/change.cem.json -> .corvint/changes/9f98b0e1….cem.json`. The seal ran the final
check (`dogfood-check: PASS`) and then printed `dogfood-seal: PASS` (load 63-65, 2 min 29 s). The
enrolled `finish` that reached satisfied took 12 minutes at load about 60.

The evaluations below are shown separately. Fields are verbatim from each envelope's `policy`.

| Step | HEAD | lifecycle | satisfied | unmet | next |
| --- | --- | --- | --- | --- | --- |
| `begin` | BASE | active | false | `selected-check-unverified:noop`, `reports-not-produced`, `final-check-required` | verify |
| `verify` | bind | active | false | `reports-not-produced`, `final-check-required` | finish |
| `finish` | bind | active | false | `review-required`, `final-check-required` | review |
| `review` | bind | active | false | `final-check-required` | finish |
| `finish` | bind | satisfied | true | none | `noActionReason: workflow-satisfied` |
| pre-seal `status` | bind | satisfied | true | none | `noActionReason: workflow-satisfied` |
| post-seal `status` | seal | satisfied | false | `selected-check-unverified:noop`, `report-set-stale` | verify |
| post-seal `finish` | seal | satisfied | false | same as status, exit 1 | verify |
| post-seal `verify` (the offered action) | seal | active | false | `report-set-stale`, `final-check-required` | finish |
| then `finish` | seal | active | false | `report-set-stale`, `final-check-required`, `evidence-bindings-required`; error `public-evidence-command-failed` from `ocm-prepare` | finish, status |

The pre-seal report-set digest is `61853113…492c`. After the seal the report set is stale and is
not shown. Successor enrollments after the seal are refused. Another session's `begin` returns
`worktree-prior-completion-stale`, and the same session's `begin` of a different plan returns
`prior-completion-stale`. After the post-seal `verify`, another session's `begin` returns
`worktree-already-enrolled`.

The ticket's failure is reproduced exactly. It is reached by following the `verify` action that
post-seal `status` offers.

## Semantics under the accepted contract

- Satisfaction is a read-only evaluation of current bindings. `LCP-V0-003` derives unmet
  conditions from the current repository. `LCP-V0-006` has status and finish revalidate current
  bindings. `LCP-V0-007` rechecks target and artifact identities through publication.
  `docs/DOGFOOD.md` says status becomes satisfied only while the bindings "remain current".
- A check binds its exact tested commit (`LCP-V0-004`). The only exception is the opt-in CEM-only
  sidecar reuse. By code reading, not run: that reuse reads `.corvint/change.cem.json`
  (`sidecar_reuse.go`), which the seal removes, so it cannot cover a seal. `finish` runs its
  public CEM/OCM commands against that same fixed path. `dogfood check` refuses a sealed `HEAD`
  (`sealed-head`, `docs/DOGFOOD.md` §4).
- So no enrollment can evaluate satisfied on a sealed `HEAD`. Post-seal stale is the result the
  contract requires, not a defect. The completion record is the satisfied evaluation on the bind
  commit, whose persisted terminal binds that target. The seal is the later transport step
  (`DCW-V0-013`). The immutable check binding is intact: no observation from the bind commit is
  reused for the seal commit. An explicit post-seal `verify` does run the check on the seal
  commit and records a new observation for it (the table above), but `finish` still cannot reach
  satisfied there.

## Friction that the contract does not settle (owner decision)

1. After the seal, `status` is neither satisfied nor actionable, but it offers `verify`
   (`LCP-V0-020` defines no action set for a persisted-satisfied but stale evaluation). Following
   it clears the terminal record (`Verify` sets `active`) and leads to a `finish` that cannot
   succeed. The worktree is then held as `worktree-already-enrolled` until an explicit `cancel`,
   which records non-success over a completion that did succeed.
2. While `HEAD` stays on the seal commit, a successor enrollment in that worktree is refused as
   stale, and only `cancel` or a fresh worktree releases it. Before any post-seal `verify`,
   checking out the bind commit again should restore current satisfaction if the retained
   artifacts are unchanged, and `begin` would then admit a successor (code reading of `Begin`,
   not run). After a post-seal `verify` clears the terminal, only `cancel` releases the
   worktree. Lanes that use one clone per change are not affected.

Removing either point means amending accepted requirements: `LCP-V0-002` (re-enrollment after
"current satisfaction"), `LCP-V0-003` (next actions) and `LCP-V0-020` (no-action reasons). The
owner's 2026-10-11 standing auto-accept covers new lane requirements, not amendments, so this
lane stops here. The options for the owner:

- (a) Keep behavior and document that the enrolled workflow ends at the bind commit, and that a
  sealed worktree needs a fresh worktree for the next enrollment.
- (b) Recognize a sealed terminal. When the only change from the terminal target is the
  rename-only seal of its own bound CEM (the same blob, at `.corvint/changes/<terminal
  target>.cem.json`), status would report a fixed no-action reason such as `workflow-sealed`
  with no continuation, and `begin` would treat that state as current for a successor. Checks
  and reports stay bound to the bind commit.

Recommendation: close V1-0493 as answered, and file the choice between (a) and (b) as an owner
question.

## Review

One Codex round (read-only) on the diff from BASE. Two P3 findings, both fixed in this entry: the
binding paragraph now separates refused reuse from an explicit post-seal re-verification, and the
recovery paragraph is qualified to a sealed `HEAD` and to before or after a post-seal `verify`.
