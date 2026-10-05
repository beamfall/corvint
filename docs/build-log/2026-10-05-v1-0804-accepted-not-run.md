## 2026-10-05 V1-0804 service helpers: accepted with qualification NOT_RUN

Human-owned intent: on 2026-10-05 the owner decided "accept V1-0804 as NOT_RUN too when it reports". The
owner also ruled that install accepts helpers only if the focused tests and the Codex review passed,
and otherwise keeps the UNSUPPORTED refusal.

### Decision

- V1-0804 (SERVICE500-007, beamfall/corvint#613) delivers service helpers, `run-helper` and bounded
  service logging as a Linux-only slice. Focused tests passed on darwin and in a Linux container, and
  Codex round 6 approved. Install therefore accepts helpers on Linux and refuses them with UNSUPPORTED
  elsewhere.
- Platform qualification is `NOT_RUN` and stays tracked by V1-0697. `make gate` and tests outside the
  focused set are `NOT_RUN`.
- The owner delegated the six owner questions in
  `docs/build-log/2026-10-05-v1-0804-service-helpers.md` to the orchestrator, which kept every
  fail-closed default:
  - a held intent recovers only by operator removal;
  - there is no helper health-based debt reset;
  - status withholds the excerpt text;
  - the decision 0397 `internal/groupreap` edge is accepted;
  - Darwin helpers stay UNSUPPORTED;
  - an interrupted resume retry is refused.
- Follow-up V1-0818 tracks those alternatives and the undelivered parts: main-controller restart
  debt (`NOT_OBSERVED`, not charged), Pdeathsig, and the helper-executable hash time-of-check window.
  V1-0804 completes natively after #613 merges.

### Limits

No platform witness is accepted by this decision, and no unrun check is reported as passed.
