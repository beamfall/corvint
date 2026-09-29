# Issue338 live qualification: opaque gitlink finalization repair

The frozen `c77d7cfa675d072466abc8b83ed6f7467022b929` qualification exposed an integration gap:
flow generation/check passed with an opaque gitlink, but generic `doccorpus.Inventory` refused the
containing scope. `Finalize` used that generic directory inventory and therefore could not admit the
advertised150-flow source through corpus Build/Open. The original scale regression stopped too early.
Root retained the real failure against source `b14fe833674b0beb66067f9ac22d4ce8cc5efeea` in the live
qualification workspace; its independent Playwright evidence was not reclassified or fabricated.

Repair stays inside the flowdocs profile. Finalization inventories each immutable regular
superproject source file as an exact corpus scope. Gitlink paths/commit identities are rederived
from Git and retained in the generation manifest and unknown provider module records; nested
contents remain outside coverage. Symlinks and other unsupported tree entries still refuse. Generic
corpus inventory is unchanged. `check` reports changed/added/retired opaque identities separately
and fails closed without suggesting knowledge of their contents. Existing corpus enrichment merges
all missing exact source scopes, preserving original evidence providers.

The extended150-flow/1,200-page regression now commits the generated provider and runs
Generate→Finalize→Build→Encode→Open with a gitlink present. It verifies no gitlink/nested path becomes
a corpus input, its commit survives as explicit uncertainty, and a later gitlink change produces
only opaque identity drift. A separate regression retains symlink refusal.

Observed focused results (local Go toolchain/cache):

- `go test -count=1 -timeout 30m ./internal/flowdocs ./cmd/corvint -run 'TestFlowDocs(ScaleDeterminismSafetyAndGitlink|RevalidatedCorpusContext|CLIEndToEnd)$'`: passed (7.636s/2.019s).
- `go test -count=1 -timeout 30m ./internal/flowdocs -run 'TestFlowDocs(ScaleDeterminismSafetyAndGitlink|SourceInventoryStillRefusesSymlink)$'`: passed (6.152s).
- `go vet ./internal/flowdocs` and `git diff --check`: passed.

This is an observed qualification failure repair. Root retains independent review, refreshed freeze,
live end-to-end qualification, enrolled checks and CEM/seal/native completion. No builder commit,
publication, native completion write or new provider/server session occurred.
