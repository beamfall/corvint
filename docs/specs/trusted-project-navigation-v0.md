# Trusted Project Navigation V0

Intent: accepted by the repository owner in the 2026-09-28 issue #328 conversation.
Delivery: TPN-V0-001..006 implemented; real local MkDocs qualification and independent review passed.

## Agent digest
- Claim: An explicitly trusted pinned MkDocs environment validates exact proposed navigation edits in an isolated copy of committed source.
- Status: accepted / delivered in the bounded trusted-project profile.
- Exists: separate `corvint docs nav --trusted-project` profile; no change to admitted plan-only or capsule execution.
- Blocked on: no blocked requirement in this profile; capsule, site-build and offline qualification remain separate.
- Read next: Requirements; Wire and execution; Acceptance and rollback.

## Intent and boundary

The owner approved a separate trusted-project execution profile after the existing HDC capsule-only
contract prevented #328 from running on macOS. This is an explicit exception to HDCV0-CAP-003 for
this profile only. It trusts the selected interpreter and installed packages, including startup
behavior. It does not contain malicious tooling, deliberate process-group escape or hostile host
pathname races. It neither changes the capsule profile nor upgrades a plan-only snapshot.

The measurable job is one clean committed project, one explicit root `nav`, one proposed page:
produce byte-exact reviewable patches and validate the isolated candidate's effective navigation
using pinned MkDocs. Build execution and offline qualification remain separate, unperformed jobs.
The simplest baseline is hand-editing YAML and loading MkDocs; this profile additionally binds the
committed source, interpreter, distribution inventory, config ownership span and both loader results.

## Requirements

- `TPN-V0-001`: Execution requires explicit `--trusted-project`. Accept only the closed canonical
  request and an exact clean HEAD. The default read-only docs and admitted plan-only paths retain
  their behavior. No apply, build, serve, install, deploy, branch, merge or publishing mode exists.
- `TPN-V0-002`: Stage only the complete admitted immutable Git manifest, with regular files and
  portable relative paths. Replacement refs are refused and replacement-object lookup is disabled. Reject symlinks, submodules, traversal, case ambiguity and input limits.
  Bind every staged blob and SHA-256, the config bytes and a committed project-lock SHA-256. New
  nonempty Markdown documents carry explicit bytes/digests, belong under effective docs_dir and cannot
  overlap existing or proposed paths. Worktree, Git and accepted prose remain unchanged.
- `TPN-V0-003`: Before loading third-party configuration, check the interpreter and installed
  distribution inventory pins. The fixed probe uses the pinned MkDocs YAML loader, rejects tags,
  anchors, aliases, duplicate/merge keys and unsupported configuration. Require exactly one root
  explicit navigation owner. Derive UTF-8 byte positions from parser node/token positions, exclude
  trailing comments, and compare the parsed value with effective MkDocs navigation. Caller claims
  and lexical matching alone never authorize a span.
- `TPN-V0-004`: Bind source, environment, fixed probe, snapshot, span, replacement, candidate config
  and exact unified diff digests. Apply the replacement only to the temporary candidate, preserving
  every byte outside the proven value span. Run the full pinned MkDocs loader again and compare
  effective navigation to the proposed value. Recheck environment pins around both runs and HEAD/
  clean status afterward. Refuse drift or unqualified observations without emitting success.
- `TPN-V0-005`: Return a deterministic bounded canonical artifact on stdout: `NAVIGATION_VALIDATED`,
  `build_strict=NOT_RUN`, `offline_qualified=NOT_OBSERVED`. No navigation result implies a successful
  site build, offline protection, true document claims, or capsule qualification. Temporary local
  paths are not artifact identity; inspection deliberately returns the selected interpreter path.
- `TPN-V0-006`: Use one 120-second deadline, bounded input/output and process groups on Darwin/Linux.
  Kill and reap remaining group members on exit or interruption. Prove grandchild cleanup with an
  interruption regression. Deliberately escaping descendants are excluded from the trusted profile.
  Remove owned temporary source, candidate, probe inputs and site/cache directories on every exit.
  Cleanup failure suppresses success and retains the temporary path in a diagnostic. Request and
  interpreter inputs must be bounded regular files; FIFOs are refused before opening.

## Wire and execution

Commands:

```sh
corvint docs nav --trusted-project --inspect-python /absolute/venv/bin/python
corvint --root /project docs nav --trusted-project --request /outside/project/request.json
```

Inspection produces `corvint-trusted-project-navigation-environment/0` with interpreter SHA-256,
installed distribution inventory SHA-256, probe SHA-256 and explicit caller-owned trust. It installs
nothing. Copy the three interpreter/inventory fields into the request environment and add the
committed lock path/hash. Inspection is an explicit execution operation, never automatic discovery.

The request is HDCV0-041 canonical JSON (sorted keys, UTF-8, compact separators, final LF), with
exact members:

```json
{"config":"mkdocs.yml","config_sha256":"<64 lowercase hex>","documents":[{"content":"# New\n","path":"docs/new.md","sha256":"<64 lowercase hex>"}],"environment":{"inventory_sha256":"<64 lowercase hex>","lock":"requirements.lock","lock_sha256":"<64 lowercase hex>","python":"/absolute/venv/bin/python","python_sha256":"<64 lowercase hex>"},"nav":[{"path":"index.md","title":"Home"},{"path":"new.md","title":"New"}],"profile":"corvint-trusted-project-navigation-request/0","revision":"<exact HEAD>"}
```

