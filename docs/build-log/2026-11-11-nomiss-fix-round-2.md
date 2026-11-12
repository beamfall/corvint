# Location sources: fix round 2 (V1-3999, lane nomiss)

Addendum to `2026-11-11-nomiss.md` after the round-2 independent review (SHIP-WITH-FIXES), on
caf131c90. AFP-V0-3661 and AFP-V0-3664 stay proposed and experimental; no owner acceptance is
claimed. Both copies of `locationsources.go` stay identical but for the package clause.

## Findings and changes

| Finding | Change | Negative tests |
| --- | --- | --- |
| P1: `bindingSafe` returned true on any re-entry of a variable being checked, so `v := os.Getenv("ROOT"); v = filepath.Dir(filepath.Dir(v))` reached the raw-entry allowance of `stringSafe` (`c.Env = []string{"ROOT=" + v}`) with a transformed value | `visiting` records the kind a variable is being checked at; a re-entry at the same kind is a copy and stays safe, a re-entry at another kind is a transform written back and fails closed (AFP-V0-3661). A kind is kept only by a copy: every transform yields `sourceString` or `sourceBounded`, never `sourceEnvValue` | use cases `entry of a self-reassigned value`, `entry of a self-extended value` (`v = v + "/.."`), `entry of a self-reassigned lookup`, `entry of a two-variable cycle` (`w := v; v = filepath.Dir(w)`); fixture `selfassign`; `TestLocationReexportHeld_AFPV03661` (planner: a body-only edit of an unrelated package selects the probe) |
| P3: `commandDirectory` allowed `Environ` on the command; with `Env` nil, `Cmd.Environ` appends `PWD=<Dir>`, so the Caller-rooted `Dir` is read back | `Environ` joins `Args` and `Path` as a member that unproves the command (AFP-V0-3664) | use case `command directory read back through Environ`; fixture `environpwd`; planner case of the same name |

Controls still not held: `entry of a raw value to a shell` (`c.Env = []string{"ROOT=" +
os.Getenv("ROOT")}`, no reassignment) and `entry of a copied raw value` (`w := v`, `w`
re-exported), in the use table and the planner test. The child's own read of `$ROOT` stays
residual R14.

## Residual table addendum

| Channel | Treatment |
| --- | --- |
| A re-exported environment variable reassigned to a transform of itself, directly or through another variable | checked, AFP-V0-3661 (fix round 2) |
| A command's `Dir` read back through `Cmd.Environ` (`PWD=<Dir>`) | checked, AFP-V0-3664 (fix round 2) |

No residual of the earlier table changes.

## Evidence

The new rows and the planner test fail at caf131c90 (the four P1 use rows return `""`, the P3 use
row `"contingent"`, fixture `selfassign` is missing from the gate and planner rule (d) sets, and
the four P1 planner cases select nothing) and pass after the change. The `environpwd` fixture and
its planner case were already held at caf131c90 at the index level; only the use row discriminates
P3 there.

Mutations, gateselect copy, `TestLocationSourceUses_AFPV03661`:

- `return visiting == kind || true` (the old short-circuit): the four P1 rows fail.
- `case "Args", "Path":` (`Environ` dropped): `command directory read back through Environ` fails.

Cost: the change only holds more. `selection-baseline-check` was not rerun, since it cannot narrow
a selection.
