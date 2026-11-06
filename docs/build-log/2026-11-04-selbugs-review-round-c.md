# Selection correctness: review round c

Owner intent: correct the two P1 findings and one P2 finding from review round c of the
selbugs lane. Lane base `6f12f4704235dc8f9bea19fc12995bfb68c746c3` and every requirement ID
stay as they were. Each fix fails closed rather than modelling more of Go's build. This
record describes local evidence only; the next independent review and integration belong
to the orchestrator.

| Finding | Requirement | Change and proving evidence |
|---|---|---|
| P1: a Darwin test disappears from both halves when another test file is excluded on Linux | ASD-V0-2403 | Before splitting, every other `.go` file of the package must be platform-neutral: no `//go:build` or `// +build` line, and no GOOS or GOARCH element in its name. Any non-Go source file (C, assembly, `.syso`, SWIG and similar) keeps the package local. Files Go ignores by name are skipped. Offload group 9 covers `//go:build !linux`, `// +build !linux`, a `_linux` name and a `_darwin_arm64.s` file. Each injects a failing test that only the local half could run. The reviewed wrapper returns 0 (`before.log`); the fixed wrapper returns the injected status 6. An unconstrained other file still splits. |
| P1: an import's initialization can make the split unsound | ASD-V0-2403 | Imports are read from canonical Go formatting. A blank, dot or cgo import, or an import block that cannot be parsed, keeps the package local. Any other import except `testing` must also appear in a platform-neutral file of the package, so the Linux half links the same initialization. Group 9 injects a failing test behind a Darwin file importing a package the rest of the package does not import, in blank, named, dot, cgo and plain forms. The reviewed wrapper splits the blank form (`before-imports.log`); the fixed wrapper keeps every form local. A shared `strings` import still splits. Platform variants inside dependency packages are outside the rule and recorded as a limit. |
| P2: a NOTHING manifest bypasses the missing-test gate | AFP-V0-2403 | Only `FULL` is exempt. `NOTHING` and any unrecognized scope are compared as an empty run set, whatever units they list. `TestAFPV02403NothingManifestMissesFailingPackages` builds the control: CI selects the failing package and the manifest selects nothing. It fails against the reviewed selector and passes after the fix. It also checks that NOTHING without failures is not flagged and that an empty scope is treated as running nothing. |

The spec amendment is a separate commit ahead of the fixes. No new requirement IDs were
allocated.

Measurement: a Darwin-stubbed harness ran `darwin_split` over the repository's 42 packages
with darwin-only files, using the reviewed and the fixed wrapper. Both split 0. Eight
packages have only `_test.go` darwin-only files. All eight are already kept local by checks
that predate this round: function-header or helper-name checks, package-level declarations,
or compound `//go:build` lines. So this round costs no measured offload. The previous
handback's "eight of 42 qualify" counted those test-only candidates, not packages that
actually split. That figure is corrected here.

Focused commands, all passing after repair:

```sh
HEAVY_WHOLE_REASON='selbugs lane' bash script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./tools/selection-baseline/
bash <group-9 runner>/lane-scripts_test.sh   # offload group 9 only, fixed wrapper
```

The before and after runners, logs and measurement harness are kept in
`/home/russell/.cache/selbugs-r1104c/`. Go and SSH are fixture stubs, so this is not native
Darwin or WSL qualification. These were NOT_RUN or NOT_PRODUCED under this round's
instructions: independent review, hosted CI, native Darwin execution, publication, ticket
writes and sealing.
