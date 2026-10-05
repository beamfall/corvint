# rc.2 version tuple and documentation

Human-owned intent: on 2026-10-05 the owner asked to "get to rc2" and to "make sure the README.md and
other documentation is fully up to date for RC2".

## Decision

- Decision 0434 moves the version tuple to `1.0.0-rc.2` and fixes the readiness record's signing row
  for rc.2 and 1.0.0 to decision 0433 (`SRR-V1-009`, `internal/releasecandidate/readiness.go`).
- `README.md` names rc.2 as the next, unbuilt candidate and rc.1 as the newest published release;
  states that rc.2 and 1.0 are unsigned under decision 0433; lists `breakage`, `step` and `delta` as
  experimental commands added after rc.1; and records that the rc.2 Core jobs run has not run.
- The readiness-record, release-artifact-integrity and host-lifecycle specs now name rc.2 where they
  named rc.1 as the next use, and say rc.1 host lifecycle results are stale for rc.2.

## Limits

- No rc.2 build, archive, harness run or lifecycle result exists yet. The README status rows are
  rewritten from those results after they run, pass or fail. `RELEASE-NOTES.md` gains its
  `## 1.0.0-rc.2` section in the runbook step-9 notes commit.
