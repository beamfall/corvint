// SPDX-License-Identifier: Apache-2.0
package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func write(t *testing.T, name string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func fixtureGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=Russell Lewis", "-c", "user.email=me@russe11.com", "-c", "commit.gpgsign=false", "-c", "core.hooksPath="}, args...)...)
	cmd.Dir = root
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, b)
	}
	return strings.TrimSpace(string(b))
}

func commit(t *testing.T, root string) string {
	t.Helper()
	fixtureGit(t, root, "add", "--all")
	fixtureGit(t, root, "commit", "-qm", "fixture")
	return fixtureGit(t, root, "rev-parse", "HEAD")
}

func selection(patterns ...string) []byte {
	slices.Sort(patterns)
	owners := []map[string]any{}
	for _, p := range patterns {
		owners = append(owners, map[string]any{"paths": p, "tests": []string{qualificationSource}})
	}
	b, _ := json.Marshal(map[string]any{"profile": "corvint-test-selection/2", "owners": owners})
	return b
}

// Fixtures use private temporary repositories, never the enclosing checkout.
func currencyFixture(t *testing.T) (gate, string, string, []byte) {
	t.Helper()
	root := t.TempDir()
	fixtureGit(t, root, "init", "-q")
	write(t, filepath.Join(root, selectionPath), selection("rank/*.go", "corpus/**"))
	write(t, filepath.Join(root, "rank", "old.go"), []byte("old\n"))
	repos, reports := []any{}, []any{}
	for i := range 3 {
		name := fmt.Sprintf("repo%d", i)
		queries, records := make([]any, 25), make([]any, 25)
		for j := range 25 {
			queries[j] = map[string]string{"id": fmt.Sprint(j)}
			records[j] = map[string]string{"query": fmt.Sprint(j)}
		}
		repos = append(repos, map[string]any{"name": name, "commit": strings.Repeat("a", 40), "evidence": "digest-only", "queries": queries})
		reports = append(reports, map[string]any{"name": name, "commit": strings.Repeat("a", 40), "evidence": "digest-only", "verdict": "PASS", "queries": 25, "records": records, "unverified": 0, "failures": []any{}, "status_before_sha256": strings.Repeat("b", 64), "status_after_sha256": strings.Repeat("b", 64)})
	}
	queries, _ := json.Marshal(map[string]any{"repositories": repos})
	write(t, filepath.Join(root, queryPath), queries)
	qualified := commit(t, root)
	pin, _ := json.Marshal(qualificationPin{Commit: qualified, ReportSHA256: strings.Repeat("c", 64)})
	write(t, filepath.Join(root, pinPath), pin)
	commit(t, root)
	reportPath := filepath.Join(t.TempDir(), "report.json")
	configPath := filepath.Join(t.TempDir(), "config.json")
	cfg, _ := json.Marshal(map[string]string{"report": reportPath})
	write(t, configPath, cfg)
	report, _ := json.Marshal(map[string]any{"schema": "corvint-rcf-qualification-report/1", "query_set_sha256": fmt.Sprintf("%x", sha256.Sum256(queries)), "verdict": "PASS", "repositories": reports})
	return gate{root: root}, configPath, reportPath, report
}

