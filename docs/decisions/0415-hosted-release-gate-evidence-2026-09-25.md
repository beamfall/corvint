# Decision 0415 — Release gates run on hosted runners

Date: 2026-09-25. Status: accepted. Authority: repository owner request "as this is open source why
can't we have the gates run on github so they don't hammer this machine and multiple ones can run
at once?" and owner answer "linux evidence is fine, run full-gate on macos too" (2026-09-25).
Amended 2026-09-26 by the owner decision recorded as decision 0420 item 2: linux/amd64 becomes a
Core platform through a native run on a hosted ubuntu-24.04 runner, added to this workflow (item 8).

## Context

Release policy version 3 (decision 0382) requires five gates: `focused-docs`, `full-gate`,
`interop-gate`, `companion-release` and `release-checklist`. Until now their evidence came from
clean-clone runs on the owner's Mac, which is heavily loaded. `make gate` alone takes about 35
minutes there, and the gates cannot run side by side without slowing each other. `beamfall/corvint`
is public, so standard GitHub-hosted runners cost nothing. Decision 0320 (`AFP-V0-017`) already runs
owner-dispatched qualification on the same hosted platform as CI.

`PRS-V1-004` names linux/amd64 a Core platform only with retained native install-lifecycle and
focused-test evidence on the candidate bytes; otherwise it is reported FALLBACK. The owner's Mac
cannot produce native linux/amd64 evidence, and decision 0420 item 2 chooses a hosted ubuntu-24.04
run over accepting FALLBACK.

## Decision

1. **Workflow.** `.github/workflows/release-gates.yml` follows the pattern of
   `pr-tests-qualification.yml`:
   - It runs on `workflow_dispatch` only, with `permissions: contents: read`.
   - Checkout uses `persist-credentials: false`, and every action is pinned by commit SHA.
   - It uses the same Go environment block as CI.
   - It commits, pins and publishes nothing.
2. **Inputs.** `sha` is a full 40-hex commit. The job checks it out with full history and refuses
   unless `git rev-parse HEAD` equals it. `release` is a label used only in artifact names.
3. **One job per gate.** Each gate runs in its own matrix job on its own runner, so the gates run
   concurrently. The commands are the policy commands, unchanged:

   | Gate | Runner | Command |
   |---|---|---|
   | `focused-docs` | ubuntu-24.04 | `make spec-requirements-check requirement-definitions-check traceability-tests-check decision-numbers-check line-citations-check` |
   | `full-gate` | ubuntu-24.04 and macos-15 | `make gate` |
   | `interop-gate` | ubuntu-24.04 | `make interop-gate` |
   | `companion-release` | macos-15 | `make companion-release-gate` |
   | `release-checklist` | each `full-gate` runner | `script/release-checklist --pre-promotion` |

   - `companion-release` runs on macos-15 only. The bundle targets `darwin/arm64` and smoke-runs its
     installed binaries, so no Linux runner can execute it.
   - `release-checklist` passes only when this clone holds the archive witness and gate receipt
     that `make gate` records under the Git directory. So it runs as a later step in each
     `full-gate` job, and its log and artifact are separate.
4. **Provisioning.** Linux runners are provisioned exactly as the `go-product` job in `ci.yml`.
   macOS runners differ in two ways:
   - ripgrep comes from Homebrew;
   - the step also proves that `worksource.PlatformPath`'s darwin search path resolves go1.27.1
     first.

   A fresh runner has no gate-ledger records and no Go test cache, so every `make gate` step runs.
5. **Logs.** Each gate writes a log under `RUNNER_TEMP`, outside the checkout. It is kept there
   because `make gate` records its receipt only from a clean worktree.
   - The first line names the gate, the sha, the runner label, the runner image and the UTC time.
   - Further header lines give the run URL, the tool versions and the command.
   - The last line is exactly `<GATE> EXIT <code>`.
   - The log is uploaded as an artifact, with its `shasum -a 256` file beside it, even when the gate
     fails.
   - The job fails when the code is non-zero.
6. **Evidence contract.** An `EXTERNAL_ATTESTATION` for a gate cites the hosted run and its log.
   - `sourceIdentity` is the run URL on the log's second line, including the attempt.
   - The log's sha256 goes in `evidence`.
   - Both together would not fit `sourceIdentity`, which is capped at 128 bytes
     (`internal/tasks/wire/limits.go:10@9ec0a45d`).

   The attestation is equivalent to a local clean-clone run of that gate only when:
   - the log's first line names the attested sha;
   - its last line is `<GATE> EXIT 0`;
   - the recorded sha256 matches the downloaded log.

   Any other log is `FAIL` evidence, or no evidence.
