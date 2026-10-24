# Opt-in self-hosted runner for the CI shard matrix

Date: 2026-10-24. Lane: claude/self-hosted-runner (base b64eb76a1). Owner request of 2026-10-24, no
ticket. Requirements `AFP-V0-420..422` in `docs/specs/affected-plan-v0.md`. They take effect only
after the owner's `ci-control-plane` review of the `.github` change.

## Decision

`go-product-shard`, the eight-shard matrix on the 26-minute critical path, may run on a self-hosted
runner labelled `self-hosted, linux, x64, corvint-box`. It does so only when the repository variable
`CORVINT_SELF_HOSTED` is `on` and the run is a `merge_group` entry or a `pull_request` whose head
repository is this repository. Every other case runs on `ubuntu-24.04` with unchanged check names:
the variable unset, a push to `main`, a fork pull request, or any other event. No other job moves.

On the box, the job verifies its tools instead of provisioning them. The box has no passwordless
sudo. The hosted provisioning step is unchanged and runs only on hosted runners. The self-hosted
step requires these, and fails with a `::error::` that names the missing prerequisite:

- `/usr/local/go/bin/go env GOVERSION` must equal the pinned Go;
- `/usr/local/bin/rg` must be the pinned ripgrep;
- the fixed `PATH` must resolve `rg` to `/usr/local/bin/rg`.

The step keeps the same user-global Git configuration as the hosted step.

## Security rationale

The repository is PUBLIC. A self-hosted runner executes code from every run that reaches its
labels, so a malicious pull request could:

- persist on the host;
- read whatever the runner user can read;
- poison later jobs.

The routing expression admits only same-repository pull requests and merge-queue entries. Only
people with write access create those.

**The routing expression is not a security boundary against fork pull requests.** The
`pull_request` event runs the workflow file from the pull request's own head. A fork pull request
can therefore edit `ci.yml` and write `runs-on: [self-hosted, corvint-box]` on any job. The
expression in `main` never sees that run.

The variable does not gate that run either. A fork pull request can target the labels whenever the
runner is registered, even with `CORVINT_SELF_HOSTED` unset.

What stops that run is the repository's fork-approval policy. It is currently
`first_time_contributors`: a contributor whose first pull request was approved runs later
workflows with no approval. **Before registering the runner, the owner should set
Settings → Actions → General → "Require approval for all external contributors"**, and should
read every fork change to `.github/` before approving its run. A runner group limited to this
repository is further defense. Unsetting the variable is an operational rollback only. The security
rollback is to remove the runner registration, or its access to this repository.

Hardening of the host itself, with the reasons:

- **Workspace.** `GITHUB_WORKSPACE` persists between jobs on a self-hosted runner. The first step
  empties it, so a previous run's files never reach checkout or the tests.
- **Build cache.** The job never restores the shared `actions/cache` Go build cache: a box build is
  not a trusted cloud build. It builds in `$RUNNER_TEMP/go-build` with `GOENV=off`, so neither an
  earlier pull request's build output nor the host `go.env` reaches a later run. The runner empties
  `RUNNER_TEMP` for every job, so the merge-queue shards always start with a cold cache.
- **Processes.** The runner's orphan-process cleanup ends the processes it tracks. A hostile
  process can escape that tracking, so it is not an adversarial boundary.
- **Privileges.** Nothing in the job runs `sudo`, `apt` or a network install on the box. A test
  asserts this.
- **Statuses.** The box never posts commit statuses. Only the owner posts `ci-control-plane`.

This is hygiene, not isolation. Code that a job runs can still write to the runner account's home,
the tool cache or the runner installation, and so affect a later merge-queue result. The test
sandbox's Landlock rules confine reads, not writes, and an undeclared package runs unconfined. A
persistent box is therefore trusted only as far as every run that reaches it is trusted: code from
the repository's writers, and fork code the owner approved. Real isolation needs a fresh
environment for each job that is destroyed afterwards, such as a just-in-time runner in a
disposable VM or container. `--ephemeral` registration alone does not reset the machine. That is
outside this change.

Recommended, not enforced by the workflow:

- run the runner as a dedicated unprivileged user that holds no credentials;
- register several runner instances, so the eight shards do not run one after another.

## Host prerequisites

- Runner labels `self-hosted`, `linux`, `x64`, `corvint-box`.
- `/usr/local/go` is the pinned Go, currently go1.27.2. It must follow `setup-go`'s `go-version`;
  `TestAFPV0421SelfHostedShardsVerifyAndNeverProvision` fails on a mismatch, so the go1.27.3 pin
  change must update the verification step too.
- `/usr/local/bin/rg` is the pinned ripgrep, 15.2.0.
- Linux Landlock ABI 2 or later. The box reports ABI 7.
- `bash`, `git`, `jq` and a C toolchain for `-race`.

## Cost drift (AFP-V0-040)

Box timings are not cloud timings. A self-hosted shard uploads no timing stream. Instead it uploads
a `ci-shard-costs-runner-<index>` marker artifact. `ci-shard-cost-drift` counts the markers and
abstains with the reason "N of 8 shards ran on the self-hosted runner". It does not compare them
against the cloud table. If a marker is lost, the missing stream still makes the report abstain.

## Failure modes and rollback

- If the box is offline, the shard jobs stay queued, for up to GitHub's 24-hour queue limit. The
  job's 75-minute timeout starts only when a runner picks the job up. The operator unsets the
  variable, cancels the queued run and starts a new one (a new push or queue entry).
- If the host drifts, verification fails with a named prerequisite.

The operational rollback is to unset `CORVINT_SELF_HOSTED`, or set it to anything but `on`. That
returns this repository's runs to the cloud with no code change. A revert of this change is also
safe. The security rollback is to remove the runner registration, because a registered runner
stays reachable by an approved fork pull request.

## Review

One Codex review round (gpt-6-astra, read-only) raised three findings, all accepted as
documentation corrections:

- **The variable does not gate fork access.** The approval policy must be set before the runner is
  registered. Removing the runner, not unsetting the variable, is the security rollback.
- **Workspace clearing is not isolation.** This is now stated, with fresh per-job environments
  named as the isolation route.
- **Queue timing.** A queued job waits up to 24 hours, not 75 minutes. Recovery is cancel and
  re-run.

No workflow logic changed in response.

## Evidence

- The tests are in `tools/ci-reuse-plan/self_hosted_runner_test.go`. They pin the routing
  expression, the cloud fallback for push, fork and other events, the unchanged hosted
  provisioning, the verification-only self-hosted step and the drift abstention. The abstention
  test runs the drift script under bash.
- Runtime behaviour on the box is `NOT_OBSERVED` until the owner registers the runner.
- Hosted routing with the variable set is `NOT_OBSERVED` until the owner sets the variable.
