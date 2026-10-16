# 2026-10-16 V1-0650: S0E packet NOT_RUN closeout (Go 1.24.13 cases 21-22, Linux lifecycle)

Base: origin/main b1ee552b3bebeaacf8f54e62f7f67572f231848e. Governing contract:
`docs/specs/cem-stable-v1.md` (S0E repository-envelope packet). No requirement is added or amended and
no product or test code changes: this entry is the evidence integration (AC3) and a re-check of AC1/AC2
against current main. Host: Darwin arm64, `GOTOOLCHAIN=local`, go1.27.2, load average about 60-68 on 12
cores (shared with other lanes) for every run below.

## Packet on current main

The packet survived the RC3 removal of cem/0.x. It lives at
`protocol/cem-1.0/stable/repository-envelope-packet/FULL-RESULT-CASES.json` (sha256
`25d4155c2235c3034146093d0c844e0e448281429c74bad5f19abf215d672146`, last changed in
7ac39a0502a037ef72a647c18c42ab53e0a550a1), 56 cases; cases 21 `go1.24.13-nonstructural-positive` and
22 `go1.24.13-structural-refusal` are unchanged. The portable test `TestStableCanonicalFullResults` is in
`interop/cem01-go/stable_repository_test.go` (sha256
`a57110ea88fc19eb3779f8a5dfb28865a419a9ffdd11a0cf52c784263cc59e5b`); it still runs cases 21/22 only when
`runtime.Version()` starts with `go1.24.` and skips them otherwise. The frozen tuple is now go1.27.2
(`protocol/cem-1.0/structural-tuples/go1.27.2/TUPLE.json`), so "54 other cases" below were recorded
against go1.27.1 and are not re-established by this entry.

## Retained evidence (2026-10-01..02)

Named in the ticket body: `GO124-TWO-CASE-QUALIFICATION.json`, `go124-two-case-qualification/{tests.log,
conformance.json,binding.json}`, `LINUX-FULL-RESULT-QUALIFICATION.json`, `LINUX-FULL-RESULT-ACTUAL.jsonl`,
`FIXTURE-IDENTITY-LINUX.json`.

Observed 2026-10-16: NOT retained. `find /private/tmp -maxdepth 4` finds only the empty directory
`/private/tmp/cem10-codex-control/linux-availability-repair/go124-two-case-qualification/cache` (0 bytes)
and `/private/tmp/cem10-build/stable-next/go124/cache`; none of the JSON/JSONL/log files exist, and no
go1.24.13 toolchain exists on disk (`~/sdk`, `~/go/pkg/mod/golang.org`, `/usr/local/go` checked; only
toolchains go1.26.4 to go1.27.2). The 2026-10-01..02 results therefore survive only as the ticket-body
record (source 1a355ea551c2745d3fe9f229a36acc348cb205d8, tree 7d34fba6882a4b7cbbec1f0ef678ede962d8bc96;
official Go 1.24.13 archive sha256 `f282d882c3353485e2fc6c634606d85caf36e855167d59b996dbeae19fa7629a`,
76442495 bytes). Their digests are not recoverable and are not asserted here.

## Re-run today

| Item | Result | Evidence |
|---|---|---|
| Cases 20-23 under go1.27.2, Darwin arm64 | 20 and 23 PASS; 21 and 22 SKIP (go1.24-only) | `go test -run 'TestStableCanonicalFullResults/^(2[0-3])-' -v` in `interop/cem01-go`, ok 23.2s |
| Full 56-case run under go1.27.2, Darwin | NOT_RUN to completion: two attempts timed out (600 s, cases reached 41 and 52) under load about 65 | partial logs only; no verdict claimed |
| Cases 21/22 under go1.24.13 | NOT_RUN today: no toolchain on disk; downloads are out of lane scope | see above |
| Linux, ad-hoc container (golang:1.27.2 linux/arm64, uid 0, `--network none`, tmpfs /tmp exec, repo copied without .git) | Not a qualification. Cases 01, 20 PASS; 21/22 SKIP; 02 FAIL `unsupported-process-containment` (sleep as PID 1 does not reap); 23 FAIL (uid 0 bypasses EACCES) | harness defects, not product evidence; container removed |

## Acceptance criteria

- AC1 (cases 21-22 under Go 1.24.13, result retained): SATISFIED historically, not re-observed. Darwin
  arm64 go1.24.13, source 1a355ea5, both RUN_REAL and PASS (exit 0 and exit 2, all 21 result fields
  compared) per the 2026-10-02 ticket record; the retained files are gone (above). Today: NOT_RUN.
  Case definitions are unchanged on main, so the 2026-10-02 result remains the latest observation but
  predates the go1.27.2 tuple migration and the RC3 cem/0.x removal. Re-execution needs an approved
  go1.24.13 toolchain.
- AC2 (stable lifecycle on Linux, or explicit exclusion): SATISFIED by 2026-10-02 Linux arm64 Go 1.27.1
  (git 2.47.3, kernel 6.8.0-117) 54/56 PASS with 21/22 NOT_RUN, zero failures, per the ticket record
  (files gone). Not re-observed today. Later Linux hosted evidence on current main exists in
  `docs/build-log/2026-10-14-v1-0668-linux-zombie-qualification.md` and
  `docs/build-log/2026-10-14-v1-1358-portable-linux-containment.md`.
- AC3 (remaining unexecuted results stay NOT_RUN): this entry. Explicit NOT_RUN exclusions: Linux amd64
  full 56-case matrix on the public packet; cross-device topology; escaped-session behaviour; cases
  21/22 on Linux; Go 1.24.13 on any platform other than Darwin arm64; full 56-case run on current main
  under go1.27.2 (this host, load); portable standalone Linux.

## Residuals

Retention gap: qualification files were written under `/private/tmp`, which is not durable. A future
qualification should copy its result JSON and digests into the build log at the time of the run.
