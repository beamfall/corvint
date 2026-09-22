# Decision 0108 — the 0.4.0a4 alpha archives are unsigned, and checksums claim integrity only

Date: 2026-09-12. Status: accepted. Authority: repository owner, verbatim instruction "I want you to
make the calls for me" (delegated owner call).

`docs/specs/release-artifact-integrity-v0.md` left two owner calls open for the release archives. The
first was `ARTIFACT-GO-V0-008`, which says archive checksums prove integrity and nothing more. It was
proposed but never accepted (decision 0010 accepted only `ARTIFACT-GO-V0-001..007`). The second was
the signing-options table, where no option was selected. Review `docs/reviews/r8-release.md` (D2)
proposed Sigstore keyless signing. It named No signing plus acceptance of `ARTIFACT-GO-V0-008` as
the choice if the release ships before a release workflow exists.

The owner call:

1. `ARTIFACT-GO-V0-008` is accepted as written. It adds no behavior and only rules out claims. The
   archive tools already create no signature, identity, attestation, SBOM, tag, upload or
   publication. The frozen `corvint.release-go-archive-report.v1` schema has no member that could
   express one.
2. **No signing** is selected for the `0.4.0a4` alpha prerelease only. Sigstore keyless signing
   binds identity to a GitHub Actions workflow, and hosted CI cannot currently start jobs (account
   billing, `docs/RELEASE-NOTES-alpha.md` "Hosted CI"). An owner-held hardware or KMS key brings
   custody and recovery duties that have no runbook. Either would also need an agent, or the
   owner, to create or use a signing identity. No agent may do that. Any later prerelease or
   stable release needs a new selection.
3. The "No signing" row said the verifier MUST report publisher identity as `NOT_VERIFIED`.
   `archive-verifier`'s report schema is frozen with `additionalProperties: false`, and adding an
   identity member would change that wire contract. The row is amended instead. The release notes
   and any publication receipt carry the `NOT_VERIFIED` statement, and a verifier `PASS` is
   documented as asserting bytes only. `docs/RELEASE-NOTES-alpha.md` gains that statement.

Nothing here creates, requests or uses a key, and nothing here tags, publishes or promotes.

Rollback: select a signing option in a later decision before the next release. Archives already
published unsigned stay unsigned; a signature can be added for a later tag.
