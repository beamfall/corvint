# 2026-10-19 V1-1939, V1-1940: response-wait diagnosis, role locators, test environment contract

Tickets: V1-1939 (GitHub #1197) and V1-1940 (GitHub #1199). Requirements: TSA-V0-200..216 in
`docs/specs/test-source-analysis-v0.md`. They are agent-drafted and accepted under the owner's
standing auto-accept (2026-10-11).

## Decisions

- **Response waits are diagnosed from a log the caller names.**
  - `corvint runs response-wait` reads a HAR or a JSON-lines server log. It takes the wait's
    start, timeout and predicate as arguments; Corvint does not capture the wait itself.
  - When no request is found, the result is `REQUEST_NEVER_SENT` only if the log covers the wait.
    A missing, empty, malformed or late-starting log is `UNKNOWN`.
  - Preceding failures are scoped to the HAR page the wait ran on, so a 500 on an earlier page is
    not blamed.
- **The role-locator check reads the mounted component only.**
  - It is limited to the four notification roles that #1197 is about.
  - It reports only when every mounted component is resolved and its markup is closed. A child
    component, a dynamic role, spread attributes or raw HTML abstains instead of being guessed.
- **The environment contract reads only committed shell bootstrap scripts.**
  - Compose and dotenv files are named in `unreadSources`, not read: there is no YAML reader in
    `go.mod`, and the slice needs none to meet the ACs.
  - Branch-sensitive shell is read as last-assignment-wins. A variable set in two profile scripts
    is therefore `UNKNOWN` rather than picked.
  - The hard-coded offset and literal warnings are advisory and live in `corvint tests env`. The
    `tests validity` projection is left to a residual: that registry is being edited by a peer
    lane (V1-1919), and the coordinator asked for minimal shared-registry edits.
- **Each rule lives in its own file.** The shared files changed are the three switch cases and
  help lines in `cmd/corvint/runs.go` and `cmd/corvint/tests.go`. `main.go` is unchanged.
- **The new spec sections are appended at the end of the file.** The peer's TSA-V0-160..176
  section sits before "Decision file format", so appending avoids an overlapping hunk.

## Independent review

One Codex round (read-only) returned 15 findings. Twelve were fixed with tests:
- an expansion bound on resolved values;
- line-by-line JSON-lines reading;
- preferring an in-window match over an earlier slow one;
- rejecting a HAR entry with a missing or oversized `time`;
- re-exports and `{children}` slots opening markup;
- skipping commented-out locators;
- `<output/>` and `<output` followed by a newline;
- `${A:-d}` with an empty `A`;
- keeping export-only variables;
- string concatenation in `process.env.X + N`;
- impact membership decided from the base value when the new one is unknown.

Three were recorded as failure modes in the spec instead: mounts per file rather than per test,
last-assignment-wins shell ordering, and route fields read up to the next registration.

## Residuals (not delivered)

- Reading compose files, dotenv files and CI workflow environments.
- Go, Flask, Rails and WireMock stand-in routes.
- The warning in the `tests validity` projection.
- The impact in `corvint impact`.
- Per-test mount scoping, ordered shell evaluation and handler-bounded route fields.
- Qualification on a real suite (NOT_RUN).
