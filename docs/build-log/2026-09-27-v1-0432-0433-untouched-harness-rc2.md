## 2026-09-27 V1-0432 V1-0433: untouched-repository harness fixes; cobra frozen; preregistrations resealed for rc.2

Run-001 on Corvint (`1.0.0-rc.1`, harness `f24533a1`) scored orientation and consequence, then
aborted in the loop rehearsal. It left only `runs/run-001.started.json`.

- **V1-0433.** The Corvint pin `94ca556f` is the PR #287 merge, and its first-parent range adds
  that PR's sealed CEM. `dogfood change` correctly refuses `sealed-cem-in-change` there. Every
  Corvint merge carries its PR's seal commit, so the loop could never bind on a Corvint merge
  target.
  - Fix: a preregistration may name `loopTarget`. The loop and the CEM level then use
    `loopTarget^..loopTarget`; the corpus and its scored cases still end at the pin.
  - The Corvint preregistration names `a5a7412c`, the last PR #287 commit before its bind and seal.
    Its range changes four docs files and no `.corvint` path.
- **V1-0432.** With no L0 bind, the L3c control opened the missing map and raised
  `FileNotFoundError`. The preregistration's rule is that a mutation that does not happen is
  HARNESS-FAILURE.
  - Fix: every loop variant is HARNESS-FAILURE when L0 run 1 committed no bind, and the L4 mutation
    returns false when `local-outcome.json` is missing.
  - `run --work` now creates the directory instead of refusing.

A harness change is a new numbered preregistration deviation, and the earlier runs stay. The
harness digest is now `b67cc95abd92ffffeef9bf1bccfce4d0316e270716d6cd065948e802994f9eb2`.

| Preregistration | Change | New digest |
| --- | --- | --- |
| go-chi/chi | none: decision 0425 keeps run-001 as evidence and does not rerun it; it keeps harness `f24533a1` | `27625e854faf7dfb2379f59d38c7c082aeea19758e744874a52b20f200551e29` |
| spf13/cobra (new, held out) | pin `adbc8813901bba65827259daa8e22ff94ec1f30e`, 20 cases (20 orientation-eligible, 11 consequence-eligible), corpus `cccd80ec` | `9631e4ce822d4d17443e5c3bfe98e82c4685a70d6289f854ae5b8c730b98e5a4` |
| Corvint | deviation 1: `loopTarget`, `1.0.0-rc.2`, new harness | `5e44a9efe6bf86189e166c516962db5fe921d40ab63c9ae5a76c25eac721543b` |
| beamfall/core (private) | deviation 1: `1.0.0-rc.2`, new harness; run-001 kept | `c91b3a3cb4b3b769a7bc8c3caf050df399566544833699e08bd54588d30b2697` |

- **Inherited text.** The Corvint and beamfall/core preregistrations inherited the go-chi/chi text
  for the loop base, the L1 file and the README.md span. Each deviation now names the values for its
  own repository in `repositoryDifferences`.
- **Held out: spf13/cobra.**
  - Its corpus came from git alone, and no Corvint binary ran against it before the seal.
  - V1-0431 changes context ranking in the `1.0.0-rc.2` candidate. It came from the go-chi/chi
    misses, so its held-out validation is the rc.2 run on cobra.
- **Smoke, not a result.** The loop ran with the `1.0.0-rc.1` binary and this harness on a plain
  Corvint clone.
  - At `94ca556f`: every L0 run refused `sealed-cem-in-change`, and every variant was HARNESS-FAILURE
    with no crash.
  - At `a5a7412c`: L0 (three times), L0c and L3c were COMPLETE. L1-L6 and M1-M6 were REFUSED with
    their declared refusal: 12 designated, 12 informative, 0 false complete.
- **Seal checks.** All three preregistrations pass the harness's pre-run checks, run without
  starting Corvint: seal line and digest, corpus and harness digests, pin and module, and a
  byte-identical corpus re-derivation.

Rollback: revert the change. Then restore the private beamfall/core preregistration from
`superseded/preregistration-867f51a0.json` in the owner's local release evidence, which keeps its
original bytes.
