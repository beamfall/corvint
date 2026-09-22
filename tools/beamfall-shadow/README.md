# Beamfall shadow harness

Private experimental developer tooling for a bounded direct observation of an
explicitly selected, separately versioned external analyzer artifact. It is **NO_SUPPORT**,
**NO_CORVINT_CORE_LAUNCH**, and **NO_ADMISSION_NO_EXACT_VERIFIER_NO_AUTHORITY**.
It is not a registry, lock, package, Core command, or supported Beamfall
integration.

The caller provides one canonical `beamfall-shadow-dogfood/2` manifest. Its
outer closed registry row names plugin/release/version, external dogfood
family, language, framework, toolchain, toolchain version, frozen-candidate
family mapping, input family, artifact, and SHA-256. The manifest is not part
of the frozen analyzer request. Unknown or mixed tuple fields, noncanonical
manifests, symlinks/hardlinks, replacement, and digest drift fail closed. Rows
without one of the frozen four-family mappings emit a bound `NOT_RUN` receipt
and never start an artifact. The caller reads at least two explicit logical
paths from `--target-repo` at `--target-rev`, constructs canonical bounded
bytes, privately stages the exact selected artifact, then starts it with an
empty environment, fresh empty `0700` working
directory, no arguments, and a process group. The child receives only the
already-verified artifact descriptor on a platform with a supported
descriptor-execution primitive. Darwin has no accepted primitive for arbitrary
artifacts: `uchg` is mutable by the same UID, so Darwin fails
`UNSUPPORTED_EXECUTION_AUTHORITY` before candidate process start. The
receipt is required at a new
absolute path under a private directory and is never written to stdout. It
contains only identities, digests,
counts, statuses, timings, and rusage; it contains no source bodies, absolute
paths, environment values, command text, or raw stderr.

Run only against a clean, reviewed, immutable candidate revision:

```sh
go run ./tools/beamfall-shadow \
  --plugin-manifest /absolute/private/plugin.json \
  --target-repo /path/to/beamfall --target-rev COMMIT \
  --input src/component.tsx --input src/component.test.tsx \
  --receipt /absolute/private-directory/receipt.json
```

`network` and `repository_read_denial` remain `NOT_OBSERVED`: the harness is
not an OS security sandbox. The receipt's `harness_executable_digest` hashes
this harness executable itself, never Corvint Core; process-group states claim
`GROUP_EMPTY_OBSERVED` for the original group only — a descendant that leaves
the group (setsid) is adopted and outside that observation. A receipt labels non-Linux containment
`PROCESS_GROUP_ONLY`; detached descendants remain outside that proof boundary.

On Linux, the harness and wrapper both establish and immediately query
`PR_SET_CHILD_SUBREAPER` / `PR_GET_CHILD_SUBREAPER` before starting the
candidate-facing process group. A failed set or probe is
`UNSUPPORTED_EXECUTION_AUTHORITY`; it starts no candidate. For a containerized
observation, use Docker's init and kernel boundaries explicitly:

```sh
docker run --rm --init --network=none --read-only \
  --tmpfs /tmp:rw,exec,nosuid,nodev,size=512m \
  --mount type=bind,src=/absolute/corvint,dst=/corvint,readonly \
  --mount type=bind,src=/absolute/beamfall,dst=/beamfall,readonly \
  --mount type=bind,src=/absolute/private-receipts,dst=/receipts \
  IMAGE go run ./tools/beamfall-shadow \
  --plugin-manifest /receipts/plugin.json --target-repo /beamfall --receipt /receipts/receipt.json ...
```

`--init` is required to provide container PID-1 reaping; the in-process
subreaper probe remains mandatory and is not satisfied by a caller flag or
this documentation. `--network=none`, read-only repository mounts, and a
private writable receipt mount constrain the container boundary only; they do
not turn the shadow harness into a supported integration or an OS sandbox.
