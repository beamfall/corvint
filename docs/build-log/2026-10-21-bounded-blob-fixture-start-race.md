# V1-0582: the bounded-blob cancellation fixture had a pid-file read race

This entry covers spike ticket V1-0582. Decisions are delegated to the orchestrator (decision 0548).
The test is `TestBoundedBlobCancellationRetiresDescendant` in `internal/cem/gitauth`, which supplies
the DLT-V0-003 evidence.

## Root cause

The test's fake `git` script starts a `sleep` descendant and writes its pid with
`echo $! > child`. The polling loop then reads that file.

- The shell creates the redirect target before `echo` writes to it, so `os.ReadFile` can return an
  empty file.
- The loop ignored the `strconv.Atoi` error and still executed `break`. The pid stayed 0, so the
  test failed with "fixture descendant never started".
- The ticket reported the failure after 1.36 s. That is shorter than the loop's 2 s polling budget
  (200 × 10 ms), which fits a loop that left early rather than one that timed out.

This was a race in the test fixture. Production cancellation was not involved: the failure happened
before the code under test was exercised.

## Measurement

`go test -count=300 -run '^TestBoundedBlobCancellationRetiresDescendant$' ./internal/cem/gitauth/` at
base `90ab9f33e`, through the lane semaphore, with host load 8.6 to 16.6:

- before the repair: 3 of 300 runs failed with "never started";
- after the repair: 150 of 150 runs passed, at load 12 to 15 (305.5 s).

## Repair

The fixture now publishes the pid by writing a temp file and renaming it (`.tmp && mv`). The poll
loop leaves only after it parses a positive pid; an empty or partial read keeps polling within the
same bound. The fixture also fails on a script write error, which it used to ignore.

The test's actual checks are unchanged: cancellation must be refused, and the descendant must be gone
within its bound. The original failure and the unchanged-input retry from the ticket stay recorded
there. Production code was not changed.
