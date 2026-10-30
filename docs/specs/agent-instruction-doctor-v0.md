# Agent Instruction Doctor V0

Owner: Russell Lewis
Date: 2026-09-30
Requirement prefix: `IID-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: owner instruction for six V1 wow additions, 2026-09-30;
V1-0502 bounded slice; related V1-0413 and V1-0414; `AGENTS.md` invariants 1–4, 7–8;
`docs/specs/task-context-packet-v0.md` TCP-V0-008 and TCP-V0-023.

## Agent digest
- Claim: Explicit default assumptions predict pinned project instruction selection, shadowing and truncation; actual session loading remains unknown.
- Status: proposed/experimental
- Exists: `internal/contextindex/instruction_loadset.go`, `cmd/corvint/taskcontext.go`, `integrations/instruction-profiles.json` and focused tests.
- Blocked on: Actual session/global loading and other host versions remain UNKNOWN; exact Codex 0.153.2 project marker probes are bounded fixture evidence.
- Read next: Requirements; Failure modes; Acceptance evidence and rollback.

## Intent and scope

An agent needs to distinguish project evidence it should inspect from files its host would
select automatically. Codex's default filename selection can hide an AGENTS.md behind an
AGENTS.override.md; its byte budget can hide a suffix or later file. These are host syntax
facts, not decisions that accept or supersede repository-owned intent.

Invocation:

```sh
corvint context --task "inspect instructions" --subject app/source.go \
  --instruction-host codex --instruction-host-version 0.153.2 \
  --instruction-cwd app --instruction-profile default
