# External criterion public case (CEM-V1-330..336)

Status: like the rest of `protocol/cem-1.0/stable/`, implemented and run by both
consumers; the technical contract is proposed and not promoted.

This case shows a `cem/1.0` map whose only criterion binding is the external
form `{id, criterionIndex, criterionSha256, externalRepository,
externalBlobOid, externalSpan}`. The requirement it binds lives in a separate
requirements repository, not in the service repository the map describes.

- `repository.recipe.json`: the service repository, identical to
  `../neutral-criterion/repository.recipe.json`; build it the same way.
- `external.recipe.json`: one flat commit holding `REQUIREMENTS.md`. Build it
  with the same plumbing and identity. Its blob ID is `EXPECTED.json`
  `externalBlobOid`.
- `stable.json`: the map. Its one criterion binds bytes `externalSpan.start`
  to `externalSpan.end` (zero-based, end exclusive) of that blob in the
  repository named `requirements`; `criterionSha256` is the SHA-256 of those
  bytes. The criterion ID is computed over the non-`id` members, with
  `externalSpan` written as `{"end":N,"start":M}`.
- `artifacts/`: the same opaque fixtures as the neutral case.
- `EXPECTED.json`: the expected exit and full result, except `runtime`, when
  the requirements repository is supplied, plus nine cases. Each case replaces
  the only criterion binding with `criterion` (when not null), rebinds
  `criterionLinks[0].criterionId` when `rebindLink` is true, and supplies the
  requirements repository only when `supplied` is true.

Run with either consumer (absolute paths):

```sh
corvint cem verify-stable --repository R --map CASE/stable.json \
  --expected-base BASE --target TARGET --artifacts CASE \
  --external-repository requirements=EXT
cem01-go verify-stable-canonical --repository R --map CASE/stable.json \
  --expected-base BASE --target TARGET --artifacts CASE \
  --external-repository requirements=EXT
```

Without `--external-repository` the result is UNSUPPORTED, stage `artifacts`,
code `artifact-unavailable`, exit 2: the external criterion was not observed,
so it is never ACCEPT. A reader that predates the external form selects the
plain form and refuses the map at stage `wire` with `missing-field`
(`evidenceId`). The tests named `TestCEMV1334*` in `internal/cem/verify` and
`interop/cem01-go` check both consumers against this file.
