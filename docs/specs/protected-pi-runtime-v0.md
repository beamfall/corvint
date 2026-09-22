# Protected Pi Runtime V0

Owner: Russell Lewis
Date: 2026-09-22
Requirement prefix: `PPI-V0`
Intent status: accepted direction (owner approval, 2026-09-22); technical details experimental
Delivery status: experimental; no protected installation or native qualification
Authoritative inputs: `agent-harness-integration-v0.md`, `protected-local-execution-v0.md`, `qualified-lifecycle-v0.md`, decision 0009

## Agent digest
- Claim: An optional fixed Pi SDK executable can participate in independently qualified protected lifecycle computation without admitting ordinary mutable Pi extensions.
- Status: accepted direction; experimental; no execution root, completed qualification or FULL claim.
- Exists: pinned Pi 0.85.1 Node SEA feasibility with native TUI/RPC, startup injection negatives and positive controls; functional Pi adapter 0.2.0 remains separate.
- Blocked on: reproducible immutable runtime closure, new closed Pi authority profile, adversarial/native qualification, independent admission and explicit operator activation.
- Read next: Requirements; Runtime boundary; Acceptance and rollout.

## Human intent and verified starting point

The owner requested complete Pi support, explicitly included protected authority for formal FULL,
and approved the separately reviewed restricted native SDK distribution with a fixed extension set.
That approval accepts implementation direction, not privileged installation, signing authority,
independent evidence, or a completed qualification. Ordinary extensible Pi remains available.

At `dac6d987027b3cfd8f77a4f1a9b5975b6c4f29ff`, adapter 0.2.0 supports Pi 0.85.1
native tools, explicit record, observations, compaction and session recovery, print/TUI/RPC and
bounded cleanup. Its selected native-host test passed; its full gate remains held for a shared
gate-ledger defect. Existing protected runtime admission is Codex-only.

A standalone Bun 1.3.11 executable with all configuration autoload disabled still executed
`BUN_OPTIONS` preload code and `BUN_BE_BUN` eval code. It cannot establish this profile's closure.
The supported Node 22.23.2 SEA alternative passed these startup probes with embedded fixed
arguments, native Pi TUI/RPC, and macOS hardened-runtime canaries. Scratch assets still reference
the mutable global Pi installation: this is feasibility evidence only.

## Requirements

- `PPI-V0-001`: Build an optional native macOS arm64 executable containing exact Pi 0.85.1 SDK code and a fixed Corvint extension. Preserve upstream InteractiveMode and runRpcMode. Pin the build inputs, toolchain, dependency closure, image, entitlements and required assets. No default Core distribution or other host admission changes.
- `PPI-V0-002`: Executable loading MUST remain closed under all admitted arguments and environment, including same-image re-exec. No project/global extension or package discovery, mutable provider code, jiti, preload, native addon or worker fallback may introduce unbound executable bytes. A truly custom ResourceLoader rebuilds only the compiled factory set on reload and session replacement; DefaultResourceLoader must never run first. Missing protected assets refuse; never fall back to the installed Pi package or a package download.
- `PPI-V0-003`: Mutable session, credential, model and settings data MUST remain distinct from executable resources. Only reviewed data fields may configure built-in providers and native UI. Shell-resolved credentials and settings that introduce executable loading refuse. Ordinary built-in coding tools retain user permissions as separate untrusted child work; neither tool success nor credential access grants protected execution authority.
- `PPI-V0-004`: Native TUI/RPC MUST preserve context, exact expansion, explicit observations/outcomes, session replacement and compaction recovery. Context framing, bounds, provenance and explicit uncertainty remain those of AHI. Successful idle completion permits at most one independently allowed remediation; abort/error never restart. Recursive completion releases unresolved. All owned descendants terminate on interruption or timeout.
- `PPI-V0-005`: Introduce a distinct closed Pi root/campaign/qualification/lifecycle profile. Existing root/1, root/2 and QLF/0, QLF/1 retain their exact meanings and refuse mixed profiles. Protected data alone selects host identity, exact native process/image and qualified surfaces; caller input cannot select a root, host, qualification or authority. The fixed protected store must not overwrite an existing non-Pi admission. Provenance remains caller-asserted and event surface unattributed.
- `PPI-V0-006`: Reuse protected computation and before/after guards. Non-Stop reads no protected enrollment/publication/private evidence; Stop derives the current enrolled handle and computes the real Frontier once. Missing, invalid, revoked, stale or cross-process admission refuses. Local completion and protected Frontier remain distinct; EMPTY cannot erase unmet local policy, and local policy cannot grant authority.
- `PPI-V0-007`: Independent runtime admission MUST bind full image SHA256, live mapped CDHash, CS_VALID/no CS_DEBUGGED, hardened code-loading policy, boot UUID, PID/birth, immediate parent topology, exact OS/architecture, actual repository cwd and protected consumer/assets. Root ownership, ancestry, file type, links, ACLs and replacement controls cover the immutable closure. Same-image exec is allowed only under 002; birth/CDHash is not an exec epoch. New lifetime requires fresh admission.
- `PPI-V0-008`: FULL requires independently completed exact-image TUI and RPC qualification: all eleven AHI cases, actual OPEN/EMPTY/recursive Stop, permission/privacy negatives, cold query and lifecycle p95, equal-critical-evidence recall, final FULL-shaped output bounds, installation/revocation and cleanup. Candidate campaigns remain bounded to 900 seconds, FALLBACK/UNQUALIFIED and never self-renew or self-promote. Source tests, manifests, signatures and local workflow receipts cannot fill missing qualification evidence.
- `PPI-V0-009`: Rollback removes/revokes only the optional Pi campaign/admission/runtime and restores ordinary FALLBACK use. Installation refuses to overwrite a different host/root or an existing unreviewed asset set. Image, asset, dependency, OS or policy drift invalidates qualification and requires fresh review; old receipts are retained unchanged.

