## 2026-10-20 V1-2052: read-only `adopt --setup` setup report (ADS-V0, proposed)

### Change

`corvint adopt --setup` extends the existing adoption inventory verb (no `doctor` verb exists, so
no parallel verb is added). It prints one canonical JSON report, profile `corvint-setup-report/0`,
with four checks: `index-snapshot`, `agent-instructions`, `hooks` and `ignored-paths`. Each carries
status, evidence and, for a gap, a suggested fix. It writes nothing (ADS-V0-007).

### Measured output

Command: `corvint --root <this worktree> adopt --setup` on a worktree build, base 2b0cfce94 plus
this lane's commits; host load high (bytes are load-independent). Exit 0, 874 bytes, `state: GAPS`,
`gap_count: 2`:

| Class | Status | Evidence |
| --- | --- | --- |
| index-snapshot | gap | `snapshot: missing-matching-snapshot; absent-stale-or-unsupported-engine` |
| agent-instructions | ok | `AGENTS.md: names corvint`; `CLAUDE.md: present, does not name corvint` (it imports AGENTS.md) |
| hooks | gap | no repository-scoped hook file; user-scope installs are not read |
| ignored-paths | ok | all 5 Corvint local state paths are ignored |

`git status` was unchanged across the run.

### Limits

- The hooks gap here is an over-report: this host installs the Corvint plugin at user scope, which
  the report does not read (ADS-V0-005 says so in the evidence).
- `ignored-paths` reads the effective Git ignore rules, so a user's global excludes file can hide
  repository drift; the tests isolate `GIT_CONFIG_GLOBAL` and `XDG_CONFIG_HOME` for this reason.
  `corvint index` writes `.corvint/.gitignore`, so `.corvint/index/` and the self-observation
  ledger are normally covered once an index exists.
- ADS-V0-001..007 accepted under the delegated 1.0 feature cut (decision 0548).
