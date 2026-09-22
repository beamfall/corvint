# CEM 0.2 conformance

This profile reuses the frozen CEM 0.1 evidence and hunk vectors and adds the exact
`excludedPath` wire contract. Run from the repository root:

```console
go test ./internal/cem/wire ./internal/cem/verify ./internal/cem/workflow
```

The native wire suite consumes every frozen structural mutation; verifier and workflow suites
retain canonical repository checks. The Python oracle driver is retired under GOC-V0-002.

Structural verification proves structural compatibility only; only the independent repository workflow may emit
`"assurance":"canonical"` after deriving and binding the patch itself. Canonical Git derivation,
raw target-side sidecar binding, and independent revision authority are exercised by the workflow
and CLI acceptance tests.
