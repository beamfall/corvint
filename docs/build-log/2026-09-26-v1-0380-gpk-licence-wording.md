## 2026-09-26 V1-0380 GPK-V0-020: the licence of new production Go paths matches LICENSING.md

`GPK-V0-020` said new production Go paths "remain Apache-2.0 subject to Commons Clause v1.0".
`LICENSING.md` and decision 0002 license every product path, including `cmd/**` and `internal/**`,
under AGPL-3.0-or-later. Plain Apache-2.0 covers only the enumerated interoperability paths and the
three owner-approved `cmd/corvint/frontier*.go` files (decision 0422, A5). The spec text therefore
named a licence that no recipient receives. It was the only mention of Commons Clause in tracked
Markdown.

Decision: `GPK-V0-020` now names AGPL-3.0-or-later, the product layer that `LICENSING.md` defines,
for new production Go paths. The Apache-2.0 paths it lists are unchanged, and so are `LICENSE`,
`LICENSING.md`, `PROVENANCE.md` and the boundary itself. The change restates the authoritative
licence file and draws no new legal conclusion.

`GPK-V0-020` is an accepted requirement, so this wording change needed the owner's approval before
it merged. The owner approved it on 2026-09-27. Rollback: revert the change, which restores the
stale wording.
