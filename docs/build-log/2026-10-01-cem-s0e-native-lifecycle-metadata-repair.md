# CEM S0E native lifecycle and metadata repair

Private native repair of the experimental Stable verifier (`cem verify-stable`)
against the public S0E amendment packet, manifest SHA-256
66af854fd1cc25372ad0c328d6401ebf88a1d23f46765837f8014b2a7340ea45 (56 full-result
cases). Base is 0f13c5ca. Everything here is experimental self-use on one Darwin
host (Darwin 25.6.0 arm64, go1.27.1); it is not release qualification. Where the
earlier repair plan and the public contract disagree, the public contract governs.

## Decisions

Owned runner. Every Stable Git child is run by `internal/groupreap.Owner`; legacy
callers keep the existing runner and no legacy command changes from working to
refused. A platform without the owner (anything but Darwin and Linux) refuses a
Stable call before the first spawn with `repository` /
`unsupported-process-containment`. Linux compiles; its lifecycle is NOT_RUN and
compile-only is not qualification.

Two-step absence (OQ-12). After the leader's exit is observed without reaping,
the group receives one SIGKILL and signal 0 is polled until it stops succeeding.
The leader is then reaped and exactly one signal-0 observation is made: ESRCH is
observed absence; anything else, including EPERM, is HOLD. No real signal is sent
after the reap. HOLD is `repository` / `unsupported-process-containment`, exit 2,
never retried or downgraded, and no later operation spawns. A HOLD in a
short-lived CLI leaves the process group to the operating system at process exit;
that is not observed cleanup and is not reported as such.

One clock (OQ-10). One 30-minute wall deadline for the call, a 10-second deadline
per logical operation capped at the wall deadline, and a single non-renewable
10-second emergency allowance anchored at the first caller cancellation or outer
expiry. The earlier "thirty minutes cumulative" child-time wording is superseded.

Commitment point (OQ-11, OQ-04). The test-seam event `cause-committed` names it:
the group is signalled (`retire-started`), the outer and per-operation causes are
sampled once, then `cause-committed` is emitted. An outer expiry first observed
later does not replace the committed cause. Arbitration happens only after owned
cleanup: unobserved cleanup; outer cancellation or expiry; per-operation deadline;
typed consumer failure; stdout overflow; stderr overflow; launch failure;
unsuccessful exit; success.

Operations. At most 1,024 logical operations; a replay reuses its reservation;
operation 1,025 is refused before spawn and that bound precedes the outer deadline.

Metadata. `gitauth.OpenStable` admits in a fixed order without reading any Git
configuration first, applies the interoperable pointer grammar, treats only an
actual not-found result as absence, retains every admitted directory by descriptor
and re-observes the whole boundary before publication. Legacy `gitauth.Open` is
unchanged.

## Timing-boundary ledger (OQ-05 B)

| Boundary | Value | Rule |
| --- | --- | --- |
| Outer wall deadline | 30 min | expired when now is not before the deadline |
| Per-operation deadline | 10 s, capped at the outer deadline | expired when now is not before the deadline; one nanosecond earlier is not a timeout |
| Emergency allowance | 10 s, once | anchored at the first cancellation (now) or outer expiry (the deadline) |
| Retirement bound without a terminal event | operation start + 10 s | cleanup still unproved at the bound is HOLD |
| Clock re-read interval | 1 ms | deadline watch |
| Group probe interval | 1 ms | step-1 poll |
| Logical operations | 1,024 | 1,025 refused before spawn |
| stderr / diff stdout / tree stream | 64 KiB / 8 MiB / 4 MiB | overflow is `unsupported-resource-limit` |
| Metadata pointer | 4,096 bytes | 4,097 refused |

## Evidence and limits

Conformance (`TestStableS0EPublicCases`, complete 21-field equality plus exit
code): 50 PASS, 4 FAIL, 2 NOT_RUN in the recorded run.

- NOT_RUN: the two go1.24.13 cases need that toolchain.
- FAIL (three, deterministic): `ledger-336-evidence`, `ledger-337-evidence` and
  `partial-drift-one-item-cancel`. Their public maps carry evidence records that
  no hunk cites; `internal/cem/wire` rejects those at the wire stage
  (`invalid-field`). That file is outside this repair's authored set, so it was
  not changed. A scratch experiment with only that check disabled passed all
  three (1,023, 1,024 and 691 admitted operations); it is an experiment, not a
  delivered behaviour.
- FAIL (intermittent): cases whose retired group contains a killed descendant
  can HOLD, because the single post-reap observation returns EPERM while the
  descendant's zombie is still waiting for the system reaper. Measured over 20
  runs: 4/20, 2/20 and 2/20 for the three overflow cases and 0/20 for the
  lingering-descendant case; an earlier direct measurement was 12 of 192. The
  literal OQ-12 rule is kept and the failures are reported, not hidden.

Other observations: E1b 12/12 (a live member after the reap is observed live);
E1c 24 trials with 0 post-reap EPERM for a group without descendants. The
T-LINKED control passed 20/20.

Open issues and unknowns:

- The legacy runner still signals a group after its leader was reaped; unchanged
  here and recorded as an open issue.
- `internal/cemcandidate` `TestStableAssemblyRetainsNarrowRepositoryEnvelope`
  now fails: the public contract removes the configuration pre-read, so the
  assembly it expected to be refused is accepted. The test is outside the
  authored set and is left for a root decision.
- Linux lifecycle, cross-device topology, escaped-session behaviour, a group
  member owned by another user, and other macOS versions are NOT_RUN.
- Linux step-1 behaviour is unknown. The portable verifier's `exit status 2`
  anomaly is not explained; the EPERM finding above is a candidate cause only.
- When a HOLD is discovered at session close after a result was otherwise
  complete, the result's issue codes are cleared; the public contract does not
  specify this.
- EACCES, ELOOP and EIO are injected at the metadata inspection seam, not
  produced by a real filesystem.

Rollback: remove the Stable owned-runner route and `OpenStable`; the legacy
runner, `gitauth.Open` and all earlier command defaults are untouched.
