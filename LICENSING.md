# Corvint licensing

Copyright 2026 Russell Lewis.

The whole Corvint repository is licensed under the **Apache License, Version 2.0**. The full text is
in `LICENSE`. There is no path boundary: every Corvint source, specification, schema, conformance,
interoperability, example and tooling path carries the same terms, and contributions are accepted
under them (Apache-2.0 section 5). The owner chose this on 2026-10-29
(`docs/decisions/0586-relicense-apache-2-0-2026-10-29.md`), replacing the earlier AGPL-3.0-or-later
product layer and its Apache-2.0 interoperability island.

## Earlier releases

Versions, tags, archives and packages published before decision 0586 remain available under the
terms they were published with: AGPL-3.0-or-later for the product layer and Apache-2.0 for the
paths `LICENSING.md` then enumerated. Relicensing does not withdraw rights already granted to anyone
holding those copies, and a recipient may keep using such a copy under its original terms.

## Notes on retained files

- `LICENSE-APACHE-2.0` and its copies under `interop/cem01-go/`, `protocol/cem-1.0/stable/`,
  `protocol/cem-1.0/stable/repository-envelope-packet/` and `protocol/cem-completeness-1/` carry the
  same Apache-2.0 text. Their opening paragraph still describes the former interoperability-layer
  boundary; those bytes are pinned by frozen protocol manifests and release-artifact evidence, so
  they are kept unchanged. That paragraph is historical and does not limit this grant.
- A few files keep a historical `SPDX-License-Identifier: AGPL-3.0-or-later` line because frozen
  evidence pins their exact bytes: `benchmarks/daily-loop-v0/harness.py` and
  `benchmarks/untouched-repository-v1/harness.py` (preregistered harness digests),
  `internal/postmergehost/testdata/paired-author/author/main.go` (fixture digest) and
  `tools/cw-trial/testdata/beamfall-roadmap-v1/build_roadmap_set.py` (task-set provenance digest).
  The frozen corpus notice fixtures under
  `internal/cem/corpusbinding/testdata/public-root-history/notices/` quote the former policy text and
  are byte-pinned by the corpus binding tests.
  Like every other Corvint file, they are licensed under Apache-2.0; `make license-headers-check`
  admits that historical line in exactly these paths and that directory.
- Historical records, such as build-log entries, accepted decisions, task-store tickets, sealed
  change evidence and recorded results, quote the former terms as evidence and are not notices.

## Rules

- Contributions are licensed under Apache-2.0. [CONTRIBUTING.md](CONTRIBUTING.md) describes outside
  contributions, and the optional [contributor agreement](CONTRIBUTOR-AGREEMENT.md) remains an
  additional grant a contributor may accept.
- Third-party dependencies keep their own licences; this document governs only Corvint's own source.
- Before any public release, verify that its source, binaries and notices agree. The
  [provenance record](PROVENANCE.md) describes actual evidence, not presumed ownership; a clean
  build, signed artifact, or provenance assertion is not legal clearance.

## Go runtime notice

The native Corvint and companion executables include the Go runtime and standard library.
Those Go components retain their upstream BSD-3-Clause terms. This notice supplements the
Corvint source license above. It is reproduced unchanged from the
[Go 1.27.0 license](https://github.com/golang/go/blob/go1.27.0/LICENSE), also used by that
release's vendored `golang.org/x/crypto`, `x/net`, `x/sys` and `x/text` packages.

```text
Copyright 2009 The Go Authors.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google LLC nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```
