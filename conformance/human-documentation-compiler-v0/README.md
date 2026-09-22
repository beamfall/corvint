# Human Documentation Compiler V0 conformance

This corpus freezes the first qualified P0 profile: Material for MkDocs. It is an independent,
native Go black-box contract. The contained Corvint self-documentation sites precede any Beamfall
shadow integration.

Self-check the corpus without executing product code:

```sh
go run ./conformance/human-documentation-compiler-v0
go test ./conformance/human-documentation-compiler-v0
```

Run every case against an adapter executable. The runner invokes the argument vector directly,
without a shell, once per case. It supplies `CORVINT_HDC_CASE`, `CORVINT_HDC_WORKSPACE`, and
`CORVINT_HDC_FIXTURE_SHA256` in a credential-scrubbed environment. The adapter emits one canonical
JSON observation on stdout and no stderr:

```sh
go run ./conformance/human-documentation-compiler-v0 --command \
  /absolute/path/to/adapter --fixed-argument
```

Actual build claims additionally require a caller-owned environment manifest. A separately produced
network-denial receipt is accepted only as unqualified evidence: this runner has no external denial
observer and therefore cannot issue `OFFLINE_QUALIFIED=PASS`:

```sh
go run ./conformance/human-documentation-compiler-v0 \
  --environment-manifest /absolute/path/to/project-environment.json \
  --network-denial-receipt /absolute/path/to/network-denial-receipt.json \
  --network-denial-harness project-network-sandbox-v1 \
  --command /absolute/path/to/adapter
```

The runner copies each fixture to an isolated temporary workspace, verifies it is unchanged after
the adapter exits, caps stdout/stderr, applies the case deadline, and kills the owned process group
on timeout, output overflow, interruption, and normal completion. No command is run when
`--command` is absent; the canonical result says `NOT_RUN`.

The exact environment is caller-owned. Its closed JSON manifest has profile
`corvint-doccompiler-environment-trust/0`, `authority: project-owned`, `trusted: true`, absolute
regular-file `mkdocsExecutable`, `pythonExecutable`, and `lockFile` paths; exact `mkdocsVersion`,
`materialVersion`, `markdownVersion`, and `pymdownVersion`; and the lock file's SHA-256. The runner
validates and hashes it but never prints its bytes. A build
may be reported only after discovering and pinning
the MkDocs executable, MkDocs version, Material version, environment digest, effective config, and
isolated output root. The required argv includes `mkdocs build --strict`, `--config-file`, and
`--site-dir`. The corpus never installs or updates dependencies, invokes `gh-deploy`, or performs a
network request.

`BUILD_STRICT_PASS` and `OFFLINE_QUALIFIED` are separate truth axes. A strict zero exit is not an
offline proof. Hooks and plugins are project code; configured privacy behavior can fetch, cache,
and rewrite remote assets, while the offline plugin changes output semantics. Configuration alone
therefore cannot prove no-network behavior. `OFFLINE_QUALIFIED` requires an external network-denial
witness executed around the complete build plus a bounded generated-asset scan, otherwise it remains
`NOT_OBSERVED` or `UNKNOWN`. A caller-supplied receipt is not proof that this runner observed denial. The compiler
must never auto-enable privacy/offline plugins or rewrite accepted configuration.

Success proves corpus integrity and, with `--command`, the observed adapter behavior for these
fixtures. A structurally valid adapter receipt cannot make this runner claim that MkDocs, Material,
or a denial harness ran; build/offline promotion requires a future external execution observer.
It does not promote the Human Documentation Compiler use case,
authorize repository mutation, accept proposed prose, publish documentation, or qualify Beamfall.
