# Citation prepare run-store integration repair

Self-review found that the generic environment-equality predicate always refused
normal prepare/citation runs: change.run adds a private stable-reuse directory to
the child environment. V1-4851's per-row subprocess elimination was unreachable
under that context, even with the C locale. This repairs DCW-V0-8760 without
changing CEM wire formats, refusals, exit codes, probe semantics or authority.

## Decision and closed admission class

Pass the actual invocation child environment explicitly to the inline citation
handler. Admit only resolved same-file executable identity, an absolute root,
and the existing generated cite (lines/bytes, optional probe output) or unknown
mark argv forms. For those forms, the only understood environment differences
are LC_ALL=C and CORVINT_OCM_STABLE_REUSE_DIR. The fresh workflow session's Cite
and Mark do not read OCM stable verdicts; gitauth's scrubbedEnv already forces
LANG=C/LC_ALL=C and excludes the reuse directory. These two settings therefore
need no process-global mutation. Every other effective environment entry must
match, including presence; unknown additions, replacements or removals fall back
to Exec. No cache or general-purpose environment emulation is introduced.

This supersedes the all-settings-equal admission decision in the earlier
independent-review entry. The generic exact-equality helper and its assertions
remain unchanged. Rollback restores dogfoodflow.Exec at the runner factory.

## Evidence

TestDogfoodInProcessCitationsInputsCheckRunStore enters the actual inputs-check
flow with four hunks. It asserts a real run-owned store on every probe, counts
external runner calls, and compares four citation children in the external arm
with zero in the inline arm (one prepare child remains). It also asserts the
original HEAD, a clean worktree and unchanged caller environment. Existing byte
parity, refusal and probe assertions remain intact. The unknown-form regression
is supplemented by unknown environment replacement/removal refusal, closing the
environment-transformation class rather than ignoring every difference.

Whole package results and command/evidence digests are retained in the lane
handback. No wall-time speedup is inferred from subprocess counts. Independent
review and host-only qualification remain with the orchestrator.
