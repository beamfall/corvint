# Change flow: build-log, citation plan, bind

This is the order of the closing steps of a substantive Corvint change. `docs/DOGFOOD.md` is the
contract for each step; this page only fixes their order, so that the citation plan is drafted
against the final hunks.

1. Finish the code, tests, spec bullets and trace rows, and commit them on the task branch.
2. Write the build-log entry, `docs/build-log/YYYY-MM-DD-<slug>.md` (decision 0423), and commit it.
   Do this before step 3. A new file gets a new hunk ID each time it is edited, so a plan drafted
   before the entry was finished names a hunk the bind no longer has.
3. Draft the plan from the committed change:
   `corvint dogfood propose-citations $BASE > plan.tsv` (`DCW-V0-150`, `DCW-V0-350`). Review every
   row: a cite must be the BASE evidence the hunk relies on. Keep an `unknown` row when no BASE line
   is that evidence, including a hunk that only names a requirement this change adds
   (`DCW-V0-439`, `DCW-V0-440`).
4. Check the inputs with `make dogfood-inputs-check`, then bind and close out as `docs/DOGFOOD.md`
   §4 describes (`make dogfood-finish` or `make dogfood-closeout`).

If a file is edited after step 3, for example a build-log fix or an amend, the pre-flight refuses
the stale row as `citation-plan-map-mismatch`. The refusal names the row, the stale hunk ID and the
current one (`DCW-V0-441`). Do not hand-diff IDs. Run
`corvint dogfood propose-citations $BASE --refresh plan.tsv > plan.new`, review each
`re-keyed OLD -> NEW` note it prints, then rerun with `plan.new` (`DCW-V0-442`). Rows whose hunks
did not change are carried verbatim.
