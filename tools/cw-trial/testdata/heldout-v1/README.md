# heldout-v1: frozen confidently-wrong trial tasks

Fifty tasks drawn from the released Agent Retrieval Bench v2 samples (v2_trace2code, v2_edit2ripple,
v2_comment2context; v2_code2test and v2_abstention are development and were excluded).
Rule: per release, sort samples by (language, sample id); take index floor(i*N/n) for i in 0..n-1;
verify every gold path is a `kind:file` row of the snapshot chunk file; on failure take the next
sample in the same order. No difficulty judgement. Counts: 20 trace2code, 12 edit2ripple,
18 comment2context; languages go 24, python 16, rust 6, typescript 1, java 1, config 1, docs 1.
Zero gold paths failed verification; `manifest.json` carries the rule, counts, and tasks sha256.
`task` is the sample's natural-language field verbatim; `query` is the whole sample query verbatim.
Authored 2026-09-01 by an agent without Corvint access; `first_observation` records the first run (2026-09-02, `benchmarks/results/cw-trial-heldout-v1-first-run.json`).
This set becomes development the moment any result from it changes Corvint.
