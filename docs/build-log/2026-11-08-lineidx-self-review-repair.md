# Lineidx self-review soundness repair — 2026-11-08

Owner intent: the three P1 findings in the self-review repair request. Delivery remains experimental; this repair does not renew production qualification.

## Decisions and requirement evidence

- V1-4364, GLS-V0-008 / LPCV-V0-7601: reject an in-order record on any uninstrumented child. Isolation cannot prove completeness of an order-dependent launch. The existing no-record assertion now covers the suite as well as isolated tests. `TestOrderedChildMutation` retains the full ordered package and observes the mutant failure for both a prebuilt Go child and an unknown shell launcher, with explicit child environments. `TestUninstrumentedChild` still checks instrumented-child admission and missing counters.
- V1-4364/V1-4366, GLS-V0-009/020: remove caller overlays from the closed build-flag allowlist and check it at direct collection too. Existing refusal text and codes are reused. `TestCallerOverlayRefused` covers both flag prefixes and an unknown transform; `TestRunnerEnvCarriesBuildFlags` covers inline and separate command-line overlay values and an unknown flag. No overlay composition or new source identity is claimed.
- V1-4364/V1-4365, GLS-V0-017/018: observe the supported Unix syscall Getenv/Environ accessors as well as os accessors. Exact toolchain source patterns must match once. Unknown platforms/forms refuse. Named reads bind their inherited value; unknown whole-environment consumers and reads before os initialization retain the whole-environment claim. The only quiet inheritance stack is syscall.Environ → internal/syscall/execenv.Default → os/exec.(*Cmd).environ → os/exec.(*Cmd).Start. All other stacks remain reads. Collector-owned quiet accessors are used only by instrumentation.
- The existing environment-table metadata value advances from audit version 2 to 3, without changing the index encoding, wire shape, field names, errors, exit codes or goldens. Version 2 cannot certify the new completeness claim; `TestEnvAuditVersionAdmission` now explicitly covers its rejection through eager/lazy readers and merge/rewrite. Unknown versions still fail closed.
- `TestSyscallEnvironmentMutation` checks direct syscall reads, time.Local TZ changes, direct syscall.Environ consumers and public Cmd.Environ consumers. Each fixture mutates F, changes its environment, observes the real failing test and retains the whole package.

## Checks and retained failure

Go go1.27.3; TMPDIR=/home/russell/lanes/lineidx.tmp (0700); heavy.sh used the configured shared semaphore without overrides. Identity variables were not exported during tests. Affected selected both touched packages at the supplied base, with retained self-binary-stale degradation.

- Full internal/linecov: initial FAIL (34.435 s): the new lower hook treated ordinary child inheritance as a whole-environment consumer. The exact inheritance-stack allowlist repaired the implementation; existing precision assertions were not relaxed.
- Full internal/linecov: PASS (41.918 s), then PASS (31.661 s) after making pre-initialization reads conservatively retain the whole-environment claim.
- Full internal/sessiontest: PASS (2.484 s). Each Go command ran through script/heavy.sh, named one whole package and used -count=1 -timeout 30m without -run.
- Staged make -s spec-requirements-check line-citations-check: PASS. Repin found no unchanged content that moved. An earlier unstaged invocation explicitly did not check the spec edits and is not claimed as final evidence.

## Limits and rollback

The orchestrator owns independent review and broad host verification. New pre-change capture, whole-repository selection, doc-gates, Windows vet, seal, publication and Tasks mutations were not run under the repair instructions. Production qualification remains source-stale; no CI enablement or ticket completion is claimed. No native/cgo environment-read qualification or non-Linux execution is claimed. The selected self-development routes were affected selection and the existing post-commit dogfood binding/checking workflow; unrelated retrieval, learning, UI and external integration routes were excluded. Roll back the implementation commit as a unit if needed, leaving narrowing disabled and retaining the failed and passing evidence; do not reuse old completeness claims as fresh evidence.

## Evidence digests

Raw evidence is outside the repository at /home/russell/lanes/lineidx-evidence/self-review-repair/. SHA-256:

- `affected-final.json`: `c1cb9dab509f3f76b5a9f3e46be514bfb6c2f7f8a47fb180272bdf34cccd4406`.
- `linecov-test.log`: `52bf3b439a25ca116ba4fb1f5762ab3aafd237f22bed270f34fa7b3dc9416ac6`.
- `linecov-test-2.log`: `581688e83a5d925361047d90f0d7bd9107f2dd90e8fc5fd68706b70282533440`.
- `linecov-test-final.log`: `738f7664894c2b7cd6480f44f79ebefd871e15044cac2ae4ca2f393034ec4edb`.
- `sessiontest.log`: `d5aed2766690bece12c839c4df6404c99d26279c60df62acfa8f17c8a2f6b436`.
- `docs-check-staged.log`: `86f6e866ae9f83920728aa7399d2d5fc1be21a549180e4f46f455571ecb914c6`.