// RCF-V1-013, AC1/AC2: each missing-evidence case fails independently;
// a current live report and both ranking runs bind success to the candidate.
func TestCurrency(t *testing.T) {
	cases := []struct{ name, want string }{
		{"unchanged", ""},
		{"unrelated change", ""},
		{"changed missing configuration", "CORVINT_RCF_QUALIFY"},
		{"added trigger missing configuration", "CORVINT_RCF_QUALIFY"},
		{"deleted trigger missing configuration", "CORVINT_RCF_QUALIFY"},
		{"renamed trigger missing configuration", "CORVINT_RCF_QUALIFY"},
		{"removed owner mapping", "CORVINT_RCF_QUALIFY"},
		{"empty owner mappings", "missing qualification trigger owners"},
		{"missing pin", "missing qualification pin"},
		{"invalid pin", "invalid qualification pin"},
		{"missing historical commit", "missing qualified commit"},
		{"missing selection", "missing selection"},
		{"invalid selection", "unsupported selection profile"},
		{"missing config file", "qualification configuration"},
		{"skipped live run", "skip"},
		{"unselected live run", "missing named live PASS"},
		{"failed live run", "live execution failed"},
		{"missing report", "missing qualification report"},
		{"stale existing report", "stale report"},
		{"stale report digest", "query-set digest mismatch"},
		{"failed report", "report is not PASS"},
		{"incomplete report", "incomplete qualification report"},
		{"failed repository", "incomplete or failing"},
		{"ranking failure", "ranking failed"},
		{"wrong candidate", "checkout does not name candidate"},
		{"dirty candidate", "checkout is dirty"},
		{"dirty after run", "checkout is dirty"},
		{"live success", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, config, reportPath, report := currencyFixture(t)
			if tc.name != "unchanged" && tc.name != "unrelated change" {
				write(t, filepath.Join(g.root, "rank", "old.go"), []byte("new\n"))
			}
			switch tc.name {
			case "unrelated change":
				write(t, filepath.Join(g.root, "README.md"), []byte("docs\n"))
			case "added trigger missing configuration":
				fixtureGit(t, g.root, "restore", "rank/old.go")
				write(t, filepath.Join(g.root, "corpus", "nested", "new"), []byte("new\n"))
			case "deleted trigger missing configuration":
				if err := os.Remove(filepath.Join(g.root, "rank", "old.go")); err != nil {
					t.Fatal(err)
				}
			case "renamed trigger missing configuration":
				fixtureGit(t, g.root, "restore", "rank/old.go")
				if err := os.Rename(filepath.Join(g.root, "rank", "old.go"), filepath.Join(g.root, "outside.go")); err != nil {
					t.Fatal(err)
				}
			case "removed owner mapping":
				write(t, filepath.Join(g.root, selectionPath), selection("corpus/**"))
			case "empty owner mappings":
				write(t, filepath.Join(g.root, selectionPath), selection())
			case "missing pin":
				if err := os.Remove(filepath.Join(g.root, pinPath)); err != nil {
					t.Fatal(err)
				}
			case "invalid pin":
				write(t, filepath.Join(g.root, pinPath), []byte(`{}`))
			case "missing historical commit":
				b, _ := json.Marshal(qualificationPin{Commit: strings.Repeat("f", 40), ReportSHA256: strings.Repeat("a", 64)})
				write(t, filepath.Join(g.root, pinPath), b)
			case "missing selection":
				if err := os.Remove(filepath.Join(g.root, selectionPath)); err != nil {
					t.Fatal(err)
				}
			case "invalid selection":
				write(t, filepath.Join(g.root, selectionPath), []byte(`{}`))
			}
			if tc.name != "unchanged" {
				commit(t, g.root)
			}
			candidate := fixtureGit(t, g.root, "rev-parse", "HEAD")
			if tc.name == "wrong candidate" {
				candidate = strings.Repeat("d", 40)
			}
			if tc.name == "dirty candidate" {
				write(t, filepath.Join(g.root, "dirty"), []byte("dirty"))
			}
			if strings.Contains(tc.name, "missing configuration") || tc.name == "removed owner mapping" || tc.name == "unchanged" || tc.name == "unrelated change" {
				config = ""
			}
			if tc.name == "missing config file" {
				config += ".missing"
			}
			if tc.name == "stale existing report" {
				write(t, reportPath, report)
			}
			calls := []string{}
			g.runTest = func(pkg, name string) error {
				calls = append(calls, name)
				if name == rankingTest {
					if tc.name == "ranking failure" {
						return errors.New("ranking failed")
					}
					return nil
				}
				switch tc.name {
				case "skipped live run":
					return namedTestPass(strings.NewReader(events(pkg, name, "skip")), pkg, name)
				case "unselected live run":
					return namedTestPass(strings.NewReader(""), pkg, name)
				case "failed live run":
					return errors.New("live execution failed")
				case "missing report":
					return nil
				case "stale report digest":
					report = []byte(strings.Replace(string(report), `"query_set_sha256":"`, `"query_set_sha256":"stale`, 1))
				case "failed report":
					report = []byte(strings.ReplaceAll(string(report), `"PASS"`, `"NOT_RUN"`))
				case "incomplete report":
					var r map[string]any
					_ = json.Unmarshal(report, &r)
					r["repositories"] = []any{}
					report, _ = json.Marshal(r)
				case "failed repository":
					report = []byte(strings.Replace(string(report), `"queries":25`, `"queries":0`, 1))
				case "dirty after run":
					write(t, filepath.Join(g.root, "dirty"), []byte("dirty"))
				}
				write(t, reportPath, report)
				return nil
			}
			got, err := g.check(candidate, config)
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) || strings.HasPrefix(got, "PASS") {
					t.Fatalf("got %q, %v; want refusal %q", got, err, tc.want)
				}
				return
			}
			if err != nil || !strings.Contains(got, "PASS candidate="+candidate) {
				t.Fatalf("got %q, %v", got, err)
			}
			if tc.name == "live success" {
				if strings.Join(calls, ",") != qualificationTest+","+rankingTest+","+rankingTest || !strings.Contains(got, fmt.Sprintf("report_sha256=%x", sha256.Sum256(report))) {
					t.Fatalf("unbound success: %s; calls %v", got, calls)
				}
			} else if len(calls) != 0 {
				t.Fatalf("unchanged scope ran tests: %v", calls)
			}
		})
	}
}

func events(pkg, name, action string) string {
	p := "github.com/Beamfall/corvint/" + strings.TrimPrefix(pkg, "./")
	return fmt.Sprintf("{\"Action\":\"run\",\"Package\":%q,\"Test\":%q}\n{\"Action\":%q,\"Package\":%q,\"Test\":%q}\n{\"Action\":\"pass\",\"Package\":%q}\n", p, name, action, p, name, p)
}

