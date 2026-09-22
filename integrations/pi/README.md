# Corvint for Pi — experimental FALLBACK

Pi 0.85.1 can load this extension explicitly:

```sh
CORVINT_BIN=/absolute/path/corvint pi -e /absolute/path/integrations/pi/index.ts
```

The native Corvint binary must include `adapter pi`. `CORVINT_BIN` selects the binary; an
empty value refuses before execution.
The extension's factory uses the host runtime `VERSION`, not an environment version claim.
Package discovery uses the `pi.extensions` declaration. Install/uninstall behavior has not yet
been qualified; the explicit path is the tested development route.

Startup and prompt context use the same native Go compiler. Prompt context is appended only
for the current turn as framed untrusted repository data. No prompt, transcript, model-message
body or image is added to adapter storage. The existing bounded core self-observation ledger
exception remains; Pi's own session policy is separate. Untrusted projects refuse native reads.
Tool results currently produce empty observations; they do not infer changed files or tests.

`/corvint-context TEXT` requests context. `/corvint-outcome JSON` accepts an explicit existing
session-end outcome schema, including taskSha256, changedPaths and verification. It does not
infer success and the legacy core reports outcome persistence unavailable. For exact expansion:

```sh
corvint adapter source-view --root ROOT --packet PATH --packet-sha256 SHA --result N
```

Optional selectors are `--evidence N`, `--commit SHA`, `--lines RANGE`, `--requirement ID`, and
`--max-bytes N`; native validation owns their meaning.

Automatic and command transport deadlines are 2000ms including normal cleanup reserve; this is
not the 250/500ms p95 qualification target. Timeout, malformed output and cleanup faults are
visible. Stop never requests continuation and never claims protected authority. JSON process exit
zero and `agent_settled` are not successful completion evidence. Windows is unsupported because
POSIX process-group cleanup is required.

The adapter is not FULL. Protected Pi identity/topology, authority, permissions, latency,
interactive fault visibility, release packaging and the full host matrix remain unqualified.

The startup cancellation latch applies only to initial non-UI startup. Later session
transitions and interactive Ctrl-C remain under Pi handlers; the print-mode startup
probe does not qualify ordinary interactive cancellation.
