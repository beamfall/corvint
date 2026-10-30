# Workspace Declaration V0

Owner: Russell Lewis
Frozen: 2026-10-27
Requirement prefix: `WSD-V0`
Intent status: accepted under the owner's standing lane auto-accept (2026-10-11; V1-2846; GitHub #1904)
Delivery status: experimental
Authoritative inputs: `docs/specs/application-map-v0.md` (aliased roots `AMAP-V0-017` to
`AMAP-V0-020`, stated revisions `AMAP-V0-200`), `docs/specs/change-compiled-e2e-v0.md`
(`corvint-test e2e` map verbs and their `--repo` roots), `internal/rootalias` (alias and root
spelling), `AGENTS.md` invariants 1, 2 and 4.

## Agent digest
- Claim: one `.corvint/workspace.json` declares the aliased repos that `flows appmap build` and `corvint-test e2e` map verbs read, replacing repeated root flags.
- Status: accepted under the owner's standing lane auto-accept (2026-10-11; V1-2846; GitHub #1904); experimental
- Exists: `internal/workspace` (schema, bounds, discovery), `appmap.Roots.Resolve` binding it through the shared root flags, `--workspace`/`--no-workspace` on `flows appmap build`, every `corvint-test e2e` map verb except `shadow`, and the source verbs `refs`, `pageobjects`, `steps`, `setup`, `links`, `infer` and `how` (WSD-V0-145); witness, CEM and review refuse a workspace naming the aliases they would ignore (WSD-V0-146).
- Blocked on: cross-repository CEM and review citation, which needs a frozen CEM wire change (V1-2910).
- Read next: Requirements; Non-goals and authority; Failure modes.

## User and measurable job

A team whose application, end-to-end suite and manifest live in separate repositories passes the
same `--repo ALIAS=DIR`, `--manifest-repo` and `--repo-revision` flags to every map verb, and one
typo silently binds a different checkout. The affected user is the operator or agent running those
verbs across a multi-repository checkout. The measurable job: one declaration file, read the same
way by every verb that binds aliased roots, produces exactly the roots the equivalent flags produce
(byte-identical map and obligations), and a verb that cannot honor a declared alias refuses naming
it instead of quietly reading a single root.

The declaration names checkouts on this machine. It is configuration, not evidence: every verb
that reads a root still pins the revision it read, exactly as with flags.

## Requirements

- **WSD-V0-001:** Schema and profile. A workspace declaration is one JSON object with the members
  `profile` (exactly `corvint-workspace/0`), `repos` (an object from alias to `{"root": PATH,
  "revision": REV}` with `revision` optional) and optional `manifest_repo` (a declared alias holding
  the manifest). No other member is admitted at either level, so a later profile adds members under
  a new profile name. Witness, CEM and review can adopt the same decoder (`workspace.Load`) without
  a schema change.
- **WSD-V0-002:** Bounds and validation. A declaration is a regular file (no symlink) of at most 64
  KiB holding exactly one object. It declares 1 to 32 aliases (`rootalias.MaxRoots`); each alias
  matches `[a-z][a-z0-9-]{0,31}`. A relative `root` resolves against the directory holding the file;
  the resolved root must be an absolute, clean path. A `revision` is at most 256 bytes, does not
  start with `-` and holds no space or control character. `manifest_repo`, when present, names a
  declared alias. Any violation refuses as `workspace-invalid`, whose message names the file and the
  failing member; nothing is read from any root.
- **WSD-V0-003:** Discovery. When a verb is given no root flag (`--repo`, `--manifest-repo`,
  `--repo-revision`), no `--workspace` and no `--no-workspace`, it reads
  `ROOT/.corvint/workspace.json` under its `--root`. An absent file is no workspace and the verb
  runs on its single root as before. A present file that is not a valid declaration refuses as
  `workspace-invalid`; it is never skipped.
- **WSD-V0-004:** Binding through the shared root flags. `--workspace FILE` names a declaration
  explicitly; `--no-workspace` disables discovery. Both are bound by `appmap.BindRootFlags`, so
  `flows appmap build` and every `corvint-test e2e` map verb that takes `--repo` (all but `shadow`)
  accept them. A workspace binds the same `Roots` the flags would: each alias's root as `--repo`,
  `manifest_repo` as `--manifest-repo` and each non-empty `revision` as `--repo-revision`. A
  workspace never merges with flags: `--workspace` with any root flag or with `--no-workspace`
  refuses as `invalid-arguments` ("does not combine"). A declared default revision for an alias the
  build reads nothing from is unused rather than refused, since a shared declaration serves verbs
  that read different aliases; an explicit `--repo-revision` for such an alias still refuses as
  `appmap-root-unavailable` (`AMAP-V0-200`).
- **WSD-V0-005:** A verb that ignores an alias refuses naming it. A verb that cannot read a declared
  or flagged alias refuses with a typed code that names the aliases it would ignore. In V0 this is
  `corvint-test e2e prove --hosted-test`, which reads its test from `--root` alone: with aliased
  roots it keeps the code `e2e-roots-unsupported` and its detail names each ignored alias and says
  `--no-workspace` proves a hosted test without a discovered workspace.
