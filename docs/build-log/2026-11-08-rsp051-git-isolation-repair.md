# RSP-V0-051 read-only Git isolation repair

The self-review found that the test-evidence reader inherited ambient Git redirects and could
lazy-fetch missing promisor trees. Both violated RSP-V0-051's immutable, read-only query binding.
The reader now uses the existing `gitauth.Open`, `Resolve` and `DeltaPaths` authority boundary.
Its bounded object reader pins repository metadata, verifies tree objects, disables lazy fetch,
refuses every transport and admits only the existing environment allowlist. Missing objects
retain the existing unresolved-diff/FULL explanation; no wire shape or refusal code is added.

The closed environment admits PATH, SystemRoot, TMPDIR, TEMP, TMP and USERPROFILE plus the
kernel's fixed isolation settings. Other inherited variables are absent; ambient alternates
are refused by the kernel before object reads. Tree transitions decode only known blob modes
100644, 100755 and 120000 and gitlink mode 160000: additions, deletions and same-kind edits.
Unknown modes/types and type changes remain undecoded. This replaces the unrestricted subprocess
boundary as a class, rather than maintaining a list of dangerous environment variables.

Whole-package `internal/testevidencedoc` tests passed, including existing goldens and read-only
checks, missing tree-filtered promisor objects with the lazy-fetch guard present and removed,
remote-access sentinels and before/after repository digests, ambient redirects, an unknown future
Git variable, and unknown entry forms. The first new regression asserted the unresolved reason
at the document level; the existing contract puts it in the runner explanation. The regression
was corrected to assert that exact nested reason and FULL mode. No pre-existing assertion or
golden changed. Independent review and host integration remain orchestrator-owned.

Rollback is to revert this repair; doing so restores both known isolation defects and must not
be represented as a qualified read-only query implementation.
