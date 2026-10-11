# Examples licence headers, shellcheck repair and recipes gate step (2026-10-10)

Batch `lic`: V1-0135, V1-0128, V1-0139, then the owner's decision 0504. No requirement is added;
`GPK-V0-020` is amended under decision 0504.

- V1-0135 and V1-0128: the eight headerless files under `examples/**` (two Go tests, two workflow
  YAML files, two shell scripts, two Python scripts) gained SPDX headers. The first commit used
  `Apache-2.0`, the licence `LICENSING.md` then assigned to `examples/**`, and left five
  `AGPL-3.0-or-later` headers under `examples/evidence-provider/v0/` in conflict with it.
  `examples/cem/verify-pr.sh` uses `CDPATH=''` (SC1007); `shellcheck examples/cem/*.sh
  examples/cem/recipes/*.sh` is clean.
- Decision 0504 (owner, 2026-10-10, "Examples AGPL, protocol Apache") settles the conflict:
  `examples/**` moves to AGPL-3.0-or-later and `protocol/**` stays Apache-2.0. All 17 non-Markdown
  files under `examples/**` now carry `SPDX-License-Identifier: AGPL-3.0-or-later` (the eight new
  headers and the four older Apache ones were changed; the five AGPL ones stand). `LICENSING.md`
  drops `examples/**`, decision 0002 gains a dated amendment, and `README.md`, `PROVENANCE.md`,
  `ROADMAP.md`, `docs/CEM-CI.md`, the recipes README and `GPK-V0-020` match. Copies already
  published under Apache-2.0 keep those terms. `conformance/**`, `interop/**` and the listed `docs/`
  files are unchanged.
- `script/check-license-headers.sh` (`make license-headers-check`, a gate step scoped to
  `examples/`) fails on an Apache-2.0 notice under `examples/` or a non-Markdown file without the
  AGPL identifier; `make license-headers-test` covers both notice forms, a missing header and a
  `protocol/` Apache file it ignores. Switching `examples/cem/verify-pr.sh` back to Apache-2.0 makes
  the check fail and name the file.
- The release-artifact manifest's `LICENSING.md` and `PROVENANCE.md` digest pins move to the
  amended bytes, as for decision 0422.
- V1-0139: `cem-recipes-test` joins `GATE_STEPS` and the `tools/gate-ledger` scope table
  (`treeScope`, like `cem-verify-pr-test`); the CEM-PILOT-024..027 evidence row no longer says
  "not a gate member". `make -n gate` invokes it; the target passes (`cem-recipes: ok`).
