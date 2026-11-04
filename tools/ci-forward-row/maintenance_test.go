// SPDX-License-Identifier: Apache-2.0
package forwardrow

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func workflowJob(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`(?ms)^  ` + regexp.QuoteMeta(name) + `:\n(.*?)(?:^  [a-zA-Z][a-zA-Z0-9_-]*:\n|\z)`)
	m := re.FindStringSubmatch(string(data))
	if m == nil {
		t.Fatalf("missing job %s", name)
	}
	return m[1]
}

// CIN-V0-7501: the PR documentation gate must enforce every job's exact permissions.
func TestCINV07501PermissionsArePremerge(t *testing.T) {
	b, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^DOC_GATE_TARGETS := (.*)$`).FindStringSubmatch(string(b))
	if m == nil || !strings.Contains(" "+m[1]+" ", " ci-least-privilege-check ") {
		t.Fatal("permission check is absent from the shared PR gates")
	}
}

// CIN-V0-7502/7503: the declared family and offline module suite cannot be advisory.
func TestCINV07502RequiredFamiliesAndOfflineModules(t *testing.T) {
	if job := workflowJob(t, "host-adapters"); !strings.Contains(job, "run: make host-adapter-test") || strings.Contains(job, "continue-on-error") {
		t.Fatal(job)
	}
	static := workflowJob(t, "go-static")
	start := strings.Index(static, "      - name: Nested module tools/local-authority")
	if start < 0 {
		t.Fatal("no nested module step")
	}
	nested := static[start:]
	for _, want := range []string{"GOMODCACHE:", "GOFLAGS: \"-mod=readonly\"", "go mod verify", "go test -count=1", "go vet ./..."} {
		if !strings.Contains(nested, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(static, "https://proxy.golang.org") || !strings.Contains(static, "fail-on-cache-miss: true") || !strings.Contains(static, "GOPROXY: \"off\"") || !strings.Contains(static, "needs: [static-reuse, local-authority-deps]") {
		t.Fatal("static module tests can fetch or skip preparation")
	}
	prep := workflowJob(t, "local-authority-deps")
	if !strings.Contains(prep, "go mod verify") || !strings.Contains(prep, "hashFiles('tools/local-authority/go.mod', 'tools/local-authority/go.sum')") {
		t.Fatal("module cache is not checksum-bound")
	}
	if product := workflowJob(t, "go-product"); !strings.Contains(product, "host-adapters") || !strings.Contains(product, `test "$HOST_RESULT" = success`) {
		t.Fatal("host family failure does not block required check")
	}
}

// CIN-V0-7504: execute the workflow's actual selection expression with reached/unreached plans.
func TestCINV07504WebFlowsConsumerJob(t *testing.T) {
	plan := workflowJob(t, "web-flows-plan")
	for _, trusted := range []string{
		"path: source", "ref: ${{ github.sha }}", "path: trusted-web-plan",
		"ref: ${{ github.event.pull_request.base.sha || github.event.merge_group.base_sha || github.event.before || github.sha }}",
		"working-directory: trusted-web-plan", `--root "$GITHUB_WORKSPACE/source" test`,
	} {
		if !strings.Contains(plan, trusted) {
			t.Fatalf("consumer selector is not isolated from tested code: missing %s", trusted)
		}
	}
	line := regexp.MustCompile(`(?m)^          required=\$\((.*)\)$`).FindStringSubmatch(plan)
	if line == nil {
		t.Fatal("missing consumer selection")
	}
	for _, tc := range []struct{ checks, want string }{
		{`["tools/web-flows/test/e2e.mjs (UNSELECTED_BINARY_CONSUMER: run it)"]`, "true"},
		{`["tools/web-flows/test/navigation-e2e.mjs (UNSELECTED_BINARY_CONSUMER: run it)"]`, "true"},
		{`["make go-vet"]`, "false"},
	} {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "web-plan.json"), []byte(`{"checks":`+tc.checks+`}`))
		cmd := exec.Command("sh", "-ec", line[1])
		cmd.Env = append(os.Environ(), "RUNNER_TEMP="+dir)
		out, err := cmd.CombinedOutput()
		if err != nil || strings.TrimSpace(string(out)) != tc.want {
			t.Fatalf("checks=%s result=%s err=%v", tc.checks, out, err)
		}
	}
	web := workflowJob(t, "web-flows")
	if !strings.Contains(web, "if: needs.web-flows-plan.outputs.required == 'true'") || !strings.Contains(web, "run: script/web-flows-gate") || strings.Contains(web, "continue-on-error") {
		t.Fatal(web)
	}
	for _, setup := range []string{"npm ci --ignore-scripts --no-audit --no-fund", "npx --no-install playwright install --with-deps chromium"} {
		if !strings.Contains(web, setup) {
			t.Fatalf("missing consumer runtime setup: %s", setup)
		}
	}
	product := workflowJob(t, "go-product")
	for _, want := range []string{"web-flows-plan, web-flows", `test "$WEB_PLAN_RESULT" = success`, `if [ "$WEB_REQUIRED" = true ]; then test "$WEB_RESULT" = success; else test "$WEB_RESULT" = skipped; fi`} {
		if !strings.Contains(product, want) {
			t.Fatalf("missing required outcome check %s", want)
		}
	}
	guard := regexp.MustCompile(`(?s)(test "\$HOST_RESULT" = success.*?)(?:          # AFP-V0-027)`).FindStringSubmatch(product)
	if guard == nil {
		t.Fatal("missing required-result guard")
	}
	for _, tc := range []struct {
		host, required, result string
		pass                   bool
	}{
		{"success", "true", "success", true}, {"success", "true", "failure", false}, {"success", "true", "skipped", false}, {"success", "false", "skipped", true}, {"failure", "false", "skipped", false},
	} {
		cmd := exec.Command("sh", "-ec", guard[1])
		cmd.Env = append(os.Environ(), "HOST_RESULT="+tc.host, "WEB_PLAN_RESULT=success", "WEB_REQUIRED="+tc.required, "WEB_RESULT="+tc.result)
		if err := cmd.Run(); (err == nil) != tc.pass {
			t.Fatalf("required result %+v: %v", tc, err)
		}
	}

}
