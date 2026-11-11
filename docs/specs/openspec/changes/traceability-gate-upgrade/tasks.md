# Tasks: `traceability-gate-upgrade`

Each task names the requirement IDs it delivers and its proof. Tick tasks in order.

## Tasks

- [ ] 1.1 First commit: copy TTG-V0-960 to TTG-V0-963 from the spec delta into
      `docs/specs/traceability-test-gate-v0.md` as experimental requirements, with their rows in the
      spec's traceability table, and regenerate the requirement index. Requirements: TTG-V0-960 to
      TTG-V0-963. Proof: `make spec-requirements-check` passes.
- [ ] 2.1 Measure before the change: list every subtest citation on the lane base and whether the
      cited body holds the ID. Requirements: TTG-V0-960, TTG-V0-961. Proof: the list, kept in the
      build-log entry.
- [ ] 2.2 Subtest citation and body check in `script/check-traceability-tests.sh`, with fixtures:
      a `TestX/REQ-1` citation that resolves, a citation whose test body lacks the ID, a suffix that
      is not a requirement ID, an ID that is a prefix of a longer ID, two declarations of one name,
      and each exempt marker. Requirements: TTG-V0-960, TTG-V0-961. Proof:
      `make traceability-tests-test` passes.
- [ ] 2.3 Last-citation rule and the `(untested: REASON)` marker in the range form, with fixtures:
      the only citing test of a requirement is deleted and its row edited, a second citation remains,
      the spec is retired, the requirement text is removed, the marker with and without a reason.
      Requirements: TTG-V0-962, TTG-V0-963. Proof: `make traceability-tests-test` passes.
- [ ] 2.4 Run the index form on the lane head and the range form for `origin/main..HEAD` and for
      the last 50 first-parent commits of `main`; record the counts and the run time. Fix any row
      that fails by naming the ID in the cited subtest. Requirements: all. Proof: the recorded
      output.
- [ ] 2.5 File the ticket for a CI step that runs the range form on pull requests (`.github`, its
      own PR), and add one paragraph on the two rules to `docs/SPEC-DRIVEN-DEVELOPMENT.md`.
      Requirements: TTG-V0-962. Proof: the ticket ID and the diff.
- [ ] 3.1 Build-log entry; complete V1-4803 through the Tasks gate. Requirements: all. Proof:
      `corvint-tasks ticket show` and `receipt audit`.