The quoted placeholders must be replaced with actual pins; SHA-256 hashes exact bytes including any
final LF. Empty documents is `[]`. The initial output navigation is a flat list of titles and local
`.md` targets; initial accepted navigation may be nested. Config is root `mkdocs.yml` or `mkdocs.yaml`.
Allowed config members are `site_name`, `nav`, `docs_dir`, `site_dir`, `site_url`,
`use_directory_urls`, `theme`, `plugins`, `markdown_extensions`. Theme is omitted or `mkdocs`;
plugins/extensions are omitted or `[]`, and the loader explicitly sets both empty. The builtin
MkDocs theme remains configuration-owned; Material, custom themes, hooks, inheritance and custom
plugins/extensions are unsupported in this first trusted profile. All navigation paths must name
regular staged Markdown beneath the admitted relative docs_dir. The probe verifies MkDocs resolves
that same directory and overrides site_dir to its temporary root. No site is built.

Manifest paths use ASCII letters/digits and `_./-`, without traversal, `.git` components, aliases or
case collisions. Maximums: 2 MiB request, 2,048 source files, 1 MiB per source/new document,
32 MiB source/candidate content, 64 proposed pages and 64 navigation entries, 8 MiB result. Package
inventory is at most 20,000 files / 128 MiB; it sorts normalized distribution names/versions and
recorded relative file paths/content SHA-256s, binds Python version, rejects duplicate names,
editable installs and package symlinks/out-of-environment files, and requires MkDocs 1.6.1.
Interpreter symlinks in an explicitly selected venv are permitted with their target bytes pinned.

The result profile is `corvint-trusted-project-navigation/0`. It contains `revision`, ordered
`sources`, `source_sha256`, `environment_sha256`, `probe_sha256`, `snapshot` and its digest,
`reload` and its digest, `candidate_sha256`, exact `replacement` and digest, `proposal_patch` and
digest, proposed `documents`, the three truth-state fields, and explicit `limitations`. Snapshot
members include the root config owner path and exact config/inventory/span/nav digests, docs_dir, nav value, start/end bytes and the
symbolic `isolated-output` site-dir override. Composite Go digests use HDCV0-041 bytes including LF;
probe nav/inventory digests use compact sorted UTF-8 JSON without LF. Raw hashes use exact bytes.

## Failure modes

Refuse missing trust, noncanonical/unknown fields, stale revision/config/lock/interpreter/inventory,
dirty source, unsupported Git objects/paths, missing/outside navigation pages, duplicate/aliased/
malformed YAML, unsupported executable config, overlapping new files, reload disagreement,
resource limits and cancellation. Errors expose bounded codes, not config values or raw Python
tracebacks. External changes during a run invalidate it; the product does not undo another actor's
writes. Explicitly trusted tooling is responsible for respecting the host trust grant.

## Acceptance and rollback

`TestTrustedNavRealRoundTrip` uses real pinned MkDocs and verifies UTF-8/CRLF/comment preservation,
Git-applicable exact patches, effective reload and source nonmutation. `TestTrustedNavRefusals`,
`TestTrustedNavRejectsLexicalAuthority`, `TestTrustedNavCLITrustBoundary` and
`TestTrustedNavCancellation` cover authority, stale input, explicit trust and interruption. The
live qualification sets `CORVINT_TRUSTED_NAV_PYTHON`; missing fixture tooling is not qualification.
Independent review checks the owning span and trust boundary. Full HDC capsule/build/offline gates
remain NOT_RUN. Revert the separate implementation and this exception to roll back; emitted
artifacts are proposals and require no migration or repository repair.

## Diagnostic codes

| Code | Meaning |
|---|---|
| `trusted-project-required` | Explicit project/toolchain trust was not granted. |
| `trusted-nav-platform` | The host lacks this profile's supported process-group lifecycle. |
| `trusted-nav-request` | The closed request, pins or request-level limits are invalid. |
| `trusted-nav-input` | An input is unavailable, nonregular, changed during acquisition or too large. |
| `trusted-nav-limit` | Request, source, candidate or result bytes exceed the profile's bounds. |
| `trusted-nav-stale` | The requested revision or config hash does not match captured source. |
| `trusted-nav-dirty` | The repository is not clean. |
| `trusted-nav-source` | Replacement refs, unsupported/ambiguous paths or invalid Git blobs prevent immutable staging. |
| `trusted-nav-source-changed` | Worktree paths/bytes or HEAD changed during verification. |
| `trusted-nav-environment` | The interpreter or project lock is unavailable or fails its pin. |
| `trusted-nav-probe` | The fixed probe returned an invalid authority snapshot. |
| `trusted-nav-probe-refused` | The pinned probe refused its input or exceeded execution/output bounds. |
| `trusted-nav-authority` | Config ownership, span or inventory identity is inconsistent. |
| `trusted-nav-document` | A proposed document is invalid, empty or overlaps another source path. |
| `trusted-nav-entry` | A navigation entry is invalid, duplicated or names a missing page. |
| `trusted-nav-reload` | Reloaded candidate navigation differs from the proposed value. |
| `trusted-nav-cleanup` | Temporary cleanup failed; no success artifact is emitted and the retained path is diagnosed. |
