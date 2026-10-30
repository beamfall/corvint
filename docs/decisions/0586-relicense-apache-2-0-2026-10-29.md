# Decision 0586 — Relicense the Corvint repository under Apache-2.0

Date: 2026-10-29. Status: accepted by the owner. Authority: owner decision in chat, 2026-10-29.
Asked for a more permissive license, the owner answered "Yes, Apache-2.0" (V1-3163).

## Context

Decision 0002 published Corvint with an AGPL-3.0-or-later product layer and an enumerated
Apache-2.0 interoperability layer. Decision 0107 chose AGPL-3.0-or-later for the companion
`corvint-taskman` component to match that product layer. Decision 0504 moved `examples/**` into the
AGPL layer, and decision 0422 (answer A5, V1-0379) recorded a three-file Apache-2.0 exception for
`cmd/corvint/frontier*.go`.

Every human commit on `origin/main` is the owner's (`russlewis@gmail.com`, `me@russe11.com`), and
the 53 Claude-authored commits are the owner's agent work, so the owner holds the rights needed to
relicense the tree. No outside contribution is included.

## Decision

1. The whole repository is licensed under Apache-2.0. `LICENSE` holds the verbatim Apache License,
   Version 2.0 text, and `LICENSING.md` states one license with no path boundary. This amends
   decision 0002 (its product/interoperability boundary ends), supersedes decision 0107's license
   call for the companion component, and supersedes decision 0504's AGPL designation of
   `examples/**`. Those decisions keep their text as historical records.
2. Every `SPDX-License-Identifier: AGPL-3.0-or-later` header in live source reads
   `SPDX-License-Identifier: Apache-2.0`. Package metadata (the npm packages under `integrations/`,
   the Claude Code plugin and marketplace manifests, the Pi skills, the VS Code extension and its
   `LICENSE`), `README.md`, `PROVENANCE.md`, `CONTRIBUTING.md`, `AGENTS.md` and the specs that named
   the AGPL layer state Apache-2.0. `.github/**` headers change in a separate commit on
   `lanes/relicense-apache-github`, because those paths are CI control plane.
3. `make license-headers-check` fails on a former AGPL SPDX identifier in any tracked non-Markdown
   file outside the historical records (`docs/build-log/**`, `docs/decisions/**`, `.taskman/**`,
   `.corvint/**`) and on a non-Markdown `examples/**` file without
   `SPDX-License-Identifier: Apache-2.0` in its first five lines.
4. The frontier exception (V1-0379) dissolves: `cmd/corvint/frontier.go`,
   `cmd/corvint/frontier_adapters.go` and `cmd/corvint/frontier_test.go` already carry the
   Apache-2.0 notice, which now matches every other file. This resolves V1-0379.
5. Versions, tags, archives and packages published before this decision remain available under the
   terms they were published with. This decision withdraws no rights already granted.
6. Contributions are licensed under Apache-2.0 (section 5). The contributor agreement stays
   available as an optional additional grant and is no longer required for any path.

## Retained bytes

- `LICENSE-APACHE-2.0` and its four copies (`interop/cem01-go/`, `protocol/cem-1.0/stable/`,
  `protocol/cem-1.0/stable/repository-envelope-packet/`, `protocol/cem-completeness-1/`) keep their
  bytes, including the preamble that describes the former boundary. Frozen protocol manifests
  (`protocol/cem-1.0/stable/manifest.json`, the repository-envelope packet's `manifest.json` and
  `INPUTS.json`) and the release-artifact manifest pin digest
  `a55dcdf3a62566e48883e0c0caba22c65f8fe5431aed6437422ba447e60cd27b`. `LICENSING.md` states that
  the preamble is historical and does not limit the grant.
- Four files keep their historical AGPL SPDX line because frozen evidence pins their exact bytes:
  `benchmarks/daily-loop-v0/harness.py` and `benchmarks/untouched-repository-v1/harness.py`
  (preregistered harness digests), `internal/postmergehost/testdata/paired-author/author/main.go`
  (fixture `definition.json` digest) and
  `tools/cw-trial/testdata/beamfall-roadmap-v1/build_roadmap_set.py` (task-set README digest).
  They are Apache-2.0 like every other file; `LICENSING.md` names them and the header check exempts
  exactly these paths.
- The embedded test reporters `internal/testrunner/dynamic/reporters/cypress.cjs` and `jasmine.cjs`
  ship in the binary, so their SPDX first line changed; no reporter behavior or produced fixture
  changed. Their recorded `reporterSha256` keeps naming the reporter that produced the evidence
  (`40d645483c318ae348b21f87e02b7f4dafb7a5a55198c6d8652496318e20d4f4` for Cypress in
  `testdata/runtime-qualification.json`, `0e6e10b1dc76cf031dac9e5267034b26074ac7587143d36d62319f1c4799e14d`
  for Jasmine in `testdata/jasmine/provenance.json`). A separate `reporterHeaderRelicense` record
  names the current header-only successor digest, and the tests restore the prior SPDX line on the
  embedded bytes and require exactly the producing digest.
- `conformance/release-artifact-v0/manifest.json` pins the new `LICENSE`, `LICENSING.md` and
  `PROVENANCE.md` digests.

## Non-goals

No release, tag or package is published by this decision. The separate `corvint-taskman`
repository's own `LICENSE` is a commit there. Historical records, results and quoted evidence keep
their text.

## Rollback

Revert this decision, the `LICENSE`, `LICENSING.md`, header, metadata, spec and check changes, the
decision 0002 amendment and the `.github/**` header commit together. Rights granted under
Apache-2.0 for the intervening commits stay granted.
