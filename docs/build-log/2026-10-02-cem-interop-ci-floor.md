# Portable interop CI compiler floor (V1-0675)

Public base `1fda1b94984245d0cd0ac0a6d17cc72572ce6619` declares portable
`go 1.23`; incoming CEM source `1a355ea551c2745d3fe9f229a36acc348cb205d8`
declares `go 1.24.0`. The existing CI pin of Go 1.23.12 cannot satisfy that
incoming minimum with `GOTOOLCHAIN=local`. This is a configuration incompatibility;
a newly updated hosted run has not yet been observed.

Pin the go-interop job to Go 1.27.1, matching the existing production, documentation
and artifact jobs. Preserve action hashes, cache settings, offline module controls,
local toolchain selection, timeout and all formatting, test, vet and Windows build
steps. Explicit setup-go installation remains the compiler setup mechanism.

The incoming source and actual Linux arm64 qualification snapshot have the same
Git tree `7d34fba6882a4b7cbbec1f0ef678ede962d8bc96`. Five scoped package results
and 54 Go 1.27.1 full-result cases passed there; two historical Go 1.24 cases
remain separately qualified on Darwin arm64 with Go 1.24.13. These results do
not establish portable Ubuntu amd64 qualification. Local checks on this branch
exercise the current public module; actual hosted qualification of the incoming
Go 1.24 floor remains required before V1-0675 completion.

The pre-edit Corvint query and impact receipts are retained privately. Workflow
impact was admitted with unknown reverse-import coverage and budget omissions;
focused portable and documentation checks resolve this narrow configuration change. The initial dogfood pass before a source commit cannot prepare
an empty committed diff; its missing-input and empty-diff reasons remain retained.

Rollback is a forward revert of the workflow pin. After the incoming floor lands,
restoring Go 1.23.12 requires a compatible replacement or integration hold.
Decision 0390 requires owner consent on the exact reviewed SHA and successful
required hosted checks; this change grants no bypass or merge authority.

Local Go 1.27.1 Darwin arm64 portable tests, vet and Windows amd64 cross-build
passed. The five focused documentation checks passed. Formatting and exact offline
portable execution are retained separately; hosted execution remains NOT_RUN.
