# Frozen replay counterexample and conservative caller retention

Requirements: AZU-V0-004, AZU-V0-009, AZU-V0-8901 and AZU-V0-8903.

The first exact-tree replay found a safety regression, not acceptance evidence. At merges
`39c031980`, `6c33df33b` and `ea770fa17`, the candidate omitted
`go:github.com/Beamfall/corvint/internal/dogfoodflow` and
`go:github.com/Beamfall/corvint/internal/localcompletion`. The baseline selected them through
`script/batch-regen.sh`'s declared callers. A new directory-construction check invalidated the
historical declaration and discarded those previously verified edges. The retained report digest
is `0aa6b8d293dea9315c1e66a27736c5e07aa728819efc4f61db3a119327e48db9`;
raw plans and witnesses remain outside the repository in `azu-new-evidence/replay-before-safety-repair`.

The repair keeps caller edges admitted by the earlier literal/shell-expansion completeness check
when only the new directory-construction check fails. Both the invalid-declaration and inbound
unknown remain. The broader checker does not accept the declaration or hide its new caller; it
preserves verified selections while abstaining about completeness. Declarations that already
failed the old guard keep their existing refusal behavior. The whole existing shell package and
`TestConstructedDriftKeepsVerifiedCallers_AZU_V0_8901` exercise this distinction.

Authority checks also distinguished two non-callers: `repin_test.sh` only matches `make -n` output;
`dogfood-change_test.sh` copies the script into a confined fixture and never runs the original site
against repository data. They are mentions of those original sites. Existing scope assertions stay
intact, and the repository acceptance probe separately requires the confined fixture test to stay
selected when the original script source changes. Scope declarations govern fixture data reads;
this is not permission to drop an actual script-source reader.

The repaired exact-tree replay and final test results are recorded below when produced. Missing
historical full-suite/mutation observations remain NOT_PRODUCED and cannot qualify AZU-V0-009.

The first repaired replay retains every baseline selection on all eight merges. Its report digest
is `e7eb15ff8aaf9da25c6bf00321661676940618a73b6457c4986cc862b6f86b07`. Full-suite and mutation
cells still say NOT_PRODUCED. The safety comparator also rejects a parent test that disappears
inside an otherwise observed unit; a unit-level completeness statement cannot conceal that gap.

Whole-package functional checks passed before the calibrated incremental-selection budget failed
at p95 225.486 ms, 5.77 calibration probes against the unchanged 5.00 bound. The ownership matcher
now rejects exact-path and literal-prefix misses before allocating its general matching state;
existing wildcard cases and additional zero-component `**` cases retain the same semantics.
Runtime caller work also exits immediately for graphs without TypeScript. The existing budget is
remeasured; its limit and assertions are unchanged. This repair is confined to observed overhead
from the ownership declarations and inbound closure.

The unchanged owner-pattern grammar cases and incremental-selection budget passed together
(`selection-budget-repair.log`, 66.059 s total test runtime). No post-repair percentile is claimed:
the passing invocation did not emit its suppressed test log. Final whole-package verification and
the replay at the final implementation revision follow; historical safety observations remain
external requirements.
