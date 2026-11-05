# Docperf repair: fixture identity isolation

The final-round verifier reported frozen default-context failures at lane head
`a3126de1c7c6a938bf133fd36023c13121b8e175`, against base
`39c03198086681fb24f509fbd00b34696d3268c4`. The repair changes the external
lane test environment and records the diagnosis; it changes no product source,
fixture, golden, assertion, wire, error, or requirement.

## Finding and decision

`git diff BASE..HEAD -- cmd/corvint` is empty. The fixture helper in
`cmd/corvint/context_summary_test.go` inherits `os.Environ()` and pins its dates.
Its `git -c user.name=t -c user.email=t@x commit` does not override explicit
`GIT_AUTHOR_NAME`, `GIT_AUTHOR_EMAIL`, `GIT_COMMITTER_NAME`, or
`GIT_COMMITTER_EMAIL`. The prior external `closeout/run.env` exported the lane
commit identity for every command, including tests. Git honors those variables
ahead of the fixture's user configuration.

Reconstructing the eight committed fixture files yields the unchanged tree
`348e320a5dbd6ab7b3151d51db4f6c3b576cd855`. With the fixture's identity and dates,
its commit is the expected `9ab69ef47d196e1a757a3de3319527eb3686ff8b`.
With the exported Russell Lewis identity and the same tree, dates and message,
its commit is exactly the reported `05d3142624a106eafdac3a4f644b6f44eacaf3eb`.
The failed default packet equals the golden after substituting only that commit
field. Running the two reported tests with the old identity environment reproduces
both failures (exit 1); clearing those four variables makes both tests pass
(exit 0) at the original implementation head.

Decision: isolate seal identity to lane commits and CEM binding, and clear it
for test processes. The external prior `closeout/run.env` now uses the mandated
`/home/russell/lanes/docperf.tmp` (0700) and unsets the four identity variables;
its previous bytes remain in `run.env.before-repair`. The fresh repair `run.env`
and `run.sh` retain the exact corrected test invocation. All lane commits and
CEM binds still export Russell Lewis <me@russe11.com>. No implementation is
withdrawn: the reported identity mismatch does not require a default-path gate
or product rollback. No new spec or requirement is introduced.

## Verification and retained limits

The current-tree binary's `affected --base BASE` selects all three changed
packages at tier 0 and the requested CLI package at tier 1. It retains 108
selected units, 514 exclusions and two LANGUAGE_FRONTIER unknowns:
`go:build-constraint-variants` and `shell:foreign-invocation-unmodelled`.
The installed binary's earlier plan reports `self-binary-stale`; the repeated
current-tree plan has no degradation. The context route was actually used and
its packet retained. Query/impact and ranking/learning evaluations are not
applicable to this environment-only repair; earlier lane limits remain visible.

The required `Freeze|GPKV0002|Golden|Context` CLI selector runs with identity
cleared. Both reported goldens pass. The command exits 1 because the existing
`TestWorkFinalCheckClosingContext/WQO-V0-032/{cancelled,deadline}` refuses its
private scratch parent: `SOURCE_UNQUALIFIED: CORVINT_WORK_RUN_PARENT has an
ancestor writable by other users`. `namei -l` identifies the shared
`/home/russell/lanes` ancestor as mode 0775. The mandated TMPDIR's own mode is
0700. The existing security check is unchanged; shared directory permissions
are outside this repair. The host verifier must rerun this selector in an
admitted ancestor environment, keeping the four identity variables unset.

Independent review remains orchestrator-owned. No model-backed reviewer,
Tasks write, push, PR, background job, or seal is run. Earlier performance
qualification gaps and ticket dispositions remain unchanged; these correctness
checks do not qualify quiet-host performance or promote the failed pack format.
Raw commands, logs and fixture reproduction are outside the repository under
`/home/russell/lanes/docperf-evidence/repair/`. The repair handback retains their
return codes, digests, whole-package results and final CEM/check outcome.
