# T10 runner protocol and discovery build

Date: 2026-11-07. Ticket: V1-5112. Requirements: RSP-V0-025 through
RSP-V0-032 and RSP-V0-041. Build-only lane; execution and independent review
remain integration-owned. No test pass, qualification or performance result is
claimed in this entry.

The discovery schema is **orchestrator-made under delegation** (owner 2026-11-02,
T10 round 2). The exact `corvint-runner-plugins/1` object, required fields,
plugin-name grammar, lowercase SHA-256, strict unknown-member refusal, uniqueness
and canonical name sorting are recorded in RSP-V0-028. This resolves the previous
round's missing-schema blocker. Repository executable paths label operator install
sources; the accepted prohibition on executing a repository path remains in force.
The operator store uses the same shape with installed absolute executable paths.

The implementation adds an optional protocol host and retains T3's compiled-in
SPI unchanged. Discovery failures retain runner claims and reasons; malformed
files produce FULL for all runners rather than a partial list. CI discovery takes
an immutable trusted-base reader from the core and invalidates the union of old
and candidate claims on pin edits. The operator command installs local bytes by
digest outside repositories and never changes repository pins or qualification.
No executable enters the release manifest and no frozen core command is changed.

Linux execution verifies a sealed descriptor and executes that same descriptor,
with no plugin PATH lookup. Unsupported platforms and refused primitives fail
closed. The host uses the existing process-group reaper and Git-status isolation,
plus bounded working-file content snapshots, to detect mutations to already dirty
and untracked files. This is process lifetime management, not a network or
privilege sandbox. The plugin contract prohibits starting runners; the core owns
runner execution and tracing.

Strict schemas cover the six methods and native components reuse T1 types.
Inventory, commands and results adapt to T3 types. Native evidence is staged until
successful shutdown, process reap, output closure and post-session mutation check;
any failure discards every runner's staged records. Qualification and trace
components cannot be supplied through the native evidence surface.

The shared conformance kit runs the same captured-artifact corpus through a
compiled-in Go reference adapter and its out-of-process twin. Both reuse the
existing native Go report parser; the reference returns absent coverage and stays
unqualified. It is an interoperability fixture rather than a production inventory
collector. Requirement-named positive and negative tests cover strict frames,
discovery, command paths, identity, bounds, deadlines, failed publication, mutation,
features and explicit installation. Tests are NOT_RUN under the build-only charter.
Overhead under CMS-V0-010 is NOT_RUN, as explicitly excluded from this lane.

The later build-only instruction forbids Go tests and dogfood binding/sealing.
The start `dogfood-change` wrapper enters the binding flow, so it is not invoked;
new pre-change query/impact receipts and a final CEM are NOT_PRODUCED this round.
The existing CEM predates T10 and must not be treated as evidence for it. The
mandated spec targets contain Go-test recipes; only their generation and static
checks are run, with those recipes explicitly disabled and retained as NOT_RUN.
Whole-package test execution, independent review, final CEM binding and native
completion remain with integration. No Tasks writes or publication are performed.

Rollback: remove the optional tool pins to restore FULL for their runners, or
revert this additive slice. The compiled-in paths, evidence-store wire and frozen
core command surface are unchanged. Removing the operator store alone cannot
retain narrower selection.
