# frontier-authority-unavailable stays a degradation (V1-0869 declined)

Date: 2026-10-21. Base: `9ae946a5c` (`origin/main`). Requirements: `AHI-054`
(`docs/specs/agent-harness-integration-v0.md`), `CF-V1-019`/`CF-V1-020`
(`docs/specs/change-frontier-profile-1.md`), `LCP-V0-001` (`docs/specs/local-completion-policy-v0.md`).
All are unchanged.

## Decision

V1-0869 asks for `frontier-authority-unavailable` to be reported as a state rather than as a
degradation on every harness and dogfood event, because it is always present. The owner delegated
the fork on 2026-10-21. The instruction was to amend the accepted requirements only if the frontier
authority is now available.

It is not available. Decision 0009 accepted option 2: Corvint keeps the local-only, single-UID
boundary, so `harness-execution-attested-v0` has an empty root set. `CF-V1-020` lists four
prerequisites for removing the degradation:

- (a) an accepted attestation root;
- (b) CF-V0-001 inputs at a stop lifecycle point;
- (c) the wiring;
- (d) the parity-corpus migration.

None of the four exists. The harness still does not consult `internal/frontier`. `LCP-V0-001`
forbids the local completion policy from removing the degradation.

The degradation is therefore accurate, and the requirements stay unchanged. Two things limit the
cost of always emitting it:

- `AHI-054`'s fixed `cause` names why the degradation holds.
- Adapters accept any subset of the recognised codes, so a future removal is not an adapter break.

The noise concern in V1-0869 is real. Reclassifying the code alone would still re-pin nine
`harness event` cli-parity cases against the frozen Python oracle. It would also contradict the
`CF-V1-020` ordering. No consumer gains a new signal from that.

## Consequences

- V1-0869 closes as declined.
- The decision should be revisited if any of the following is accepted:
  - an attestation root (decision 0009 option 1);
  - an answer to `change-frontier-profile-1.md` Q4;
  - `CF-V0-027` closing as a permanent product boundary (Q5).
- If one is accepted, amend `AHI-054`, `CF-V1-020` and `LCP-V0-001` together, with a
  divergence-register entry for the parity cases.

Rollback: delete this entry and reopen V1-0869.
