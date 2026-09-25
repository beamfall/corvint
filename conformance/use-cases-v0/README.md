# Corvint main use-case conformance V0

Run from the repository root:

```sh
go run ./conformance/use-cases-v0
```

Success means the closed ledger and every populated evidence envelope are internally valid and
content-addressed. It does not mean an `UNPROVEN` use case is delivered, nor independently establish
the truth or quality of a receipt subject. The canonical `/1` ledger has twenty-two rows.
The three narrowly scoped daily-workflow jobs (decision 0332) each hold one receipt from each of
the six evidence classes under `receipts/` (ticket V1-0011). Only `UC-TASK-ORIENTATION` is
`verified`/`VERIFIED`; ticket V1-0341 returned the other two to `experimental`/`UNPROVEN`, so
twenty-one rows are `UNPROVEN`. On a `verified` row the runner derives each dogfood `PASS` from
the retained tool packet or `corvint-dogfood-change/0` report, not from the attested token
(`UCV0-014`, proposed).
The reader still accepts the exact historical nineteen-row `/0` profile. Old readers reject `/1`;
do not rewrite archived `/0` evidence or add new IDs under that profile.

A contract receipt may pin a clause extract (`contract/clauses.json`, profile
`corvint-use-case-clauses/0`) instead of a whole spec file, so unrelated spec edits do not force a
repin (`UCV0-016`, proposed). If a cited clause changes, the validator reports
`clause-drift:<ID>`: re-read the clause, copy its current text into the extract, then repin the
extract digest in the receipt and the receipt digest in `ledger.json`.
