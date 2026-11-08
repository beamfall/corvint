# Avgo self-review repair: exact conjunctions and source-backed paths

Owner intent: repair only the three P1 self-review findings supplied on 2026-11-08.
The existing V1-5000 contracts AFP-V0-9050 and AFP-V0-9051 govern the repair;
AFP-V0-3128 supplies the operand-bounded certification rule. The active spec entry
remains `affected-zero-unknown-v0.md`. No wire shape, diagnostic, exit code, golden
or existing assertion changes.

## Decisions and class closure

- AFP-V0-9050: an empty unexpanded `and` argument is false, including ASCII spaces
  and tabs. Return empty immediately; only `or` skips it. GNU Make dry runs for
  all three spellings produce `go test ./b` in the reported conditional recipe.
  Unknown function forms continue to retain both branches.
- AFP-V0-9051: a glob's containing-directory edge does not prove its expanded
  read set. Admit only `literal-dir/*literal-suffix` without parent traversal,
  with a plain containing path, a successful listing of at most 4096 entries,
  and regular files for every matched non-hidden entry. Links, matched
  directories, unfamiliar types, failed listings and other syntax retain the
  frontier. This also keeps future added descendants under the directory edge.
- AFP-V0-9051: Go package certification admits only `all`, `std`, `cmd`, relative
  directory patterns with optional trailing `/...`, and main-module directory
  patterns. Local roots, implicit cwd and separate/attached `-C` directories
  must be source-proven plain directories. Missing paths, links, parent traversal,
  unsupported characters and unknown import forms retain the frontier.
  Shell directory changes preserve uncertainty before cleaning can erase links.
  Decision: retain linked invocations instead of pretending lexical aliases are
  resolved package edges. This satisfies the existing no-link certification
  contract and selects both the Make target and its dependent CI job.

## Acceptance evidence and limits

`TestMakeAndEmptyArguments_AFP_V0_9050` covers the reported empty argument, spaces,
tabs, first/last empty arguments and an unknown function with both possible branches.
`TestDriverSourcePathCertification_AFP_V0_9051` covers matched symlinks, escaping
and missing targets, unsupported glob syntax, matched directories, linked package
roots, recursive patterns, both `-C` forms, implicit packages, `cd alias/..`, and
unknown package/directory forms. Plain-file and plain-directory controls retain
precise selection. These targeted regressions passed through `script/heavy.sh`.

Self-use: the requested affected plan selected `internal/liveverify/affected`;
its UNKNOWN scope and stale-installed-binary degradation are retained. No savings
or candidate selection accuracy is inferred from that plan. Existing pre-change
capture is reused; no change-start capture is rerun. Final CEM binding/checking is
attempted once after the final edits and commit; the external handback retains
its exact disposition, including any sandbox refusal. Independent review and
host qualification remain with the orchestrator. Learning, mutation, external
providers, publication and Tasks writes are outside this repair's scope.

## Rollback

Revert this repair's source, tests and spec clarification together. That restores
all three known selection gaps; it does not authorize changing frozen outputs or
weakening their assertions. No ticket completion or release qualification is claimed.

## Retained verification

Final whole-package `GOTOOLCHAIN=local go test -count=1 -timeout 30m
./internal/liveverify/affected`, run through `script/heavy.sh`, passed in 186.340s.
The prior package run passed in 171.824s; final focused regressions passed in 0.065s.
`make -s spec-requirements-check line-citations-check` and `make changed-checks
BASE=a7d38e73236cea3a237ce972f3bd50e11e723c29` passed. The citation check retains its
180 untracked-basename exclusions. Final repinning found no moved citation.
Go 1.27.3 was verified. No Darwin execution evidence or whole-repository gate is claimed.

Raw evidence remains outside the repository in `/home/agent/lanes/avgo-evidence/`.

| Artifact | SHA-256 |
| --- | --- |
| `self-review-package-final.log` | `df3779e6b2a265e66251eed6075b2aef3b8d61a43edf804d84d8adf9886bde43` |
| `self-review-regressions-final.log` | `e15a91fe7fd69dda2b70438fd0470d0fc47d9fc99b9c518ef9bc7dee1ae6d6e8` |
| `self-review-spec-check.log` | `32acbe96b2890d90f430832e62c9c466ad377a65daa56368eaffe47b9ad0e80e` |
| `self-review-changed-checks.log` | `175b6954cc339fa76f6049aec97d899edb23fa35a5256718071b628a77c78e4b` |
| `self-review-repin-final.log` | `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133` |
| `self-review-line-final.log` | `855a4551b199f407e4e7cec705511746022a39aa0f70383fc13aac9acc748bba` |
| `self-review-affected.json` | `d0a3c7a967f753a982dcdc266518afe0f14213c1d1c57c9879a24c0d12922dd0` |
| `self-review-affected.stderr` | `688d15831531a430ee6f67865ee6ee59851297e95e9430373304565841e0b71a` |
