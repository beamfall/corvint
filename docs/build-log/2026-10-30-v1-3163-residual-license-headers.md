# V1-3163 residual: convert the two fixture headers, keep the sealed harnesses pinned

Of the 11 files on `origin/main` (93b9f5b08) that matched `SPDX-License-Identifier: AGPL`, only four
carry a real header; the rest quote the former line as text (`LICENSING.md`, decisions 0002, 0504
and 0586, and the three byte-pinned public-root-history notice fixtures) and stay as they are.

Converted to `SPDX-License-Identifier: Apache-2.0`:

- `internal/postmergehost/testdata/paired-author/author/main.go`; its digest in the fixture's own
  `definition.json` `files` map is updated with it. `TestPairedAuthorSourceConformance` PASS.
- `tools/cw-trial/testdata/beamfall-roadmap-v1/build_roadmap_set.py`; the task-set README's
  provenance digest is updated. The script reproduces the same `tasks.json` (it reads only Git),
  so the task-set digest is unchanged.

Kept byte-pinned: `benchmarks/daily-loop-v0/harness.py` and
`benchmarks/untouched-repository-v1/harness.py`. Each harness refuses to run unless its own SHA-256
equals the `harnessSha256` sealed in its preregistration (daily-loop-v0; Corvint, cobra, urfave and
viper for untouched-repository-v1), and recorded runs, decision 0432 and the 1.0 release
specification cite that digest. Changing one header would require re-sealing preregistrations
after runs were recorded against them, which weakens the frozen evidence (AGENTS.md invariant 1).
`LICENSING.md` already states these files are Apache-2.0 regardless of the historical line.
The orchestrator confirmed this on 2026-10-30: the two harness exemptions are intentional and
permanent for the v0/v1 preregistrations, no preregistration is re-sealed, and any future relicense
of those harness bytes needs a new preregistration, not an edit.

`script/check-license-headers.sh` now exempts only the two harnesses and the notice directory;
`script/check-license-headers_test.sh` proves a former header in the converted cw-trial fixture
path fails. `make license-headers-check` and `make license-headers-test` PASS. Package metadata
(`extensions/vscode`, `integrations/{opencode,pi,pi-protected}`, `interop/spec-toolchains/npm`)
already states Apache-2.0; no `.github/**` file carries a former header on this base.
