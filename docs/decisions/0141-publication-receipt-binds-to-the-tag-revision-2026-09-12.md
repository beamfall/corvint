# Decision 0141 — the publication receipt binds to the tagged revision, not HEAD

Date: 2026-09-12. Status: accepted. Authority: repository owner, verbatim instruction "I want you to
make the calls for me" (delegated owner call).

## The conflict

Two accepted statements disagree about which revision a publication witness is bound to.

- `ARTIFACT-RDY-V0-001` (`docs/specs/release-artifact-integrity-v0.md`): go-archive, tag,
  publication and promotion `PASS` "require a current-revision machine-readable witness", where the
  current revision is `HEAD`.
- `docs/decisions/0109-alpha-publication-set-receipt-and-readiness-2026-09-12.md` item 2: the receipt
  is a record committed after the owner tags and publishes, naming the tagged commit and tree.

A commit cannot name itself, so a receipt committed after the tag is never in the tagged commit's
tree. Read literally, the publication row can only see a receipt at a descendant of the release, and
there the receipt's recorded revision is never `HEAD`. Under `ARTIFACT-RDY-V0-001` the row could never
pass. It could pass only if the reader ignored the binding, and invariant 2 rules that out.

## Options weighed

1. **Bind to the receipt's recorded revision (chosen).** Read the receipt from `HEAD`'s commit
   object. Require the exact release tag to name the recorded revision, the recorded tree to be
   that commit's tree, and `HEAD` to descend from it. Cross-check every attached digest against the
   private archive witness bound to that recorded revision.
2. **Keep the binding to `HEAD`, and keep the receipt in the private Git directory** as a witness
   written at the candidate. Rejected: that file is not immutable Git content (invariant 1), and it
   carries no owner approval. Decision 0109 requires a committed owner record.
3. **Put the receipt in the tag message or a note ref.** Rejected: it adds a second receipt location
   and a ref-mutation step, and a moved or re-signed tag would silently rewrite it.

## The call

1. For the publication row only, "current revision" in `ARTIFACT-RDY-V0-001` means the receipt's
   recorded release revision. The go-archive and tag rows still bind to `HEAD`. So does promotion,
   which stays `NOT_RUN` by decision 0109.
2. The receipt keeps decision 0109's prose decision record, which is the owner's approval. Beside it,
   in the same commit, sits a machine-readable file, `docs/releases/<tag>/publication-receipt.json`.
   The file names that record. Prose alone stays `NOT_RUN`.
3. Only a positive binding passes. The row is `NOT_RUN` when the tag is absent, when the tag names
   another commit, when the recorded revision is not an ancestor of `HEAD`, when no archive witness
   is bound to the recorded revision, or when an attached file has no local witness. It is `FAIL`
   when a receipt is malformed, its recorded tree differs from its commit, an attached digest differs
   from the witness, or the witness at that revision is `FAIL`.
4. The reader proves only a local binding. A `PASS` does not show that the remote release exists or
   holds those bytes, and publisher identity stays `NOT_VERIFIED` (decision 0108).

The contract is `ARTIFACT-RDY-V0-006..009`. It is implemented by
`conformance/release-artifact-v0/publication_status.go` and the publication row of
`script/release-checklist`.

## Consequences

- A checklist run at the receipt-bearing descendant shows tag `FAIL` and go-archive `NOT_RUN`,
  because both rows still judge `HEAD`. Decision 0109's readiness bar is judged at the frozen candidate,
  before publication. The publication row is read after the receipt commit.
- If the optional companion bundle is attached, its digest has no local witness, so the publication
  row stays `NOT_RUN` until a companion witness exists.

Rollback: revert the reader, its tests and the checklist row to the unconditional `NOT_RUN`, and
restore `ARTIFACT-RDY-V0-001/004` to their prior text. No receipt, tag or release is touched.
