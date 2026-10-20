# 2026-10-19 V1-1997: corpus MCP accepts an absolute --artifact below --root

GitHub #1256. `corvint-corpus-mcp --root R --artifact /abs/path/under/R` exited 2 at startup with
`repository unavailable`; only a root-relative `--artifact` worked.

## Problem

`corpusbridge.New` reads the artifact through `doccorpus.ReadCorpusFile`, whose bounded reader
admits only canonical root-relative paths (`LocalInputPathRule` refuses a leading `/`). The bridge
maps every startup error to one exit message, so the absolute path surfaced as
`repository unavailable`.

## Decisions

- `DCP-V1-205`: the MCP command rewrites an absolute `--artifact` below `--root` to the equivalent
  root-relative path before `corpusbridge.New`. The bridge, the bounded reader and its path rules
  are unchanged, so both spellings open the same file and the session bytes are identical.
- `DCP-V1-206`: containment is lexical first, then on symlink-resolved root and artifact directory,
  so either side may be spelled through a symlinked ancestor (macOS `/var` -> `/private/var`).
- `DCP-V1-207`: an absolute path not strictly below `--root` exits 2 with
  `corvint-corpus-mcp: --artifact: absolute path is not below --root`. No new error code: this is
  stderr text of the command, not a typed corpus error.
- `corvint docs corpus` shares the bounded reader but already refuses an absolute `--artifact` with
  `corpus-refused` naming the flag (`DCP-V1-054`, decision 0520). That is an accepted contract and
  already names the argument, so it is left unchanged here.
- V1-1992 (broad `repository unavailable` mapping) is not fixed wholesale: relative paths and the
  other startup failures keep their message. The change narrows the mapping for one case only.

## Evidence

- `TestDCPV1205AbsoluteArtifactInsideRoot`, `TestDCPV1206AbsoluteArtifactThroughSymlinkedRoot`,
  `TestDCPV1207AbsoluteArtifactOutsideRootRefused` (cmd/corvint-corpus-mcp) pass; with
  `cmd/corvint-corpus-mcp/main.go` reverted to the base all three fail with
  `repository unavailable`.
- Artifact identity: no corpus bytes change; a relative `--artifact` session is byte-identical to
  the absolute spelling (asserted by the tests).

## NOT_RUN

- `make gate` (not required for scoped issue work).

## Rollback

Revert the commit; relative `--artifact` behavior is unchanged either way.
