<!-- SPDX-License-Identifier: Apache-2.0 -->
# ghci scheduled invocation evidence repair

V1-5098 / CIN-V0-8720: the supplied P1 replay showed that package-set equality
could forgive an invocation that emitted no JSON. The existing outcome reducer
prefers fail over missing, so a listed failure hid a second silent slice of the
same package even when both diagnostic reruns succeeded.

Authorization now consumes the workflow's existing expected manifest and requires
exact equality between scheduled counts and complete observed lifecycles for every
package. It rejects unscheduled packages and excess invocations as well as missing
slices and entirely absent packages. The class is incomplete schedule coverage;
the schedule is never inferred from the potentially incomplete stream.

The closed manifest allowlist is one ASCII import path per LF-terminated row,
with components drawn from letters, digits, underscore, dot and hyphen, at least
one slash, and no dot or dot-dot components. Repeated rows count as separate
invocations. Empty, missing, malformed and unknown forms refuse, as do inputs over
1 MiB or 4096 rows. The existing closed event-transition/output allowlists still
apply before an invocation is counted. No outcome JSON, error/exit code, stderr,
golden, core-freeze output or existing assertion was weakened or changed.

The actual workflow-block regression executes outcomes.sh/outcomes.jq and flaky.sh:
one listed failure plus a silent failed invocation yields package fail, two
successful diagnostic reruns retain FLAKY, and authorization keeps shard failure.
Two complete listed failures with those reruns still authorize. Unit regressions
cover an entirely absent package, excess/unscheduled invocations, unknown manifest
JSON, malformed rows and both caps. The Go workflow pin additionally requires the
expected-manifest argument before status can clear.

Verification against base 6339f980e13e36c8ff51a593568da3d935ffae77:
corvint affected selected both CI helper packages directly and retained UNKNOWN
with 75 LANGUAGE_FRONTIER and one NO_SELECTABLE_TEST rows. Both existing whole
packages passed through script/heavy.sh: ci-forward-row 40.605 s and ci-reuse-plan
13.369 s. All 11 known-flakes and four go-product Python tests passed. The permitted
spec-requirements-check and line-citations-check passed. Spec generation changed
no derived files; repin found no moved unchanged citations. Go version is 1.27.3.
Identity was scoped to commits/binding; no heavy lock override was used.

Self-development routes used: original workflow/evidence inspection, affected
selection and final CEM binding/report inspection. Pre-change recapture is excluded
by the repair charter. Broad selection, repository gates, live hosted qualification,
Darwin qualification and model-backed review are NOT_RUN; the orchestrator owns
review and host checks. Windows vet is NOT_RUN under the focused repair scope;
no push occurs. No Tasks writes, publication or seal. The cited report remains
local self-use evidence, not external qualification or native ticket completion.

Raw evidence is retained outside the repository in ~/lanes/ghci-evidence.

| Evidence | SHA-256 |
|---|---|
| invocations-affected.log | `fea76172f99b6db0eac4bcc20f8f483ec6ced4318a1ee1ccc8f54084a9b2e016` |
| invocations-ci-forward-row.log | `e2e63575f8cf5779a09f07cf4a6ff5b6dec4755169591abac5044e1fe226169c` |
| invocations-ci-reuse-plan.log | `3cfa58b7b6f7330552049aace106b6c1b90a874f253b7c1b268f09021e27b6de` |
| invocations-known-flakes.log | `5cabd4adde32c068c8e45ce96da08a80ccabd00cccbe75d537b8e4afe2037c62` |
| invocations-go-product.log | `04a3ba0eb1deb1cc4046427a4a131abbb0286628007dd92f3e9baa0c6cdcb807` |
| invocations-spec-check.log | `c393efe91e9bb4f976d71a64279ae6e8db695fda66d8ab243588b6bf9f4b7974` |
