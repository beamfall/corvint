// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testplatform"
)

// stepBlock returns the step of job block B whose name line starts with NAME, up to the next step.
func stepBlock(t *testing.T, b, name string) string {
	t.Helper()
	head := "      - name: " + name
	if n := strings.Count(b, head); n != 1 {
		t.Fatalf("want exactly one step named %q, found %d", name, n)
	}
	_, rest, _ := strings.Cut(b, head)
	if i := strings.Index(rest, "\n      - "); i >= 0 {
		rest = rest[:i+1]
	}
	return head + rest
}

// AFP-V0-420: only go-product-shard may leave the hosted image, and only for a merge-queue entry
// or a same-repository pull request while CORVINT_SELF_HOSTED is on; every other case, including
// a push to main and a fork pull request, falls back to ubuntu-24.04 with the same check names.
func TestAFPV0420ShardRunsOnKeepsTheCloudFallback(t *testing.T) {
	ci := readWorkflow(t, "ci.yml")
	const runsOn = "    runs-on: ${{ (vars.CORVINT_SELF_HOSTED == 'on' && (github.event_name == 'merge_group' || " +
		"(github.event_name == 'pull_request' && github.event.pull_request.head.repo.full_name == github.repository))) && " +
		`fromJSON('["self-hosted","linux","x64","corvint-box"]') || 'ubuntu-24.04' }}`
	wantLine(t, "go-product-shard", jobBlock(t, ci, "go-product-shard"), runsOn)
	for _, m := range regexp.MustCompile(`(?m)^    runs-on: (.*)$`).FindAllStringSubmatch(ci, -1) {
		if m[0] != runsOn && m[1] != "ubuntu-24.04" {
			t.Errorf("a job other than go-product-shard leaves the hosted image: %q", m[0])
		}
	}
	if n := strings.Count(ci, "vars.CORVINT_SELF_HOSTED"); n != 1 {
		t.Errorf("ci.yml reads vars.CORVINT_SELF_HOSTED %d times, want only the go-product-shard runs-on", n)
	}
	if strings.Contains(runsOn, "'push'") || strings.Contains(runsOn, "pull_request_target") {
		t.Error("the self-hosted routing must not admit a push or pull_request_target run")
	}
	// The required checks match by name: the matrix job keeps its name and go-product still needs it.
	if !strings.Contains(regexp.MustCompile(`(?m)^    needs: .*$`).FindString(jobBlock(t, ci, "go-product")), "go-product-shard") {
		t.Error("go-product must still need go-product-shard")
	}
	if strings.Contains(jobBlock(t, ci, "go-product-shard"), "\n    name: ") {
		t.Error("go-product-shard must not gain a job name, which would rename its checks")
	}
}

// AFP-V0-421: the hosted provisioning step is unchanged and hosted-only; the self-hosted runner
// provisions nothing with sudo or the network and only verifies the pinned Go and ripgrep, keeps
// the same Git configuration, and never shares a workspace or Go build cache across jobs.
func TestAFPV0421SelfHostedShardsVerifyAndNeverProvision(t *testing.T) {
	shard := jobBlock(t, readWorkflow(t, "ci.yml"), "go-product-shard")
	hosted := stepBlock(t, shard, "Provision the host tools the product's fixed PATH resolves\n")
	verify := stepBlock(t, shard, "Verify the host tools the product's fixed PATH resolves (self-hosted runner, AFP-V0-421)")
	wantLine(t, "hosted provisioning", hosted, "        if: runner.environment != 'self-hosted'")
	wantLine(t, "self-hosted verification", verify, "        if: runner.environment == 'self-hosted'")
	hostedScript, verifyScript := runScript(t, hosted), runScript(t, verify)
	for _, line := range []string{
		`sudo rm -rf /usr/local/go`,
		`sudo ln -s "$(go env GOROOT)" /usr/local/go`,
		`echo "$RG_SHA256  $RUNNER_TEMP/$rg_name.tar.gz" | sha256sum --check --strict`,
		`sudo install -m 0755 "$RUNNER_TEMP/$rg_name/rg" /usr/local/bin/rg`,
	} {
		if !strings.Contains(hostedScript, line+"\n") {
			t.Errorf("hosted provisioning lost %q", line)
		}
	}
	if strings.Count(shard, "sudo ") != strings.Count(hostedScript, "sudo ") {
		t.Error("go-product-shard runs sudo outside the hosted-only provisioning step")
	}
	for _, word := range []string{"sudo", "curl", "install -m", "apt"} {
		if strings.Contains(verifyScript, word) {
			t.Errorf("self-hosted verification must provision nothing, but runs %q", word)
		}
	}
	one := func(name, text, pattern string) string {
		t.Helper()
		m := regexp.MustCompile(pattern).FindAllStringSubmatch(text, -1)
		if len(m) != 1 {
			t.Fatalf("%s: %d matches of %q, want 1", name, len(m), pattern)
		}
		return m[0][1]
	}
	if h, v := one("hosted", hosted, `(?m)^          RG_VERSION: "(.+)"$`), one("verify", verify, `(?m)^          RG_VERSION: "(.+)"$`); h != v {
		t.Errorf("self-hosted ripgrep pin %s differs from the hosted pin %s", v, h)
	}
	goPin := "go" + one("setup-go", shard, `(?m)^          go-version: "(.+)"$`)
	for name, script := range map[string]string{"hosted": hostedScript, "verify": verifyScript} {
		if !strings.Contains(script, strconv.Quote(goPin)) {
			t.Errorf("%s step does not check /usr/local/go against the pinned %s", name, goPin)
		}
	}
	if !strings.Contains(verifyScript, "::error::self-hosted host prerequisite: /usr/local/go/bin/go must be") ||
		!strings.Contains(verifyScript, "::error::self-hosted host prerequisite: /usr/local/bin/rg must be ripgrep") {
		t.Error("a failed verification must name the missing host prerequisite")
	}
	_, hostedGit, _ := strings.Cut(hostedScript, "git config --global user.name")
	_, verifyGit, _ := strings.Cut(verifyScript, "git config --global user.name")
	if hostedGit == "" || hostedGit != verifyGit {
		t.Errorf("the self-hosted Git configuration differs from the hosted one:\n%s\nwant\n%s", verifyGit, hostedGit)
	}
	if !strings.Contains(verifyScript, `printf 'GOENV=off\nGOCACHE=%s\n' "$RUNNER_TEMP/go-build" >> "$GITHUB_ENV"`) {
		t.Error("the self-hosted shard must build in a per-job cache and ignore the host go.env")
	}
	clear := stepBlock(t, shard, "Clear the persistent workspace (self-hosted runner only, AFP-V0-421)")
	wantLine(t, "workspace clear", clear, "        if: runner.environment == 'self-hosted'")
	if !strings.HasPrefix(strings.TrimPrefix(shard[strings.Index(shard, "    steps:\n"):], "    steps:\n"), "      - name: Clear the persistent workspace") {
		t.Error("the workspace clear must be the shard's first step")
	}
	restore := stepBlock(t, shard, "Restore the trusted Go build cache")
	if !strings.Contains(restore, "        if: runner.environment != 'self-hosted' && (") {
		t.Error("the self-hosted runner must never restore the shared Go build cache")
	}
}

