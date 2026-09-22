# CEM 0.1 conformance

This directory is the portable, implementation-neutral CEM 0.1 conformance seed. A conforming
verifier MUST accept `valid.cem.json` with the exact bytes of `change.patch`, and MUST reject every
map mutation and invalid patch in `cases.json`. Acceptance or rejection is normative; diagnostic wording and error-code
names are not.

`referenceCode` records the Corvint reference verifier's current first diagnostic and is advisory.
When a case declares `acceptableCodes`, those values are an explicitly registered part of that
case's contract. Implementations with different diagnostic taxonomies may omit codes entirely.
The mutation operations are `set`, `remove`, and `set-many`; JSON Pointer escaping is not needed by
the current vectors. `set-many` represents one coherent semantic mutation where several wire
fields must change together. `invalidPatches` names exact patch fixtures and the matching digest a
consumer places in the otherwise-valid map before verifying rejection.

The wire hunk `reason` is enumerated by disposition. `supported` MUST use `evidence-backed`;
`unknown` uses an explicit unknown reason; `mechanical` uses a mechanically provable reason. Free
text rationale belongs outside the CEM 0.1 verification envelope.

Recreate the exact SHA-1 Git base from `base/` with:

```sh
git init -q --object-format=sha1
git config user.name "CEM Conformance"
git config user.email "cem@example.invalid"
git add .
GIT_AUTHOR_DATE="2000-01-01T00:00:00+0000" \
GIT_COMMITTER_DATE="2000-01-01T00:00:00+0000" \
git commit -qm "CEM 0.1 base"
```

The commit MUST be `4ca153370afd9bd8c6034ad73acc3925150ab681`. Git SHA-256 repositories
are covered by a runtime-generated vector because their object IDs depend on Git's supported object
format. The test skips that vector only when the installed Git rejects `--object-format=sha256`.

Run the public vectors independently of unittest:

```sh
go test ./internal/cem/verify
```

The retired Python driver is replaced by native tests consuming the same frozen inputs.
CLI and canonical repository behavior remain covered by the native workflow and command suites.

Run the expanded reference conformance tests:

```sh
go test ./internal/cem/... ./cmd/corvint
```

The expanded tests additionally cover SHA-1/SHA-256 object IDs; create, delete, and modified rename
patches; positive mechanical classifications; CR, CRLF, LF, no-final-newline, Unicode, orphan and
overlapping data; surplus hunk payload, malformed UTF-8 JSON, metadata disagreement, and cumulative
resource bounds.
