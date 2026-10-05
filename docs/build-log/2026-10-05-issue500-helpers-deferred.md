## 2026-10-05 Issue 500: helpers deferred, issue closed

Human-owned intent: on 2026-10-05 the owner deferred helpers for issue #500 and asked to close it.

### Decision

- Helpers, `run-helper` and service logging (`SERVICE500-007`) are deferred. Install keeps refusing
  helpers with `UNSUPPORTED`, so no behavior or wire contract changes.
- Native ticket V1-0804 tracks the deferred helper work, including runtime restart-debt charging.
- V1-0697 stays open. On 2026-10-04 the owner deferred platform qualification
  (`SERVICE500-004`, `-005`, `-010`) until a disposable Darwin/Linux target exists, and this decision
  does not change that.
- Issue #500 closes with the runtime slice delivered: install, status and uninstall, stop and drain,
  the shared launch fence and the legacy stop file. These merged to main by 618680492cf8.

### Limits

- No `SERVICE500-007` witness is accepted by this deferral, and every platform witness stays NOT_RUN.
