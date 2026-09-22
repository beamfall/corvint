# Decision 0107 — corvint-taskman ships under AGPL-3.0-or-later, and its provenance stays the owner's attestation

Date: 2026-09-12. Status: accepted. Authority: repository owner, verbatim instruction "I want you to
make the calls for me" (delegated owner call).

`PUB-V0-003` requires each companion bundle component to carry its applicable license and provenance
files. `internal/companionrelease/companionrelease.go` `componentNotices` requires `LICENSE` and
`PROVENANCE.md` in each component's exported tree. The 2026-09-12 companion gate run
(`docs/BUILD-LOG.md`) refused at the `atm` notices step because `corvint-taskman` `1269ec5` tracks
neither file. No license text exists there to infer from.

The owner call: `corvint-taskman` is licensed under the GNU Affero General Public License, version 3.0
or later. That matches the Corvint product layer (decision 0002) that the console bundles alongside
`atm`. It keeps one copyleft term across the optional bundle's product binaries, and it adds no
second license boundary to explain in the release notes. A permissive license would be the right
call only for interoperability material. `atm` is a product, and the task-store wire contract it
shares with the console is already governed from Corvint.

Two parts of this remain outside a delegated call:

- `PROVENANCE.md` for `corvint-taskman` must state its copyright basis. Corvint's own `PROVENANCE.md`
  rests on the owner's attestation that no employer, client, co-author, confidentiality or
  third-party restriction applies. That is a statement of fact only the owner can make, so this
  decision does not write it and no agent may supply it.
- Adding `LICENSE` (the verbatim AGPL-3.0 text, as in Corvint's `LICENSE`) and `PROVENANCE.md` is a
  commit in the `corvint-taskman` repository. It is listed on the human release checklist in
  `docs/plans/public-release-v0-gap-2026-09-12.md`. No Corvint code changes: the notices check
  already refuses without both files.

Rollback: before any bundle is published, supersede this decision with another license choice. The
bundle gate reads whatever `LICENSE` the pinned `corvint-taskman` commit tracks. After publication,
the published terms stay attached to the published artifacts.
