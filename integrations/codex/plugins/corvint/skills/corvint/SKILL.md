---
name: corvint
description: Use Corvint for impact analysis of a change or path, with explicit uncertainty. Trigger for planning or reviewing a change, investigating likely side effects, or selecting relevant tests.
---

# Corvint for Codex

Use the installed `corvint` command on `PATH` against the current Git repository.

When developing Corvint itself, or when the current repository explicitly opts into its own
`docs/SELF-DEVELOPMENT.md`, first check that the guide exists. If absent (including older Corvint
checkouts), retain repository policy and the general routes below. If present, read it and select only routes
applicable to this task. Preserve actual outputs and reasons for unavailable or inapplicable
features. Do not look for or impose this Corvint-specific guide in other repositories; their existing
policies and the general routes below remain operative. This is feature-use guidance, not formal
FULL support, automatic feature fanout or new execution authority.

1. Before broad search, run the narrowest applicable command:
   - To ask what a clean committed change or range affects against the captured `HEAD`, resolve the
     excluded lower boundary to its full immutable commit ID. For one commit at `HEAD`, the base is
     that commit's explicitly chosen parent (normally its first parent), not the changed commit;
     for a multi-commit range, it is the lower endpoint immediately before the included changes.
     Prefer
     `corvint impact --base FULL_COMMIT_ID --limit 20` on macOS or Linux when `corvint` is
     available. Do not use this profile for a dirty worktree or a range that does not end at `HEAD`.
   - For tracked nested `.go` file questions on macOS or Linux in a slash-qualified Go module,
     retain positional path mode: `corvint impact PATH... --limit 10` when `corvint` is available.
     The same path and range modes admit Ruby (`.rb`), JavaScript (`.js .jsx .mjs .cjs`) and
     TypeScript (`.ts .tsx`) under the experimental `non-go-syntax-v0` profile, which always
     reports dynamic-dispatch unknowns; preserve them.
   - Use `corvint impact --working-tree-untracked PATH... --limit 10` only when the requested
     targets are explicitly identified as untracked `.go` files. Never use it as a substitute for
     committed-range or tracked-path impact.
   - For an explicit feature ID, use `corvint feature FEATURE_ID --limit 1` on macOS or Linux
     for known ledger-only context, including repositories with non-Go sources. A larger limit
     admits Go candidate ranking; known non-Go ranking remains unsupported. Preserve unknown-ID
     OUT_OF_SCOPE results; feature inventory remains unsupported.
   - Other suffixes the repository index admits (for example `.py`, `.md`) get positional
     evidence with missing-rule uncertainty, not reverse-import evidence; preserve that uncertainty.
   - For unsupported targets or profiles, including path suffixes the index does not admit
     (`unsupported-impact-path-suffix`), preserve the explicit capability gap and use
     repository-owned authority/context tools. Do not approximate a Corvint receipt.
   - To learn which Corvint command or binary answers a question, run `corvint help`
     and `corvint COMMAND --help`, or read the catalog at
     https://github.com/beamfall/corvint/blob/main/docs/CAPABILITIES.md (maturity and refusals).
2. Treat source selectors, blob identities, and authority labels as evidence. Treat uncertainty,
   omissions, stale-index state, and explicit gaps as part of the answer. In particular, non-Go
   range omissions, `impact-range-drift`, `BUDGETED`, and any reported uncertainty keep the impact
   frontier open: preserve them, widen or re-run as indicated, and do not present the receipt as
   complete proof of impact or verification coverage.
3. Expand exact cited paths only when the packet says widening is required. Never call Corvint output
   proof of behavior unless it names an independently verified behavioral witness.
4. For a material diff, follow the repository's CEM/OCM policy. Do not fabricate citations for an
   edit the agent did not base on evidence.
