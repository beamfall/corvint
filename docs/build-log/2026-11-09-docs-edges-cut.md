# Docs edges cut: `corvint-docs` binary and the docs-tool envelope (V1-4907)

- Date: 2026-11-09
- Decision: 0594 item 3 (documentation stays as a product behind a separate `corvint-docs` binary;
  core packages do not import documentation packages), item 8 (layers, enforced import direction).
- Spec: `docs/specs/core-bundled-apps-v0.md` (`CMS-V0-003`, `CMS-V0-004`, `CMS-V0-009`).

## What changed

- `cmd/corvint-docs` is the documentation app. It takes the argument vectors `corvint docs ...`
  and `corvint --corpus=FILE ...` take, byte for byte, plus `help`, `help --json` and the
  protocol verb `docs-tool --root ROOT TOOL`. The docs command files moved from `cmd/corvint`
  with their tests; the CLI helpers they shared with the core (argument errors, the stderr
  envelope, root resolution, bounded stdin) are copied into `cmd/corvint-docs/shared.go`
  because the core's command package is not a library.
- `corvint docs` and a `--corpus=FILE`/`--corpus-depth=N` read are exec-only shims
  (`cmd/corvint/docs_shim.go`): same argv, stdin, stdout, stderr and exit status, no added flags
  or environment. A missing executable is code `not-installed`, a named `NOT_INSTALLED` gap.
  `corvint help docs` is the shim's page; the subcommands are catalogued by
  `corvint-docs help --json` (`internal/capcatalog/trees/corvint-docs.json`).
- The `--corpus=FILE` join inside `corvint-docs` runs the native read in the `corvint` executable
  beside it or on PATH instead of in-process, so the app is an untrusted caller of the core's
  command line.
- `internal/docsenvelope` (kernel) is the published envelope: binary names, `Locate`, the
  `docs-tool` verb and its `corvint-docs-tool/1` result. `internal/mcp/docsbridge` keeps its
  descriptors, bounds, root binding and four-way result, and runs the verb per call;
  `internal/docstools` is the native implementation it used to be. corvint-mcp's wire bytes
  are unchanged.
- The import-ratchet area manifest places `docsenvelope` and `mcp/rootbind` in the kernel and
  `docstools` with the documentation packages.

## Evidence

- `cmd/corvint-docs`, `internal/docstools`, `internal/mcp/docsbridge`, `cmd/corvint-docs-mcp`,
  `internal/mcp/cli` (docs groups), `tools/capabilities-check`, `internal/capcatalog` and the
  focused `cmd/corvint` help, catalog, knob, read-only and shim tests pass; `go vet` and the
  Windows vet twin pass; `tools/capabilities-check` reports ok.
- Test packages that reach the docs tools build `corvint-docs` from source onto PATH in
  `TestMain` (`testsupport.BuildOnPATH`); the corpus join tests build `corvint`.

## Behaviour notes

- `corvint help docs corpus` and `corvint docs corpus OP --help` print the shim's page; the
  authored docs help is `corvint-docs help docs corpus [OP]`. The agent guide's docs examples
  name `corvint-docs`.
- `docs draft` and `docs consume` still best-effort append `unsupported-*` failures to
  `.corvint/self-observations.jsonl` (SOL-V0-007) from the app; `CMS-V0-001`'s write ratchet,
  when it lands, must either admit that ledger or move it.
- Still open for V1-4901 (module split): `internal/mcp/toolgroup` and `internal/mcp/cli` import
  `doccorpus` and `corpusbridge` for the corpus tool profile; `cmd/corvint` tests import
  `doccorpus` through `appflows` fixtures.
