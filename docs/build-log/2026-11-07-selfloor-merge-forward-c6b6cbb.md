# Selfloor merge-forward to c6b6cbb

The owner requested mainline `c6b6cbb0a862c36d3e2d3c28b0bd0dc0da800175`
merged into lane `claude/selfloor` at
`923d1bc55315499be698ad15bb498c6dd06e4bf6`, preserving both behaviors and
binding the whole lane against that new mainline base. Merge commit
`1a89f587833149b72044986972e20fe5de99c376` retains both parents unchanged.

Conflicted paths and resolutions:

- `docs/specs/affected-zero-unknown-v0.md`: retain the lane's AFP-V0-3120 to
  AFP-V0-3128 evidence rows and mainline's AZU-V0-9920 to AZU-V0-9923 rows.
- `internal/liveverify/affected/driver.go`: retain mainline's build-only path
  imports alongside the lane's evaluated-expansion and uncertified-read state.
- `internal/liveverify/affected/driver_shell.go`: retain mainline's output-feeding,
  interpreter and xargs analysis alongside the lane's operand certification,
  read-redirection edges and unbounded-read state. An xargs command retains both
  its fed-argument analysis and uncertified reads.

The generated `docs/specs/REQUIREMENTS.tsv` and `docs/specs/INDEX.json` took
mainline's bytes for the merge. The Makefile names `make spec-requirements` as
owner of both files and the README projection. Regeneration restored the lane's
nine requirement rows; the index and README needed no changes. No lane-added
requirement definition collides with mainline, so no ID was renumbered.
`script/dogfood-merge-base.sh --paths` also lists the generated requirements file.

Prior lane reviews and evidence remain in ancestry and the tree. Pre-change
receipts from the old base are historical and are not fresh observations at this
base. The external handback retains this round's independent review, focused
checks, binding identities, every NOT_PRODUCED reason and friction. This merge
makes no new qualification claim: the owner restricts verification to the
conflict-resolved Go package and spec-requirements/line-citations checks; host
verification and batch CI cover the remaining integration. No seal is requested
because this lane was unsealed. No push, PR or Tasks write is authorized.