```

The four instruction flags and tracked subject are required together; repeat flags and summary/
expansion combinations refuse. `default` explicitly assumes a trusted project, one environment,
the default `.git` root marker, Git root equal to host project root, no fallback filenames, a
32768-byte project budget, readable regular files and pinned tree equal to host instruction
inventory. Untracked host instruction files remain UNKNOWN. Core does not inspect or assert actual
host trust, settings, permissions, environment count/order, global/user instructions, other
injected instructions, or session state. A repository-only prediction cannot attest a complete
host load set. Without a host project-root marker, Codex checks only cwd; this Git-root profile
does not model that alternative and retains actual root-marker resolution as UNKNOWN.

Rule provenance is pinned to
[Codex 0.153.2 agents_md.rs](https://github.com/openai/codex/blob/rust-v0.153.2/codex-rs/core/src/agents_md.rs),
SHA256 `8bbaf068c099fdeeaf4fe49076d398da7671f20c9a962fde5c4eb7653008fed4`.
Its exact line selectors are in `integrations/instruction-profiles.json`. The public
[Codex instruction guide](https://learn.chatgpt.com/docs/agent-configuration/agents-md)
is supporting documentation, not a substitute for exact-version source semantics.
[Current Claude documentation](https://code.claude.com/docs/en/memory) requires 2.1.277+
for direct AGENTS.md support, with further session qualifications; the compatibility matrix's
2.1.267 must not inherit that newer rule. This slice emits UNKNOWN for Claude rules.

## Requirements

- `IID-V0-001`: The optional diagnostic MUST require explicit host, version, cwd, default
  assumption profile and tracked subject. Cwd MUST be normalized, repository-relative and an
  ancestor of the subject. Traversal, absolute paths, repeat flags, mixed views and depth over
  64 directories MUST refuse. All evidence MUST bind to the captured tree and source blob;
  no home/config/session discovery, instruction execution or remote retrieval occurs.
- `IID-V0-002`: Only exact compiled, source-bound host/version profiles MAY predict loading.
  Unconfirmed versions/hosts MUST report UNKNOWN with no guessed rule source or load rows.
  The manifest MUST retain source URL, content SHA256, rule line selectors and assumptions.
  Actual trust/config/root resolution/environment state and global/injected instructions MUST
  remain named UNKNOWNs even when the conditional prediction is available.
- `IID-V0-003`: The Codex 0.153.2 default profile MUST walk assumed root to cwd, selecting the
  first regular file per directory in order AGENTS.override.md, AGENTS.md. A selected empty or
  whitespace-only source MUST shadow later names without consuming inner raw budget. Nonempty
  selected sources MUST use raw byte-prefix truncation before Rust-compatible lossy UTF-8
  decoding. Report original bytes, retained raw bytes, decoded bytes, inner raw budget and outer
  decoded-entry budget separately. Outer accounting saturates at zero. Multiple environments
  are not predicted. Selected unavailable bytes or non-regular identity MUST produce UNKNOWN
  and withhold downstream budget certainty. Missing source bytes MUST never mean an empty file.
- `IID-V0-004`: Candidate sources MUST carry visible bounded warnings for zero-width,
  directional and tag Unicode controls (Unicode Cf plus combining grapheme joiner), with code
  point, line and byte offset. This is a bounded format-control screen, not exhaustive injection
  or invisible-glyph detection. At most 32
  controls per file are listed followed by an explicit cap signal. Dirty instruction paths
  MUST visibly state that the prediction uses committed bytes. No comparison base is accepted;
  source modification in a diff under review remains UNKNOWN. Warnings are inspection signals,
  never proof of malicious intent or a downgrade of existing authority. No source text is emitted.
- `IID-V0-005`: The attachment MUST distinguish host-loaded, empty-selected, shadowed,
  ignored, truncated, exhausted and unknown rows without changing governing rows, trust labels,
  ranking or default packet output. CLAUDE.md being ignored by default Codex and AGENTS.md being
  shadowed MUST explicitly preserve the distinction from project-owned governance. Semantic
  conflicts between prose instructions remain unresolved; no contradiction detector is claimed.
- `IID-V0-006`: Evaluation MUST use bounded pinned index bytes only. Symlinks, excluded sources,
  unreadable/unloaded bodies and over-bound sources MUST retain uncertainty rather than reading
  targets from the filesystem. Snapshot behavior that withholds invalid-text raw bytes MUST
  remain UNKNOWN; where pinned raw bytes are admitted, invalid UTF-8 replacement follows the
  exact source rule. Identical inputs MUST produce deterministic canonical JSON; read commands
  MUST not write repository/index/trace state.
- `IID-V0-007`: Focused tests MUST cover default wire/authority preservation, selection and blank
  shadowing, truncation and invalid/multibyte UTF-8, source unavailable, bounds, source-modification
  and Unicode warnings, unknown versions/hosts and refusal cases. A compiled Core prediction
  MUST be compared with the same controlled root/nested and whitespace-override fixtures used
  in the retained exact-version host marker probe. A successful host probe establishes only
  MODEL_REPORTED_PROJECT_MARKERS for its fixture, launch configuration and no-tool receiver.

## Non-goals

No complete actual-session load-set claim, host dispatch or configuration editing, trust
verification API, new persistent ledger, arbitrary profile evaluation, multi-environment
prediction, imports, globals, remote rules, symlink following, semantic authority-conflict
resolution, instruction injection remediation, CEM authority downgrade, or V1-0414 completion.
This is a native-Go local diagnostic. Default Core wires and existing governance remain intact.

## Failure modes

The host may have untrusted projects, different markers/fallbacks/budget, multiple environments,
permission-dependent read errors, global configuration, or injected text. These invalidate an
actual-load claim; the receipt remains explicitly conditional. Exact source first selects a
regular file before reading it: empty overrides do not fall back. Source read errors can abort
an environment; top-level handling differs depending on sandbox presence. Pinned evidence cannot
prove host access, so actual access is UNKNOWN. UTF-8 decoding can increase decoded byte length
beyond the retained raw prefix; outer and inner accounting must not be conflated. A snapshot
may lack invalid-text raw bytes: abstention is safer than substituting a different byte stream.

## Acceptance evidence and rollback

| Requirements | Witness |
|---|---|
| IID-V0-001,002,005,006 | `TestInstructionDoctorFlagsAndDefaultWire`, `TestInstructionLoadSetUnknownsAndSafeScope`, `TestInstructionProfileManifestBinding` |
| IID-V0-003 | `TestInstructionLoadSetDefaultSelection`, `TestInstructionLoadSetPrefixBudgetAndLossyBytes` |
| IID-V0-004,006 | `TestInstructionLoadSetWarningsAndBounds` |
| IID-V0-007 | focused unit receipts and compiled CLI comparison against controlled host marker fixtures retained in the owner build log |

Passing fixtures prove the local evaluator under the stated assumptions. They do not prove
model compliance, whole-session completeness, automatic adoption or all-host compatibility.
The owner retains host probe events, argv, exits, final messages, source bindings and global
instruction UNKNOWNs independently of this experiment's unit assertions.

Rollback: remove the optional parser/attachment and its isolated implementation, tests and
profile manifest. Default packet bytes and governing evidence require no migration. The spec
remains as experimental provenance; no source instruction files were changed by the diagnostic.
