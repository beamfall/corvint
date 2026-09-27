## 2026-09-26 decisions 0415, 0420: hosted linux/amd64 Core platform job

Decision 0420 item 2 makes linux/amd64 a Core platform through a native hosted run instead of
FALLBACK. `release-gates.yml` gains a `linux-core` job on ubuntu-24.04 in the same dispatch.
Decision 0415 item 8 records it.

- The job reuses the gate job's input check, checkout, Go setup, provisioning and log helper by
  YAML alias. So it has one log format and one evidence contract.
- `release-archive`: runbook steps 5 and 6 at the sha. The log prints `SHA256SUMS`, so it is
  candidate-byte evidence only when the linux/amd64 row equals the release's own row.
- `n1-archive`: the published N-1 linux/amd64 archive, verified against that release's
  `SHA256SUMS`. It is downloaded rather than rebuilt: the v0.8.1 tag rebuilds as build 83, while
  the published binary is build 82.
- `install-lifecycle` and `install-lifecycle-n1` (runbook step 8), then `HLQ-V1` for the `cli`,
  `codex` 0.153.2 and `claude-code` 2.1.267 tuples, then `hostile-regressions`.
- The plugin hosts need no secret or model call. `HLQ-V1-003` isolates their homes, and
  `HLQ-V1-004` calls the hooks directly. Their network use beyond `npm install` is unobserved.

Checks: `actionlint` exit 0. A copy of `check-ci-least-privilege.sh` pointed at
`release-gates.yml` exit 0. The new shell was dry-run on darwin with stubbed archives and a stubbed
`go`:
- digests and extraction worked;
- a tampered or missing N-1 row failed the step, with `n1-archive EXIT 1` as the log's last line;
- the HLQ loop kept every log and report and failed the step when one tuple failed;
- the real HLQ runner refused a dirty `--source`, and the refusal ended the log in `EXIT 1`.

NOT_RUN: the job on ubuntu, `npm install` on the runner image, the real archive build, and
whether its digest reproduces a published release.
