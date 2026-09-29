# Explicit published predecessor for hosted lifecycle qualification

V1-0234 preparation at base `1b6ac29fa9cb90a07fb5d9cef97e49cc0b1432f5` found that preserving published
release tags across a main-history rewrite leaves the hosted lifecycle selector with no reachable
previous tag. The archived rehearsal reproduced an empty hosted selector and a failing default Core
`git describe`; it did not rewrite public refs. The owner authorized this isolated selector repair,
and the independent Gate A review found no HIGH concerns.

The hosted workflow now accepts an optional paired predecessor tag and direct object ID. The helper
resolves only the exact tag ref, requires the full candidate to name a commit directly, records the
candidate/tag/direct OID/peeled commit, and rejects incomplete inputs, unsafe names, missing refs,
identity drift, noncommit tags, the candidate itself, descendants, and Git failures. Automatic
selection still uses the newest reachable prior tag but refuses an empty or malformed selection
without falling back. Git reads ignore replacement objects, local/ambient grafts and inherited Git
environment. A lightweight tag's direct OID is its commit OID.

This is the existing published-archive lifecycle route for `SOP-V0-003`, documented in the release
runbook. A disconnected explicit predecessor is an operator choice, not proof of chronological order.
Published archive checksums remain the existing check; no new source-build attestation is claimed.
Core's accepted `CCF-V1-007` replay remains the separate opt-in
`make core-n1-replay CORE_N1_TAG=vW.V.U`; the Makefile and stable baseline are unchanged.

Focused validation: `bash script/select-previous-release_test.sh` covers ordinary and preserved
history, annotated/lightweight identities, unsafe and multiline inputs, noncommit candidate/tag,
missing/moved refs, no older fallback, actual `.git/info/grafts`, ambient `GIT_GRAFT_FILE`, replacement
objects and injected Git failures. It also executes the workflow's actual archive run block and log
helper with stubbed archive/network commands: refusal prevents `gh` invocation; success logs the
selected identities and produces the existing log checksum. `shellcheck` and `actionlint` pass. The
helper also successfully selected the preserved rc.1 tag against the real disposable rewrite; no
hosted qualification or release publication was run. Required focused documentation checks and
post-commit dogfood evidence accompany delivery; a full repository gate remains NOT_RUN under the
scoped-work instruction, not equivalent to these focused checks.

Corvint routes used: pre-change query and baseline/changed `affected` receipts, retained outside the
checkout; substantive dogfood enrollment/change/CEM/OCM/check/seal. The query uses immutable tree
identity, not the commit ID. Affected retains its non-Go/language-frontier unknowns and static full
suite advice; the owner explicitly requested focused verification and no full gate for this repair.
The change-start dogfood pass at an empty base-to-HEAD range reported `git-diff-failed`/map-unavailable;
that initial refusal is retained, not called a successful binding. Historical SHA-bearing receipts
were classified separately and not edited. No benchmark, learned ranking, native host, or published
artifact behavior is changed or independently qualified by this patch.

Rollback is reverting the helper, workflow inputs and runbook clarification. No source history, tag,
remote protection, shared task queue or historical release receipt is changed by this repair.
