# Triggered automation contract

This page says which Corvint commands can run as a stateless, read-only step in a trigger that
someone else owns: a CI job, a git hook, or a team automation that fires on push, branch change,
pull request or merge. It is operator guidance for existing commands. It adds no command, no
requirement and no new status label, and it is not a release or qualification claim. Every row
below was checked against the source it cites and run from a fresh clone on 2026-09-23 (darwin
arm64, Go 1.27.1, at `01b6804`); the worked example at the end is that run.

## What a triggered run is, and what it is not

- **Corvint runs no always-on component.** Each step below is one foreground process that starts,
  reads, prints and exits. Nothing is installed as a service, daemon, launch agent or watcher, and
  nothing keeps running between triggers. The 1.0 Core scope accepted by decision 0373
  (`docs/decisions/0373-corvint-1.0-scope-ratified-2026-09-23.md`) defines the Core loop as one
  native Go binary with no account, network, hosted service, database service, embeddings or daemon
  (`docs/specs/corvint-1.0-product-and-release-v1.md:90@efcd3ea7`) and rejects a permanent daemon in the
  default binary (`docs/specs/corvint-1.0-product-and-release-v1.md:206@b7b9e11e`). Decision 0081
  (`docs/decisions/0081-local-admin-console-accepted-2026-09-07.md:24-31@4e183223`) keeps even the optional
  console operator-started, foreground only, loopback only, with no service and no durable state.
  The trigger, its schedule, its runner and its retention belong to the automation host, not to
  Corvint.
- **A triggered run grants no authority.** A step's exit code and JSON are evidence for the policy
  that invoked it; they do not approve, merge, accept intent or waive a gate. Project-owned authority
  outranks syntax, history and learned traces (AGENTS.md invariant 3). Decision 0081 states that the
  console "carries no authority" (line 25), and decision 0373 took formal host FULL and protected
  authority off the Core path (row 6, line 19); a CI or hook invocation of the same binary gains
  nothing either path lacks. In particular: `affected` never claims omitted tests are safe to skip
  (`cmd/corvint/help.go:979-980@d5d6e05c`); a valid CEM establishes provenance completeness, not semantic
  correctness ([Change Evidence Map](CHANGE-EVIDENCE-MAP.md)); and `witness` exits 0 on a
  report whose obligations are all unproven (worked example below).
- **Network.** "Local-only; no network or telemetry" (root help support boundary)
  (`cmd/corvint/help.go:413@f1b66bd9`). Fetching the commits a step needs is the runner's job.

## Mutation classes (AGENTS.md invariant 4)

Invariant 4 allows read commands exactly two bounded writes, both under `.corvint/`. This page uses
three classes:

| Class | Meaning |
|---|---|
| `R0` | Writes nothing: no repository, trace, `.corvint/` or Git-directory state. |
| `R0+SOL` | As `R0`, except that an `unsupported-*` refusal may append one bounded row to `.corvint/self-observations.jsonl` (the self-observation ledger). |
| `W` | Writes local state beyond the two ledgers. Not a read-only step; listed only as an exclusion. |

The self-observation append happens only when that ledger path is gitignored by the root
`.gitignore` or `.corvint/.gitignore` together with its `.self-observations.*` temporaries
(`internal/observations/observations.go:309-311@f504ccce`, `internal/observations/observations.go:733-740@b40a3bba`).
At `01b6804` the root `.gitignore` names the ledger but not its temporaries, so in a fresh clone
with no `.corvint/.gitignore` (which `corvint index` creates) no row is written; the worked example
and a forced `unsupported-impact-path-suffix` refusal both left `.corvint/` unchanged. A ledger
storage failure never alters the response (`cmd/corvint/help.go:419-420@3c9a8d56`).

The second ledger, `.corvint/unplanned-reads.jsonl`, is written only by host-adapter hook paths
while the operator marker `.corvint/unplanned-reads.enabled` exists
(`cmd/corvint/host_adapter.go:379@45edc402`, `cmd/corvint/host_adapter_experimental.go:111-128@765210a5`) and the
marker only by `corvint reads` (`cmd/corvint/reads.go:92-96@19937f8b`). None of the commands below writes
either file.

