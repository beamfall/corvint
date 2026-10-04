## 2026-10-04 V1-0711 AHI-044: fail-open conformance matrix for every hook adapter

Human-owned intent: the owner asked to start V1-0711, filed from the claude-mem finding that a hook
must never block or hang its host. The ticket asks for a table-driven test that runs every shipped
hook adapter under these faults:

- the corvint binary absent
- an unavailable index
- empty and malformed stdin
- stdout closed before the write
- a host deadline shorter than the adapter's budget

Every case must exit with the host's non-blocking status and name its cause, and must not write
outside the declared ledgers. The test must also count spawns and prove that no adapter starts a
login shell.

### Findings

The matrix found three real defects:

1. **Closed stdout.** When stdout was closed before the write, the Go adapter died from `SIGPIPE`.
   The host saw a signal death with no named cause. The Gemini wrapper raised an unhandled `EPIPE`
   in the same case.
2. **Gemini input codes.** The Gemini wrapper reported empty and malformed stdin under a generic
   code, not `malformed-hook-json`.
3. **Scratch leak.** When the adapter watchdog or the dogfood event deadline abandoned a Git status
   read, the read's private `$TMPDIR/corvint-git-status-*` metadata copy stayed behind, because
   `os.Exit` runs no deferred cleanup. A first fix registered the scratch and removed it at exit,
   but it still leaked. Removal ran after deregistration, so an exit could slip between the two.
   The removal now happens before deregistration, under the lock. Directories are now created with
   `Mkdir` instead of `MkdirAll`, so an abandoned read cannot recreate a scratch that the exit path
   already removed.

A fourth finding is filed, not fixed. Git children abandoned at a deadline can outlive the adapter.
`os.Exit` runs before exec's asynchronous context cancel, which is the process-group `SIGKILL`.
Most runs show 0–43 ms. Three slow-Git runs out of 231 showed 3.8–4.8 s, which is the shim's
`sleep` surviving. The fix needs a live-group registry shared by many spawn sites, so it is tracked
as V1-0734 (labels `agent-memory`, `bugs`).

### Change

- `cmd/corvint/signals_unix.go` `notifyBrokenPipe` makes a broken pipe a write error instead of a
  signal death. Unlike ignoring the signal, this setting is not inherited by children.
- `cmd/corvint/host_exit.go` `hookStdout` reports a failed stdout write as `Corvint FALLBACK
  degraded: hook-stdout-unwritable` on stderr. Hook adapters still exit 0. The `pi-tool` command,
  which is not a hook, keeps its own failure status.
- `internal/gitstatus/scratch.go` tracks live scratch directories. `exitProcess` in
  `cmd/corvint/host_exit.go` calls `gitstatus.CloseScratch()` before `os.Exit`. `main.go` changes
  only in place, so the line citations into it stay valid.
- `integrations/gemini-cli/hooks/corvint-hook.mjs` handles stdout and stderr stream errors, and
  names unparseable input `malformed-hook-json`.
- `cmd/corvint/host_adapter_fail_open_test.go::TestAHI044HookAdaptersFailOpen` builds the real
  binary and reads every Claude Code and Codex command from the shipped `hooks.json` files: 11
  invocations. It runs each one under seven faults:
  - cold start without a snapshot
  - empty stdin
  - malformed stdin
  - stdout closed before the write
  - unavailable index
  - a host that never closes stdin
  - slow Git on `PATH`

  The sandbox `PATH` holds only shims. The `git` shim logs every spawn and its pid, and each run
  may spawn at most 15. Shims for `sh`, `bash`, `zsh`, `dash`, `ksh`, `fish`, `csh` and `tcsh`
  fail the test if they run. The test waits for abandoned Git children to exit before it compares
  the repository, home and temporary trees.

  The fixture's `.corvint/.gitignore` ignores both ledgers, so the self-observation writer is live
  and the ledger allowance is exercised. At least one run across the matrix must write the
  self-observation ledger, or the test fails.
- `internal/gitstatus/scratch_test.go` checks the scratch registry without timing:
  `TestAHI044ScratchRemovedAtClose` checks release, close of an abandoned directory, and refusal
  after close. `TestAHI044ScratchCloseRacesReads` races 32 create/release loops against
  `CloseScratch` and requires an empty parent.
- A Gemini case under `TestHostAdapterJavaScriptHosts` covers empty and malformed input, an absent
  binary and a closed stdout.
- `AHI-044` and its trace row are in `docs/specs/agent-harness-integration-v0.md`.

### Evidence

- `TestAHI044HookAdaptersFailOpen` passed three times in a row (`-count=3`, 231 subtests), and
  again after the review fixes.
- Both scratch tests pass, including under `-race`.
- After merging `origin/main`, a run at load average about 35 with `-parallel 48` failed one healthy
  cold start, which named `repository-probe-timeout`. The cold-start and unavailable-index cases now
  accept that code and any deadline code as load-derived (decision 0082). The matrix then passed
  three times at load average about 50.
- Healthy Git spawn counts per invocation:

  | Invocation | Git spawns |
  |---|---|
  | session start | 15 |
  | prompt | 15 |
  | stop | 10 |
  | Codex session end | 10 |
  | post-tool | 5 |
  | empty or malformed input | 0 |

- No shell shim ran in any case.
- Mutation checks, each reverted before commit:
  - Removing the `notifyBrokenPipe` call failed all 11 closed-stdout cases with
    `signal: broken pipe`.
  - Removing the `gitstatus.CloseScratch()` exit call failed 18 of the delay-case subtests (`-count=2`) by
    leaking `tmp/corvint-git-status-*`.
  - Renaming the Gemini stdout error handler failed the Gemini case.
  - Renaming the fixture's ignored ledger to `x-self-observations.jsonl` failed the matrix with
    `no run wrote the self-observation ledger, so the ledger allowance went unexercised`.

### Limits

These cases are exempt, documented in `AHI-044`:

- **Claude Code and Codex, binary absent.** The host's own spawn fails, and no Corvint code runs.
- **OpenCode.** It is an in-process plugin, not a hook. Its missing-binary path is covered by the
  existing OpenCode cases.
- **Roots outside Git.** A root outside a Git repository stays silent `{}` (decision 0178).
- **A host deadline shorter than the declared kill.** This criterion is replaced, not met, and needs
  explicit acceptance when the ticket closes. No budget the adapter derives can meet a host deadline
  shorter than the declared kill. `TestAHI017AdapterHostKillMatchesDeclaredHooks` keeps the shipped
  timeouts equal to the declared kill table, and the two delay cases force each adapter onto its own
  deadline first. A host that `SIGKILL`s early would, by inference, leave scratch, temporary files
  and Git children behind; that path was not run.

Elapsed time is logged, not asserted, because 77 parallel subtests run under load (decision 0082).
Observed maxima were about 1.96 s against the 2 s declared kill (session start with slow Git), and 0.69 s against session-end's
1 s. V1-0734 tracks the Git children that outlive the adapter. Spawn counts see only `PATH`
lookups, so an absolute-path spawn or a re-exec of the binary itself would go uncounted. Real Claude
Code, Codex and Gemini hosts were not run (`NOT_OBSERVED`).

Rollback: delete the test, the Gemini case, `AHI-044` and its trace row, and regenerate
`REQUIREMENTS.tsv`. Reverting `host_exit.go`, `notifyBrokenPipe` and the scratch registry restores
the signal death on a closed stdout and the scratch leak.