// RCF-V1-013: a package-level pass cannot substitute for the named live test.
func TestNamedTestPass(t *testing.T) {
	const pkg = "./cmd/corvint-docs"
	for _, tc := range []struct {
		name, input string
		pass        bool
	}{
		{"pass", events(pkg, qualificationTest, "pass"), true},
		{"skip", events(pkg, qualificationTest, "skip"), false},
		{"fail", events(pkg, qualificationTest, "fail"), false},
		{"empty", "", false},
		{"different test", events(pkg, "Other", "pass"), false},
		{"different package", events("./other", qualificationTest, "pass"), false},
		{"malformed", "not json\n", false},
		{"duplicate", events(pkg, qualificationTest, "pass") + events(pkg, qualificationTest, "pass"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := namedTestPass(strings.NewReader(tc.input), pkg, qualificationTest)
			if (err == nil) != tc.pass {
				t.Fatalf("error %v; want pass=%v", err, tc.pass)
			}
		})
	}
}

// RCF-V1-013: malformed trigger configuration never means no affected paths.
func TestTriggerPatterns(t *testing.T) {
	for _, p := range []string{"", "../x", "a/**b", "a//b", "a/[b]", "a\\b"} {
		if _, err := triggerPatterns(selection(p)); err == nil {
			t.Errorf("accepted %q", p)
		}
	}
	if _, err := triggerPatterns(selection("rank/*.go", "corpus/**")); err != nil {
		t.Fatal(err)
	}
}

// RCF-V1-013, V1-5692: ambiguous or invalid declarations cannot narrow triggers
// or proceed to live qualification, even with an otherwise unchanged candidate.
func TestTriggerDeclarationsFailClosed(t *testing.T) {
	valid := string(selection("corpus/**", "rank/*.go"))
	for _, tc := range []struct{ name, raw, want string }{
		{"duplicate owners", strings.Replace(valid, `"owners":`, `"owners":[],"owners":`, 1), `duplicate object member name "owners"`},
		{"duplicate paths", strings.Replace(valid, `"paths":"rank/*.go"`, `"paths":"rank/*.go","paths":"unrelated/**"`, 1), `duplicate object member name "paths"`},
		{"unknown member", strings.Replace(valid, `"owners":`, `"owner":[],"owners":`, 1), `unknown object member name "owner"`},
		{"unknown owner member", strings.Replace(valid, `"paths":"rank/*.go"`, `"path":"rank/*.go","paths":"unrelated/**"`, 1), `unknown object member name "path"`},
		{"invalid unrelated owner", strings.Replace(valid, `"owners":[`, `"owners":[{"paths":"../invalid","tests":["other/test.go"]},`, 1), `owners paths "../invalid"`},
		{"invalid mustRun", strings.Replace(valid, `"owners":`, `"mustRun":[{"tests":"other/test.go","reason":""}],"owners":`, 1), `reason must be`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, _, _, _ := currencyFixture(t)
			write(t, filepath.Join(g.root, selectionPath), []byte(tc.raw))
			candidate := commit(t, g.root)
			g.runTest = func(_, _ string) error {
				t.Fatal("invalid declaration reached live qualification")
				return nil
			}
			result, err := g.check(candidate, "")
			if result != "" || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("result=%q err=%v; want no result and %q", result, err, tc.want)
			}
		})
	}
}

// RCF-V1-013: exercise the real subprocess/event adapter with a temporary
// heavy wrapper; no private repositories or nested Go tests are needed.
func TestLiveTest(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is unavailable")
	}
	// A configured remote must not receive qualification with host-local inputs.
	t.Setenv("HEAVY_REMOTE", "1")
	t.Setenv("HEAVY_REMOTE_HOST", "qualification-remote.invalid")
	t.Setenv("HEAVY_LOCAL_REASON", "")
	for _, action := range []string{"pass", "skip", "fail"} {
		t.Run(action, func(t *testing.T) {
			root := t.TempDir()
			const pkg = "./cmd/corvint-docs"
			script := "#!/usr/bin/env bash\nset -eu\n" +
				"test \"$HEAVY_REMOTE_HOST\" = qualification-remote.invalid\n" +
				"test \"$HEAVY_REMOTE\" = 0\n" +
				"test -n \"$HEAVY_LOCAL_REASON\"\n" +
				"test \"$*\" = 'go test -json -count=1 -timeout 30m -run ^" + qualificationTest + "$ " + pkg + "'\n" +
				"cat <<'EVENTS'\n" + events(pkg, qualificationTest, action) + "EVENTS\n"
			write(t, filepath.Join(root, "script", "heavy.sh"), []byte(script))
			err := (gate{root: root}).liveTest(pkg, qualificationTest)
			if (err == nil) != (action == "pass") {
				t.Fatalf("action %s returned %v", action, err)
			}
		})
	}
}
