# Examples licence headers, shellcheck repair and recipes gate step (2026-10-10)

Batch `lic`: V1-0135, V1-0128, V1-0139. No requirement is added or amended.

- V1-0135 and V1-0128: the eight headerless files under `examples/**` (two Go tests, two workflow
  YAML files, two shell scripts, two Python scripts) now carry `SPDX-License-Identifier: Apache-2.0`,
  the licence `LICENSING.md` assigns to `examples/**`. No existing header was changed. `examples/cem/verify-pr.sh`
  uses `CDPATH=''` (SC1007); `shellcheck examples/cem/*.sh examples/cem/recipes/*.sh` is clean.
- Observed, not changed: five existing files under `examples/evidence-provider/v0/` carry
  `AGPL-3.0-or-later` headers although the path is Apache-2.0 under `LICENSING.md`. Relicensing is
  an owner decision, so the headers stand and a follow-up is proposed.
- V1-0139: `cem-recipes-test` joins `GATE_STEPS` and the `tools/gate-ledger` scope table
  (`treeScope`, like `cem-verify-pr-test`); the CEM-PILOT-024..027 evidence row no longer says
  "not a gate member". `make -n gate` invokes it; the target passes (`cem-recipes: ok`).
