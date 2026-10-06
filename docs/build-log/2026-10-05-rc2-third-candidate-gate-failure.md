# 2026-10-05: 1.0.0-rc.2 third candidate fails the hosted macOS gate

## Result

Candidate `7a83f52d6590a7919c6e1bcb65c68e0e51679363` (PR #618) failed one gate and was not tagged: the
hosted macOS `full-gate` job (Release gates run 37403195661). One test failed:
`TestStableS0EPublicCases/normal-exit-lingering-descendant-contained`, with `git-diff-failed` at stage
`repository` after 8 operations.

Every other hosted job passed. Every local runbook step passed on darwin/arm64, including `make gate`,
the three HLQ tuples and the linux/arm64 container steps. Its evidence is retained under the private
release-evidence directory `1.0.0-rc.2-7a83f52d`.

## Defect: the S0E shim runs a different Git (V1-0846, test-only)

- The case's shim sets `PATH=/bin:/usr/bin` and then execs the verifier's Git argv. Its binary is the
  bare name `git`.
- On the hosted runner that resolves to Apple `/usr/bin/git`, not the `git 2.55` on the job's PATH
  that the verifier itself runs. The canonical diff passes the `--attr-source` global option, which
  older Git releases reject, so the product correctly reports a failed Git.
- The runner's `/usr/bin/git` version was not printed. That it rejects `--attr-source` is inferred
  from the reproduction below and the runner's Xcode 16 toolchain.
- The local release host's `/usr/bin/git` is 2.54, so the case passed there.
- A fake Git that rejects `--attr-source`, placed first in the shim's PATH, reproduces the exact
  hosted signature locally.
- Fix: the harness resolves the binary with `exec.LookPath` before handing it to the shim, so the shim
  runs the verifier's Git. The same reproduction passes with the fix.

## Correction to the second candidate's record

`2026-10-05-rc2-second-candidate-gate-failure.md` attributed the same hosted failure to a refused fork
(V1-0843). That was wrong for the hosted runs:

- The stderr side-file diagnostic added by V1-0843 logged nothing on this candidate's hosted run.
- A capped `RLIMIT_NPROC` produced the same signature locally, but by a different cause.

The S0E case was added after rc.1, so it had never passed on hosted macOS. The side-file diagnostic
stays, because a fork-starved host would still fail the case.
