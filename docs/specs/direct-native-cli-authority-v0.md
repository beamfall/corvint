# Direct native CLI authority V0

Owner: Russell Lewis
Date: 2026-09-15
Requirement prefix: `DCLI-V0`
Intent status: prospective implementation profile; root Gate A approved bounded implementation 2026-09-15
Delivery status: experimental; no operator admission
Authoritative inputs: `docs/specs/protected-local-execution-v0.md`, `docs/specs/qualified-lifecycle-v0.md`, `docs/decisions/0009-harness-authority-boundary.md`

## Agent digest
- Claim: One closed direct-native Codex CLI topology reuses protected lifecycle computation without inheriting Desktop qualification or execution authority.
- Status: prospective implementation profile; root Gate A approved bounded implementation 2026-09-15; experimental; no operator admission; no accepted execution root or completed native qualification.
- Exists: actual installed Codex CLI 0.153.2 hook parent feasibility; bounded implementation under review.
- Blocked on: independent implementation review, exact native qualification, independent admission and operator activation.
- Read next: Requirements; Wire contract; Acceptance and rollback.

## Human intent

The owner requires formal FULL on actual native CLI surfaces. Root reviewed the closed Codex-only
profile and the actual SessionStart/Stop exec observer probe: both immediate parents were the same
native CLI process and image. This permits source implementation, not execution-root admission.
Other hosts, interpreters, authentication, event-origin proof and default local workflow are excluded.

## Requirements

- `DCLI-V0-001`: **One real host.** Root-protected candidate/completed records bind host=`codex`, surface=`codex-cli`, executable bytes/CDHash, exact OS build/arm64, boot UUID and actual PID/birth. One process occupies one host role: no App=Engine, terminal-as-App, desktop qualification inheritance, shared-daemon substitution, or host selection from input/environment/version text. Completed records retain the exact host process pin; restart needs independent readmission. No process IDs or host paths enter public output.

- `DCLI-V0-002`: **Smallest parent topology.** Consumer's immediate kernel parent must be that exact host process, at both verification boundaries. Reuse `inspectProcess` and `verifyHostImage`; capture/recheck the complete child and parent records and CDHash. The reviewed hook command must use `exec` through any host-selected command shell so the protected native adapter/consumer remains the same child process. If Codex actually leaves an intermediate shell, broker or daemon, refuse this profile; do not scan past it. A separately specified exact bridge would be a material amendment. The host's own parent is not an authority principal; capture/recheck its numeric parent relation during each observation, but no terminal identity grants trust. Reparenting during a check refuses. This admits host-descended ordinary tool invocations too: that is handled by 004, not hidden.

- `DCLI-V0-003`: **Current immutable image and existing admission boundary.** Bind exact executable path/SHA256/CDHash in the independently admitted record. Reuse `verifyHostImage`: actual mapped code must retain CS_VALID, no CS_DEBUGGED and matching CDHash (`native_darwin.go:211–257`). SHA256 is a complete-byte **admission** pin; the existing host reader does not rehash disk bytes. Independent admission must establish its correspondence to the signed image and audit external code/resources and injection/loader surfaces, just as the existing AppInstance audit does. Bind that review by `runtimeAdmissionEvidenceSHA256`; the digest alone is not an audit. Reuse existing protected-file checks where admission stages files; this proposal mandates neither host relocation nor a new closure-manifest engine. If a mutable external code source defeats that audit, report that specific gap. Do not require every possible stronger hardening property, claim interpreter/package closure, or infer all loaded code from an executable CDHash. Child tools/repository text remain untrusted data or separate processes, never admitted host code.

Birth+path+CDHash does **not** prove an exec generation. Same-image re-exec can preserve birth, but does not change protected root, policy, repository/target permissions or admitted current image. It is therefore an admissible limitation of this profile's process-lifetime/current-code claim, not a new gate. Different current path/CDHash refuses; a changed admission-relevant code/resource boundary invalidates the audit. No continuous-session or internal-event provenance is promised, and no exec monitor is introduced. Do not claim detection of an exec-and-return between observations.

