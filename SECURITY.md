# Security policy

## Supported versions

Support window for every 0.x version: only the latest published release receives security fixes.
A release is outside the window the moment its successor is published, and a withdrawn release
(see the [release runbook](docs/RELEASE-RUNBOOK.md)) is outside it immediately. The 1.0 support
window is set by the owner at V1-0021 (accept and promote Corvint 1.0 stable) and recorded here.

| Version | Supported |
|---|---|
| latest published 0.x release | yes |
| any earlier 0.x release | no |

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
