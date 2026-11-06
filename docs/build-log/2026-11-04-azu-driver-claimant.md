# Driver claimant and ownership audit (AZU-V0-001, AZU-V0-002; V1-4684, V1-4685)

Base: `83d703f98` (`origin/main` `8e2c13aa9` plus the AZU-V0 spec commit).
Scope: AZU-V0-001 and AZU-V0-002 of `docs/specs/affected-zero-unknown-v0.md`. Soundness rule: no
unit that reads, executes, imports or embeds a changed path is dropped; unknowns turn into
selections only.

## AZU-V0-001: driver claimant

A built-in `Driver` claimant owns every path `driverInput` recognises. Its units are make targets
(`driver:make:<file>#<target>`) and workflow jobs (`driver:ci:<file>#<job>`). A hunk inside one
recipe or step selects that unit and its dependents. Each fork took the wider choice:

- A hunk outside every recipe or job selects every unit of the file.
- An unparsable base, or a `--path` plan with no base, selects every unit of the file. An unparsable
  head file also keeps a `driver:unparsable-file` frontier entry (for example YAML anchors in
  `release-gates.yml`).
- A removed block or a deleted driver file selects every driver unit.
- A recipe or step with a computed invocation is selected with witness `FRONTIER_SITE` on every
  non-empty plan, with its dependents.
- `confinedReaders` skips driver units, and `corvint test` does not run `DRIVER_TARGET` or `CI_JOB`
  selections; consumers report them as left to make or CI.
- There is no `NO_SELECTABLE_TEST` relief; that is AZU-V0-006.

One before/after replay of `corvint affected --base 39c031980` at `8e2c13aa9`:

| | before | after |
|---|---|---|
| unknown | 92 | 89 |
| `LANGUAGE_FRONTIER` | 81 | 81 (+1 `driver:unparsable-file`, -1 `swift:conditional-compilation`) |
| `UNOWNED_DIRTY_PATH` | 5 | 2 (`.github/forward-rows/tiers.jq`, `script/ci-least-privilege-exceptions.tsv`) |
| selected | 266 | 488 |

Every unit selected before is still selected. The frontier count barely moves because a unit-level
frontier bears whenever its unit is reached, and the two remaining unowned paths keep the AFP-V0-511
cascade. `tiers.jq` is deliberately left undeclared.

## AZU-V0-002: ownership audit (partial)

`corvint affected --ownership-audit` lists every tracked path at `HEAD` of a clean worktree that a
plan would report as `UNOWNED_DIRTY_PATH`: it runs the plan's own seed and declaration steps, so it
cannot drift from selection. It builds without the cache, writes nothing, refuses a dirty worktree,
and exits 0 when the list is empty, 1 when not, and 2 on an error.

`.corvint/non-source-paths.json` (profile `corvint-non-source-paths/0`) takes the runtime-readers
grammar. It runs after the reader rules, so every unit whose embed, enclosing directory, path
literal, read scope or unbounded read reaches a declared path stays selected. Unlike documentation
and runtime readers, the class stays unowned for the frontier rules, so a declared path still bears
the AFP-V0-511 cascade. Only `.agent-evidence/` is declared: a plan for one of its files was checked
to still select the `conformance/release-artifact-v0` walker and the unbounded readers.

At `HEAD` 1694 tracked paths are unowned. Most are testdata, because `confinedReaders` is false in
a multi-language repository, so the Go read-data rule does not claim them. Other unowned paths are
`go.mod`, results trees, and the `.corvint/` declarations that a unit names. Declaring them would
need per-path proof that no test reads them, so the maintained test
`TestOwnershipAuditRatchet_AZU_V0_002` is a ratchet over
`internal/liveverify/affected/testdata/ownership-audit-baseline.txt`. It fails on any new unowned
path, and on any baseline line that is claimed or no longer tracked. The empty list that AZU-V0-002
names is still open.
