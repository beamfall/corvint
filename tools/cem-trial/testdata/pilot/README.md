# Pilot set

Five Beamfall changes selected by `cem-trial select` with seed `pilot-2026-09-03` from a population
of 620 candidates, under the rule frozen verbatim in `tasks.json` (`selection_rule`).

This is a `pilot` partition, never held-out evidence: under `benchmarks/README.md`'s
first-observation rule its results may tune prompts and timeouts, and are then development forever.
The judged set is a separate `--partition heldout` selection with its own seed, which passes this
manifest to `--exclude` so the pilot's changes are excluded from the held-out population.

`patches/<id>.patch` holds the exact bytes each arm is shown — the change's source hunks alone. The
test-or-spec files the same commit touched are withheld and recorded as that change's gold.
