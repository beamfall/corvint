# Decision 0415 — Release gates run on hosted runners

Date: 2026-09-25. Status: accepted. Authority: repository owner request "as this is open source why
can't we have the gates run on github so they don't hammer this machine and multiple ones can run
at once?" and owner answer "linux evidence is fine, run full-gate on macos too" (2026-09-25).

## Context

Release policy version 3 (decision 0382) requires five gates: `focused-docs`, `full-gate`,
`interop-gate`, `companion-release` and `release-checklist`. Until now their evidence came from
clean-clone runs on the owner's Mac, which is heavily loaded. `make gate` alone takes about 35
minutes there, and the gates cannot run side by side without slowing each other. `beamfall/corvint`
is public, so standard GitHub-hosted runners cost nothing. Decision 0320 (`AFP-V0-017`) already runs
owner-dispatched qualification on the same hosted platform as CI.

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

## Rollback

Delete `.github/workflows/release-gates.yml`. Release evidence then comes only from local
clean-clone runs again. No store record, spec or other workflow depends on the file.

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
