# V1-0609: metadata-only changes get a typed refusal

## Problem

`cem prepare` failed on a real change that added eleven zero-byte report files. The error was
`malformed-patch: expected --- file header`, which reads like a corrupt patch. The actual cause is a
capability limit.

For three kinds of change, Git's canonical diff emits a headerless group: a `diff --git` line plus
two metadata lines, with no file header and no hunk. Those kinds are an empty-file creation, an
empty-file deletion, and a mode-only change. The `cem/1.0` grammar has no form for such a group.
The portable consumer refused the same input with codes that differed by position: `metadata`,
`file-header` or `unsupported-tree-mode`.

## Decisions

- **Representation is deferred.** Whether `cem/1.0` should represent metadata-only changes is a
  wire change and an open owner decision. This change adds no wire form, identity or binding, and
  it does not invent a text hunk.
- **One typed refusal on both consumers.** The native parser and `interop/cem01-go` now return the
  same code, `metadata-only-change-unsupported`, when a group has the exact Git shape. The refusal
  names the kind (`empty-file-creation`, `empty-file-deletion` or `mode-only`) and the path. It is
  operational: a documented limit, not invalid input (`CEM-V1-300`).
- **Exact recognition.** Every other shape keeps its earlier error (`CEM-V1-301`). That includes a
  mode change that comes with a text change, which Git emits with headers.
- **Stable bytes are not changed.** `protocol/cem-1.0/stable/manifest.json` pins `ALGORITHMS.md`
  and `RESULT.schema.json`, and the schema's closed `code`/`issueCodes` enums have no
  metadata-only code. Both stable verifiers therefore map the new refusal back to the result they
  produced before (`CEM-V1-302`). The new code is documented only in `cem-stable-v1.md`;
  `ALGORITHMS.md` is untouched. `make cem-stable-regen-check` passes with no stable byte change.

## Reproductions

The vectors in `interop/cem01-go/testdata/metadata-only/` come from Git 2.54 using the canonical
`gitauth` diff argv, with `cases.json` as the manifest:

- creation, deletion and mode-only;
- a mixed text change with an empty-file creation, in both orders;
- a mode-and-text change;
- four hand-built near misses.

`TestPrepareRefusesMetadataOnlyChanges` reproduces the original `cem prepare` symptom on real
commits.

## Findings retained

- Native stable already reports `issueCodes: ["malformed-patch"]` for these inputs. That code is
  outside the frozen `RESULT.schema.json` issue enum, and this is not new.
- Native and portable stable issue codes disagree on these inputs (`malformed-patch` against
  `metadata`/`file-header`/`unsupported-tree-mode`).

Both are kept as they were. Fixing either one means a stable byte change, which belongs with the
owner's representation decision.

## Outcome

Both consumers now give a typed, path-naming refusal for metadata-only changes, and the frozen
stable profile is unchanged. V1-0609 stays open until the owner decides whether metadata-only
changes become representable.
