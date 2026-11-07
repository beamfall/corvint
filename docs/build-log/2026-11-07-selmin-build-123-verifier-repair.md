# Selmin verifier repair 1: unsealed evidence and final binding

Requirements: ASD-V0-573, DOGFOOD-003, DCW-V0-2143.
Input head: 257e7e20746aa6e19338069106242be7e8d5c67c.
Base: e19823d919cfa653ee0ed05708d03f5fd12ca929.

The host verifier found one documentation gate failure: the generated, unsealed
`.corvint/change.cem.json` exceeds the 4096-byte line limit. Add that exact path
to `.corvint/agent-grep-exclude`, alongside the existing archived CEM glob, as
ASD-V0-573 requires for generated evidence. This keeps the long-line checker and
ordinary source searches consistent while leaving the CEM available to explicit
evidence reads. No blanket JSON exclusion, checker change, golden, wire contract,
or test assertion changes. The charter requires retaining the unsealed map.

The second failure is `dogfood-report-drift`: the report still names the commit
before the previous CEM sidecar commit. Refresh the final citation plan after this
repair commit and use the normal unsealed bind/check workflow. Do not edit report
identities manually. Retain any process-containment refusal and incomplete result
in the external handback; a cited map alone is not a passing final check.

Class: generated-evidence classification and stale coordination metadata, not
an analyzer soundness gap. No analyzer forms are added or removed. The exclusion
admits only the exact active CEM path plus the already declared archived CEM paths;
unlisted long-line files still fail under the existing gate. No new analysis
allowlist or denylist is introduced.

Verification: `make -s long-lines-check` PASS, exit 0. Its external log
`~/lanes/selmin-evidence/verifier1-long-lines.log` is 21 bytes, SHA-256
`21058b12bb1fac32446949e70e331322e85ef684592e76be1ca578d3c78e881c`.
The permitted `corvint affected --base` command succeeded; its full receipt stays
outside the repository. No Go package, spec, line citation or test changed in this
round, so no Go package test or spec gate was rerun. The host owns broad validation
and independent review. Final bind/check results belong in the external handback.

V1-5026 / AFP-V0-9009, V1-5029 / AFP-V0-9010 and V1-5030 / AFP-V0-9011 retain
their prior implementation and qualification evidence; this repair changes only
their shared evidence closeout. No Tasks completion write, publication or seal.
Rollback: revert this repair commit and rebind the resulting patch normally.
