# CEM CI examples

Two ways to check a checked-in Change Evidence Map (CEM) on a pull request:

| Example | Verifier | Profile |
| --- | --- | --- |
| `github-actions.yml` + `verify-pr.sh` | a reviewed `corvint` executable the adopter checks in | `cem/0.2` |
| `github-actions-portable.yml` + `verify-portable.sh` | the dependency-free `interop/cem01-go` module, digest-pinned | `cem/0.1` |

[`recipes/`](recipes/README.md) composes these with `corvint impact`, `affected`, `context` and
`cem` into three tested understand, review and CI-verification recipes.

This runbook covers the portable verifier (`CEM-PILOT-020`..`023` in
`docs/specs/cem-pilot-kit.md`). It uses no LLM, no index, no retriever, and no network after
the fetch step. It reads the base and PR-head commits as Git data, derives the exact patch with
the profile in `docs/CEM-CI.md`, verifies the map at `.corvint/change.cem.json` in the head tree,
and prints one JSON report line.

## 1. Choose and pin a verifier revision

Pick a reviewed Corvint commit and resolve its pseudo-version (a network step on a trusted
workstation):

```sh
GOTOOLCHAIN=local GOFLAGS= go list -m -json github.com/Beamfall/corvint/interop/cem01-go@<commit>
```

Its `Version` is the `CEM_VERIFIER_VERSION` pin. The Go checksum database checks the module
zip for that version at every later fetch.

Compute the executable digest with Go 1.27.1, building exactly as the workflow does:

```sh
env -u GOBIN GOTOOLCHAIN=local GOFLAGS= GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go install -trimpath github.com/Beamfall/corvint/interop/cem01-go@<version>
shasum -a 256 "$(go env GOPATH)/bin/linux_amd64/cem01-go"
```

On a linux/amd64 host the executable is `$(go env GOPATH)/bin/cem01-go` instead. That digest
is `CEM_VERIFIER_SHA256`. Put both pins in the workflow's top-level `env`.

## 2. Install the workflow

Copy `github-actions-portable.yml` to `.github/workflows/` and `verify-portable.sh` to
`examples/cem/` in the protected base branch. The workflow:

1. checks out the base as trusted code and the PR head as data only;
2. runs `go install -trimpath "$CEM_VERIFIER_MODULE@$CEM_VERIFIER_VERSION"` with
   `CGO_ENABLED=0`, the only step with network access. To resolve the nested module path,
   `go install` also downloads the root `github.com/Beamfall/corvint` module zip. Both zips are
   source only and checked against the Go checksum database;
3. runs the script inside `sudo -E unshare --net -- setpriv ...`, a network namespace with no
   usable interface, as the runner user.

The script copies the executable into a private directory, checks its SHA-256 against
`CEM_VERIFIER_SHA256`, and runs it only on a match:

```sh
CEM_REPOSITORY="$PWD/base" CEM_HEAD_REPOSITORY="$PWD/head" \
CEM_BASE_SHA=<base sha> CEM_HEAD_SHA=<head sha> \
CEM_VERIFIER="$RUNNER_TEMP/cem-bin/cem01-go" CEM_VERIFIER_SHA256=<pinned sha256> \
  examples/cem/verify-portable.sh
```

The script runs the verifier as:

```sh
cem01-go ci --repository <base checkout> --base <base sha> --head <head sha> \
  --map .corvint/change.cem.json
```

## 3. Read the result

| Exit | `verdict` | Meaning |
| --- | --- | --- |
| 0 | `accepted` | The map is valid for the exact patch, no hunk is `unknown`, and all evidence is stable or relocated. |
| 1 | `rejected` | The map or the patch failed a structural check, or cited evidence drifted unsafely. |
| 2 | `operational` | Invalid invocation or an I/O failure. Also the script's exit before the verifier runs (bad input, digest mismatch); then there is one `cem-ci:` line on stderr and no report. |
| 3 | `missing-evidence` | No map at the map path (`map-absent`), or a valid map that leaves hunks `unknown` (`unknown-hunks`). |
| 4 | `unsupported-profile` | The map declares a `spec` other than `cem/0.1`. |
| 5 | `repository-mismatch` | A declared commit is not in the repository, or the map's `baseRevision` is not the declared base. |

A structurally valid map ranks unsafe drift (1) before unknown hunks (3). A `cem/0.1` map that
fails a structural check exits 1 even when its `baseRevision` also differs from the declared base.

The ticket's terms map to exits and report `code` values as follows:

| Term | Exit | `code` |
| --- | --- | --- |
| verified | 0 | `accepted` |
| unverified | 3 | `map-absent` or `unknown-hunks` |
| stale | 1 | `unsafe-drift`: the map is structurally valid, but cited evidence is `stale`, `ambiguous`, or `deleted` in the head tree (see `drift`) |
| structural failure | 1 | any other code, such as `patch-digest` or `map-entry`: the map or the patch itself is invalid |
| unsupported | 4 | `unsupported-profile` |
| error | 2 | `invocation` or an I/O code; also the script's pre-run exit with no report |
| error (repository mismatch) | 5 | `base-unavailable`, `head-unavailable`, or `base-revision-mismatch` |

The report is one line of JSON with the schema `cem-ci-report/0` and exactly these members:
`schema`, `verdict`, `exit`, `code`, `profile`, `base`, `head`, `mapPath`, `mapSha256`,
`patchSha256`, `hunks` (`supported`, `mechanical`, `unknown`), `evidence`, `drift` (evidence ID,
path, status, target blob ID and target span), and `limits`. It holds digests, paths, spans and verdicts and never source
or diff text. It is deterministic for the same inputs and never exceeds 6 MiB.

## Limits

- The check is structural. An `accepted` verdict does not prove that cited evidence supports
  the change (`limits` says so in every report).
- Paths and digests can still be sensitive. Treat the report as repository-private.
- Only `cem/0.1` maps are supported. `cem/0.2` needs `verify-pr.sh` and a `corvint` executable.
- The pinned digest is specific to the Go version, target platform and build flags above.