// AFP-V0-422: a self-hosted shard keeps a marker instead of its timing stream, and the drift
// report abstains with the runner as its reason rather than comparing box timings with the table.
func TestAFPV0422SelfHostedShardsMakeTheDriftReportAbstain(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the drift step runs under bash")
	ci := readWorkflow(t, "ci.yml")
	shard := jobBlock(t, ci, "go-product-shard")
	wantLine(t, "stream retention", stepBlock(t, shard, "Retain package outcomes for the advisory cost-drift report"),
		"        if: steps.tests.outputs.costs == '1' && runner.environment != 'self-hosted'")
	marker := stepBlock(t, shard, "Mark a self-hosted shard for the advisory cost-drift report (AFP-V0-422)")
	wantLine(t, "marker", marker, "        if: steps.tests.outputs.costs == '1' && runner.environment == 'self-hosted'")
	wantLine(t, "marker", marker, "        continue-on-error: true")
	wantLine(t, "marker upload", stepBlock(t, shard, "Retain the self-hosted marker for the advisory cost-drift report (AFP-V0-422)"),
		"          name: ci-shard-costs-runner-${{ strategy.job-index }}")
	drift := jobBlock(t, ci, "ci-shard-cost-drift")
	wantLine(t, "drift download", drift, "          pattern: ci-shard-costs-*")
	script := runScript(t, stepBlock(t, drift, "Require the outcomes of every shard"))
	run := func(markers int) (string, string) {
		t.Helper()
		dir := t.TempDir()
		costs := filepath.Join(dir, "shard-costs")
		if err := os.MkdirAll(costs, 0o700); err != nil {
			t.Fatal(err)
		}
		for i := range 8 {
			if err := os.WriteFile(filepath.Join(costs, "shard-"+strconv.Itoa(i)+".json"), []byte("{}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		for i := range markers {
			if err := os.WriteFile(filepath.Join(costs, "shard-"+strconv.Itoa(i)+".self-hosted"), []byte("self-hosted\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		output, summary := filepath.Join(dir, "output"), filepath.Join(dir, "summary")
		cmd := exec.Command("bash", "-eo", "pipefail", "-c", script)
		cmd.Env = append(os.Environ(), "RUNNER_TEMP="+dir, "SHARDS=8", "SHARDS_RESULT=success",
			"GITHUB_OUTPUT="+output, "GITHUB_STEP_SUMMARY="+summary)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("the outcome check failed: %v\n%s", err, out)
		}
		o, _ := os.ReadFile(output)
		s, _ := os.ReadFile(summary)
		return string(o), string(s)
	}
	if out, summary := run(0); out != "ok=1\n" || summary != "" {
		t.Errorf("eight hosted streams must report: output %q, summary %q", out, summary)
	}
	// Even with every hosted-shaped stream present, one self-hosted shard abstains.
	out, summary := run(1)
	if strings.Contains(out, "ok=1") || !strings.Contains(summary, "Abstained: 1 of 8 shards ran on the self-hosted runner") {
		t.Errorf("a self-hosted shard must abstain: output %q, summary %q", out, summary)
	}
}
