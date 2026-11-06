# Selection correctness: review round d

Owner intent: correct the three P1 findings from review round d of the selbugs lane. The
reviewer chose the remedy for two of them. Lane base `6f12f4704235dc8f9bea19fc12995bfb68c746c3`
and every requirement ID stay as they were. Each fix fails closed. This record describes local
evidence only; the next independent review and integration belong to the orchestrator.

| Finding | Requirement | Change and proving evidence |
|---|---|---|
| P1: `go test ./a ./b` checked only the packages holding darwin-only files | ASD-V0-2403 | `darwin_split` now parses the whole command first and records every package argument. A darwin-only file outside those arguments refuses the split. The platform-neutral, non-Go-source and import-coverage checks then run over every argument, not just the ones with darwin-only files. Offload group 9 builds the case: `./mtest` is eligible, and `./mtest2` has a failing `//go:build !linux` test. The reviewed wrapper split the command (an SSH offload, rc 0, `before.log`). The fixed wrapper keeps it local and returns the injected status 6. |
| P1: a common test can build everywhere and still behave per platform | ASD-V0-2403 | The reviewer chose a conservative rule, implemented as the new helper `runtime_gated`. A package stays local if any of its other Go files names one of these whole words: `GOOS`, `GOARCH`, `testenv`, `Skip`, `SkipNow` or `Skipf`. That covers common tests and package helpers, under any import name, in code, strings or comments. A file that cannot be read also keeps the package local. Group 9 injects a test that skips or returns on Linux and fails on Darwin, gated in seven ways: `runtime.GOOS` with `Skip`, an aliased `runtime` import's `GOARCH`, `os.Getenv("GOOS")`, `testenv`, `SkipNow`, `Skipf`, and a package helper naming `runtime.GOOS`. The reviewed wrapper split all seven (rc 0). The fixed wrapper keeps each one local and returns 6. `internal/analyzercap` stays local, and its `runtime_admission_test.go` matches the rule. |
| P1: removing `script/*.sh` from the shell test input declaration hid a Python call | AFP-V0-2400 | The reviewer chose this remedy: any dirty `.corvint/shell-test-inputs.json`, deletion included, adds the `LANGUAGE_FRONTIER` unknown `shell:test-inputs-changed`. This mirrors `shell:read-scopes-changed`. `tools/shell-test-select` already runs every step for a `shell:` frontier. `TestShellTestInputsDeclarationChanged_AFPV02400` covers both a removed input and a deleted declaration, each with a fault in a script that only the dropped input covered, plus a clean-declaration control. It failed before the fix (`planner-before.log`). End to end, the fault was injected into a scratch clone: `script/*.sh` dropped (or the file deleted) and a Python call added to `script/batch-regen.sh`. With the reviewed binaries, shell-test-select narrowed to 14 of 45 steps without `no-python-runtime-dependency-test`. With the fixed binaries it ran FULL 45 of 45 and named the frontier. With the declaration left clean (control), it narrowed to 15 of 45 steps, including that test. |

The spec amendment is a separate commit ahead of the fixes. No new requirement IDs were
allocated.

Measurement: the Darwin-stubbed harness ran `darwin_split` over the repository's 42 packages that
have darwin-only files. It split 0 packages with the reviewed wrapper and 0 with the fixed one.
The split path therefore has no measured payoff in this repository today. It remains a
fail-closed path for future test-only darwin packages.

Focused commands, all passing after the fixes:

```sh
HEAVY_WHOLE_REASON='selbugs lane' bash script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/liveverify/affected/shell/
HEAVY_WHOLE_REASON='selbugs lane' bash script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/liveverify/affected/
HEAVY_WHOLE_REASON='selbugs lane' bash script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./tools/shell-test-select/
bash <group-9 runner>/lane-scripts_test.sh   # offload group 9 only, fixed wrapper
```

The runners, logs, binaries and end-to-end plans are kept in
`/home/russell/.cache/selbugs-r1104d/`. Go and SSH are fixture stubs in group 9, so this is not
native Darwin or WSL qualification. These were NOT_RUN or NOT_PRODUCED under this round's
instructions: independent review, hosted CI, native Darwin execution, publication, ticket
writes and sealing.
