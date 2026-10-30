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

## Repinning receipts after a subject edit

Each receipt pins its subject files, and the ledger pins each receipt, by the SHA-256 of their exact
bytes (`UCV0-004`, `UCV0-005`). A change that edits a pinned subject, such as
`docs/specs/daily-change-evidence-workflow-v0.md` or `cmd/corvint/taskcontext.go`, fails this
runner with `subject-digest-mismatch`, and the CI `doc-gates` job fails with the list of receipts
to repin. Repin in the same change, after the subject edit is final:

```sh
script/repin-use-case-receipts.sh          # rewrite stale receipts and ledger digests, list each
go run ./conformance/use-cases-v0          # must print "valid":true
```

The script sets each changed subject's `sha256`, sets the receipt's `repositoryRevision` to
`git rev-parse HEAD`, and updates that receipt's `sha256` in `ledger.json`; no other byte changes.
`--check` lists stale receipts without writing. It refuses, writing nothing, when a subject or
receipt is missing, when a receipt's bytes already differ from its ledger pin (a hand edit needs
review, not a repin), or when an old digest is not unique in its file (a sealed benchmark seal).
Rebase or merge `origin/main` first when another merged change may have added a receipt, so the
repin covers every receipt the merged tree will contain.
