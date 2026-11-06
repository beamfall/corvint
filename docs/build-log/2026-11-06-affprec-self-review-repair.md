# Affected precision replay: reconcile the integrated measurement domain

Owner direction: 2026-11-06 self-review repair, V1-4497 / AFP-V0-9003.
Base: `c5379b4cf039fa8dd2df2aa4ee6e5b7b897cf514`.

The previous repair retained a failing all-unit comparison with the historical
86-unit report. The owner now explicitly requests reconciliation with the
acceptance contract while preserving required drivers and true-reader controls.
This supersedes the repair-2 decision to leave that integration conflict open.
AFP-V0-9003 requires reported counts, retained frontiers and a same-path true
reader; AZU-V0-001 additionally requires Make targets and CI jobs. Removing those
drivers or raising the historical language-unit budget would obscure the result.

Keep `languages.All()` and the original below-86 language-unit budget. Report the
full count alongside language, Make target and CI job counts. On this integrated
tree heavy-only selects 285 = 72 language + 185 Make + 28 CI units, retaining
`shell:foreign-invocation-unmodelled`. Test-only selects 274 = 61 + 185 + 28,
retaining Go build-constraint and shell foreign-invocation frontiers. The
declaration/spec replay selects 482 = 269 + 185 + 28, retaining shell
foreign-invocation and changed-foreign-callers frontiers. These are current-tree
observations, not a reproduction of the unavailable original 86-unit receipt.

The regression checks every runnable driver that names heavy.sh or carries a
computed-invocation frontier, and verifies that non-runnable pattern templates
are reached. Initial control attempts incorrectly treated pattern templates as
runnable selections, then used Go test-file presence as driver runnability; both
failed and were corrected using the existing explicit-target/job contract.
All same-path script, test, JSON and specification reader controls remain.
The full replay passes without changing selection behavior, uncertainty, wire
members, frozen outputs, goldens or true-reader assertions. The companion spec's
acceptance paragraph records the measurement domain; no requirement is weakened.

Focused and final whole-package commands, original logs, digests and the final
binding outcome are retained in `/home/russell/lanes/affprec.handback.md` and
`/home/russell/lanes/affprec-evidence/self-review-repair/`. Earlier failed
qualifications retain their original bindings. Independent review, native Tasks
completion and host qualification remain with orchestration. Rollback reverts
this test/acceptance clarification; no production migration is involved.
