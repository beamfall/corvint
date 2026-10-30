# paired-trial

This tool is the offline analysis for a paired agent trial: `AHI-217` in
`docs/specs/agent-harness-integration-v0.md`, ticket V1-2756.

What it does:
- reads one JSON Lines file of `corvint-paired-trial-receipt/0` receipts, one line per agent session;
- pairs the `off` and `on` arms by task and replicate;
- prints a `corvint-paired-trial-analysis/0` report.

It runs no model and needs no network.

```sh
go run ./tools/paired-trial --receipts trial.jsonl [--resamples 10000] [--seed 1] [--ci 0.95] [--require-complete]
```

## Receipt

```json
{"profile":"corvint-paired-trial-receipt/0","task":"p01","replicate":0,"arm":"on","resolved":true,"cost_usd":0.31,
 "frame":{"bytes":412,"sha256":"<hex>","path":"frames/p01-on.txt","base_tree":"<tree sha>"},
 "steps":[{"tool":"read","input_tokens":9800,"output_tokens":120,"cache_read_tokens":0,"cache_write_tokens":9800,"cost_usd":0.05}],
 "grader":{"tests":[{"name":"tests/test_x.py::test_y","outcome":"pass"}]}}
```

- **Frame.**
  - `frame` holds the exact injected bytes, as a count and a SHA-256, optionally with the stored file.
  - `base_tree` is the tree the frame was probed on. It must be the task's base tree, not the
    agent-edited checkout.
  - An arm that injects nothing records `{"bytes":0}`.
- **Steps.** `steps` is the per-step ledger. Its costs must sum to `cost_usd`.
- **Grader.** `grader.tests` holds the grader's per-test outcomes.

## Report

The report contains:
- **Per pair:** the cost delta (on minus off) and the resolution flip.
- **Cost ratio:** the pooled ratio on/off, with a percentile bootstrap interval over pairs. The seed
  is fixed, so the interval is reproducible.
- **McNemar:** the discordant counts and the exact two-sided p-value.
- **Data quality:** a `not_observed` count per missing field, the tasks with only one arm, and each
  session whose step ledger does not sum to its cost.

A missing field is reported, never dropped. `--require-complete` refuses the input instead of
producing a report.

The tool does not write receipts. The trial harness that runs the sessions must emit them.