## Runtime boundary

The first implementation uses Node SEA with `execArgvExtension: "none"` and embedded
`--disable-sigusr1`, `--openssl-config=/dev/null`, and a final `--` argument separator.
The OS-owned macOS null device is the OpenSSL configuration dependency, not a protected regular
file. Hardened runtime retains library validation and permits only the JIT entitlement required by
V8. Positive canaries must prove the tested preload paths actually execute against the control.
No debugger, DYLD or OpenSSL environment path may supply code to the admitted host.

The complete fixed asset set must be independently staged under protected ancestry and bound to
the executable's build. Mutable global npm/Bun installs, cwd settings, caches and environment paths
cannot supply fallback code. An ad-hoc signature is a local image identity, not publisher trust or
operator admission. Child tools and repository content never become admitted host code.

The versioned wire definition is implemented and reviewed with the authority slice; this document
accepts no reinterpretation of existing schemas. Normal Pi remains the simpler default. Linux,
Windows, arbitrary extensions/providers implemented as code, automatic root admission, a permanent
service, and event-origin attestation are outside this bounded profile.

## Acceptance and rollout

1. Prove the complete native prompt/TUI/RPC path before authority machinery. Keep prototype gaps visible.
2. Build from pinned inputs with mutable global Pi unavailable. Exercise hostile cwd/global configuration, preload/eval arguments and environment, reload, new/resume/fork and interruption. Independently review the immutable closure.
3. Implement the new profile with closed-decoder, process substitution, stale admission, cwd/root drift, currentness and recursion negatives. Preserve old-profile golden behavior.
4. Freeze the final source/artifact, run repository gates and all native qualification cases and measurements. Record failures and NOT_RUN without relabelling.
5. Prepare a concrete protected installation/admission for operator approval. Only independent completed evidence can authorize FULL. Revoke/remove the Pi installation for rollback.

## Traceability

| Requirement | Implementation/evidence | Current gap |
|---|---|---|
| PPI-V0-001..003 | Scratch pinned Node SEA, native Pi SDK and startup canaries | reproducible repository implementation and immutable assets NOT_PRODUCED |
| PPI-V0-004 | Ordinary adapter `integrations/pi/host.test.mjs`; separate SDK TUI/RPC feasibility | protected lifecycle and recursion NOT_PRODUCED |
| PPI-V0-005..007 | Existing authoritystore/QLF are reference mechanisms only | new Pi profile and runtime admission NOT_PRODUCED |
| PPI-V0-008..009 | AHI conformance and this accepted scope | qualification, operator installation and rollback evidence NOT_RUN |

Stop promotion when a mutable executable dependency remains, real native behavior fails, or any
required independent/measurement evidence is missing. Repair implementation within this intent;
never weaken the admission or FULL definition to make the prototype pass.
