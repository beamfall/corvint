# 2026-10-20 Ruby provider: live RSpec, Corvint formatter, run container and RSpec falsification binding (V1-2010, V1-2013, V1-2014, V1-2027)

Human-owned intent:

- `docs/specs/live-proof-carrying-verification-v0.md`, the sections "RSpec example selection,
  runtime identity and shared-example origin" and "Corvint RSpec formatter, run container and live
  qualification" (`LPCV-V0-136..139`).
- `docs/specs/browser-behavior-falsification-v0.md`, `BBF-V0-029..033`.

All nine requirements were accepted under the owner's standing auto-accept (2026-10-11). They
address GitHub #1271 (V1-2010 residual, V1-2013, V1-2014) and the GitHub #1274 residual (V1-2027).

BASE: `f3ca72248bb693534f006181301889bf797cc073`.

## Problem

`LPCV-V0-131..135` left three things `NOT_RUN`:

- a real RSpec run;
- a real container prefix;
- a formatter that emits `shared_group_inclusion_backtrace`. RSpec's stock JSON formatter never
  emits it, so shared-example origin was always `not-reported`.

The runtime probe also could not name the run's own container or its image. The runner-neutral
falsification profile (`BBF-V0-024..028`) bound only `go test`.

## Decisions

- **Opt-in shipped formatter (`LPCV-V0-136`).** A flag, `--formatter corvint-rspec-json/0`, opts in.
  - The provider writes an embedded subclass of RSpec's `JsonFormatter` into its own ignored report
    directory, `.corvint/rspec/`. It requires the file by relative path and removes it after the
    run.
  - The formatter always emits the inclusion array. `[]` means not shared, so a stock report's
    absence (`not-reported`) stays distinct from `not-shared`.
  - The stock path is unchanged, and the document carries a closed `formatter` member only when the
    Corvint formatter was used.
- **Inclusion-line selection (`LPCV-V0-137`).** A `path:line` selector on an `it_behaves_like` line
  is `MATCHED` when the report states the inclusion. Under the stock formatter it stays
  `UNATTRIBUTED`. This is the observed RSpec behavior, not a guess.
- **Run container from inside, image from the host (`LPCV-V0-138`).**
  - The formatter reads its own container ID from the `/etc/hostname` bind mount in
    `/proc/self/mountinfo`. The probe container is never substituted for it.
  - The host then runs `docker <global flags> container inspect --format {{.Image}} <id>` and admits
    only `sha256:<64 hex>`.
  - `--rm` removes the container before the inspection, so `image` is
    `unknown: container-not-inspectable` there. That is recorded as an unknown, not hidden.
  - The value is the daemon's image ID, not a registry repository digest.
- **RSpec falsification binding (`BBF-V0-029..033`).**
  - The plan admits RSpec example ids (`spec/x_spec.rb[1:2]`), including as unrelated and setup
    observations.
  - The run forces `BUNDLE_FROZEN=true` and an empty `SPEC_OPTS`, and writes a fixed
    project-relative JSON report. A container wrapper that shares the directory therefore writes
    where the runner reads.
  - Only RSpec's expectation and mock error classes count as assertions; any other exception is a
    `panic`.
  - `execute-receipt` and `verify-receipt` fail closed with `receipt-profile-unsupported`.

## Owner-approved toolchain install (2026-10-20, scoped)

The owner approved, for this lane only, installing pinned RSpec gems with verified checksums. The
gems were to go only inside a disposable Docker image built from the local `ruby:3.4-bookworm`
image, never on the host Ruby, with no shell-profile or system-setting changes.

- **Image:** `corvint-rspec-qual:2026-10-20`, ID
  `sha256:b4202eef8ad08df2c18d7d38008a02910377d5f37943c54b13a9827a83b532db`.
- **Base:**
  `ruby:3.4-bookworm@sha256:246b2dc3f6e40bba3af18503c22997a34dbb27c9f97e198dde6dd727895115c5`.
- **Runtime in the image:** ruby 3.4.11 (2026-09-23 revision 592f1ffdb3) aarch64-linux, Bundler
  2.6.9.
- **Install procedure:**
  - `gem fetch` each pinned version inside the build.
  - `sha256sum -c --strict` against the SHA-256 values that the rubygems.org versions API publishes
    (all five `OK`).
  - Require exactly five archives, then `gem install --local --no-document`.
  - Keep the checksum list in the image at `/usr/local/share/corvint-rspec-gems.sha256`.

| Gem | Version | SHA-256 |
| --- | --- | --- |
| rspec-core | 3.13.6 | `a8823c6411667b60a8bca135364351dda34cd55e44ff94c4be4633b37d828b2d` |
| rspec-expectations | 3.13.5 | `33a4d3a1d95060aea4c94e9f237030a8f9eae5615e9bd85718fe3a09e4b58836` |
| rspec-mocks | 3.13.8 | `086ad3d3d17533f4237643de0b5c42f04b66348c28bf6b9c2d3f4a3b01af1d47` |
| rspec-support | 3.13.7 | `0640e5570872aafefd79867901deeeeb40b0c9875a36b983d85f54fb7381c47c` |
| diff-lcs | 1.6.2 | `9ae0d2cba7d4df3075fe8cd8602a8604993efc0dfa934cff568969efb1909962` |

`diff-lcs` is a required runtime dependency of `rspec-expectations` and `rspec-mocks`. It was
pinned and verified the same way; it is disclosed here because the approval named only the four
`rspec-*` gems. The image is disposable: remove it with `docker image rm
corvint-rspec-qual:2026-10-20`.

## Live evidence

The host was Colima (arm64). The project lived under a disposable `~/.cache` directory, because
Colima mounts only the home directory. The bind was relative (`-v .:/work -w /work`), so no host
path reaches a recorded argv.

`TestLPCVV0139LiveRSpecThroughDocker` passed. It retained three documents under
`internal/rubytestprovider/testdata/live/`, which `TestLPCVV0139RetainedLiveDocuments`
re-validates in CI:

- **`corvint-lines`** (`--rm`):
  - `:7` and the inclusion line `:10` are `MATCHED`; `spec/empty_spec.rb` is `NO_EXAMPLE`.
  - Shared origin is `spec/support/shared.rb:2`, included at `spec/widget_spec.rb:10`.
  - `runContainer` is distinct from `probeContainer`, and `image` is `container-not-inspectable`.
- **`corvint-ids-kept`** (no `--rm`): `spec/widget_spec.rb[1:2]` is `MATCHED`, and `image` equals
  the image ID above. The test removes both reported containers by ID.
- **`stock-lines`:** `:10` is `UNATTRIBUTED`, origin is `not-reported`, and `image` is
  `not-observable-inside-prefix`.

`TestBBFV0033LiveRSpecNegativeControls` passed through a docker-prefixed toolchain wrapper. It
covers five controls:

| Control | Result |
| --- | --- |
| kill | `killed` |
| raise outside an expectation | `invalid_control`, `panic` |
| unused perturbation | `survived` |
| undeclared key | `invalid_control` |
| writes a tracked file | classified; the protected repository stayed at its revision |

After the runs, no lane-created containers were left.

## Limits

- One host and one image were observed. No Ruby, Bundler or image tuple is qualified.
- The image value is a daemon image ID, not a registry digest.
- Podman and Compose service containers report `host-inspect-unavailable` or run-container unknowns
  by design.
- `BBF-V0-032` keeps RSpec falsification evidence out of retained receipts until a native report
  binding exists.

## Rollback

Revert the change. Documents with `formatter` or `runContainer`, and RSpec provider plans, are then
refused by closed decoding. The stock formatter path and the Go provider are unchanged.
