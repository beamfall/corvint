# Provenance and publication status

## Copyright and authority

All Corvint source in this repository was authored by Russell Lewis, who holds copyright in it and has
attested that no employer, client, co-author, confidentiality, or third-party restriction prevents
its release under the terms below. The deterministic engine at the root of this work was extracted
from an earlier codebase of the same author, itself licensed under AGPL-3.0; the extracted material
and all subsequent Corvint work are published here under compatible copyleft terms. Per-file digests
for the extracted material are recorded in `docs/extraction-manifest.json`.

## Maintaining the rights record

The owner's attestation above does not establish ownership of future outside contributions.
Before merging any such work, record its author or rights-holding organization, origin, destination
license, and any additional grant required by [CONTRIBUTING.md](CONTRIBUTING.md). Maintain private
acceptance evidence separately from public source. The agreement permits an intentional public
acceptance statement; keep supporting identity/authority documents and privately supplied signatures
private. Amend the ownership statement when outside work is incorporated rather than continuing
to describe a mixed-origin tree as solely owner-authored.

The [contributor agreement](CONTRIBUTOR-AGREEMENT.md) provides a manual acceptance process. Its
publication is not evidence that anyone has accepted it, and grants no rights retroactively.
A commercial release must verify its actual source/dependency inventory and grants as described in [LICENSING.md](LICENSING.md).

## Licensing

Corvint is licensed under the **GNU Affero General Public License, version 3.0 or later** (`LICENSE`).
An explicitly enumerated interoperability layer — protocol descriptions, schemas, conformance
material, examples, and interop implementations — is licensed instead under the **Apache License
2.0** (`LICENSE-APACHE-2.0`), so that agent, IDE, CI, and context-engine vendors can implement the
portable standard without the copyleft attaching, including in proprietary software.

`LICENSING.md` is the authoritative path boundary. `docs/decisions/0002-future-publication-transition.md`
records the decision that fixed it.

## Publication

This repository is published as a single parentless root commit built from the approved tree. It
carries no predecessor commit graph, no grafts or replace refs, and no imported history. Release
artifacts, tags, and packages are separate outward actions governed by
`docs/specs/release-artifact-integrity-v0.md`.

## Brand assets

The marks in `assets/brand/` were authored for Corvint on 2026-08-25. The SVGs are deterministic
redraws of a text-only visual concept; the PNG theme variants were produced by image generation from
that project-owned concept. No third-party visual assets were used. The universal mark and `CORVINT`
lockup are deterministic outlined SVG constructions, with PNG exports rendered from those sources.
The marks are part of the Corvint product layer described in `LICENSING.md`; this statement makes no
trademark-clearance claim.
