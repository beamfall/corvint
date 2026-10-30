# Decision 0379 — OpenCode file-change repository refusal is expected, and the real binary is tested

Date: 2026-09-24. Status: accepted. Authority: repository owner, "there is always messages in the
terminal with support:FALLBACK,and code:invalid-arguments. I want those gone for good" (2026-09-24).
It amends `AHI-022` and extends decision 0378.

## Context

Decision 0378 stopped the `invalid-arguments` notice by registering nothing outside a Git
repository. The same audit found two more gaps:

- Every OpenCode test ran against `integrations/testfixture`. That fixture accepts any `--root`,
  so no test could see the real `notRepositoryRootRefusal`, and the 0378 fault shipped unobserved.
- The real binary answers a `file.edited` event for a `.go` path in a repository without `go.mod`
  with `unsupported-impact-repository` (exit 2). The plugin treated only
  `unsupported-impact-path-suffix` as expected, so an ordinary edit printed a terminal warning.
  This is the same kind of best-effort refusal and has no user action.

## Decision

- On `file-change` the OpenCode plugin classes `unsupported-impact-repository` with
  `unsupported-impact-path-suffix`. It records the code in `client.app.log` and prints no terminal
  warning. Every other failure keeps its `AHI-022` warning.
- `TestHostAdapterJavaScriptHosts` also builds `./cmd/corvint` and passes it to the JavaScript
  suite as `CORVINT_TEST_REAL_BINARY`. A new test drives a full OpenCode lifecycle against it in a
  committed repository and asserts:
  - no terminal warning other than a disclosed `timeout` bound;
  - every logged code is a recognised degradation, an expected file-change refusal or a guard;
  - a non-repository root is refused as `invalid-arguments`;
  - the plugin registers nothing when opened with worktree `/`.

## Consequences

- OpenCode adapter 0.2.8 (`AHI-020`).
- The JavaScript host test builds one more binary, which adds build time but no new dependency.

## Rollback

Remove `unsupported-impact-repository` from `EXPECTED_FILE_CHANGE_REFUSALS` in
`integrations/opencode/src/index.js`, and remove the real-binary build, its env var and the two
new tests. The warning then returns for `.go` edits in repositories without `go.mod`.
