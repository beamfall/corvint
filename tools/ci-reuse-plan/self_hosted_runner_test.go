// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
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
		"(github.event_name == 'pull_request' && github.event.pull_request.head.repo.full_name == github.repository)) && " +
		"!(needs.docs-plan.result == 'success' && (needs.docs-plan.outputs.mode == 'DOCS' || needs.docs-plan.outputs.mode == 'REUSE'))) && " +
		`fromJSON('["self-hosted","linux","x64","corvint-box"]') || 'ubuntu-24.04' }}`
	wantLine(t, "go-product-shard", jobBlock(t, ci, "go-product-shard"), runsOn)
	// AFP-V0-462: the hosted route for DOCS and REUSE needs the docs-plan job that decides them,
	// and it routes exactly the modes in which the race invocation never runs on either runner.
	if !strings.Contains(regexp.MustCompile(`(?m)^    needs: .*$`).FindString(jobBlock(t, ci, "go-product-shard")), "docs-plan") {
		t.Error("go-product-shard must need docs-plan, whose mode routes DOCS and REUSE to the hosted image")
	}
	tests := stepBlock(t, jobBlock(t, ci, "go-product-shard"), "Go tests (full until matching shadow qualification)\n")
	wantLine(t, "Go tests", tests, "        if: needs.docs-plan.result != 'success' || (needs.docs-plan.outputs.mode != 'DOCS' && needs.docs-plan.outputs.mode != 'REUSE')")
	// Other jobs stay on GitHub-hosted images; cce-live runs on hosted macos-15 (V1-2580).
	hosted := map[string]bool{"ubuntu-24.04": true, "macos-15": true}
	for _, m := range regexp.MustCompile(`(?m)^    runs-on: (.*)$`).FindAllStringSubmatch(ci, -1) {
		if m[0] != runsOn && !hosted[m[1]] {
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
	hostedGit = "git config --global user.name" + hostedGit
	if verifyGit := selfHostedGitLines(t, verifyScript); verifyGit != hostedGit {
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

const gitGlobalOpen = "git_global=\"$tmp_root/$RUNNER_NAME.gitconfig\"\nrm -f -- \"$git_global\"\nprintf 'GIT_CONFIG_GLOBAL=%s\\n' \"$git_global\" >> \"$GITHUB_ENV\"\nexport GIT_CONFIG_GLOBAL=\"$git_global\"\n"

// selfHostedGitLines returns the self-hosted verification script's Git lines, which follow the
// per-runner global file's setup, in the hosted step's form.
func selfHostedGitLines(t *testing.T, script string) string {
	t.Helper()
	if strings.Count(script, gitGlobalOpen) != 1 {
		t.Fatal("the self-hosted Git configuration must write one per-runner global file (AFP-V0-421)")
	}
	_, rest, _ := strings.Cut(script, gitGlobalOpen)
	if !strings.HasPrefix(rest, "git config --global user.name") {
		t.Fatalf("the per-runner global file must hold the Git configuration lines, holds:\n%s", rest)
	}
	return rest
}

// AFP-V0-421 (V1-2634): concurrent runner instances of one account each write their own global
// Git file and never replace the account's ~/.gitconfig, which other jobs' confined tests read.
func TestAFPV0421SelfHostedGitConfigIsPerRunner(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the verification step runs under bash")
	verify := runScript(t, stepBlock(t, jobBlock(t, readWorkflow(t, "ci.yml"), "go-product-shard"),
		"Verify the host tools the product's fixed PATH resolves (self-hosted runner, AFP-V0-421)"))
	_, block, _ := strings.Cut(verify, gitGlobalOpen)
	block, _, _ = strings.Cut(block, "git --version\n")
	block = gitGlobalOpen + block
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	const runners = 8
	failures := make(chan string, runners)
	for i := 0; i < runners; i++ {
		go func() {
			cmd := exec.Command("bash", "--noprofile", "--norc", "-e", "-c", "tmp_root="+dir+"\n"+block)
			cmd.Env = append(os.Environ(), "HOME="+home, "RUNNER_NAME=corvint-box-"+strconv.Itoa(i), "GITHUB_ENV="+filepath.Join(dir, "env-"+strconv.Itoa(i)), "GIT_CONFIG_NOSYSTEM=1")
			out, err := cmd.CombinedOutput()
			if err == nil {
				out = nil
			} else if len(out) == 0 {
				out = []byte(err.Error())
			}
			failures <- string(out)
		}()
	}
	for i := 0; i < runners; i++ {
		if out := <-failures; out != "" {
			t.Errorf("a concurrent runner failed to configure Git: %s", out)
		}
	}
	if _, err := os.Lstat(filepath.Join(home, ".gitconfig")); !os.IsNotExist(err) {
		t.Errorf("the step must not write ~/.gitconfig: %v", err)
	}
	for i := 0; i < runners; i++ {
		global := filepath.Join(dir, "corvint-box-"+strconv.Itoa(i)+".gitconfig")
		env, err := os.ReadFile(filepath.Join(dir, "env-"+strconv.Itoa(i)))
		if err != nil || string(env) != "GIT_CONFIG_GLOBAL="+global+"\n" {
			t.Errorf("runner %d GITHUB_ENV = %q %v, want GIT_CONFIG_GLOBAL=%s", i, env, err, global)
		}
		for key, want := range map[string]string{"user.name": "corvint-ci", "user.email": "corvint-ci@users.noreply.github.com", "maintenance.auto": "false", "gc.auto": "0"} {
			cmd := exec.Command("git", "config", "--global", "--get", key)
			cmd.Env = append(os.Environ(), "HOME="+home, "GIT_CONFIG_GLOBAL="+global, "GIT_CONFIG_NOSYSTEM=1")
			if out, err := cmd.Output(); err != nil || strings.TrimSpace(string(out)) != want {
				t.Errorf("runner %d git config %s = %q %v, want %q", i, key, out, err, want)
			}
		}
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

// AFP-V0-460 (V1-1753): each self-hosted shard's marker records its summed package time, and when
// every shard ran on the self-hosted runner the drift job reports their spread and warns, never
// failing, once the slowest shard exceeds the bound; a missing sum reports no measurement.
func TestAFPV0460SelfHostedShardBalanceWarnsPastTheBound(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the marker and balance steps run under bash")
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq is not installed")
	}
	ci := readWorkflow(t, "ci.yml")
	marker := runScript(t, stepBlock(t, jobBlock(t, ci, "go-product-shard"), "Mark a self-hosted shard for the advisory cost-drift report (AFP-V0-422)"))
	drift := jobBlock(t, ci, "ci-shard-cost-drift")
	balanceStep := stepBlock(t, drift, "Report the self-hosted shard balance (advisory, AFP-V0-460)")
	wantLine(t, "balance", balanceStep, "        if: steps.complete.outputs.boxed == '1'")
	wantLine(t, "balance", balanceStep, "        continue-on-error: true")
	wantLine(t, "balance", balanceStep, `          BOUND_PERCENT: "125"`)
	bash := func(script string, env ...string) string {
		t.Helper()
		cmd := exec.Command("bash", "-eo", "pipefail", "-c", script)
		cmd.Env = append(os.Environ(), env...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("step failed: %v\n%s", err, out)
		}
		return string(out)
	}
	// The marker sums package-level terminal events only, never test events.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "shard-costs"), 0o700); err != nil {
		t.Fatal(err)
	}
	stream := `{"Action":"start","Package":"a"}
{"Action":"pass","Package":"a","Test":"TestX","Elapsed":50}
{"Action":"pass","Package":"a","Elapsed":100.6}
{"Action":"skip","Package":"b","Elapsed":0.2}
{"Action":"pass","Package":"c","Elapsed":200.5}
`
	if err := os.WriteFile(filepath.Join(dir, "shard-costs", "shard-3.json"), []byte(stream), 0o600); err != nil {
		t.Fatal(err)
	}
	bash(marker, "RUNNER_TEMP="+dir, "SHARD=3", "RUNNER_ENVIRONMENT=self-hosted")
	if b, err := os.ReadFile(filepath.Join(dir, "shard-runner", "shard-3.self-hosted")); err != nil || string(b) != "self-hosted\npackageSeconds=301\n" {
		t.Fatalf("marker = %q %v", b, err)
	}
	// An unreadable stream keeps the bare marker and never fails the step.
	if err := os.WriteFile(filepath.Join(dir, "shard-costs", "shard-4.json"), []byte("not json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	bash(marker, "RUNNER_TEMP="+dir, "SHARD=4", "RUNNER_ENVIRONMENT=self-hosted")
	if b, err := os.ReadFile(filepath.Join(dir, "shard-runner", "shard-4.self-hosted")); err != nil || string(b) != "self-hosted\n" {
		t.Fatalf("marker of an unreadable stream = %q %v", b, err)
	}
	complete := runScript(t, stepBlock(t, drift, "Require the outcomes of every shard"))
	balance := runScript(t, balanceStep)
	run := func(sums []string) (string, string, string) {
		t.Helper()
		dir := t.TempDir()
		costs := filepath.Join(dir, "shard-costs")
		if err := os.MkdirAll(costs, 0o700); err != nil {
			t.Fatal(err)
		}
		for i, s := range sums {
			body := "self-hosted\n"
			if s != "" {
				body += "packageSeconds=" + s + "\n"
			}
			if err := os.WriteFile(filepath.Join(costs, "shard-"+strconv.Itoa(i)+".self-hosted"), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		output, summary := filepath.Join(dir, "output"), filepath.Join(dir, "summary")
		env := []string{"RUNNER_TEMP=" + dir, "SHARDS=8", "SHARDS_RESULT=success", "BOUND_PERCENT=125",
			"GITHUB_OUTPUT=" + output, "GITHUB_STEP_SUMMARY=" + summary}
		bash(complete, env...)
		o, _ := os.ReadFile(output)
		log := ""
		if strings.Contains(string(o), "boxed=1\n") {
			log = bash(balance, env...)
		}
		s, _ := os.ReadFile(summary)
		return string(o), log, string(s)
	}
	even := []string{"100", "100", "100", "100", "100", "100", "100", "100"}
	out, log, summary := run(even)
	if !strings.Contains(out, "boxed=1\n") || strings.Contains(log, "::warning") ||
		!strings.Contains(summary, "slowest shard 100 s of package time is 100 percent of the 100 s mean over 8 self-hosted shards (bound 125 percent; shards: 100 100 100 100 100 100 100 100).") {
		t.Errorf("balanced shards: output %q, log %q, summary %q", out, log, summary)
	}
	skewed := append([]string{"300"}, even[1:]...)
	if _, log, _ := run(skewed); !strings.Contains(log, "::warning title=CI shard balance::slowest shard 300 s of package time is 240 percent of the 125 s mean") {
		t.Errorf("a skewed shard must warn: %q", log)
	}
	// Exactly at the bound does not warn: 125 s of a 100 s mean.
	edge := []string{"125", "95", "95", "95", "95", "95", "100", "100"}
	if _, log, _ := run(edge); strings.Contains(log, "::warning") || !strings.Contains(log, "is 125 percent of the 100 s mean") {
		t.Errorf("exactly 125 percent must not warn: %q", log)
	}
	// Past the bound before rounding warns: 1255 s of a 1000 s mean is 125.5 percent.
	over := []string{"1255", "963", "963", "963", "963", "963", "963", "967"}
	if _, log, _ := run(over); !strings.Contains(log, "::warning title=CI shard balance::slowest shard 1255 s of package time is 125 percent of the 1000 s mean") {
		t.Errorf("125.5 percent must warn: %q", log)
	}
	missing := append([]string{""}, even[1:]...)
	if _, log, summary := run(missing); log != "" && strings.Contains(log, "::warning") || !strings.Contains(summary, "Not measured: 7 of 8 self-hosted shards recorded their package time.") {
		t.Errorf("a marker without its sum must not measure: log %q, summary %q", log, summary)
	}
	// Fewer than every shard on the self-hosted runner leaves the balance step off.
	if out, _, _ := run(even[:7]); strings.Contains(out, "boxed=1") {
		t.Errorf("7 of 8 self-hosted shards must not report balance: %q", out)
	}
}

// AFP-V0-427 to AFP-V0-429: the host-local seed is restored only on the self-hosted runner, for a
// merge-queue entry on main or a same-repository pull request where the race invocation runs; only
// a passing self-hosted merge-queue shard on main refreshes it; the hosted cache steps are unchanged.
func TestAFPV0427SelfHostedSeedRestoreAndRefreshEvents(t *testing.T) {
	ci := readWorkflow(t, "ci.yml")
	shard := jobBlock(t, ci, "go-product-shard")
	wantLine(t, "go-product-shard env", shard, "      CORVINT_BOX_GOCACHE_ROOT: /home/russell/ci-cache")
	if n := strings.Count(ci, "CORVINT_BOX_GOCACHE_ROOT"); n != 3 {
		t.Errorf("ci.yml names CORVINT_BOX_GOCACHE_ROOT %d times, want the job env and its two shard readers", n)
	}
	restore := stepBlock(t, shard, "Restore the host-local Go build seed (self-hosted runner, AFP-V0-427)")
	refresh := stepBlock(t, shard, "Refresh the host-local Go build seed (self-hosted merge-queue shards on main, AFP-V0-428)")
	wantLine(t, "seed restore", restore, "        if: runner.environment == 'self-hosted' && "+
		"(needs.docs-plan.result != 'success' || (needs.docs-plan.outputs.mode != 'DOCS' && needs.docs-plan.outputs.mode != 'REUSE')) && "+
		"((github.event_name == 'merge_group' && github.event.merge_group.base_ref == 'refs/heads/main') || "+
		"(github.event_name == 'pull_request' && github.event.pull_request.head.repo.full_name == github.repository))")
	wantLine(t, "seed refresh", refresh, "        if: runner.environment == 'self-hosted' && github.event_name == 'merge_group' && "+
		"github.event.merge_group.base_ref == 'refs/heads/main' && steps.tests.outcome == 'success'")
	for name, step := range map[string]string{"restore": restore, "refresh": refresh} {
		wantLine(t, name, step, "        continue-on-error: true")
		script := runScript(t, step)
		if !strings.Contains(script, "cp -a -- ") || strings.Contains(script, "cp -l") || strings.Contains(script, "cp -al") ||
			regexp.MustCompile(`(?m)\bln (-f )?--`).MatchString(script) || strings.Contains(script, "exit 1") {
			t.Errorf("%s must copy (never hardlink) and never fail the shard:\n%s", name, script)
		}
	}
	wantLine(t, "seed refresh", refresh, `          SEED_CAP_KIB: "8388608"`)
	// Order: the restore follows the verification that sets GOCACHE and precedes the tests; the
	// refresh follows the tests.
	at := func(name string) int { return strings.Index(shard, "      - name: "+name) }
	if v, r, g, f := at("Verify the host tools"), at("Restore the host-local Go build seed"), at("Go tests (full until"),
		at("Refresh the host-local Go build seed"); v < 0 || !(v < r && r < g && g < f) {
		t.Errorf("want verify < restore < tests < refresh, got %d %d %d %d", v, r, g, f)
	}
	// -count=1 stays on every race invocation of the shard, so no test result is reused.
	for _, m := range regexp.MustCompile(`(?m)^.*go test -exec .*$`).FindAllString(shard, -1) {
		if !strings.Contains(m, " -count=1 ") {
			t.Errorf("a shard race invocation lost -count=1: %q", m)
		}
	}
	// The hosted V1-1098 restore and save keep their events and stay off the self-hosted runner.
	wantLine(t, "hosted restore", stepBlock(t, shard, "Restore the trusted Go build cache (main pushes and merge-queue entries only)"),
		"        if: runner.environment != 'self-hosted' && (needs.docs-plan.result != 'success' || (needs.docs-plan.outputs.mode != 'DOCS' && "+
			"needs.docs-plan.outputs.mode != 'REUSE')) && ((github.event_name == 'push' && github.ref == 'refs/heads/main') || "+
			"(github.event_name == 'merge_group' && github.event.merge_group.base_ref == 'refs/heads/main'))")
	wantLine(t, "hosted expire", stepBlock(t, shard, "Expire cached test results before saving the build cache"),
		"        if: github.event_name == 'push' && github.ref == 'refs/heads/main' && steps.gocache.outcome == 'success' && steps.tests.outcome == 'success'")
	wantLine(t, "hosted save", stepBlock(t, shard, "Save the trusted Go build cache (main pushes only)"),
		"        if: steps.testcache.outcome == 'success'")
}

// AFP-V0-427/428 behaviour: the scripts run under bash against a temporary root. A missing seed
// leaves a cold cache; a refresh swaps in a copy and removes the replaced one; a restore copies it
// (an independent file, not a link); an unsafe root or a cache above the cap never fails.
func TestAFPV0428SelfHostedSeedRefreshSwapsAndRestoreCopies(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the seed steps run under bash")
	if runtime.GOOS != "linux" {
		t.Skip("the seed steps use GNU cp, mv -T and readlink -e, as on the linux self-hosted runner")
	}
	shard := jobBlock(t, readWorkflow(t, "ci.yml"), "go-product-shard")
	restore := runScript(t, stepBlock(t, shard, "Restore the host-local Go build seed (self-hosted runner, AFP-V0-427)"))
	refresh := runScript(t, stepBlock(t, shard, "Refresh the host-local Go build seed (self-hosted merge-queue shards on main, AFP-V0-428)"))
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	// A go spy: `go env GOCACHE` prints the job cache; `go clean -testcache` marks it.
	spy := "#!/bin/sh\ncase \"$1\" in env) printf '%s\\n' \"$GOCACHE\" ;; clean) : > \"$GOCACHE/testexpire.txt\" ;; esac\n"
	if err := execfile.WriteFile(filepath.Join(bin, "go"), []byte(spy), 0o700); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "ci-cache")
	run := func(path, script, temp, runner, capKiB, tag string) (string, error) {
		t.Helper()
		if err := os.MkdirAll(temp, 0o700); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("bash", "--noprofile", "--norc", "-eo", "pipefail", "-c", script)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "PATH="+path+":"+os.Getenv("PATH"), "RUNNER_TEMP="+temp,
			"GOCACHE="+filepath.Join(temp, "go-build"), "RUNNER_NAME="+runner, "CORVINT_BOX_GOCACHE_ROOT="+root,
			"SEED_CAP_KIB="+capKiB, "SEED_TAG="+tag)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	exec1 := func(script, temp, runner, capKiB, tag string) string {
		t.Helper()
		out, err := run(bin, script, temp, runner, capKiB, tag)
		if err != nil {
			t.Fatalf("the step failed: %v\n%s", err, out)
		}
		return out
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	seed := filepath.Join(root, "go-build-race-corvint-box-1")
	// No root and no seed: a cold cache with a notice.
	if out := exec1(restore, filepath.Join(dir, "t1"), "corvint-box-1", "", ""); !strings.Contains(out, "this shard builds in a cold cache") {
		t.Errorf("a missing seed must leave a cold cache with a notice: %s", out)
	}
	// A refresh creates the seed link and its directory.
	t2 := filepath.Join(dir, "t2")
	write(filepath.Join(t2, "go-build", "ab", "ab-d"), "first")
	exec1(refresh, t2, "corvint-box-1", "8388608", "1-1-0")
	if got, err := os.Readlink(seed); err != nil || got != "go-build-race-corvint-box-1.1-1-0" {
		t.Fatalf("the seed must be a relative link to the refreshed copy: %q %v", got, err)
	}
	if b, _ := os.ReadFile(filepath.Join(seed, "testexpire.txt")); b == nil {
		t.Error("the refreshed seed must have its test results expired")
	}
	// A second refresh swaps the link and removes the replaced copy and any leftover.
	t3 := filepath.Join(dir, "t3")
	write(filepath.Join(t3, "go-build", "ab", "ab-d"), "second")
	if err := os.MkdirAll(seed+".9-9-9.tmp", 0o700); err != nil {
		t.Fatal(err)
	}
	exec1(refresh, t3, "corvint-box-1", "8388608", "2-1-3")
	entries, _ := filepath.Glob(seed + "*")
	if len(entries) != 2 || entries[0] != seed || entries[1] != seed+".2-1-3" {
		t.Errorf("after a swap want only the link and the new copy, got %q", entries)
	}
	// A restore copies the seed into an independent per-job cache.
	t4 := filepath.Join(dir, "t4")
	if out := exec1(restore, t4, "corvint-box-1", "", ""); !strings.Contains(out, "AFP-V0-427: restored") {
		t.Fatalf("a present seed must be restored: %s", out)
	}
	restored := filepath.Join(t4, "go-build", "ab", "ab-d")
	if b, _ := os.ReadFile(restored); string(b) != "second" {
		t.Errorf("restored %q, want the newest seed", b)
	}
	write(restored, "rewritten")
	if b, _ := os.ReadFile(filepath.Join(seed, "ab", "ab-d")); string(b) != "second" {
		t.Error("rewriting a restored entry changed the seed: the restore must copy, not link")
	}
	// A refresh that fails mid-copy (a cp spy that leaves a partial copy, as on ENOSPC) first removes
	// abandoned generations and earlier interrupted copies, removes its own partial copy, and keeps
	// the live seed.
	failbin := filepath.Join(dir, "failbin")
	write(filepath.Join(failbin, "cp"), "#!/bin/sh\nmkdir -p \"$4\" && : > \"$4/partial\"\nexit 1\n")
	if err := os.Chmod(filepath.Join(failbin, "cp"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, stale := range []string{seed + ".6-6-6", seed + ".7-7-7.tmp"} {
		write(filepath.Join(stale, "ab", "ab-d"), "stale")
	}
	tf := filepath.Join(dir, "tf")
	write(filepath.Join(tf, "go-build", "ab", "ab-d"), "unfinished")
	if out, err := run(failbin+":"+bin, refresh, tf, "corvint-box-1", "8388608", "5-1-0"); err == nil {
		t.Errorf("the spy copy failure must fail the step: %s", out)
	}
	if entries, _ := filepath.Glob(seed + "*"); len(entries) != 2 || entries[0] != seed || entries[1] != seed+".2-1-3" {
		t.Errorf("after a failed refresh want only the live link and copy, got %q", entries)
	}
	if b, _ := os.ReadFile(filepath.Join(seed, "ab", "ab-d")); string(b) != "second" {
		t.Errorf("a failed refresh changed the live seed: %q", b)
	}
	// Another runner has no seed of its own: a cold cache.
	if out := exec1(restore, filepath.Join(dir, "t5"), "corvint-box-2", "", ""); !strings.Contains(out, "cold cache") {
		t.Errorf("another runner must not read this runner's seed: %s", out)
	}
	// A cache above the cap removes the seed instead of refreshing it.
	t6 := filepath.Join(dir, "t6")
	write(filepath.Join(t6, "go-build", "ab", "ab-d"), strings.Repeat("x", 64<<10))
	if out := exec1(refresh, t6, "corvint-box-1", "16", "3-1-0"); !strings.Contains(out, "above the 16 KiB cap; seed removed") {
		t.Errorf("an over-cap cache must remove the seed: %s", out)
	}
	if entries, _ := filepath.Glob(seed + "*"); len(entries) != 0 {
		t.Errorf("an over-cap refresh left %q", entries)
	}
	// A group-writable root is refused by both steps without failing.
	if err := os.Chmod(root, 0o770); err != nil {
		t.Fatal(err)
	}
	t7 := filepath.Join(dir, "t7")
	write(filepath.Join(t7, "go-build", "ab", "ab-d"), "third")
	if out := exec1(refresh, t7, "corvint-box-1", "8388608", "4-1-0"); !strings.Contains(out, "not refreshing") {
		t.Errorf("an unsafe root must not be refreshed: %s", out)
	}
	if out := exec1(restore, filepath.Join(dir, "t8"), "corvint-box-1", "", ""); !strings.Contains(out, "cold cache") {
		t.Errorf("an unsafe root must not be restored: %s", out)
	}
	// A runner name that is not a plain path component is refused.
	if out := exec1(restore, filepath.Join(dir, "t9"), "../x", "", ""); !strings.Contains(out, "not a plain seed name") {
		t.Errorf("a path-like runner name must be refused: %s", out)
	}
}

// AFP-V0-430: on the self-hosted runner only, the verification step points TMPDIR at a per-runner
// directory on the ext4 /var/tmp, emptied for every job, before the tests run, so t.TempDir() never
// lands on the box's RAM-backed /tmp nor under an ancestor of the confined checkout (RUNNER_TEMP is
// under /home/russell, which test-confine never lets a test open); no other step and no hosted
// step sets it.
func TestAFPV0430SelfHostedTMPDIRIsPerRunnerOffTheCheckoutAncestors(t *testing.T) {
	ci := readWorkflow(t, "ci.yml")
	shard := jobBlock(t, ci, "go-product-shard")
	verify := stepBlock(t, shard, "Verify the host tools the product's fixed PATH resolves (self-hosted runner, AFP-V0-421)")
	wantLine(t, "self-hosted verification", verify, "        if: runner.environment == 'self-hosted'")
	for _, line := range []string{
		`            '' | *[!A-Za-z0-9_-]*)`,
		`          tmp_root=/var/tmp/corvint-ci`,
		`          mkdir -m 0700 -- "$tmp_root" 2> /dev/null || true`,
		`          if [ -L "$tmp_root" ] || [ ! -d "$tmp_root" ] || [ ! -O "$tmp_root" ]; then`,
		`          job_tmp="$tmp_root/$RUNNER_NAME"`,
		`          find -P "$job_tmp" -type d ! -perm -0700 -exec chmod u+rwx {} \; 2> /dev/null || true`,
		`          rm -rf -- "$job_tmp"`,
		`          mkdir -m 0700 -- "$job_tmp"`,
		`          printf 'TMPDIR=%s\n' "$job_tmp" >> "$GITHUB_ENV"`,
	} {
		wantLine(t, "self-hosted verification", verify, line)
	}
	name, guard := strings.Index(verify, `case "$RUNNER_NAME" in`), strings.Index(verify, `[ ! -O "$tmp_root" ]`)
	restore, clear := strings.Index(verify, `find -P "$job_tmp"`), strings.Index(verify, `rm -rf -- "$job_tmp"`)
	if name < 0 || guard < name || restore < guard || clear < restore {
		t.Errorf("the runner-name check, the root guard and the permission restore must precede the removal: name %d, guard %d, find %d, rm %d", name, guard, restore, clear)
	}
	if strings.Contains(verify, `"$RUNNER_TEMP/tmp"`) {
		t.Error("TMPDIR must not be under RUNNER_TEMP, whose ancestors are the confined checkout's")
	}
	if all, here := strings.Count(ci, "TMPDIR"), strings.Count(verify, "TMPDIR"); all != here {
		t.Errorf("ci.yml names TMPDIR %d times, %d of them outside the self-hosted verification step", all, all-here)
	}
	at := func(name string) int { return strings.Index(shard, "      - name: "+name) }
	if v, g := at("Verify the host tools"), at("Go tests (full until"); v < 0 || g < v {
		t.Errorf("TMPDIR must be set before the tests step: verify at %d, tests at %d", v, g)
	}
}

// AFP-V0-430: the TMPDIR block run under bash with its root moved into a test directory. A path-like
// runner name and a linked root are refused before anything is removed; a leftover tree whose
// directories lack owner write or search permission is emptied without following a link out of it.
func TestAFPV0430SelfHostedTMPDIRBlockEmptiesLeftoversAndRefusesUnsafeRoots(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the verification step runs under bash")
	if os.Geteuid() == 0 {
		t.Skip("root ignores the permission bits this test leaves behind")
	}
	verify := stepBlock(t, jobBlock(t, readWorkflow(t, "ci.yml"), "go-product-shard"),
		"Verify the host tools the product's fixed PATH resolves (self-hosted runner, AFP-V0-421)")
	start, end := strings.Index(verify, `          case "$RUNNER_NAME" in`), strings.Index(verify, `"$job_tmp" >> "$GITHUB_ENV"`)
	if start < 0 || end < start {
		t.Fatalf("the AFP-V0-430 block is missing: start %d, end %d", start, end)
	}
	block := strings.ReplaceAll(verify[start:end+len(`"$job_tmp" >> "$GITHUB_ENV"`)], "\n          ", "\n")
	dir := t.TempDir()
	t.Cleanup(func() { _ = exec.Command("chmod", "-R", "u+rwx", dir).Run() })
	root := filepath.Join(dir, "corvint-ci")
	const fixed = "tmp_root=/var/tmp/corvint-ci\n"
	if strings.Count(block, fixed) != 1 {
		t.Fatalf("the block must name its root once as %q", fixed)
	}
	block = strings.Replace(block, fixed, "tmp_root="+root+"\n", 1)
	env := filepath.Join(dir, "github-env")
	run := func(runner string) (string, error) {
		t.Helper()
		cmd := exec.Command("bash", "--noprofile", "--norc", "-e", "-c", strings.TrimPrefix(block, "          "))
		cmd.Env = append(os.Environ(), "RUNNER_NAME="+runner, "GITHUB_ENV="+env)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	mkdir := func(path string, mode os.FileMode) {
		t.Helper()
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := run("../x"); err == nil || !strings.Contains(out, "not a plain directory name") {
		t.Fatalf("a path-like runner name must be refused: %v %s", err, out)
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatalf("a refused runner name created the root: %v", err)
	}
	// Leftovers of an interrupted job: a populated 0500 directory, a 0000 directory below it, and a
	// link to a 0500 directory outside the tree, whose mode must not change.
	outside := filepath.Join(dir, "outside")
	mkdir(outside, 0o700)
	if err := os.WriteFile(filepath.Join(outside, "keep"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	mkdir(outside, 0o500)
	job := filepath.Join(root, "box-1")
	locked := filepath.Join(job, "locked")
	mkdir(filepath.Join(locked, "sealed"), 0o700)
	if err := os.WriteFile(filepath.Join(locked, "stale"), nil, 0o400); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(job, "link")); err != nil {
		t.Fatal(err)
	}
	mkdir(filepath.Join(locked, "sealed"), 0o000)
	mkdir(locked, 0o500)
	if out, err := run("box-1"); err != nil {
		t.Fatalf("leftovers must be emptied: %v %s", err, out)
	}
	if entries, err := os.ReadDir(job); err != nil || len(entries) != 0 {
		t.Fatalf("the job directory must be empty and recreated: %v %v", entries, err)
	}
	for path, want := range map[string]os.FileMode{root: 0o700, job: 0o700, outside: 0o500} {
		if info, err := os.Lstat(path); err != nil || !info.IsDir() || info.Mode().Perm() != want {
			t.Errorf("%s: want a directory mode %o, got %v %v", path, want, info, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(outside, "keep")); err != nil {
		t.Errorf("the link target outside the tree was touched: %v", err)
	}
	if data, err := os.ReadFile(env); err != nil || string(data) != "TMPDIR="+job+"\n" {
		t.Errorf("GITHUB_ENV = %q %v", data, err)
	}
	// A root that is a link is refused before anything under it is removed.
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "target")
	mkdir(filepath.Join(target, "box-1"), 0o700)
	if err := os.Symlink(target, root); err != nil {
		t.Fatal(err)
	}
	if out, err := run("box-1"); err == nil || !strings.Contains(out, "must be a directory this account owns, not a link") {
		t.Fatalf("a linked root must be refused: %v %s", err, out)
	}
	if _, err := os.Stat(filepath.Join(target, "box-1")); err != nil {
		t.Errorf("a refused linked root removed its target's runner directory: %v", err)
	}
}
