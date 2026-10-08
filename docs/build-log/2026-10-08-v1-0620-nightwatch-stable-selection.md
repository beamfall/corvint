# 2026-10-08: Nightwatch stable selection expectation (V1-0620)

## Intent

Ticket V1-0620 records that Nightwatch native test identities carry the WebDriver session ID
(`ModulePath::Name::TestEnv::SessionID::Case`). Exact `ExpectedTests` must be approved before
launch, when a fresh session ID is unknown, so pre-admitted exact coverage could never be proven.
The last browser qualification left `ExpectedTests` empty rather than invent coverage.

The change adds proposed TRE-V0-021 to TRE-V0-023 to `docs/specs/test-runner-execution-v0.md`,
pending owner acceptance.

## Decisions

- **Separate contract, unchanged identity.** A new optional request field, `expectedSelection`
  `{version, matcher, tests}`, carries the stable expectation. Native IDs keep the session, and the
  field is `omitempty`, so historical plan and receipt bytes and plan digests are unchanged. A
  frozen plan and receipt generated at `0c94c66c` are pinned in the companion tests.
- **One version, one matcher, one runner.** `corvint-test-selection/1` with
  `nightwatch-session-elided/1`, bound to `nightwatch`. Any other runner, including WebdriverIO,
  needs its own matcher version. `ExpectedTests` and a selection are mutually exclusive, so
  coverage has one source.
- **Admission at the shared build entry.** `registry.Build` calls `AdmitSelection`. Both `plan`
  and `run` re-derive the invocation through it, so an invalid selection refuses before launch,
  and the selection is part of the approved plan digest.
- **Projection from structured fields.** The matcher does not split the ID string. It checks
  that the ID is `File::Suite::<env>::<session>::Name` for the test's own fields, requires a
  non-empty session and separator-free components without edge colons, and so stays injective.
  Missing, extra, aliased (one key from two native tests, for example two sessions of one
  module), unprojectable or invalid selections make the observation incomplete through shared
  `Normalize`.
- **Closed pointer decoding.** `DecodeDocument` now recurses through pointer fields, so the
  nested selection object rejects unknown and duplicate fields like other nested documents.

## Evidence

Focused tests in `internal/testrunner`, `internal/testrunner/dynamic`,
`internal/testrunner/registry` and `cmd/corvint-test-runner` pass. The dynamic test replays the
runner-generated Nightwatch fixture under two session IDs. The companion test drives `plan` and
`run` with a pinned stand-in executable that chooses its session ID at launch (`$$`), and
observes two differing sessions matching one pre-admitted selection.

A real pinned Nightwatch 3.16.0 / ChromeDriver browser run is NOT_RUN. No Nightwatch package or
ChromeDriver was installed locally (Chrome 154.0.8037.98 was present), and the earlier raw
`qualification.json` no longer exists. Session bytes in this evidence are synthetic.
