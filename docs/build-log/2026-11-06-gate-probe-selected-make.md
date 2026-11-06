# Gate probe uses its selected Make executable

PR #107's portability review found that the isolated mandatory-Node fixture linked
`command -v make` but launched `/usr/bin/make`. The probe now launches its own
`bin/make` link. This preserves the fixture's restricted PATH and Node-absence
assertions while supporting hosts whose selected Make lives elsewhere. Gate-ledger
GL-V0-007 and the existing gate-receipt shell probe govern this mechanical repair;
no product or wire contract changes.

Focused evidence: `TMPDIR=/private/tmp sh -x script/gate-receipt_test.sh` passed on
macOS with ordinary PATH and with a scratch Make wrapper prepended to PATH. The
wrapper observed all five mandatory-fixture launches through the fixture link;
both ledger modes still rejected missing Node. Scratch evidence is retained under
`/tmp/corvint-4743-portability`. Rollback is reverting the one-line fixture change.

Corvint change-start, context, query, impact and affected reads were retained before
verification. Change-start reported prior-base agent receipts absent; separately
captured reads are not claimed as bound pre-change slots. The installed build88
reported stale-binary degradation for affected; impact was budgeted and its returned
revision did not match this worktree's captured base, so its current applicability
remains NOT_OBSERVED. Focused shell execution supplies behavioral evidence.
Linux execution, full gate, fresh whole Node suite and new Windows vet are NOT_RUN;
the repair changes no Go files. Prior broader checks remain historical evidence at
their original target. CEM/OCM structural closure does not prove test adequacy or
independent interoperability; unlinked requirements and missing observations stay
visible. Parent owns independent review, publication and native ticket completion.
