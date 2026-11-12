# Fault-injection harness

Private release-evidence tool for V1-5405, contract `docs/specs/fault-injection-harness-v1.md`
(FIH-V1). It proves, per blocking runner, that the selected run (the plan `corvint affected`
produced for a PR, executed by the runner) executes every test that kills a fault put into a line
the PR changed. It generalises `tools/affected-edit-oracle`, which stays unchanged; it is not a
product command and never changes the selector or writes a source repository.

```sh
GOTOOLCHAIN=local go build -o /tmp/fault-harness ./tools/fault-harness
/tmp/fault-harness check --preregistration go.prereg.json --hash go.prereg.json.sha256
/tmp/fault-harness select-reverts --preregistration go.prereg.json --hash go.prereg.json.sha256 \
  --rule tools/fault-harness/revert-rule.json --candidates go.candidates.json
/tmp/fault-harness generate-go --preregistration go.prereg.json --hash go.prereg.json.sha256 \
  --root REPO_DIR --pr 1234 --base BASE_SHA --head HEAD_SHA
/tmp/fault-harness seal --preregistration go.prereg.json --hash go.prereg.json.sha256 \
  --rule tools/fault-harness/revert-rule.json --candidates go.candidates.json \
  --set go.faults.json --out go.seal.json
/tmp/fault-harness run --preregistration go.prereg.json --hash go.prereg.json.sha256 \
  --set go.faults.json --seal go.seal.json --root REPO_DIR --clones /tmp/fh-clones \
  --record go.run.json --out go.results.json
/tmp/fault-harness verdict --preregistration go.prereg.json --hash go.prereg.json.sha256 \
  --set go.faults.json --seal go.seal.json --results go.results.json --record go.run.json \
  --out go.verdict.json
/tmp/fault-harness parity --root . --clones /tmp/fh-clones --out /tmp/parity.json
```

`fault-harness COMMAND --help` prints each command's flags and an example. Exit 0 means the
measurement passed, 1 a measured failure or a named refusal (`refused: CODE: detail`), 2 an
invocation or input error.

Files, each one strict JSON with a `profile`:

- `fault-harness-preregistration/1`: runner, language, repository SHA and pin date, sampling
  frame, the reverted-fix rule's path and SHA-256, operators per language, seeds, over-sample
  (at least 100), exclusion budget and rules; the Playwright flow manifest hash; a held-out
  draw record. Every command refuses `preregistration-changed` when its SHA-256 differs from
  the committed hash file (`sha256sum` format), and `held-out-not-drawn` for a held-out
  pre-registration with no matching draw record.
- `revert-rule.json`, hashed in `revert-rule.json.sha256`: the A17 reverted-fix rule. Its
  numbers are fixed; another value is refused.
- `fault-harness-revert-candidates/1`: observed facts about merged PRs that the rule reads.
- `fault-harness-fault-set/1`, sealed into `fault-harness-seal/1`; `run` and `verdict` refuse a
  set or pre-registration whose hash differs from the seal, and `verdict` refuses results that are
  not the measurement of exactly the sealed cases and faults (`fault-set-changed`,
  `results-not-sealed`) or a set sealed with fewer than 30 non-code cases. A case result must be
  one the run could have written: an `EXCLUDED` case only with reason `pin-unavailable`, no fault
  results and the recorded refusal naming its head; a `KILLED` fault names its killing tests and
  plan kind, and its misses are among its kills under a `NARROWED` plan; a `NO_KILL` fault lists no
  kills or misses; a case status agrees with its faults.
- `fault-harness-results/1` and `fault-harness-run/1`: `run` writes the run record beside the
  results with the SHA-256 of the results' compact JSON encoding, the runner, the pre-registration
  and the sealed fault-set hash; `verdict` recomputes the digest and refuses a mismatch
  (`results-digest-mismatch`). The digest is unkeyed: it refuses results edited without the record
  being rewritten, not someone who rewrites both, so it is not tamper resistance.
- `fault-harness-verdict/1`.

Fail-closed rules: a baseline needs three runs, and a test that does not pass in all three is
quarantined; a kill needs three of three failures of a test that passed every baseline run; a
faulted tree that does not build or crashes (including the runner's own test timeout) kills
nothing and is dropped; a faulted run or clone step stopped by cancellation, the harness's own wall
bound or an unproved process cleanup, and a selected-run failure after a confirmed kill, are failed
measurements that make the verdict `INVALID`; an excluded case counts toward neither N nor the
non-code minimum of 30 measured cases; a miss is never waived.
The release sentence is printed only for a `ZERO_MISS` verdict.

The Go and pytest adapters exist. `--go-test` sets the argv prefix that runs `go test` (Corvint runs use
the repository's heavy wrapper, as the oracle's corpus does). `--pytest` sets the argv prefix that
runs pytest (default `python -m pytest`); the pytest adapter selects node IDs and reads pytest's
JUnit XML report through the compiled-in runner contract (PYR-V1-010), and its dependent units are
every collected test file, a conservative superset. Runner commands are trusted operator
input with process-group containment, not a sandbox. Clones are private, removed after each case,
and refused when the destination exists.
