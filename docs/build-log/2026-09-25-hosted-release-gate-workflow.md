## 2026-09-25 decision 0415: hosted release-gate workflow

The owner asked for the release gates to run on GitHub instead of the loaded local Mac, so that
several can run at once. The new `.github/workflows/release-gates.yml` is owner-dispatched with
inputs `sha` (full 40-hex, checked against `git rev-parse HEAD`) and `release` (artifact label).
It follows the `pr-tests-qualification.yml` pattern (decision 0320, `AFP-V0-017`).

- Each policy-v3 gate runs in its own matrix job. `full-gate` runs on ubuntu-24.04 and macos-15.
  `companion-release` runs on macos-15 only, because its bundle targets `darwin/arm64` and
  smoke-runs the installed binaries.
- `release-checklist --pre-promotion` needs the archive witness and gate receipt that `make gate`
  records in the same clone. So it runs after `make gate` inside each `full-gate` job, with its own
  log and artifact.
- Every log starts with gate, sha, runner, image and UTC time, and ends with `<GATE> EXIT <code>`.
  It is uploaded with its sha256 even on failure. Logs are written under `RUNNER_TEMP`, because
  `make gate` records its receipt only from a clean worktree.
- Owner call: "linux evidence is fine, run full-gate on macos too". An ubuntu-24.04 `PASS`
  qualifies a gate. `full-gate` needs `EXIT 0` on both legs at the same sha.
- `sourceIdentity` is capped at 128 bytes, so an `EXTERNAL_ATTESTATION` puts the run URL there and
  the log sha256 in `evidence`.

Checks: `actionlint` clean. The log helper was exercised locally with a failing command: its first
and last lines were as specified, the exit code propagated, and `shasum -c` verified the digest.
The focused doc checks pass. NOT_RUN: the workflow itself, which can be dispatched only from
`main`. The hosted runtimes, the macOS Go resolution through `/usr/local/go`, and whether each gate
passes on the hosted images are therefore unobserved.
