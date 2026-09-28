# OpenCode qualification diagnostic ownership

PR 333 doc-gates job 109126927858 rejected sealed commit
`94dd4d191f79b87d0f3b384421b3b0e9a96366f4`: `qualification-in-progress`, emitted by
`internal/opencodequalification/record.go`, had no owning spec entry. The AHI-032 failure-code
table now names that existing INCOMPLETE-record reason and its UNQUALIFIED consumer meaning.

This follow-up changes documentation only. Runtime, package 0.7.2, prior native qualification,
and installed package bytes remain unchanged. The original CI failure is retained at
`/tmp/corvint-opencode-docs/evidence/original-ci-failure.log`. The selected check runs the exact
six-target failed CI docs command plus package-version and Go-only dependency checks; unchanged
native/PTY qualification is retained at tested binding `848381f2e682fbeaa27e976716240d00262bc48c`.
Repository-wide validation, merge and native ticket completion remain coordinator-owned.