7. **Owner call: Linux evidence qualifies.** The owner's answer is "linux evidence is fine, run
   full-gate on macos too".
   - A `PASS` on ubuntu-24.04 qualifies `focused-docs`, `interop-gate` and `release-checklist`.
   - `companion-release` has only the macos-15 leg, so that leg qualifies it.
   - `full-gate` also runs on macos-15. Both legs must end in `full-gate EXIT 0` at the same sha
     before `full-gate` is attested.
8. **linux/amd64 Core platform job (decision 0420 item 2).** The `linux-core` job runs on
   ubuntu-24.04 in the same dispatch. It reuses the gate job's input check, checkout, provisioning
   and log helper by YAML alias, so its logs follow items 5 and 6. It is not a release-policy gate.
   It is evidence for `PRS-V1-004` and `PRS-V1-006`. Each log, and what an `EXIT 0` proves:

   | Log | Proves |
   |---|---|
   | `release-archive` | `conformance/release-artifact-v0 archive` at the sha (runbook steps 5 and 6) built the archives and `SHA256SUMS` verifies. The log prints `SHA256SUMS`, the extracted binary's digest and `--version`. |
   | `n1-archive` | The published `corvint_linux_amd64.tar.gz` of N-1 matches that release's published `SHA256SUMS`, and so does its extracted binary. N-1 is the newest `v*` tag merged into the sha that does not contain it. |
   | `install-lifecycle` | `script/check-install-lifecycle.sh` passes on the sha's linux/amd64 archive (runbook step 8). |
   | `install-lifecycle-n1` | The same script passes on the N-1 archive, upgraded to the sha's binary (runbook step 8). |
   | `host-lifecycle-cli`, `-codex`, `-claude-code` | The `HLQ-V1` runner (`conformance/host-lifecycle-v1`) on the sha's binary, with N-1 as `--base-corvint`. It is a tuple pass only when the `SUMMARY` line of its raw report has `status=PASS` and `passed=9`. |
   | `hostile-regressions` | `script/check-hostile-regressions.sh` passes natively at the sha (runbook step 8). |

   - The hosts are the exact versions of the Core plugin rows in `integrations/compatibility.json`:
     `@openai/codex@0.153.2` and `@anthropic-ai/claude-code@2.1.267`, installed with npm.
     `HLQ-V1-003` gives each host a private home and config directory. `HLQ-V1-004` calls the
     installed hook commands directly. So no secret, credential or model call is used.
   - The artifact holds every log with its sha256, plus the three raw `--report` files. Committing
     those reports under `conformance/host-lifecycle-v1/results/` (`HLQ-V1-008`) is a separate change.
   - The job's archives are the candidate bytes only when the `corvint_linux_amd64.tar.gz` row in
     the `release-archive` log equals that row in the release's own `SHA256SUMS`. When the rows
     differ, the job is evidence for the sha's source and not for the release.

## Rollback

Delete `.github/workflows/release-gates.yml`. Release evidence then comes only from local
clean-clone runs again. No store record, spec or other workflow depends on the file.
Without the `linux-core` evidence, or equivalent evidence from another native linux/amd64 host,
linux/amd64 is reported FALLBACK (`PRS-V1-004`).

## What this does not claim

- It does not claim that the workflow has run. It can be dispatched only once it is on `main`, so
  its first run, its runtimes, and whether each gate passes on hosted runners are `NOT_RUN`.
- It does not change any gate command, the release policy, the store, or `ci.yml`.
- It does not make hosted runs mandatory. A local clean-clone run remains valid evidence.
- It does not qualify platforms other than the runner images named in each log.
- It does not claim the hosted Node.js version matches a local host. The `tools:` header line
  records the version used.
- It does not make the workflow a required check or a `pull_request` trigger. Under decision 0390,
  this `.github/` change merges only after an admin posts `ci-control-plane` `success` on its head.
- It does not claim that `linux-core` has run. Whether npm installs the hosts on the runner, and
  whether each log ends in `EXIT 0`, are `NOT_RUN`.
- A `host-lifecycle` pass does not raise any host tuple above FALLBACK (`HLQ-V1-005`), and it
  involves no model session. Network or background calls the hosts make on their own were not
  observed.
- It does not add linux rows to `integrations/compatibility.json` or `CCF-V1-007` N-1 replay.
