# Retained qualification at the two launchers

V1-5694, RMV-V1-008 and RMV-V1-009. The earlier parity tests exercised the
reader and editor comparison. They missed the core session's selection call
and the installed launcher's check of the release environment.

The core regression starts a real session with a fixture JS provider and
compares the first qualification capture with the exact receipt on stdout.
A pre-existing second capture stops the session at its exclusive write. The
installed runner regression supplies both executable provider paths, omits
the retired MCP variable and supplies an out-of-release VSIX. Its refusal
at the VSIX check proves the runner passed provider validation. The fixture
redirects the macOS Code and temporary-directory paths for this preflight.
The editor comparison now hashes the same UTF-8 value span it parses from
the retained reader's buffer.

Both assertions failed with the consumers from
`1d8be9f4ccdd6e4ca8c2db4717eab01b18b6cbcc`. The core session raised
`ReferenceError: mcpPath is not defined`. The installed runner raised
`Error: CORVINT_TEST_VALIDITY_MCP must be an absolute path`.
On the repaired tree, `make -s interactive-alpha-test` passed 10 tests
with two opt-in browser tests skipped. `npm --prefix extensions/vscode test`
passed all 93 tests. Installed macOS release qualification remains unmeasured
by this run.
