## 2026-09-27 V1-0295 LCP-V0-003 LCP-V0-008: Stop names another session's active enrollment

Found by the pre-1.0 panel. A native hook evaluates only its own session key (LCP-V0-002). A session
that lost its key, for example after `/clear` in Claude Code, evaluates as `inactive` while the
worktree owner's enrollment is still active. Its Stop released with `local-policy-inactive` and no
output, so a false completion claim passed without any sign that the gate existed.

Decision: an inactive evaluation reads the worktree owner file and, when the owner is another key
whose saved lifecycle is `active`, adds that key as the evaluation's `owner`. A Stop with that owner
releases with the new reason `local-policy-other-session-active`, and the dogfood-event completion
carries the `owner`. The Codex and Claude Code adapters render a fixed `systemMessage` that this
session is not gated, followed by the keyed `dogfood status` argv for the owner; an owner that is
not 64 lowercase hex renders nothing. The release does not block: this key cannot satisfy or take
over another session's plan (LCP-V0-002), so blocking would loop. The qualified lifecycle's closed
completion now projects only `decision` and `reason`, so its strict decoders and the pi runner are
unchanged. `dogfood status` for such a key also shows `owner`.

Evidence: `TestInactiveKeyNamesActiveWorktreeOwner` covers no enrollment, an active owner, the owner
evaluating itself, and a cancelled owner. `TestDogfoodEventStopLifecycle` covers the reason, the
non-Stop case and a forged owner. The `LCP-V0-008 other session` subtest of
`TestDogfoodEventReadOnlyEnrolledStopAndPrompt` runs a real Stop event for a second key over an
enrolled worktree and checks the completion and the rendered notice. `TestQualifiedLifecycleStopComposition`
checks the qualified completion stays closed and passes the native wire validator.

Rollback: revert the change; such a Stop again releases silently as `local-policy-inactive`.
