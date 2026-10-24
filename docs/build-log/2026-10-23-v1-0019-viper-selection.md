# V1-0019: spf13/viper selected, admitted and frozen

Decision 0555 records the owner-delegated choice of spf13/viper at
`528f7416c4b56a4948673984b190bf8713f0c3c4` as the untouched public repository for the next
V1-0019 run. urfave/cli is no longer held out (decision 0539). The selection was committed
(`a4ce7bec`) before the clone and before any corpus derivation.

The bounded declared and historical corpus identity check (decision 0432 scope) found no prior
viper use. `benchmarks/untouched-repository-v1/viper/admission.json` retains the 21 metadata path
digests, the ref inventory digest, the ledger and untracked-file searches, and their limits.

The unchanged harness `b67cc95a` derived 20 single-parent cases from the pinned tree:

| Workflow | Cases |
|---|---|
| Orientation (`UC-TASK-ORIENTATION`) | 20 eligible |
| Consequence (`UC-CHANGE-CONSEQUENCE`) | 20 run, 13 scored |
| Completion (`UC-EVIDENCE-CARRYING-COMPLETION`) | 6 CEM-level and 6 loop-level designated cases, with their controls, at PIN^..PIN |

Seven cases change no root-module Go file, so they run under E5 but are not scored for
consequence. The nested `remote/` module is never critical. viper's subjects are short (for
example "clean-up" and "better naming"). The task rule takes them as written, and the rule was
not changed after this was observed. PIN^..PIN changes only two `.github/octoslash` files. The
loop is therefore a structural evidence check on non-Go hunks, with the README.md 1-3 literal
citation fixture.

Digests: corpus `3d39b9338c5a8b00f64062892a12d6786a116936ad2ce4b1ff440c80444b8f65`,
preregistration `e2d6a26a77a0d3a1488dccc4d9f897e11fdef738c4409957b605092769e2be5e`, target
`1.0.0-rc.3`. A read-only Codex review returned APPROVE on these exact digests before
`preregistration.sha256` was written. It re-derived the corpus byte for byte.

Nothing was executed. No Corvint invocation, source test or scoring ran against viper.

Execution prerequisites:

- The harness calls `os.getloadavg()` after it writes the started marker. A native Windows Python
  would therefore abort and burn the run id, so the Windows box must run the harness inside WSL2
  Ubuntu (linux/amd64).
- The candidate directory must be the verified `1.0.0-rc.3` release-artifact-v0 directory that
  carries `corvint_linux_amd64.tar.gz`.
- If V1-2281 or the V1-0777 reader changes `harness.py` first, a numbered deviation and a reseal
  must come before the run.
