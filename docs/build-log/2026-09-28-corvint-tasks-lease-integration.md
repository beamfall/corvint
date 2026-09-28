# Agent lease integration and CI fixture identity

The owner requested landing the reviewed agent lease stack and qualified CAL-V0-026 work.
Integration base `b4028d4a17f2fb7fae9061d341fa98d6df317937` combines current public main
`cec1edbb6995a022dd7e2d336a66eb42c1fdc3c6`, the S7/S8 stack and qualified CAL26 head
`d00de368be2893e4dba38aff960ee79727690d9c`. All prior commits and sealed evidence are preserved.
The Tasks and context-index source at that integration base is byte-identical to the qualified head.

## CI failure and repair

The existing S5, S6 and S1 CI runs failed in `TestCALV0017_CompletionRefusals` because its
`git commit-tree` command lacked an explicit identity. The test disabled global/system Git config
but depended on the host inferring an author. The other fixture commits already supplied their
identity. The repair gives this command the same per-command name and email. It changes no
production behavior, requirement or qualification configuration.

The failure reproduced before the repair with author/committer/email environment variables unset
and `user.useConfigOnly=true`. With the repair, all `TestCALV0017_` store tests passed under that
same environment using Go 1.27.1, `-count=1` and `-timeout 30m`. Independent read-only review passed:
the dangling commit remains unreachable and the refusal/state-integrity assertions are unchanged.
Reverting the one test-line change restores the previous fixture behavior.

## Evidence and remaining boundaries

The frozen CAL26 measurement and its host/configuration limits remain in
`2026-09-28-corvint-tasks-lease-lock-qualification.md`: MET with `GOMAXPROCS=2`, claim p95
121.136 ms and renew p95 104.774 ms. It was not rerun or attributed to this integration commit.
The repository's required GitHub checks will validate the final combined source before merging.

The initial no-diff evidence pass retained `git-diff-failed`, `cem-map-not-produced`, OCM
`exit-2`, `intent-scope-drift` and `map-unavailable`; prechange query/impact agent receipts were
NOT_OBSERVED. The direct task-context packet identified the requirement and exact failing test;
the affected plan and focused reproduction are retained outside the checkout. The final CEM binds
this fixture repair and integration record against the integration base.

CAL-V0-018's remaining import work, non-fixture release support and standalone Tasks source-archive
packaging remain separate recorded tickets V1-0436, V1-0449 and V1-0456. This merge does not mark
those requirements or the whole lease spec complete.