- `DCLI-V0-004`: **Provenance/trust.** Always `requestProvenance=caller-asserted`, `eventSurface=unattributed`; qualifying a surface proves its independently exercised capabilities, not this request's native origin. Same-host synthetic calls cannot supply AHI evidence. Host trust cannot admit keys, execute protected checks or close Frontier independently. Decision 0009 remains unchanged; any actual execution root still requires separately accepted policy and operator admission. Claude's static signature failure is neither measured live-kernel rejection nor an automatic waiver; this Codex topology admits no Claude/Gemini/Pi tuple or interpreter closure.

- `DCLI-V0-005`: **Common computation/currentness.** Preserve fixed protected paths, release/adapter/Git verification, root/floor/policy/current enrollment/publication, actual cwd, before/after checks, one Frontier computation, bounded deadlines/output, privacy and cleanup. Candidate is at most 900 seconds, exact root/floor/enrollment/target and native-event scope, FALLBACK/UNQUALIFIED with no qualification digest. Present invalid completed qualification never falls through to a candidate. Completed FULL requires independent exact-tuple AHI/native latency/equal-critical-recall evidence and later operator admission; no self-promotion. Local completion remains separate from verified Frontier authority.

- `DCLI-V0-006`: **Negative acceptance.** Closed decoder rejects old/new mixed fields, unknown hosts/surfaces/topologies, fake App aliases and caller identity. Runtime refuses sibling/second identical host, PID reuse/birth drift, boot/OS drift, extra parent, reparenting during observation, different exec image, signature/debug status drift, protected-file symlink/hardlink/ACL/mode/substitution, expiry/revocation/floor rollback and changed root/cwd/target. Independent admission rejects a mutable or unbound host-code dependency; a disk-path substitution must not be misreported as mapped-image drift when kernel code identity did not change. Test invalidated admission through its actual protected revocation/pin update, not an invented dependency scanner. Test ordinary same-host tool submission as **unattributed**, not falsely rejected as a forged native event. Test same-image re-exec against the explicitly limited claim; never label it detected without an exec-generation witness.

- `DCLI-V0-007`: **Rollback.** Revoke/raise protected generation floor, withdraw the direct qualification/campaign and remove the explicitly installed direct hook. New binaries keep old-profile behavior; old binaries reject new profiles safely. Never downgrade root data in place or carry FULL evidence across changed pins.


## Wire contract

All objects below are closed; fields are required unless explicitly nullable. Canonical encoding, duplicate/unknown-member rejection, existing size/time/digest/path rules and safe-reader ownership apply. `Image`, `ProcessInstance`, `SurfaceQualification` retain their current definitions. Digest means 64 lowercase hex; CDHash means 40 lowercase hex. Placeholders below describe types, never runnable admission values.

## Version dispatch — no reinterpretation of old fields

| Existing schema | New schema | Allowed pairing |
|---|---|---|
| `corvint-protected-root/1` | `corvint-protected-root/2` | /1 retains only old qualification/campaign schemas; /2 admits only direct variant below |
| `corvint-native-qualification-campaign/0` | `corvint-native-qualification-campaign/1` | /1 only under root/2 |
| `corvint-native-qualified-host/0` | `corvint-native-qualified-direct-host/0` | direct only under root/2 |
| `corvint-qualified-lifecycle/0` request/result | `corvint-qualified-lifecycle/1` request/result | /1 only for root/2; /0 behavior unchanged |

Root/2 contains **exactly** all current RootDocument members (`types.go:58–84`), except `profile` is root/2 and `hostQualification` is null or DirectQualification. No App/Engine optional aliases. Root/floor files and independent admission mechanics stay fixed. Floor schema is unchanged; roots cannot change version to evade its epoch/generation checks. Profile mismatch refuses, never fallback to another protected profile.

```typescript
type DirectRuntime = {
  topology: "direct-native-cli";
  host: "codex";
  surface: "codex-cli";
  bootSessionUUID: string; // nonempty canonical observed UUID
  hostInstance: ProcessInstance; // PID >1, start >0, usec <1e6
  hostImage: Image; // absolute canonical audited immutable regular file
  hostCDHash: CDHash;
  runtimeAdmissionEvidenceSHA256: Digest;
  parentPolicy: "immediate-host";
  osBuild: string; // exact observed, nonempty
  architecture: "arm64";
};
type DirectQualification = {
  profile: "corvint-native-qualified-direct-host/0";
  evidenceSHA256: Digest; // independently completed tuple evidence
  runtime: DirectRuntime;
};
```

