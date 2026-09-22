# Security policy

## Supported versions

Corvint is in alpha. Only the most recent published alpha receives security fixes.

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
