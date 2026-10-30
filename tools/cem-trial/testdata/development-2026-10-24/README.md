# Development set, 2026-10-24

Ten Beamfall changes selected by `cem-trial select --partition development --seed dev-2026-10-24
--since 2026-05-01 --exclude tools/cem-trial/testdata/pilot/tasks.json --limit 10` from a
population of 569 candidates (the pilot's five changes excluded), under the rule frozen verbatim in
`tasks.json` (`selection_rule`).

This set exists to estimate the between-change variance that freezes the held-out pair count
(CRT-V0-008). Its lanes have been observed, so it is development forever under
`benchmarks/README.md`'s first-observation rule: it is never registered, and every held-out
selection must pass this manifest, beside the pilot's, to `--exclude`. Its report is
`benchmarks/results/cem-reviewer-trial-development-2026-10-24.json`.

`patches/<id>.patch` holds the exact bytes each arm is shown, which are the change's source hunks alone.
