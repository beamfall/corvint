# Experimental protected local runner

This separate optional Go module executes exact `internal/wp3codec/codec.go` bytes via a fixed
Go 1.27.0 WASI wrapper and pinned wazero 1.12.0 interpreter. The default Corvint module has no runtime
dependency. Owning proposed contract: `../../docs/specs/protected-local-execution-v0.md`.

`prototype SOURCE_CODEC_GO ABSOLUTE_GO_BINARY` emits an unsigned `authority: NONE` diagnostic.
It is not an enrollment, attestation, arbitrary native test runner or FULL qualification.

Protected operations are explicit and fixed: `install-release BUNDLE FINAL_MANIFEST_SHA256`,
`setup-accounts`, `operator-keygen`, `stage-enrollment HANDLE BUNDLE`, `operator-run HANDLE`,
`remove-release RELEASE_ID`, and `retire-accounts`. They require an independently controlled root
operator and protected release. No command creates accepted-root.json or minimum-generation.json.
A stale operator.lock requires explicit process-cleanup audit; no automatic stale-lock bypass exists.
Private protocol-complete terminals include authentic assertion FAIL. The consumer independently
computes closure and requires PASS for every mandatory selected check.

Unprivileged packaging is `prepare-release OUTPUT CONSUMER GO_ROOT GIT_BINARY SOURCE_REVISION ADAPTER_TEMPLATE`.
The original release ID commits the immutable inputs and original adapter source; the separately
hashed final manifest includes a non-executable authority-hook.json declaration. The corvint consumer contains the native normalizer and bootstraps into the closed protected environment. The execution-policy proposal is a sidecar,
not accepted policy. Use the exact Apple CLT Git executable, not /usr/bin/git's dispatch shim; the
preparer refuses non-system dylib dependencies. Original and prepared hashes remain explicit.
Optional qualified lifecycle packaging is explicit:
`prepare-qualified-hooks BUNDLE TEMPLATE OUTPUT`, using the final prepared bundle and
`../../integrations/codex/plugins/corvint/hooks/qualified-hooks.proposed.json`.
It creates an exclusive caller-owned sidecar directory with `hooks.json` and `preparation.json`.
The receipt binds source/release, final manifest, actual consumer/rendered adapter, template and
four-event hooks hashes; it carries authority NONE. Input drift or partial failure leaves no success
receipt. Neither this command nor install-release changes Codex configuration or admits qualification.

Before activation the operator reviews the sidecar receipt against the final immutable release,
confirms the concrete Codex hook configuration destination/owner for the selected host, retains the
existing legacy hooks as rollback material, and installs that exact reviewed four-event configuration
without duplicate legacy handlers for those events. This source gives no guessed host destination
and performs no activation. The rendered commands invoke the final immutable corvint native-hook entry point
with --qualified-lifecycle. Revoking the optional admission and restoring the saved legacy hooks
reverses activation; old evidence remains retained.

Initial bootstrap must copy the reviewed binary/scripts into root-protected storage and verify those
protected copies before execution. The shell wrappers refuse unprotected self/executable ancestry.

Tests require an explicit checked module identity. Native process/RSS tests additionally require
CORVINT_NATIVE_CAMPAIGN=1 outside the desktop sandbox. `../../conformance/local-authority-v0/campaign.py`
records kernel birth identities and cleans observed trusted test descendants; that observation is not
principal separation or generic native-code confinement. The recorded v1.12 unprivileged campaign
passes good/bad candidate, WASI loop/memory/import/output checks and guardian interruption tests.
Actual protected accounts, key isolation, signed public flow, installer lifecycle and native host
qualification remain NOT_PRODUCED pending reviewed privileged activation. Test-package worker hashes
are not release-binary hashes. The accidental root-module test invocation lacks a prior lifetime
inventory and retains cleanup-proof NOT_PRODUCED, separately from the later supervised campaign.

