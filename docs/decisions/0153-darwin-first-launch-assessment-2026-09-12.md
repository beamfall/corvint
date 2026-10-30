# Decision 0153 — Keep the Darwin invocation cap and disclose first-launch assessment

Date: 2026-09-12. Status: accepted. Authority: owner delegation to coordinator.

## Decision

Keep the 100 ms ACC-V0 invocation cap and ACC-V0-019 zero retries unchanged. The coordinator
preferred moving Darwin's first-exec assessment to a separately bounded, disclosed staging step,
but authorized the conservative fallback if no sound mechanism could be verified. This leaf
could not qualify a non-payload mechanism on this host and takes that fallback: document a
first-invocation `TIMEOUT` as expected Darwin behavior. No production warmup is introduced.
This is an evidence limitation, not proof that macOS has no possible non-executing assessment API.

A future staging assessment requires a separately accepted, host-verified mechanism: it must
never execute analyzer code outside the ACC-V0-002 sandbox, receive repository input, count as
an invocation result/retry/evidence, or change the invocation's behavior on assessment failure.
The invocation must still be allowed to time out honestly. No successful warm cache is promised.

## Host investigation and measurements

Observed on macOS 26.6.2, build 25G83, Darwin arm64, 2026-09-12 local time:

| Probe | Observed result | Elapsed | Load averages (1/5/15 min) |
|---|---|---|---|
| `/usr/sbin/spctl --assess --type execute -vv` on a fresh synthetic native executable | exit 1, `internal error in Code Signing subsystem` | 15.61 ms | 37.01 / 41.46 / 50.18 |
| `/usr/bin/syspolicy_check distribution` on another fresh copy | killed/reaped at the 5 s hang detector; `kern.bootargs getsize returned -1.` | 5003.73 ms | 34.61 / 40.88 / 49.93 |
| Exact production sandbox, fresh staged helper, before fallback | exit 71, `sandbox_apply: Operation not permitted`; payload latency `NOT_RUN` | 7.88 ms refusal | 25.87 / 35.85 / 46.79 |
| Exact production sandbox, another fresh staged helper, after fallback | same refusal; payload latency `NOT_RUN` | 7.19 ms refusal | 25.87 / 35.85 / 46.79 |

The 5 s policy-tool bound was an investigation hang detector, not an invocation budget: it
allows substantial slack over decision 0148's 170–200 ms assessment observation while bounding
a nonresponsive policy service. It is not a production staging parameter. The synthetic native
probe prints a marker from main; it was never executed outside the sandbox. No repository input
was supplied to either sandbox probe. The parent sandbox refuses installation before exec, so
an exact-profile failed-before-main mechanism cannot be verified in this session. No security
policy settings were changed and no broader sandbox was substituted for the production probes.

Apple documents [`spctl --assess --type execute`](https://developer.apple.com/library/archive/documentation/Security/Conceptual/CodeSigningGuide/Procedures/Procedures.html)
as a policy check, not a guarantee that subsequent exec avoids first-inode assessment. This host's
`syspolicy_check distribution --help` specifies an application bundle, not the bare native analyzer
contract. Tool failure and the parent sandbox refusal do not prove assessment completed. The
historical 170–200 ms first-exec and 15–30 ms warm-invocation measurements remain decision 0148's
observations; they are not newly reproduced here. A before/after payload latency delta is unavailable.

## Test contract and rollback

`ACC-V0-002` discloses the first-inode timeout; `ACC-V0-019` explicitly keeps it terminal.
`TestFirstInvocationTimeoutIsNotRetried` injects an over-cap launch assessment without executing
an analyzer and checks one launch attempt, a typed timeout, and no fabricated output or completion.
`TestNewStaticCoreUsesContainedNativeInvocation` waits after Core setup for a successful synthetic
probe of the same staged executable completing in at most 50 ms, then issues authority and calls
Core exactly once. The probe wall must precede issuance, which starts the invocation deadline.
Half the cap is a test precondition motivated by the recorded 45 ms pre-launch cost under load;
it cannot guarantee future scheduling. The existing 60 s probe wall remains a test hang detector,
and a real Core timeout still fails rather than being retried. A sandbox-install refusal remains
an explicit skip, never evidence of successful contained execution.

Amendment 2026-09-13: at load average about 220 the invocation terminal still reached
`FAILED`/`TIMEOUT` after a warm probe, because the cap runs from issuance through launch. The
test now retries only a typed `TIMEOUT` from issuance or its single invocation under a fresh
single-use authority, within the same 60 s hang detector (decision 0082). The 100 ms cap and
ACC-V0-019 zero in-invocation retries are unchanged; any other result, or `TIMEOUT` after the
hang detector, still fails.

Rollback: revert the spec disclosure and test precondition if a later accepted decision replaces
this fallback with a qualified staging assessment. Production runtime behavior is unchanged.

Validation: `GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/analyzerexec
./internal/analyzercap` and touched-package vet exited 0. The injected first-launch timeout test
passed; the real contained Core test explicitly skipped on the observed sandbox refusal. Thus
native completion under high load is unverified. Local Go is 1.27.1, not the repository's 1.27.0
pin. These results are leaf checks, not full integration or analyzer promotion evidence.
