# PMR-V2-006 host process collector slice (#395, V1-0542)

The PMR-V2-006 verifier (`internal/postmergeproof`) needs a raw `postmerge-process-proof/2` to
check. Without a collector, nothing could produce one, so no qualification campaign could run and the
PMR-V2-002 decision emitter had no possible token source. This slice adds the collector and the
pinned observer role. It does not wire them into the native producer run, and it mints no token.

Decisions:

- The observer is a separate pinned process selected by argv. The host launcher binary gets an
  `--internal-process-observer` mode beside `--internal-envelope`. This avoids a second pinned
  executable. The verifier requires the observer's executable to equal the policy observer identity.
  It also requires the final sweep to come from a completed `exit:0` launch that is not the
  supervisor.
- The trusted-start observer starts at the barrier before the workflow root. Its `TrustedStartV2`
  stdin, which names the root's start-barrier capture, is written only after that capture exists.
  Its invocation preimage is retained at delivery time. The observer checks that the named observer
  identity is the hash of its own `/proc/self/exe`.
- The absence-sweep observer blocks on an empty stdin. The parent closes that stdin only after
  capturing the observer's birth. The sweep therefore cannot precede the observer's own capture,
  and the swept inventory contains the observer, as the verifier requires. After the sweep starts,
  the collector refuses every further owned launch.
- Each executable is retained from `cmd.Path` before start, and it must equal the captured
  `/proc/PID/exe` content. `cmd.Start` returns before the new argv is visible, so the collector
  waits, within a bound, until `/proc/PID/cmdline` equals the requested argv bytes before capturing.
- The supervisor is recorded with the allowlisted subset (`LANG, LC_ALL, PATH, TMPDIR, TZ`) of its
  own environment. Owned launches must pass an explicit environment inside that allowlist.
- New refusal codes, all BLOCKED:
  - `process-collection-refused`: misuse.
  - `process-collection-failed`: capture, retention, settle or exit failure.
  - `process-observer-refused`: observer mode, identity or sweep.

  Unsupported hosts stay NOT_OBSERVED with `process-observation-unsupported`.
- The end-to-end test uses the `postmergeproof` test binary as both supervisor and observer, through
  a TestMain dispatch. The observer identity then equals the supervisor executable without building
  the launcher. The test feeds the collector's output to the real verifier through a test-only export
  of the logical graph, and confirms that the production `VerifyProcessV2` stays BLOCKED
  `process-qualification-unavailable`. A planted surviving child is REJECTED
  `process-sweep-survivor`.
- `internal/postmergehost/envelope_linux.go` used `syscall.Dup2`, which does not exist on
  linux/arm64. The package therefore did not build there. That breakage predates this slice. This
  slice replaces the call with `syscall.Dup3` (flags 0), guarded so that it is skipped when the null
  descriptor is already 0. `Dup3` exists on every Linux port, and `Dup3` with equal descriptors
  returns EINVAL.

Evidence:

- darwin: `go test` passes for `internal/postmergehost`, `internal/postmergeproof/...` and
  `cmd/corvint-postmerge-host-launcher`.
- Linux arm64 container (go1.27.1): the same packages pass, and with `-race -v` all new tests pass.
- `go vet` is clean for darwin, linux/arm64 and linux/amd64.
- NOT_RUN: Linux amd64 runtime.

Limits retained:

- No protected on-disk retention: artifacts go to a caller-supplied `ProcessRetainerV2`.
- Provenance is trusted-local only.
- Not yet produced:
  - wiring into the producer run and the host launcher request path;
  - native process locators;
  - `playwright-node-worker/0` and `chromium-switch-role`;
  - an actual exact-tuple qualification.

Rollback: revert this change. Nothing consumes the collector yet.