Reader admission is explicit `admit-reader ROOT_OWNED_AUDIT_JSON`; withdrawal is `withdraw-reader`.
The closed audit template is `../../conformance/local-authority-v0/templates/reader-admission-audit.template.json`.
It requires an independent retained audit of the group's other privileges and local-only identity
configuration. These are operator assertions with a digest, not conclusions inferred by the program.
The program checks local Directory Services records and all primary/supplementary members, resolves
system/search identities, and refuses pre-existing reader membership. It admits one exact reader to
the dedicated authority group. That reader can read all source/history included in any capsule.
No accepted-root or floor is written. Independent Root /1 admission must match the ledger's intended
root/epoch/generation, authority UID/GID and reader UID. Membership replacement requires independent
revocation or a new policy epoch/generation and fresh process credentials before requalification.

A durable `installed-reader.json` records intent before mutation, directory device/inode before
exposure, and observed membership completion separately. Failures retain it. Withdrawal first
restricts that exact held evidence parent to 0700, then checks Directory Services and removes only
its recorded membership. Drift/DS outage leaves a partial withdrawal with the ledger retained;
unexpected ACLs prevent a complete source-access-withdrawal claim. Removing group membership alone
does not change running process credentials, and 0700 does not revoke already-open descriptors.
Normal shutdown of the exact owned candidate processes belongs in the approved rollback sequence.
Account retirement requires withdrawal and archives the evidence parent to root:wheel 0700 through
a bound descriptor before deleting principals. It never recursively chowns source evidence.
A retained evidence directory or retired ledger is not automatically reused for a new admission;
independent archival/recovery and a new reviewed policy are required.

`native-bootstrap.template.json` supersedes obsolete proposals that admitted completed host
qualification before exercising the candidate. It remains inert until final release, fixture,
configuration and runtime values are known. Reader admission precedes fresh app/engine launch;
actual credentials and hook child cwd precede a campaign. OPEN and EMPTY use distinct fixed targets,
fresh enrollment nonces and separately predeclared <=900-second campaign documents. Publish each
handle immediately before its phase because publication selects the active handle. Failure or
expiry stops the finite sequence. Preserve startup/prompt/end hooks and select exactly one protected
Stop policy; restore the precise saved native configuration. Completed HostQualification is a later
independent decision requiring actual evidence. The eleven-case fixture/collector schedule is still
NOT_PRODUCED by these lifecycle templates.

### Prospective direct Codex CLI sidecar

`prepare-qualified-direct-hooks BUNDLE TEMPLATE OUTPUT` accepts only the uninstalled
`integrations/codex/plugins/corvint/hooks/qualified-direct-hooks.proposed.json` shape,
whose four commands begin with `exec` and use `--qualified-direct-lifecycle` (QLF/1).
It writes only a caller-owned review sidecar; root/2, direct runtime admission and
native qualification remain separate operator actions under DCLI-V0. Legacy preparation
refuses this template. No hook or protected file is installed by preparation.

### Experimental protected Pi packaging

`prepare-pi-release OUTPUT BUILD_DIR GO_ROOT GIT_BINARY SOURCE_REVISION ADAPTER_TEMPLATE`
uses the sealed Pi build's `corvint`, `pi-protected` and `manifest.json`. Its distinct
`corvint-pi-authority-release/0` manifest binds these images and the copied build metadata,
with the existing execution tools and inert adapter declaration. Copied image hashes are
rechecked before the manifest is written. `install-pi-release BUNDLE MANIFEST_SHA256`
requires the independent root operator, exact reviewed manifest and no existing non-Pi
admission. Legacy installation refuses the Pi release profile. No command creates an
accepted root, campaign or completed qualification. `remove-release` retains exact-file
ownership/drift checks and requires revocation of an admission referencing the release.
Revoked Pi admission permits the same guarded reader withdrawal as legacy root/1.
Actual installation, rollback, input/license closure and native FULL qualification remain
unverified; see `../../docs/specs/protected-pi-runtime-v0.md`.