## Commands safe as a triggered step

Each takes `--root PATH` (default: the current directory) and reads the checkout at `HEAD`. Every
base must be a full 40-hex commit id that is present in the clone, so a shallow CI checkout must
fetch enough history to contain it (`cmd/corvint/help.go:925@a892984d`). On an argument or input refusal each
prints one JSON object with `"ok": false` to stderr and exits 2.

Scope: the table lists the steps that answer a question about one change against a fixed base,
which is what a CI job, git hook or team automation asks. Other verbs that help also describes as
read-only (`init`, `adopt`, `query`, `docs`, `harness`, `lrf` at `cmd/corvint/help.go:415-416@a54374e4`, and the
experimental `batch`, `depsource`, `necessity`, `surprise`, `answerability`, `kernel` and `reads`
in the command-maturity list at `cmd/corvint/help.go:366-403@7d74726e`) are not classified here. They need a per-session task, packet or
request as input, or they are experimental without a stability promise. This page makes no claim
about their safety as a triggered step.

| Step | Class | Exit codes | Output profile (stdout) | Required inputs | 1.0 label |
|---|---|---|---|---|---|
| `corvint affected --base BASE` | `R0` | 0 plan written; 2 refusal or error | `affected-plan/0`, one canonical JSON line, `"mutates": false` | full `BASE`; a checkout whose `HEAD` is the change head | Core |
| `corvint cem verify --map MAP --expected-base BASE --target TARGET --artifacts ROOT` | `R0+SOL` | 0 accepted; 1 rejected (`"ok": false`); 2 refusal or error | JSON with `"tool": "cem-verify"`, `"mutates": false` and the `stableVerification` result; the map must be `cem/1.0` (decision 0525 refuses `cem/0.x` with `unsupported-spec`) | CEM path, full `BASE` and `TARGET`, absolute artifact root | Core |
| `corvint cem status` (same arguments, plus optional `--max-unknown N`, `--max-mechanical N` and the experimental `--max-deferred-intents N`) | `R0+SOL` | as `cem verify`; an exceeded ceiling or a deferral-record issue exits 1; `"state"` is `accepted`, `rejected` or `unsupported` | JSON with `"tool": "cem-status"`, `"mutates": false` | as `cem verify` | Core |
| `corvint witness --base BASE [--cem MAP] [--evidence FILE]... [--evidence-in-repo PATH]... [--json] [--check]` | `R0+SOL` | 0 report written, whatever it says; with `--check`, 1 when admission is not `COMPUTED` or any obligation is not closed; 2 refusal or error | `corvint-witness/0` (text headed `WITNESS corvint-witness/0`, or JSON with `--json`) | any `BASE` revision Git resolves to a commit (the report carries the full object ID); optional CEM path; a `--head` other than the checked-out `HEAD` is read from the object store and bound `COMMIT-PINNED` (AGW-V0-007); each `--evidence` test-run document (a Playwright receipt or RSpec provider document) bound to the head tree adds non-closing `CALLER_REPORTED` witnesses and an unreadable one exits 2 (AGW-V0-008, AGW-V0-010); `--evidence-in-repo` names one by a checkout-relative path and refuses an absolute, `..` or link-escaping path with exit 2 (AGW-V0-011) | experimental |
| `corvint impact --base BASE [--range-profile expanded-256]` | `R0+SOL` | 0 packet written; 2 refusal or error, including a range over capacity (`unsupported-impact-range`) | JSON with `"tool": "impact"`, `"mutates": false`; `context.profile` is `corvint-range-impact/0` | full `BASE`; the default range capacity is 100 paths, 256 with the explicit profile | Core |

Where each row was verified:

- `affected`: profile `cmd/corvint/affected.go:38@af9fb447`; exits `cmd/corvint/affected.go:817-880@4f765e59`; its
