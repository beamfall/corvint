# Security policy

## Supported versions

Historical support window for every 0.x version: only the latest published release receives
security fixes. A 0.x release is outside the window when its successor is published; a withdrawn
release is outside it immediately.

From 1.0.0, security fixes cover the latest published, non-withdrawn **stable Corvint release
artifact set** until its next stable successor is published (decision
[0433](docs/decisions/0433-rc2-stable-signing-and-support-policy-2026-09-29.md)). A prerelease does
not end the current stable release's support. Withdrawn releases are unsupported immediately.
Release candidates are evaluation builds with no stable support guarantee. Exact independently
versioned component support is listed with the release artifacts; experimental and FALLBACK
surfaces do not gain a stable Core promise.

| Version | Supported |
|---|---|
| latest published, non-withdrawn stable release from 1.0.0 | yes, until its next stable successor |
| earlier stable or withdrawn release | no |
| release candidate | no stable support guarantee |

No older-version backport promise or fix-time SLA is made. The private reporting channel and
seven-day acknowledgment target below remain unchanged.

## Reporting a vulnerability

Report vulnerabilities privately through GitHub's
[private vulnerability reporting](https://github.com/Beamfall/corvint/security/advisories/new)
for this repository. Do not open a public issue for an unpatched vulnerability.

Include the version or commit, a reproduction, and the impact you observed. Reports are
acknowledged within seven days.

## Scope

Corvint is a local-first tool that reads Git repositories and writes derived state under `.corvint/`.
Reports about the following are in scope:

- Reading or writing outside the repository root or the documented derived-state locations.
- Secret leakage through learned traces, evidence packets, or the self-observation ledger.
- Unauthorised network activity from the default local binary.
- Authority or evidence forgery: content presented as pinned to Git that is not.