- **WSD-V0-145:** (experimental under the standing lane auto-accept; V1-2927, GitHub #1933) The
  source verbs `flows appmap refs`, `pageobjects`, `steps`, `setup`, `links`, `infer` and `how`
  take `--workspace FILE` and `--no-workspace`. A named workspace, or else, with no `--repo` and no
  `--no-workspace`, the one discovered under `--root` (`WSD-V0-003`), binds each alias's root as
  the matching `--repo ALIAS=ROOT` would, so `--source ALIAS:DIR` answers byte-identically to the
  flag form. Its `revision` and `manifest_repo` members are unused, since a source verb reads each
  root at the revision the map pinned. `--workspace` with `--repo` or `--no-workspace` refuses as
  `invalid-arguments` ("does not combine"); an invalid declaration refuses as `workspace-invalid`.
- **WSD-V0-146:** (experimental under the standing lane auto-accept; V1-2927, GitHub #1933)
  `corvint witness`, `corvint review` and every `corvint cem` action except the closed-argv
  `verify-stable`, `gate-verdict` and `bundle` read one repository, so a workspace refuses them
  (`WSD-V0-005`): `--workspace FILE`, or a declaration discovered under `--root` without
  `--no-workspace`, refuses as `workspace-unsupported`, exit 2, naming the verb, each declared
  alias it would ignore, how the declaration was found, and the remedy (a paired witness
  `--range app=... --range tests=... --range-root NAME=ROOT`; for CEM and review, one repository
  per `--root`, with cross-repository citation not delivered, V1-2910). `--no-workspace` runs the
  verb on `--root` alone exactly as without a declaration; `--workspace` with `--no-workspace`
  refuses as `invalid-arguments`. A paired witness (`--range`) names its repositories itself, so a
  discovered declaration does not refuse it; a named one still does.

## Non-goals and authority

The source verbs read a workspace only for alias roots (`WSD-V0-145`) and keep their `ALIAS:DIR`
arguments. Witness, CEM and review do not read it in V0 and refuse it instead (`WSD-V0-146`); no CEM
or review wire changes. The
declaration is read from the working filesystem, never from a Git object, and it is not pinned
evidence; the map and obligations pin the revisions they read exactly as with flags. No merging of a
workspace with flags, no search above `--root`, no environment variable, no per-verb section. A
workspace adds no authority: it cannot admit a path, a root or a suite the flags could not.

## Failure modes

Malformed, oversized, symlinked, unknown-member or out-of-bound declaration, named or discovered:
`workspace-invalid`, exit 2 (`flows appmap build`) or the corvint-test error, nothing read. Workspace
combined with a root flag or `--no-workspace`, an empty or repeated `--workspace`:
`invalid-arguments`, exit 2. A declared root that is missing or not a Git checkout: the existing
`appmap-root-unavailable` refusal of the verb that reads it. Hosted prove over aliased roots:
`e2e-roots-unsupported` naming the aliases (`WSD-V0-005`). Witness, review or CEM with a named or
discovered workspace and no `--no-workspace`: `workspace-unsupported`, exit 2 (`WSD-V0-146`).

## Acceptance evidence and traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| WSD-V0-001 | `internal/workspace/workspace.go` `Workspace`, `decode` | `TestWSDV0001Load` |
| WSD-V0-002 | `readBounded`, `decode`, `validRevision` | `TestWSDV0001Load` |
| WSD-V0-003 | `workspace.Discover`; `appmap.Roots.Resolve` | `TestWSDV0003Discover`, `TestWSDV0004BuildWorkspace`, `TestWSDV0004BuildCLI` |
| WSD-V0-004 | `internal/appmap/rootflags.go` `BindRootFlags`, `Resolve`; `compile.go` revision check; `cmd/corvint/flows_appmap.go`; `cmd/corvint-test/main.go` | `TestWSDV0004BuildWorkspace`, `TestWSDV0004BuildCLI`, `TestWSDV0004ObligationsWorkspace` |
| WSD-V0-005 | `cmd/corvint-test/main.go` hosted prove refusal | `TestWSDV0004ObligationsWorkspace` |
| WSD-V0-145 (experimental) | `cmd/corvint/flows_appmap_knowledge.go` `bindSourceRepos`, `sourceRepos.resolve`; refs, infer and how call sites | `TestWSDV0145SourceVerbsWorkspace` |
| WSD-V0-146 (experimental) | `cmd/corvint/workspace_guard.go` `guardWorkspace`, `checkWorkspace`; `parseWitnessInvocation`, `parseGuidanceInvocation`, the cem guard in `runContext` | `TestWSDV0146SingleRootVerbsRefuseWorkspace` |

Compatibility: no wire change. Without a workspace file every verb behaves as before; the hosted
prove refusal keeps its code and only its detail text changes. With a discovered declaration,
witness (single range), review and CEM now refuse until `--no-workspace` is given (`WSD-V0-146`).

Rollback: delete `internal/workspace`, the `--workspace`/`--no-workspace` flags and `Resolve` in
`internal/appmap/rootflags.go`, the `Workspace` condition in `compile.go`, the corvint-test and
build call sites, their tests and help lines, this spec and its README/INDEX rows. Rollback of
WSD-V0-145/146 alone removes the source-verb `--workspace` binding, `cmd/corvint/workspace_guard.go`
and its three call sites, their tests and help paragraphs. A declaration
file left in a checkout is then ignored.
