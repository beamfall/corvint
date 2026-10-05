# V1-0783: the host fixture publishes its child PID by rename

Date: 2026-10-05. Ticket: V1-0783 (P3, tests). Test: `TestHostAdapterJavaScriptHarnessInterruption`
(`cmd/corvint/host_adapter_javascript_test.go`, GOC-V0-008).

## Finding

The test failed once under load on 2026-10-04 with `harness ended before descendant witness: <nil>`
(Go 1.27.1, darwin/arm64). The cause was a race in the test fixture, not in the adapters.

- **Fixture write:** in `hang` mode, `integrations/testfixture/main.go` published the child PID
  with `os.WriteFile`. That call creates the file before it writes the bytes.
- **JS read:** the `opencode interruption leaves no descendant` test polls `existsSync(f.childPID)`
  and then copies the file into the Go witness. A read inside that window copies an empty PID.
- **Go side:** the outer Go test parses the PID as 0 and keeps polling.
- **Ending:** the inner harness is never interrupted. It finishes on the adapter's own 2 s kill
  timer and exits successfully, so the outer test reports that the harness ended first.

The `-orphan` witness in the same fixture already published by temporary file and rename.

## Evidence

- **Reproduction:** a scratch fixture that created the empty file and slept 300 ms before writing
  failed 3 of 3 runs (`-count=3`). Each failure had the original message and took 6–9 s; the
  original failure took 6.22 s.
- **Fix:** the `hang` writer now writes `<childPID>.tmp` and renames it into place.
- **Fix under the same delay:** with the 300 ms delay kept between the temporary write and the
  rename, the test passed 3 of 3.
- **Assertions unchanged:** the interruption assertion and the witness deadline are not changed.
- **Side effect:** the JS timeout tests read the same file with `Number(readFileSync(...))`. An
  empty read there gave `NaN`, and `process.kill(NaN, 0)` throws, so the test read the child as
  already gone and could pass falsely. Publishing by rename closes that window as well.

## Limits

- The original failure was not reproduced under natural load. The mechanism was proven by widening
  the window deterministically.
- `corvint affected` reports scope `UNKNOWN`: the fixture has no selectable test, and unbounded
  readers select broadly. The fixture's only consumer is `host_adapter_javascript_test.go`, and
  that test was run.

## Rollback

Revert the commit. The empty-read window returns.
