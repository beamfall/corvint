# Decision 0331 — prepare the OpenCode compatibility patch as v0.5.0a4

Date: 2026-09-22. Status: accepted for candidate preparation; exact artifact publication awaits
owner approval. Authority: the owner requested OpenCode compatibility repairs, current MCP support,
useful native tooling, a new patch release, and explicitly chose to patch the current 0.5 release
before the separate 0.6.0 work. The next unused alpha token is `0.5.0a4`.

## Decision

The candidate starts at the public `v0.5.0a3` tag and carries the reviewed OpenCode compatibility
repair plus its setup/skill and release tuple changes. It does not include unfinished 0.6 work.
`VERSION`, native version output, archive smoke expectations, exact editor admission, installation
instructions and current-version tests move together. Historical evidence and tags stay immutable.

MCP `2026-07-28` remains the default. OpenCode's current initialization handshake uses the explicit
`--protocol-version 2025-11-25` profile. The core and test-validity servers have actual native
OpenCode tool-call development evidence; neither that evidence nor the packaged tool-routing skill
promotes the native adapter beyond `FALLBACK`. Docs and corpus transports share the repair; the
experimental corpus binary remains outside the existing shipped companion set.

The exact clean candidate must pass the full repository gate. Retain decisions 0108, 0109, 0327
and 0329's unsigned GitHub prerelease, publisher `NOT_VERIFIED`, four non-Windows core archives,
unchanged gate-produced checksums and no-promotion boundaries. The optional existing macOS arm64
companion bundle is a separate artifact: attach it only after its own same-candidate reproducible
build and full installed-bundle qualification, including the separate task-manager identity and
MCP checks, succeed. Missing or failed qualification is
visible and never inherited from an older release. No tag or asset is published before the owner
approves the exact reviewed packet.

## Rollback

Withdraw an unpublished candidate without moving an existing tag. After publication, keep the tag
and assets immutable and supersede them with a later patch. Removing the compatibility selector
restores the strict latest-protocol path; it does not make an older OpenCode client compatible.
