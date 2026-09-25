# Corvint main use-case conformance V0

Run from the repository root:

```sh
go run ./conformance/use-cases-v0
```

Success means the closed ledger and every populated evidence envelope are internally valid and
content-addressed. It does not mean an `UNPROVEN` use case is delivered, nor independently establish
the truth or quality of a receipt subject. The canonical `/1` ledger has twenty-two rows.
Nineteen are `UNPROVEN`. The other three are narrowly scoped daily-workflow jobs (decision 0332),
which are `verified`/`VERIFIED`, with one receipt from each of the six evidence classes under
`receipts/` (ticket V1-0011).
The reader still accepts the exact historical nineteen-row `/0` profile. Old readers reject `/1`;
do not rewrite archived `/0` evidence or add new IDs under that profile.

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
