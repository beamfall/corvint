# Native CEM adapter dependency claim

Intent status: proposed
Delivery status: experimental (checked by `TestCEMSeamsDependOnlyOnStdlibAndGit`)

## Agent digest
- Claim: CEM seams depend only on the Go standard library and local Git, not kernel, provider, analyzer, or network surfaces.
- Status: proposed/experimental (checked by `TestCEMSeamsDependOnlyOnStdlibAndGit`)
- Exists: standard-library-only CEM seams checked by `TestCEMSeamsDependOnlyOnStdlibAndGit`.
- Blocked on: none stated; the claim excludes the rest of the `corvint` binary.
- Read next: the scoped dependency statement and binary-scope caveat below.

## Dependency boundary

The CEM seams and their command dispatch —
`internal/cem/{cemcode,wire,patch,sim,gitrun,gitauth,verify,publish,mdreport,workflow,cli}` —
depend only on the Go standard library and the local Git executable. They
import no `internal/gokernel`, runtime, provider, analyzer, or network
surface, and make no network request. This claim is checked by
`TestCEMSeamsDependOnlyOnStdlibAndGit`, whose `go list -deps` closure covers
the dispatch package because it lives under `internal/cem`.

The `corvint` binary as a whole also hosts the other experimental slices
(`query`, `impact`, `harness`), which carry their own internal dependencies;
the claim above is scoped to the CEM packages, not to the whole binary.
