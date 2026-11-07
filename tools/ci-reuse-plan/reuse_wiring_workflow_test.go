// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

// TestAFPV02327ReuseWiring: the test-reuse-plan job runs only for a pull request while
// the execution-evidence hold is removed and CORVINT_PR_REUSE is on, builds its decider from the tested merge's first parent, and admits only
// this pull request's records; a reused run never writes a tested-tree or narrowed-tree record, and
// a shard records reuse evidence only with the trusted main-built corvint.
func TestAFPV02327ReuseWiring(t *testing.T) {
	ci := readWorkflow(t, "ci.yml")
	plan := jobBlock(t, ci, "test-reuse-plan")
	wantLine(t, "test-reuse-plan", plan, "    if: false && github.event_name == 'pull_request' && vars.CORVINT_PR_REUSE == 'on' && !contains(github.event.pull_request.labels.*.name, 'ci:batched')")
	wantLine(t, "test-reuse-plan", plan, "        continue-on-error: true")
	for _, want := range []string{
		`base=$(git rev-parse --verify -q "$GITHUB_SHA^1")`,
		`git worktree add -q --detach "$RUNNER_TEMP/reuse-tool" "$base"`,
		`--trust main,pull_request --pull-request "$PR_NUMBER"`,
		`git merge-base --is-ancestor "$sha" "$base"`,
		"persist-credentials: false",
	} {
		if !strings.Contains(plan, want) {
			t.Errorf("test-reuse-plan lacks %q", want)
		}
	}
	// Negative control: a repository variable alone cannot admit writable PR evidence.
	if !strings.Contains(plan, "UNAUTHENTICATED_EXECUTION") {
		t.Fatal("CI reuse must name its missing protected execution collector")
	}

	for _, job := range []string{"ci-tested-tree-record", NarrowedRecordJob} {
		b := jobBlock(t, ci, job)
		if !strings.Contains(b, "test-reuse-plan]") || !strings.Contains(b, "needs.test-reuse-plan.outputs.mode != 'REUSE'") {
			t.Errorf("%s must need test-reuse-plan and refuse a reused run", job)
		}
	}
	shard := jobBlock(t, ci, "go-product-shard")
	wantLine(t, "go-product-shard", shard, "    needs: [docs-plan, narrowing-verdict, test-reuse-plan]")
	for _, want := range []string{
		`if [ "$GITHUB_EVENT_NAME" = pull_request ] && [ "$REUSE_MODE" = REUSE ] && [ -z "$narrowed" ]; then`,
		`.head == $head' "$RUNNER_TEMP/ci-test-reuse/decision.json"`,
		`"$RUNNER_TEMP/corvint-pr-bin/corvint" test reuse record --outcomes "$outcomes"`,
		"        if: false && always() && vars.CORVINT_PR_REUSE == 'on' && (github.event_name == 'pull_request' || github.event_name == 'merge_group') && steps.pin-tool-build.outcome == 'success'",
	} {
		if !strings.Contains(shard, want) {
			t.Errorf("go-product-shard lacks %q", want)
		}
	}
}

// TestAFPV02344JobPlanWiring: the job plan runs only for a pull request while CORVINT_PR_JOB_PLAN
// is on, builds corvint from the first parent, and turns into SKIP only an exact SKIP decision of a
// plan of this base.
func TestAFPV02344JobPlanWiring(t *testing.T) {
	ci := readWorkflow(t, "ci.yml")
	plan := jobBlock(t, ci, "job-plan")
	wantLine(t, "job-plan", plan, "    if: github.event_name == 'pull_request' && vars.CORVINT_PR_JOB_PLAN == 'on'")
	wantLine(t, "job-plan", plan, "        continue-on-error: true")
	for _, want := range []string{
		`git worktree add -q --detach "$RUNNER_TEMP/job-plan-tool" "$base"`,
		`affected jobs --base "$base"`,
		`.ok == true and .base == $base`,
		`.[0].decision == "SKIP" and .[0].reason == "NO_DECLARED_INPUT_CHANGED" then "SKIP" else "RUN" end`,
	} {
		if !strings.Contains(plan, want) {
			t.Errorf("job-plan lacks %q", want)
		}
	}
	if strings.Contains(plan, "merge_group") || strings.Contains(plan, "permissions:") {
		t.Error("job-plan must stay pull-request only with the workflow's read-only permissions")
	}
}
