# Decision 0420: release-policy selections for 1.0.0-rc.1

Date: 2026-09-26. Status: accepted (owner answer 2026-09-26: "go with your recommendations").
Tickets: V1-0016, V1-0018, V1-0020. Applies `PRS-V1-004` and the signing rule of
`release-artifact-integrity-v0.md`.

## Context

A read-only gap analysis of the rc.1 freeze at `984770f6` found five release-policy questions
with no recorded answer:

- No spec defines the "stable-readiness record" that V1-0018, V1-0020 and V1-0021 require.
- The native linux/amd64 host for `PRS-V1-004` is unnamed.
- There is no signing selection after `0.5.0a3` (decision 0329). `release-artifact-integrity-v0.md`
  says any later prerelease needs a new selection.
- `native-performance` is `NOT_RUN` in every checklist run (`GOC-V0-005`), and no decision says
  whether 1.0 accepts that.
- No vulnerability check exists.

The Core module's `go.mod` declares no `require` directives, so the Core binary links only the Go
standard library.

## Decision

1. **Stable-readiness record.** The agent drafts a proposed spec,
   `stable-readiness-record-v1.md` (`SRR-V1`), and a generator and verifier in
   `internal/releasecandidate`. The record is one canonical JSON document bound to one commit and
   tree, and it lists every `NOT_RUN` row with its accepting decision.
   - The spec stays proposed until a later decision accepts it.
   - The `release-checklist` exit semantics are unchanged.
2. **linux/amd64.** linux/amd64 is qualified by a native run on a GitHub-hosted `ubuntu-24.04`
   runner through the owner-dispatched release-gate workflow of decision 0415.
   - The run covers the install lifecycle and the plain-CLI host lifecycle on the candidate bytes.
   - Until that evidence is retained for the candidate, linux/amd64 is reported FALLBACK
     (`PRS-V1-004`).
3. **Signing for `1.0.0-rc.1`.** The selection is **No signing**: archives ship with `SHA256SUMS`
   only.
   - The release notes and any publication receipt state publisher identity as `NOT_VERIFIED`.
   - The selection covers `1.0.0-rc.1` only. `1.0.0` and any later prerelease still need their own
     selection, and no agent may create or use a signing identity.
4. **native-performance.** `native-performance` stays `NOT_RUN` under `GOC-V0-005` for 1.0.
   - It does not block rc.1 or 1.0. The stable-readiness record lists it as `NOT_RUN` accepted by
     this decision.
   - No release note claims measured native performance.
5. **Vulnerability check.** The 1.0 Core vulnerability check has two conditions, and the
   stable-readiness record carries both:
   - the Core module's `go.mod` has no `require` directives;
   - the build toolchain is exactly the pinned `go1.27.1`.

   The check has these limits:
   - No scanner runs and nothing reaches the network.
   - Before tagging, the owner compares that toolchain against the Go security releases. A newer
     patch release with a standard-library fix moves the pin before the tag.
   - Companion and interop modules are outside this check.

## Not decided here

- The untouched public repository for V1-0019 (`PRS-V1-008`) is still unnamed.
- The P0/P1 ticket set that blocks rc.1 (V1-0020 AC0) is unset.

## Non-goals

This decision does not accept the `SRR-V1` spec, tag or publish anything, or change a store
release. It also does not change `script/release-checklist`.

## Rollback

Revert this change. Nothing is tagged, signed or promoted under it; a later decision can select
signing, a different linux/amd64 host or a vulnerability tool.
