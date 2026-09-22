# Corvint dashboard snapshot V0 conformance

This directory is an independent, dependency-free Go 1.27 verifier for canonical
`corvint-dashboard-snapshot/0` bytes. It imports no `internal/dashboard` package and performs no
repository, network, process, or filesystem mutation beyond reading the explicitly named snapshot.

Verify a snapshot:

```sh
GOTOOLCHAIN=local go run ./conformance/dashboard-snapshot-v0 \
  --snapshot /absolute/path/to/snapshot.json
```

Run the frozen corpus:

```sh
GOTOOLCHAIN=local go test -count=1 ./conformance/dashboard-snapshot-v0
GOTOOLCHAIN=local go test -race -count=1 ./conformance/dashboard-snapshot-v0
GOTOOLCHAIN=local go vet ./conformance/dashboard-snapshot-v0
```

`testdata/valid-observed.json` freezes the exact empty, valid local-trace-store snapshot. Its four
inventory metrics distinguish one configured aggregate from zero retained members and prove that a
closed empty universe is measured zero. It is canonical wire, identity, registry, trace-store hash,
source-set hash, inventory-arithmetic, and snapshot-hash evidence. It is not acquisition, Git
authority, trace-row semantic, or production-provider conformance.

`cases.json` freezes hostile mutations and stable rejection codes for duplicate/unknown fields,
noncanonical JSON, JSON numbers, Unicode outside the closed ASCII V0 wire domain, timestamps,
registry/source/cohort/issue identities, unavailable-versus-zero truth, inventory arithmetic,
scan-state consistency, dangling references, privacy text, ordering, the 4 MiB bound, and the
null-preimage snapshot hash.

The snapshot alone intentionally cannot prove retained trace outcome counts: its public member rows
contain revision, content digest, and bytes, not normalized outcome counts. The owning specification
therefore assigns that proof to the separate planted-repository black-box trace conformance runner.
Passing this verifier must not be presented as evidence that source acquisition, trace validation,
Git reachability, provider output, UI, server, Pulse, harness, Frontier, CEM/OCM, or Beamfall support
has shipped.
