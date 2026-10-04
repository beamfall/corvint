# Dispatch host-pressure launch throttle: issue 497

Human-owned intent: GitHub issue 497 (owner request), native V1-0694. The new normative text is
CAL-V0-068/S18 in `docs/specs/corvint-tasks-agent-leases-v0.md`, with cross-references in
CAL-V0-052 (optional `pressure` member), CAL-V0-054 (budget after the static fences) and CAL-V0-058
(`throttled` joins the closed event vocabulary). The coordinator assigned CAL-V0-068/S18; no origin
ref used it at base `4b10a02144faed4a77b36eba4b0d1cbc315706d3`, and the change was rebased onto
`4620e3436ceddc7e9b1ccb392a93f157155a7154`, keeping main's issue 494 text.

Decisions:

- The issue's free-form `exempt` list becomes explicit `exemptRoles` (must name configured roles)
  and `exemptTickets`; pins are always exempt. Critical-path, review and integration work is exempt
  only when named. Nothing is inferred from role or ticket names. This refines the issue's example
  and does not add new authority.
- Pressure is admission only. The budget runs inside the pure roster after the global cap and the
  key, skip and per-role fences, so a held candidate reserves no slot or key. `Roster` is unchanged
  for configurations without `pressure`, because it delegates with a nil budget.
- Running non-exempt workers count against the level cap, and nothing is ever stopped or signalled.
- An UNKNOWN sample keeps the level and cancels dwell. A restart keeps the recorded level but
  cancels dwell and starts UNKNOWN, so a dispatcher restart cannot release a throttle.
- One `throttled` event is emitted only when the level, the sample knowledge or the held-key set
  changes. This reuses the existing event writer and ledger save rather than a separate envelope.

Reuse: seven read-only Codex leaf files (`/private/tmp/corvint-497-leaf-20261004`, checkout
21191633) were reused. Five remain byte-identical, so their reviewed SHA-256 values are retained:
`pressure_sample.go` 3a1f37f7…f788, `pressure_sample_darwin.go` beb9ef35…e31c,
`pressure_sample_linux.go` 86a611c9…9250, `pressure_sample_other.go` 23c5dbdb…6c34 and
`pressure_test.go` aa9c4974…383e. Two were modified after review: `pressure.go` (leaf f363e0c9…b721,
now b71a416a…73a2; comment only) and `pressure_sample_test.go` (leaf bcc8c35a…2b6742, now
107c1a82…966b). The test change fixes a race in `TestIssue497_CommandLifecycle`: the poll accepted
the fixture's PID file as soon as it existed, before the PID was written, and failed once after the
rebase with an empty read. It now waits for content.
The preparation plan's composition with issues 498/499/502 and its separate bounded event
envelope were superseded. This change does not stack on the issue 464 branch: the two are
functionally independent and conflict only textually in `loop.go` and the status renderer.

Independent review: two read-only reviews, the first on the pre-rebase commit and the second on
`4620e343..51be2074`, both returned PASS_WITH_NITS. Fixed before binding:

- MED: held over-reported candidates that the role or global cap would have blocked anyway. Holds
  are now charged to shadow role and global counters, and a candidate is held only while those caps
  would still admit it. Admission is unchanged. Regression: `TestCALV0068_HeldRespectsStaticCaps`.
- LOW: the sample source and problems are made valid UTF-8 before truncation at a rune boundary.
- LOW: the status cap is `NONE` at level 0 even without a configuration.
- LOW: a restart clears the previous run's sample, and a budget build failure appends an `alert`.
- LOW/nit: the spec now states that there is no 2-to-1 step-down, that held respects static caps,
  where the status cap comes from, and that a downgrade needs one start without `pressure`
  (an older binary's ledger decoder refuses unknown fields). The `pressure.go` exemption comment
  now matches the actual validation.

Evidence at the frozen source:

- `go test -count=1 -timeout 30m` over `internal/tasks/dispatch`, `internal/tasks/cli` and
  `internal/specindex` passed (rc 0).
- A race run of the CAL-V0-068, issue-497 and CAL-V0-05x dispatch tests passed (rc 0).
- `go vet` passed on the dispatch and cli packages, and `GOOS=linux` and `GOOS=windows` vet of the
  dispatch package passed (rc 0). gofmt is clean.
- `TestIssue497_LiveSampler` exercised the real macOS sysctl sampler.

Not established:

- Linux sampling is fixture-parsed only; live Linux sampling is NOT_RUN.
- Windows and other operating systems are compile-only and always UNKNOWN.
- A live multi-agent dispatch under real host saturation is NOT_RUN.
- Current-main landing, PR/CI and native completion of V1-0694 remain pending.
