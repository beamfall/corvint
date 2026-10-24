# Windows behaviorfalsify and Boost.Test path logic (V1-2524)

Date: 2026-10-24. Ticket: V1-2524. Base: `origin/main` `025120e1`. Spec:
`docs/specs/browser-behavior-falsification-v0.md` `BBF-V0-215`. Follows
`2026-10-23-windows-production-path-logic.md` (V1-2488). No Windows host was available: each
cause below is the first failure the code reaches when read, and the Windows rerun is `NEEDS-HOST`.

## Decision

- `internal/behaviorfalsify` (production, class P): `repositoryRelative` compared a plan path with
  `filepath.Clean`, which on Windows turns `tests/x.spec.ts` into `tests\x.spec.ts`, so every slash
  path failed with `must be repository-relative`. Plan paths are Git paths that every consumer
  already reads as slash text (`gitBlob` uses `ToSlash`, the provider package uses `FromSlash`), so
  the check now uses package `path` and refuses `\` and `:` on every host (BBF-V0-215). Chosen over
  normalising input with `ToSlash`: the path is part of the digested plan identity, so accepting two
  spellings of one file would let the same plan have two digests. Cost: Linux no longer admits a
  backslash inside a plan file name.
- `internal/testrunner/native` Boost.Test (test logic, class T): `buildBoostTest` joins the sink
  with `filepath.Join`, which is the right native path for a native executable. The test's
  `ReportDir: "/fresh"` has no volume on Windows, so the sink became `\fresh\boost-junit.xml`
  while the test expected `/fresh/boost-junit.xml`. Production is unchanged; the fixture now comes
  from a GOOS-parameterised `boostFixtureDir` (`C:\fresh` on Windows) and the expected argument from
  `boostWantSink`, both pinned for the Windows spelling on every host.
- Same pattern seen and left alone (outside the ticket): the cmocka and ginkgo tests expect
  `/fresh/...` too, and hook artifact paths in `execute.go`/`receipt.go` still use native cleaning.
