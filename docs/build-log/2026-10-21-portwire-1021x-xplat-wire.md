# Unqualified Unix cross vet and machine-readable version identity

Date: 2026-10-21. Lane: claude/portwire-1021x (base f29ddc0db). Tickets: V1-0393 and V1-0300
(Core part) delivered; V1-2132 already delivered; V1-0453 and V1-0345 not delivered. Requirements
`PRS-V1-019` and `PRS-V1-020` in `docs/specs/corvint-1.0-product-and-release-v1.md` are accepted
under the owner's standing auto-accept (2026-10-11).

## Problem

- V1-0393: the 2026-10-10 refusal fallbacks (ecee67944) fixed three packages, but nothing kept
  the check running, so `go vet ./...` regressed on GOOS=freebsd, openbsd and netbsd
  (`internal/worktreeimpact` had platform code only for darwin, freebsd, linux and windows, and
  six `internal/tasks/store` tests tagged `unix` used darwin-or-linux helpers) and on solaris
  (`syscall.Flock` and `syscall.Mkfifo` do not exist there).
- V1-0300: a plugin or script had no machine-readable way to learn which verbs and harness hosts
  an installed binary supports; skew surfaced as a parse failure.

## Decisions

- `internal/worktreeimpact/platform_other.go`: on every platform outside darwin, freebsd, linux
  and windows, opening a working-tree target and taking its link identity refuse, like the
  existing Windows identity refusal.
- `internal/stepnegation` and `tools/gate-ledger`: Solaris takes the existing `!unix` fallback
  (exclusive-create lock; the ledger refuses its directory and runs every step, GL-V0-005).
- Tests that need `syscall.Mkfifo` exclude Solaris; store tests that need darwin-or-linux helpers
  are tagged `darwin || linux`. The darwin, linux and windows file sets are unchanged.
- `make cross-vet` (a `make gate` step) now vets the root module and `interop/cem01-go` for the
  four platforms on amd64, so a regression fails the gate (`PRS-V1-019`). The checks qualify no
  platform (`PRS-V1-004`).
- `corvint --version --json` prints the `corvint-version/1` identity: version, build, sorted
  dispatched verbs (`topLevelCommands`) and admitted harness hosts (`PRS-V1-020`). It is an option
  on the existing `--version`, not a new verb, so the verb catalog does not change.

## Evidence

- `go vet ./...` for GOOS=freebsd, openbsd, netbsd and solaris (amd64), root module and
  `interop/cem01-go`: PASS after the change; FAIL before (worktreeimpact, store tests, Flock,
  Mkfifo). `make cross-vet`: PASS.
- `TestPRSV1020VersionJSONNamesVerbsAndHosts`: PASS; fails without the main.go change.
- V1-2132: 73d457f0a converted the 19 `Errorf(CodeOf(err), ...)` sites to `wire.Annotatef`, and
  348b80b7e added `TestCALV0078_NoRewrapDropsNotRetryable` (an AST guard over all non-test Go).
  Annotatef keeps the mark (`internal/tasks/wire/retry_test.go`). No former site's cause can carry
  the mark today, because each wraps an OS or decoder error, so no former call site is driven
  directly.

## NOT_RUN

- Native FreeBSD, OpenBSD, NetBSD and Solaris execution, including
  `TestReadTargetOnceOtherRefusesRegularFile`, which only those platforms compile and run.
- Plugin trees reading `corvint --version --json` (V1-0300 residual). Claude Code and Codex hooks
  are declarative command lists, so a check needs a host-side step qualified under a live host.
- V1-0453: needs retained live OpenCode 2 evidence. V1-0345: decision 0398 classifies D15 as a
  defect, but the fix changes the Core release archive contract (plugin trees plus an install
  verb) and needs a release-scoped route.

## Rollback

Revert this change. The fallbacks and build constraints affect only unqualified platforms, and
`--version --json` is additive.
