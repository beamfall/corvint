## 2026-10-05 Issues 395, 398 and 388: accepted with qualification NOT_RUN

Human-owned intent: on 2026-10-05 the owner decided "accept the other three as NOT_RUN" for issues #395
(replay historical merges through the post-merge workflow), #398 (reference CI host templates and host
adapter contract) and #388 (post-merge maintenance workflow epic), and closed them.

### Decision

- #395 closes with the delivered replay slice (fixture `basis`, `replay --change --dry-run`, identical
  second run, reports split by basis). No historical replay set exists and the qualification campaign is
  `NOT_RUN`. The author and draft stages are owner-deferred, not delivered. V1-0542 completes on this basis;
  follow-up V1-0805 tracks the rest.
- #398 closes with PCH-V0-001..008 and PCH-V0-015 delivered. The hosted dry-run of the replay set
  (blocked on #395's set) is `NOT_RUN`, and physical isolation (`PCH-V0-011`, `PCH-V0-014`) is
  unqualified (`NOT_RUN`). V1-0545 completes on this basis; follow-up V1-0806 tracks both.
- #388 closes together with #395 and #398. The final rerun of gap checks (a) to (d) is `NOT_RUN` and
  is tracked by follow-up V1-0807; V1-0535 completes on this basis.
- Spec notes added to `postmerge-replay-v0.md` and `postmerge-ci-host-v0.md`; no requirement ID or
  behavior changes.

### Limits

- No unrun witness is accepted by this decision, and no `PCH-V0` or replay witness is marked passed.
  Whole-workflow qualification stays `NOT_OBSERVED`.
