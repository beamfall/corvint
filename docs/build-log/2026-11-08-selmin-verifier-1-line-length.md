# Selmin verifier repair 1: traceability row length

The host verifier at a188e5a2488a37bf2f1a90e4e5b86bd55dd16754 rejected the
4,264-byte AFP-V0-9010 acceptance row in affected-plan-selection-v0.md.
Split its test witnesses across two rows under the same requirement, preserving
all implementation references, tests and explanations. No requirement, runtime
behavior, frozen output, test assertion or checker exception changes.

This finding is documentation layout, not an analysis soundness gap: no new
classifier forms or allowlist changes are needed. V1-5029 / AFP-V0-9010 owns the
row; V1-5026 / AFP-V0-9009 and V1-5030 / AFP-V0-9011 are unchanged. No Go package
is touched in this repair, so the earlier package qualifications remain historical.

The other host failure is dogfood-report-drift. The prior report is incomplete:
OCM preparation refuses unsupported-process-containment. Rebind the final diff
once, commit the CEM, refresh its clean binding and run dogfood-check. Preserve
any remaining refusal as an external qualification blocker, without changing the
verifier or claiming a passing outcome. The final commands and evidence live in
/home/agent/lanes/selmin.handback.md and selmin-evidence/ outside the repository.

Validation: long-lines-check and make -s spec-requirements-check
line-citations-check passed (exit 0). The latter ran internal/specindex as a whole
package and the required internal/lrfrepo subset through script/heavy.sh, each
exit 0. No package source or test was changed. Repinning the staged index found
no moved citation. The affected plan was UNKNOWN (520 selected, 71 language
frontiers); its installed binary reported 14 differing source paths. The owner delegates whole-repository checks and independent
review to the host, forbids publication and Tasks writes, and requires stopping
without a seal. Rollback is a new revert of this formatting change.
