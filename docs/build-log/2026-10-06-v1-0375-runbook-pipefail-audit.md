# 2026-10-06: V1-0375 release-runbook pipeline audit

## Intent

Ticket V1-0375 (audit 2026-09-26, REL-02): a release-runbook pipeline such as
`gate | tee log` must not report success when the gate fails, and a passing gate must still keep
its whole log. Owning contracts: `SOP-V0-012` (exact runbook commands) and `CCF-V1-007` (N-1
replay).

## Finding

The step 8 fix had already landed in PR #267 (commits 4c29b211, 2dbc4f5c, 6e8a68fe and ff6ec6fc).
Both `tee` pipelines run as `bash -o pipefail -c '... 2>&1 | tee "$1"' _ "<log>"`. This audit
checked the rest of the runbook and the scripts it tells the operator to run at b7b21205. It found
no remaining masked gate:

- Runbook steps 1 to 10 and the rollback steps run every gate command unpiped. The exceptions are
  the two step 8 `tee` pipelines and step 10's
  `bash -o pipefail -c 'git rev-list --first-parent origin/main | grep -x FULL_COMMIT'`; all three
  use pipefail.
- `script/check-hostile-regressions.sh` captures each `go test` status as `go_status` from a
  command that redirects to a file rather than piping. It decides FAIL from a marker file, so the
  `matrix | while` subshell cannot mask a failure.
- `make core-n1-replay` chains its recipe with `&&` and pipes nothing.
- `script/release-checklist`, `script/check-install-lifecycle.sh` and
  `script/check-release-artifact-reproducibility.sh` pipe only to extract values (`shasum | awk`,
  `wc | tr` and similar). A failed producer leaves the value empty, and the comparison then fails
  closed. `script/corvint-companion-release-gate` and `script/public-release-check` run under
  bash `pipefail`.
- The `SOP-V0-012` and `CCF-V1-007` text does not show the masked form, so the specs are unchanged.

## Evidence

The repository has no maintained test that runs runbook blocks, so this is a scratch check, not a
new test. The check took the two step 8 lines verbatim from `docs/RELEASE-RUNBOOK.md` and swapped in
a scratch log directory. Each line ran under `zsh`, `sh` and `bash` against a stub
`script/check-hostile-regressions.sh` and a stub `make` that write to stdout and stderr:

| stub exit | zsh / sh / bash exit, both lines | retained log |
|---|---|---|
| 1 | 1 / 1 / 1 | all 3 lines, stderr included |
| 0 | 0 / 0 / 0 | all 3 lines, stderr included |

Control: the unfixed form `sh -c 'script/check-hostile-regressions.sh 2>&1 | tee /dev/null'`
exited 0 with a failing stub, which reproduces the audit finding.

The real hostile matrix and N-1 replay were NOT_RUN. No release qualification is implied.
