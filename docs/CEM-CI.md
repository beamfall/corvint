# CEM CI integration (experimental)

This is the smallest zero-service CI integration for `cem/0.1` and `cem/0.2`. It validates an exact
base-to-PR-head change and its checked-in CEM sidecar locally. It sends no source, map,
prompt, credential, or telemetry to Corvint or any hosted service.

**This is not a release instruction.** Corvint `0.4.0a4` is a local staging build, not a
published package or GitHub release. Before enabling CI, build `corvint` for the runner from the
Corvint revision you reviewed (`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/corvint`),
record its source commit and SHA-256 in your own process, and check it into a protected path such
as `tools/corvint/corvint`. Alternatively point `CORVINT_BIN` at an internally managed, immutable
executable. Do not use an unpinned network install in this experimental integration.

## Files to copy

Copy these two files into the protected/default branch of the adopting repository:

- [`../examples/cem/verify-pr.sh`](../examples/cem/verify-pr.sh), normally as
  `examples/cem/verify-pr.sh` and marked executable;
- [`../examples/cem/github-actions.yml`](../examples/cem/github-actions.yml), normally
  as `.github/workflows/cem.yml`.

The workflow intentionally checks out the **base** separately from the PR head. It runs
the verifier script and checked-in executable only from the base checkout; the head checkout
is treated as Git object and map data. This avoids the common mistake of executing a
contributor-modified verification script. For production, pin `actions/checkout` to a
reviewed full commit SHA and maintain its checksum/review policy.

Commit a CEM sidecar at `.corvint/change.cem.json` in the PR. The script derives patch
bytes with exactly this command profile:

```sh
git -c core.quotePath=false -c diff.algorithm=myers -c diff.context=3 \
  diff --binary --full-index --no-color --no-ext-diff --no-textconv --no-renames \
  --no-indent-heuristic --diff-algorithm=myers --unified=3 \
  --src-prefix=a/ --dst-prefix=b/ --ignore-submodules=none \
  "$BASE_SHA" "$HEAD_SHA" -- . ':(exclude).corvint/change.cem.json'
```

Use that exact profile when producing the map. The sidecar is excluded because its own
creation cannot recursively cite itself; all other textual hunks are mandatory CEM
subjects. Put generated or other metadata files under an explicit, separately reviewed
policy instead of silently excluding them here.

For local creation, derive the exact profile from committed revisions instead of hand-building the
patch:

```sh
corvint cem prepare --base "$BASE_SHA" --target "$HEAD_SHA"
# Complete the worklist, then commit .corvint/change.cem.json.
corvint cem status --map .corvint/change.cem.json \
  --expected-base "$BASE_SHA" --target HEAD \
  --max-unknown 0 --max-mechanical 0
corvint cem report --map .corvint/change.cem.json \
  --expected-base "$BASE_SHA" --target HEAD \
  --max-unknown 0 --max-mechanical 0
```

`prepare` returns a numbered worklist. `cite` and `mark` accept the one-based selector as well as
the full hunk ID. Re-running prepare resumes an identical valid map and refuses an outdated map
unless `--replace` is explicit. Preparation deliberately ignores an inherited stale sidecar at its
code target; committing the generated candidate creates the final revision. Run the returned
literal `--target HEAD` action only after that commit (or supply its exact full OID). Consumers then
require a present target sidecar to be raw-byte identical, while still allowing an absent external
artifact. Read the local report before review; it omits source and diff bodies, but its paths and
digests still make it sensitive.

## Generic POSIX CI

Run from a trusted base checkout that contains the verifier and its executable:

```sh
CEM_BASE_SHA="$BASE_SHA" \
CEM_HEAD_SHA="$HEAD_SHA" \
CEM_PROFILE=cem/0.2 \
CEM_MAP_PATH=.corvint/change.cem.json \
CEM_MAX_UNKNOWN=0 \
CEM_MAX_MECHANICAL=0 \
CORVINT_BIN=/opt/corvint/corvint \
./examples/cem/verify-pr.sh
```

If CI checks out the base and head separately, provide the latter only as object data:

```sh
CEM_REPOSITORY="$BASE_CHECKOUT" \
CEM_HEAD_REPOSITORY="$HEAD_CHECKOUT" \
CEM_BASE_SHA="$BASE_SHA" \
CEM_HEAD_SHA="$HEAD_SHA" \
CEM_PROFILE=cem/0.2 \
CORVINT_BIN=/opt/corvint/corvint \
"$BASE_CHECKOUT/examples/cem/verify-pr.sh"
```

Both IDs must be exact lowercase 40- or 64-hex commit IDs. The script validates that
each resolves byte-for-byte, imports the declared head only from the supplied local
checkout when needed, and reads the map with `git show`. For `CEM_PROFILE=cem/0.2`, Corvint derives
the canonical patch itself from the independent revisions and verifies exact target-side sidecar
bytes. The legacy default `cem/0.1` branch writes a private temporary patch and passes it explicitly.
Temporary map and patch inputs are staged as regular, repository-relative files inside the trusted
base checkout; Corvint rejects input symlinks and absolute inputs outside that root.
Both call `corvint cem verify` with explicit unknown and mechanical caps and never run code from a
map, patch, or PR worktree.

## Policy and failure behavior

Use the exit code as a required PR check. CEM verifies provenance completeness and
integrity, not semantic correctness of an agent edit.

| Condition | Result | Required response |
|---|---:|---|
| Valid map, exact patch, stable evidence | `0` | Permit the ordinary review policy to continue. |
| Map absent from the head revision | `3` | Add the sidecar or explicitly exempt the PR in a separate, audited policy. |
| Invalid map, patch digest mismatch, unsupported patch, or binary patch | `2` | Regenerate with the exact command profile; `cem/0.1` does not support binary patches. |
| Evidence stale, ambiguous, or deleted at PR head | `1` | Re-derive/re-cite evidence or review manually. Exact unique relocation is accepted; fuzzy or whitespace-normalized matching is not. |
| Invalid base/head ID, shallow/missing object, or local fetch failure | `3` | Fetch the exact commits; never substitute a branch name or merge ref. |
| Unknown or mechanical hunk over the configured cap | `1` | Cite it, justify an explicit exception, or change repository policy visibly. This example defaults both caps to zero. |

The reference CLI emits JSON to stdout for a completed verification and JSON to stderr
for parse/validation failures. Treat both map and patch as confidential repository
metadata. Do not upload them as public artifacts or logs.

## Forks, private repositories, and permissions

Keep `on: pull_request`, never `pull_request_target`. Use only `contents: read`,
`persist-credentials: false`, no repository write token, no secrets, no artifact upload,
and no deployment environment. The example performs two `actions/checkout` operations
with explicit event SHAs and only fetches Git objects locally between those checkouts.

For public forks, use the platform's normal approval rules for first-time contributors.
For private repositories or private forks, enable this workflow only where your CI
provider permits read access to both declared commits without broadening token scope.
If the platform cannot provide that safely, run the generic POSIX check in a trusted
internal PR worker; do not switch to `pull_request_target` or inject a maintainer secret
into untrusted PR execution.

Maps are local-only by default. Blob hashes, paths, and reason codes can identify private
code, so retain them according to the repository's normal access policy and do not pool
them externally without explicit owner approval.

## Limits of this first integration

- It verifies a CEM map already produced by a person or agent; it does not generate
  evidence or decide semantic adequacy.
- It is deliberately text-patch-only. Route binaries, pure renames, and excluded
  metadata through a separate explicit policy.
- It does not sign maps. Authenticate the enclosing commit/CI result with your existing
  source-control and CI controls.
