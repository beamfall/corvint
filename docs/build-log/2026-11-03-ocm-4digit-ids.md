<!-- SPDX-License-Identifier: Apache-2.0 -->
# V1-4347: OCM reads four-digit requirement IDs

Base: `origin/main` `030c7b57`. Branch `claude/cloud-ocm-4digit`. Requirement: `OCM-V0-952`.

## Problem

Lanes now allocate four-digit requirement-ID blocks, and `script/gen-spec-requirements.sh` lists them
in `docs/specs/REQUIREMENTS.tsv` (its definition grammar is `[A-Z][A-Z0-9-]*-[0-9]{3,}`). The OCM
intent parser still compiled `^[A-Z][A-Z0-9-]{2,31}-[0-9]{3}$`, so it skipped every four-digit line,
and a spec holding only such IDs was refused `missing-requirements`. The win-w59 lane seal
(2026-11-02) hit this and had to bind a different intent spec.

## Decision

`OCM-V0-952` widens the number to `[0-9]{3,}` and keeps the `{2,31}` prefix bound. The widened
grammar admits only lines the old one refused. Every three-digit result, refusal code and bound
stays as it was, and the frozen `conformance/ocm-v0` bytes hold no four-digit ID. The change
covers each consumer of an OCM obligation ID, so a four-digit intent is not refused one step later:

| Pattern | Changed | Reason |
| --- | --- | --- |
| `internal/lrfrepo/ocm.go` `requirementID` | yes | the intent parser in this ticket |
| `internal/lrf/terms.go` `obligationIDPattern` | yes | `ValidateStableRequest` checks every OCM obligation; left at three digits, it would refuse a four-digit intent as `obligation projection is invalid` |
| `internal/dogfoodflow/ocm_coverage.go` `requirementLine` | yes | the coverage reader mirrors `OCM-V0-001`'s line syntax and would otherwise report a four-digit intent as having no requirements |
| `internal/console/chain.go` `requirementLine` | yes | the clause display mirrors the same syntax |
| `internal/cem/verify/support.go` `supportRequirementID` | no | the frozen lexical-relevance-floor term-scan grammar (decision 0265), which the portable CEM consumer mirrors byte for byte; changing it changes CEM verdict terms |
| `internal/dogfoodflow/ocm_coverage.go` `requirementRange` | no | the `OCM-V0-900`/`OCM-V0-950` range-tracing form; four-digit ranges are a separate amendment (follow-up) |
| `internal/dogfoodflow/propose*.go` (`requirementToken`, `requirementDefinition`, `changeRequirementID`) | no | the dogfood proposal heuristics, outside the OCM admission path; widening them changes proposal ranking (follow-up) |
| `internal/contextindex/taskcontext.go` (`contextClauseLine`, `contextRequirementID`) | no | these mirror `script/check-requirement-definitions.sh`, which is still three digits; changing only one side would make them drift (follow-up) |
| `internal/capcatalog/guide.go` `requirementPattern` | no | capability-catalog cross-references, outside OCM |
| `internal/lrfrepo/ocm_toolchain_forms.go` spec-kit patterns | no | already `[0-9]{3,}` |

## Evidence

The `TestOCMV0952*` tests in `internal/lrfrepo`, `internal/lrf`, `internal/dogfoodflow` and
`internal/console` pass, and so do the existing OCM tests of `internal/lrfrepo` (see the PR). Dogfood
binding (`make dogfood-change`) was `NOT_RUN` in this cloud session, because the installed
`corvint` binary and the local Tasks runtime were unavailable. `make gate` was `NOT_RUN`, as the
owner asked.

## Rollback

Restore `[0-9]{3}` in the four changed patterns and drop `OCM-V0-952`.
