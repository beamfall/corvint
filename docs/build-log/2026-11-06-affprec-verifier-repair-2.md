# affprec verifier repair 2: frozen replay acceptance conflict

Owner direction: reproduce only the supplied affected-package failure against
base c5379b4cf039fa8dd2df2aa4ee6e5b7b897cf514; preserve existing assertions and
wire contracts. Reuse the pre-change capture and leave sealing to orchestration.

## Evidence and decision

At 5678e887101ef4148366cf9ce012f7d2e845e22d, a fresh current-tree observer's
`affected --worktree --path script/heavy.sh` selects 285 units: 72 language
units, 185 DRIVER_TARGET units and 28 CI_JOB units. The retained frontier is
shell:foreign-invocation-unmodelled. The raw receipt is outside the repository:
`/home/russell/lanes/affprec-evidence/verifier-repair-2/heavy.json`, SHA-256
`fe5720a4fd9e99f00ad23c383a7402d34f4c591baf83753dff235fcd6833c1cb`.
For example, Makefile's adopter-ci-advisory-test, agent-preflight-test and
ai-coauthor-trailers-check have PATH_LITERAL_READER witnesses for heavy.sh.

The integrated base's AZU-V0-001 requires Make/CI driver units and their literal
invocation edges; `languages.All` includes that claimant. AFP-V0-9003's replay
test counts all selected units and asserts fewer than 86 for heavy.sh. Even
removing every language unit would leave 213 required driver units. Narrowing
Go reader provenance cannot satisfy this assertion on the integrated graph.

V1-4497 / AFP-V0-9003 qualification remains PARTIAL due to this external
integration conflict. Do not filter the test's graph, change its counting domain,
raise its threshold, delete it, or remove required driver edges. Those choices
would weaken existing assertions or change behavior outside this repair's scope.
No production, test, specification, declaration or frozen output changes are
made in this round. There is no faulty lane production change whose reversal
would remove the required driver count. The owner/orchestrator must reconcile
the historical absolute threshold with the integrated driver's contract.

V1-4143 and V1-4535 (AFP-V0-9003/9005) retain prior measured precision and
true-reader evidence, but this live replay is still a failed qualification.
V1-4036 (AFP-V0-9000/9001), V1-3992 (AFP-V0-9002) and V1-4453 (AFP-V0-9004)
retain their preceding evidence with no new implementation claim. Whole-package
reproduction, final CEM outcome, retained missing evidence and exact commands
are recorded in the external handback. Structural dogfood acceptance cannot
turn a failed package into passing acceptance evidence.

Rollback removes this record only; the earlier repair's diagnostic conclusion
and all existing assertions remain. Native ticket writes, independent review,
publication, broad verification and sealing are reserved for orchestration.
