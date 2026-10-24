# Caller-owned extractor profile pinned by digest

Date: 2026-10-24. Lane: claude/corpus-profile-1662 (base bcbd5ded0, stacked on
claude/docs-corpus-gh1662). Issue: GitHub #1662, which #1669 left as NEEDS-OWNER (`DCP-V1-478`
drafted only). Requirements `DCP-V1-478` and `DCP-V1-503..506`
(`docs/specs/documentation-corpus-v1.md`) are proposed and experimental. They are implemented
under the orchestrator's 2026-10-24 decision, and owner acceptance is still open.

## Problem

`docs corpus extract --profile` read the profile only as a Git blob at the revision, and the
local-input rule (`DCP-V1-054`) refused any absolute path. A caller who cannot commit a profile to
the extracted repository, such as a third-party checkout, had no way to extract.

## Decisions

- **Admission needs a digest.** A profile outside the repository is admitted only with
  `--profile-sha256 HEX`, which must equal the SHA-256 of the file's exact bytes. Without the
  digest, the existing refusal stays and now names the flag. A digest with a relative path, a
  non-clean or relative path, or a path inside the repository's resolved root is refused.
  Committed profiles keep their Git-blob route.
- **Reading the file.** The directory is resolved with `EvalSymlinks` before the inside-repository
  check. The file is read through `os.OpenRoot` with the bounded no-follow regular-file reader. A
  missing, unsafe or oversized file is reported as `corpus-input-unavailable` without echoing the
  local path.
- **Evidence pinning.** Sources are still read only from Git blobs at the full revision. For the
  profile, the inventory records `origin: "caller"`, the file `sha256`, the `canonical_sha256` of
  the canonical encoding, and the canonical profile document itself. The local path is never
  recorded. A fixed limitation says the profile is caller-owned configuration. The provider
  record's `subjects` capability reason names `sha256:<digest>`, so built artifact provenance
  carries the digest with no schema change.
- **Authority.** A caller profile gets the same generated trust and external-provider authority as
  a committed one. It is usable whether or not the repository commits its own profile.

## Format note

`corvint-extractor-inventory/1` gains only optional members (`profile.origin`,
`profile.canonical_sha256`, `profile.document`), and the repository-origin `path` and `blob`
members became omitempty. A repository-origin inventory always sets `path` and `blob` and leaves
the new members empty, and member order is unchanged, so its bytes are unchanged. The existing
`DCP-V1-390..398` extractor tests pass unmodified. The artifact, manifest and provider-record schemas are
unchanged. A caller-origin inventory still validates as a provider record (`DCP-V1-504`). The one
in-tree inventory consumer, the `extractor-inventory` flow import (`internal/appflows`), decodes
leniently and reads only the schema and the provider subjects, so it is unaffected by the new
members. No benchmark applies, because the extract hot path is unchanged for committed
profiles and the caller read is one bounded file.

## Rollback

Revert the commit. Committed-profile extraction and its bytes are unaffected. Caller-origin
inventories already produced remain self-describing by digest and canonical document, but stop
being producible.
