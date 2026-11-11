# Lane instfresh: install-lifecycle MCP code (V1-4942) and stale-binary review and audit (V1-5012)

Two P2 bugs on base `b55635262788687c3eea9a08a94378e4ab8856a4`.

## V1-4942: `mcp-unavailable-root` expects `invalid-root`

Not reproduced, and nothing was changed. On the base, on the Linux lane host,
`script/check-install-lifecycle.sh` (one build, run from inside the checkout) printed
`step mcp-unavailable-root: ok`, and `script/check-install-lifecycle_test.sh` printed
`check-install-lifecycle_test.sh: ok`. Run by hand, the step's `tools/call corvint.status` on a
non-repository root answered `"isError":true` with `"code":"invalid-root"`, and stderr printed
`corvint-mcp: repository unavailable: --root is not inside a Git worktree; serving tools/list, and
every repository tool call answers invalid-root`.

What the contract says (`docs/specs/mcp-server-2026-07-28-v0.md`):

- A root that cannot be resolved (here, `--root is not inside a Git worktree`) starts the server
  UNAVAILABLE, and every listed repository tool call MUST return an `isError` result with code
  `invalid-root`: `MCPV0-950`, lines 880-895, restated by `MCPV0-8300` at line 1109.
- `repository-unavailable` covers a Git read failure on a bound root: `MCPV0-038` at lines
  624-629, and the sanitized failure mapping at line 552.

So the script, its line-197 comment and the server all agree with `MCPV0-950`. The constant
`invalidRoot` in `internal/mcp/cli/unavailable.go` and the script's expectation are the same at
`83660e9c5^`, `83660e9c5` (the batch named in the ticket), `c78d71e77`, `4ce19d2a4` and `2230dd6f3`.
The 2026-11-07 failure cannot be explained from those files. The ticket stays open for the owner,
who has the original host.

## V1-5012: the review plan and `--audit-changes` freshness

- `corvint review --base` (RGV-V0-013, amended as proposed and experimental): the plan review
  embeds comes from this binary's selector. RGV-V0-013/015 require it to equal the default
  `corvint affected --base` document and the stdout of `corvint affected --base X --full`, and
  both of those refuse a stale binary (AFP-V0-9730). So review refuses rather than degrades. It
  exits 2 with `self-binary-stale` and a `fix:` line, and it offers no `--accept-stale-binary`,
  which review does not have. A `self-binary-unverified` binary still reviews: it gets the same
  degradation members the standalone receipt carries, and one `unknown` line.
- `corvint affected audit --changes` (AFP-V0-11033, amended as proposed and experimental): the
  report gains the additive `producer` member, the PUB-V0-027 identity of the binary that replayed
  the changes. The stale refusal was already in place under AFP-V0-11033. The audit has no text
  form, so no text line was added.

Evidence: `TestGuidanceReviewJudgesSelfBinaryFreshness_V15012` was committed failing first. The
stale review exited 0 with an undegraded plan. Removing the refusal, or the degradation, makes it
fail again. `TestAffectedAuditNamesItsPlanningBinary_V15012` fails when the assignment is removed.
`internal/capcatalog/catalog.json` was regenerated for the two help sentences.
