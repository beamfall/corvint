# TCQ 0 conformance

Run the deterministic reference vectors from the repository root:

```console
go test ./internal/tcq ./internal/frontierrepo
```

The suite constructs one fixed SHA-1 repository, verifies canonical CEM 0.2 and OCM 0.1 authority,
and compares static and caller-reported dynamic TCQ bytes exactly. It also checks malformed input and
hostile XML fail closed. Passing these vectors establishes reference wire conformance only; it does
not authenticate the caller, prove test adequacy, establish reporter compatibility, or satisfy the
independent corpus and ten-change promotion gates.
