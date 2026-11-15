# Impact reads a directory operand as its admitted files (V1-5436)

Ticket V1-5436 in `queue:corvint:main`: `corvint impact internal/pairing` refused with
`unsupported-impact-path-suffix` on corvint 1.0.0-rc.3.dev build 189.

## Decision and behavior

- A `corvint impact` operand with no admitted suffix that is a directory under the root is replaced
  by the indexed files directly inside it whose suffix the index admits. The receipt is
  byte-identical to naming those files, alone or mixed with file operands (`GPK-V0-883`).
- Direct children only. Recursion would turn a source-tree operand into more paths than the bound
  allows and would stop one operand meaning one package.
- A directory with no admitted file directly inside refuses with the new code
  `unsupported-impact-directory`, naming the directory; it is never dropped from a mixed list.
  An unadmitted file keeps `unsupported-impact-path-suffix` unchanged.
- This supersedes the directory clause of `GPK-V0-880`, which left directories out because an
  aggregate would need new receipt bytes. Expansion adds none, so no parity-manifest case changes.
- `impact --capabilities` gains a `directories` member; help and `docs/CAPABILITIES.md` state the rule.

## Limits

- `batch` impact rows, `harness event` `file-change` and MCP `corvint.impact` still refuse a
  directory by suffix.
- A directory whose own name carries an admitted suffix (for example `site.js/`) is not detected.
- The directory test reads the working tree; membership comes from the index.
