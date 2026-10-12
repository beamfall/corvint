# Neutral criterion public case (CEM-V1-083)

Status: PROPOSED / EXPERIMENTAL, like the rest of `protocol/cem-1.0/stable/`.

This case shows a `cem/1.0` map whose only criterion binding is the plain,
provider-neutral form `{id, criterionIndex, criterionSha256, evidenceId}`. The
repository is not a Corvint repository: it has no `.taskman/`, no `.corvint/`
and no Tasks store, and the map carries none of the eight Corvint Tasks fields.

- `repository.recipe.json`: two flat commits (`base`, then `target` with
  `base` as parent). Build each tree with `git hash-object -w`, `git mktree`
  (mode `100644`, names sorted bytewise) and `git commit-tree`, using
  `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1` and the recorded
  identity and date for author and committer. The commit IDs are then
  `EXPECTED.json` `baseRevision` and `targetRevision`.
- `stable.json`: the map. Its one criterion binds the evidence span of
  `SPEC.md`; `criterionSha256` equals that evidence's `spanSha256`.
- `artifacts/runner-plan.json`, `artifacts/runner-receipt.json`: opaque
  fixtures. The stable verifier checks artifacts only by SHA-256 against the
  map, so their content is not a real runner plan or receipt.
- `EXPECTED.json`: the expected exit and full result, except `runtime`, for
  `--artifacts` set to this directory, plus seven refusal mutations. Each
  refusal replaces the map's only criterion binding with `criterion`; when
  `rebindLink` is true, `criterionLinks[0].criterionId` is also set to the new
  ID.

Run with either consumer (absolute paths):

```sh
corvint cem verify-stable --repository R --map CASE/stable.json \
  --expected-base BASE --target TARGET --artifacts CASE
cem01-go verify-stable-canonical --repository R --map CASE/stable.json \
  --expected-base BASE --target TARGET --artifacts CASE
```

Both consumers must return the same result. The tests named `TestCEMV1083*` in
`internal/cem/verify` and `interop/cem01-go` check this.
The map was assembled by hand during the walkthrough recorded in
`docs/build-log/2026-10-12-cem-neutral-criterion-walkthrough.md`, because no
producer emits the plain form yet.