Campaign/1 contains exactly campaign/0's existing members (`campaign.go:25–47`), changes `profile` to /1 and `runtime` to DirectRuntime. All current common field validations persist, including status `NATIVE_QUALIFICATION_ONLY`, ≤900 seconds, digest-bound floor, target, images and scope. No qualification evidence/surfaces are accepted inside a candidate.

## Runtime admission evidence — reuse, not a new manifest engine

`runtimeAdmissionEvidenceSHA256` binds the independently reviewed exact executable-byte/CDHash/process/OS tuple and applicable code/resource/loader admission evidence. It is present in candidate and completed runtime records but is **not** completed native AHI qualification evidence. Root ownership/independent admission confers trust; merely adding a digest cannot self-admit a candidate. The review must preserve existing `verifyHostImage` limits: runtime checks signed mapped-code identity; complete-byte/resource correspondence and relevant external-code protections belong to admission (`native_darwin.go:211–257`). No additional host-closure file/schema, installation path, broad memory reader or dependency-manifest subsystem is added. Audit evidence stays independently reviewable; unavailable properties are explicit. No admission is performed here.

## QLF/1

Request has the same three members and existing four normalized event inputs as QLF/0; only profile changes. It carries no host, root, path, tuple or trust selector. Protected native adapter/renderer gains one explicit versioned mode that constructs/accepts /1; it cannot choose a host identity. Cross-version request/root combinations refuse.

Result retains **exactly** QLF/0 top-level members and common invariants, changes profile to /1, and replaces only `qualifiedHost`:

```typescript
type DirectQualifiedHostResult = {
  host: "codex";
  digest: string; // empty candidate; canonical DirectQualification digest when completed
  evidenceSHA256: string; // empty candidate; exact admitted evidence when completed
  hostSHA256: Digest;
  runtimeAdmissionEvidenceSHA256: Digest;
  adapterSHA256: Digest;
  osBuild: string;
  architecture: "arm64";
  supportScope: "candidate-direct-native-runtime" | "qualified-direct-native-runtime";
  eventSurface: "unattributed";
  qualifiedSurfaces: Array<SurfaceQualification>;
};
```

Candidate: FALLBACK/UNQUALIFIED, first degradation `native-tuple-unqualified`, empty digest/evidence/surfaces. Completed: FULL/QUALIFIED, exact singleton surface `codex-cli` whose evidenceSHA256 equals the qualification evidence; no App/Engine digests, inherited Desktop entry or caller-provided version. Existing compaction degradations and Frontier rules persist. Non-Stop authority stays NONE. Stop may report VERIFIED only from the existing independent protected computation; qualification alone grants nothing. Recompute native byte reservation with maximal /1 serialization and retain old /0 checks.

Standalone `corvint-authority-event/0` wire is unchanged; its public entry refuses root/2. Internal QLF/1 resolution may supply the same existing protected computation through a non-public typed path, with no request flag that bypasses version checks. Root review must reject any legacy renderer that advertises the direct tuple under old Desktop meaning.

## Explicit scope of process identity

ProcessInstance means process lifetime, not exec epoch. Same-image exec is not detected by birth/path/CDHash sampling and is not itself disqualifying: it changes neither admitted current code nor the protected authorization scope. Current different image/path/CDHash or invalidated admission refuses. This profile promises no uninterrupted session or internal native-event provenance, so it adds no exec-generation witness. Stronger provenance would require a separate justified contract; it is not a prerequisite invented for this topology.

## Acceptance and rollback

Closed schema and runtime hostile fixtures plus existing old-profile regressions are mandatory.
Candidate/qualified serializer and native adapter tests must preserve provenance, complete native
byte bounds, one computation and truthful uncertainty. Source fixtures do not qualify native AHI
or immutable host-code admission. Raise the protected floor/revoke and withdraw direct hooks to
roll back; old schema meanings remain unchanged. No protected publication is written by this change.
