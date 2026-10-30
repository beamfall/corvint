# V1-3208: license-headers-check exempts the frozen public-root-history notice fixtures

`make license-headers-check` (a `make gate` step) failed on `origin/main` b469ab710 because three
byte-pinned fixtures under `internal/cem/corpusbinding/testdata/public-root-history/notices/`
quote the former `SPDX-License-Identifier` line as historical policy text. The corpus binding tests
pin those bytes, so editing them is not an option.

Decision: exempt exactly that directory in `script/check-license-headers.sh`, like the four other
byte-pinned frozen-evidence paths, and record it in `LICENSING.md`. Detection elsewhere is unchanged;
`script/check-license-headers_test.sh` now proves a fixture-style quote in that directory passes
while a real former header in a sibling path of the same tree (`internal/cem/corpusbinding/`) fails.

V1-3200 needed no change: base already carries TCD-V0-178 (`ReleaseReasonRefusal` lists every
admitted code; `release --help --verbose` lists `reasonCodes`); the reporter used an older installed
binary. Terse help stays lean by CAL-V0-170, which a pinned test enforces.
