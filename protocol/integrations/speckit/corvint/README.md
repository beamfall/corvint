<!-- SPDX-License-Identifier: Apache-2.0 -->
# Corvint extension for Spec Kit (experimental)

This directory is a Spec Kit extension that asks Corvint for context before
`/speckit.implement` and for a review after it. It is the `STA-V0-008` package in
`docs/specs/spec-toolchain-adapters-v0.md`. Supported Spec Kit releases are only those listed in
`interop/spec-toolchains/versions.json` (`STA-V0-001`); the manifest range `>=1.2.1,<2` is an
install guard, not a support claim for every version in it.

## What it provides

| Name | Kind | Runs |
|---|---|---|
| `speckit.corvint.context` | command | `corvint context --task TEXT --subject PATH` for the feature's `spec.md` and `plan.md` |
| `speckit.corvint.review` | command | `corvint affected --base BASE` and `corvint review --base BASE` |
| `before_implement` | hook, `optional: true` | `speckit.corvint.context` |
| `after_implement` | hook, `optional: true` | `speckit.corvint.review` |

The manifest declares `effect: read-only`. Both commands only read and report Corvint's output,
including every `unknown` and `NOT_PRODUCED`. They never block the workflow, edit a file or commit.
Optional hooks are offered as prompts; the agent may skip them. A missing `corvint` binary or a
Corvint refusal is reported and the workflow continues.

`corvint review` reads a clean HEAD only. Right after `/speckit.implement` the worktree is usually
dirty, so the review command reports `corvint affected` for the working diff and reports the review
refusal as printed until the change is committed. It does not commit to make the review run.

## Install

Requires the `corvint` binary on `PATH` and Spec Kit 1.2.1.

```sh
specify extension add --dev /path/to/corvint/protocol/integrations/speckit/corvint
```

Remove it with `specify extension remove corvint`.

## Reading Spec Kit specs as OCM intent

Corvint reads a Spec Kit `spec.md` as declared intent with the explicit `speckit-spec` form
(`STA-V0-005`); `FR-NNN` and `SC-NNN` IDs are kept unchanged:

```sh
corvint ocm prepare --target "$TARGET" --expected-base "$BASE" \
  --intent specs/001-feature/spec.md --intent-form speckit-spec \
  --cem .corvint/change.cem.json --map .corvint/change.ocm.json
```

That form ships in a separate lane of the same spec; until it lands, the installed binary refuses
the form name.
