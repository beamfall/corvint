package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func expectFailure(t *testing.T, want string, fn func()) {
	t.Helper()
	defer func() {
		e := recover()
		if e == nil {
			t.Errorf("expected %s", want)
			return
		}
		err, ok := e.(conformanceError)
		if !ok {
			panic(e)
		}
		if !strings.Contains(string(err), want) {
			t.Errorf("wanted %s, got %s", want, err)
		}
	}()
	fn()
}
func testFixture(t *testing.T, id string) (map[string]any, string, string, string) {
	t.Helper()
	for _, c := range loadCases(".") {
		if c["id"] == id {
			root := resolvedPath(t.TempDir())
			workspace, output := filepath.Join(root, "workspace"), filepath.Join(root, "site")
			must(true, os.Mkdir(output, 0700))
			materializeFixture(filepath.Join("fixtures", str(c["fixture"])), workspace)
			return c, workspace, output, treeHash(workspace)
		}
	}
	t.Fatal("missing case", id)
	return nil, "", "", ""
}
func testPlan(workspace, sourceDigest string) map[string]any {
	config := filepath.Join(workspace, "mkdocs.yml")
	raw := readBounded(config, "config")
	start := bytes.Index(raw, []byte("nav:"))
	end := len(raw)
	authority := hash([]byte("null"))
	if start < 0 {
		start = len(raw)
	} else {
		n := bytes.Index(raw[start:], []byte("\ntheme:"))
		if n >= 0 {
			end = start + n + 1
		}
		authority = hash(raw[start:end])
	}
	nav := "nav:\n  - \"Proposed evidence\": \"proposed-evidence.md\"\n"
	content := "# Proposed evidence summary\n\nReview is required before acceptance.\n"
	evidence := filepath.Join(workspace, "docs/index.md")
	return map[string]any{"applied": false, "claims": []any{map[string]any{"evidence": []any{map[string]any{"lineEnd": 1, "lineStart": 1, "path": "docs/index.md", "sha256": fileHash(evidence)}}, "state": "SUPPORTED", "text": splitLines(string(readBounded(evidence, "evidence")))[0]}, map[string]any{"evidence": []any{}, "state": "UNKNOWN", "text": "Maintainer acceptance is unknown."}}, "navPatches": []any{map[string]any{"anchorSha256": fileHash(config), "authorityNavSha256": authority, "endByte": end, "navOwner": "mkdocs.yml", "operation": "edit_nav", "originalSha256": hash(raw[start:end]), "replacement": nav, "replacementSha256": hash([]byte(nav)), "startByte": start}}, "profile": planProfile, "reviewRequired": true, "sourceTreeSha256": sourceDigest, "writes": []any{map[string]any{"beforeSha256": nil, "endByte": 0, "operation": "create_file", "originalSha256": hash(nil), "path": "docs/proposed-evidence.md", "replacement": content, "replacementSha256": hash([]byte(content)), "startByte": 0}}}
}
func testObservation(c map[string]any, workspace, sourceDigest string) map[string]any {
	allowed := c["allowed"].([]any)[0].(map[string]any)
	for _, a := range c["allowed"].([]any) {
		m := a.(map[string]any)
		if m["decision"] == "ABSTAINED" {
			allowed = m
			break
		}
	}
	var plan any
	if allowed["decision"] == "PLANNED" {
		plan = testPlan(workspace, sourceDigest)
	}
	return map[string]any{"build": nil, "code": allowed["code"], "decision": allowed["decision"], "id": c["id"], "plan": plan, "profile": observationProfile, "repositoryMutated": false, "signals": c["requiredSignals"], "sourceTreeSha256": sourceDigest, "truth": map[string]any{"buildStrict": "NOT_RUN", "offline": "NOT_OBSERVED"}}
}
func testBuild(workspace, output string, b bindings, offline bool) map[string]any {
	must(true, os.WriteFile(filepath.Join(output, "index.html"), []byte("<!doctype html><title>Corvint</title>\n"), 0600))
	status := "NOT_OBSERVED"
	if offline {
		status = "PASS"
	}
	return map[string]any{"argv": []any{"/project/.venv/bin/mkdocs", "build", "--strict", "--clean", "--config-file", filepath.Join(workspace, "mkdocs.yml"), "--site-dir", output}, "configSha256": fileHash(filepath.Join(workspace, "mkdocs.yml")), "environmentSha256": b.environmentHash, "exitCode": 0, "markdownVersion": "3.8", "materialVersion": "9.6.0", "mkdocsExecutable": "/project/.venv/bin/mkdocs", "mkdocsVersion": "1.6.1", "networkDenial": map[string]any{"harness": b.denialHarness, "receiptSha256": b.denialHash, "status": status}, "outputTreeSha256": treeHash(output), "profile": buildProfile, "pymdownVersion": "10.16", "pythonExecutable": "/project/.venv/bin/python", "status": "PASS", "strict": true}
}
func testBindings() bindings {
	return bindings{environmentHash: strings.Repeat("a", 64), environment: map[string]any{"markdownVersion": "3.8", "materialVersion": "9.6.0", "mkdocsExecutable": "/project/.venv/bin/mkdocs", "mkdocsVersion": "1.6.1", "pymdownVersion": "10.16", "pythonExecutable": "/project/.venv/bin/python"}}
}
func TestHDCCorpusAndHonestObservations(t *testing.T) {
	cases := loadCases(".")
	if len(cases) != 19 {
		t.Fatal(len(cases))
	}
	if corpusHash(".", cases) != corpusHash(".", cases) {
		t.Fatal("nondeterministic")
	}
	r, status := run(context.Background(), []string{"--corpus", "."})
	if status != 0 || r["execution"] != "NOT_RUN" {
		t.Fatal(r)
	}
	for _, id := range caseIDs {
		t.Run(id, func(t *testing.T) {
			c, w, o, d := testFixture(t, id)
			before := treeHash(w)
			observation := testObservation(c, w, d)
			validateObservation(c, w, o, d, canonical(observation, true), bindings{})
			if before != treeHash(w) {
				t.Fatal("mutated")
			}
		})
	}
}
func TestHDCEnvironmentBindingAndPrivacy(t *testing.T) {
	root := resolvedPath(t.TempDir())
	lock := filepath.Join(root, "requirements.lock")
	must(true, os.WriteFile(lock, []byte("mkdocs==1.6.1\nmkdocs-material==9.6.0\n"), 0600))
	executable := resolvedPath(must(os.Executable()))
	value := map[string]any{"authority": "project-owned", "lockFile": lock, "lockFileSha256": fileHash(lock), "markdownVersion": "3.8", "materialVersion": "9.6.0", "mkdocsExecutable": executable, "mkdocsVersion": "1.6.1", "profile": environmentProfile, "pymdownVersion": "10.16", "pythonExecutable": executable, "trusted": true}
	p := filepath.Join(root, "environment.json")
	must(true, os.WriteFile(p, canonical(value, true), 0600))
	b := loadBindings(p, "", "")
	if b.environmentPath != p || b.environmentHash != fileHash(p) || b.environment["mkdocsVersion"] != "1.6.1" {
		t.Fatal(b)
	}
	must(true, os.WriteFile(lock, []byte("tampered\n"), 0600))
	expectFailure(t, "lock-digest-mismatch", func() { loadBindings(p, "", "") })
	for _, name := range []string{"AWS_SECRET_ACCESS_KEY", "GITHUB_TOKEN", "HTTPS_PROXY", "NO_PROXY"} {
		t.Setenv(name, "must-not-leak")
	}
	c, w, o, d := testFixture(t, "corvint-material-plan")
	env := scrubbedEnvironment(c, w, o, d, root, bindings{})
	if strings.Contains(strings.Join(env, "\n"), "must-not-leak") {
		t.Fatal(env)
	}
	denial := filepath.Join(root, "denial.json")
	must(true, os.WriteFile(denial, []byte("not an attestation\n"), 0600))
	expectFailure(t, "invalid-json", func() { loadBindings("", denial, "project-network-sandbox-v1") })
}
func TestHDCBuildReceiptsCannotInventObservation(t *testing.T) {
	for _, offline := range []bool{false, true} {
		name := "strict-build-project-environment"
		if offline {
			name = "offline-qualification-external-harness"
		}
		t.Run(name, func(t *testing.T) {
			c, w, o, d := testFixture(t, name)
			b := testBindings()
			if offline {
				b.denialHash = strings.Repeat("b", 64)
				b.denialHarness = "project-network-sandbox-v1"
				b.denial = map[string]any{"environmentSha256": b.environmentHash, "harness": b.denialHarness, "network": "DENIED", "profile": denialProfile, "scope": "PROCESS_TREE", "status": "PASS"}
			}
			obs := testObservation(c, w, d)
			obs["build"] = testBuild(w, o, b, offline)
			obs["decision"] = "BUILT"
			obs["code"] = "strict-build-pass"
			truth := map[string]any{"buildStrict": "PASS", "offline": "UNKNOWN"}
			if offline {
				obs["code"] = "offline-qualified"
				truth["offline"] = "QUALIFIED"
			}
			obs["truth"] = truth
			want := "build-not-observed-by-runner"
			if offline {
				want = "offline-observer-not-executed"
				expectFailure(t, "unbound-network-denial", func() { validateObservation(c, w, o, d, canonical(obs, true), testBindings()) })
			}
			expectFailure(t, want, func() { validateObservation(c, w, o, d, canonical(obs, true), b) })
		})
	}
}
func TestHDCObservationRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		mutate     func(map[string]any)
	}{{"stale", "stale-source", func(o map[string]any) { o["sourceTreeSha256"] = strings.Repeat("f", 64) }}, {"hallucinated", "claim-not-exactly-supported", func(o map[string]any) {
		o["plan"].(map[string]any)["claims"].([]any)[0].(map[string]any)["text"] = "This project is production-qualified everywhere."
	}}, {"mutation", "repository-mutation", func(o map[string]any) { o["repositoryMutated"] = true }}, {"review", "mutation-or-review-bypass", func(o map[string]any) { o["plan"].(map[string]any)["reviewRequired"] = false }}} {
		t.Run(tc.name, func(t *testing.T) {
			c, w, o, d := testFixture(t, "corvint-material-plan")
			obs := testObservation(c, w, d)
			tc.mutate(obs)
			expectFailure(t, tc.want, func() { validateObservation(c, w, o, d, canonical(obs, true), bindings{}) })
		})
	}
	t.Run("symlink", func(t *testing.T) {
		_, w, _, _ := testFixture(t, "docs-dir-symlink-escape")
		expectFailure(t, "symlink", func() { containedFile(w, "docs-link/index.md", "symlink-evidence") })
	})
	t.Run("hostile-pass", func(t *testing.T) {
		c, w, o, d := testFixture(t, "unknown-plugin-canary")
		obs := testObservation(c, w, d)
		obs["truth"].(map[string]any)["buildStrict"] = "PASS"
		expectFailure(t, "strict-pass-on-hostile-case", func() { validateObservation(c, w, o, d, canonical(obs, true), bindings{}) })
	})
	t.Run("empty-output", func(t *testing.T) {
		_, w, o, _ := testFixture(t, "strict-build-project-environment")
		b := testBindings()
		build := testBuild(w, o, b, false)
		must(true, os.Remove(filepath.Join(o, "index.html")))
		expectFailure(t, "expected-output-missing", func() { validateBuild(w, o, build, "build", false, b) })
	})
	t.Run("argv-identity", func(t *testing.T) {
		_, w, o, _ := testFixture(t, "strict-build-project-environment")
		b := testBindings()
		build := testBuild(w, o, b, false)
		build["argv"].([]any)[0] = "/other/mkdocs"
		expectFailure(t, "argv-executable-mismatch", func() { validateBuild(w, o, build, "build", false, b) })
	})
	t.Run("duplicate-json", func(t *testing.T) {
		expectFailure(t, "duplicate-key:profile", func() { parseJSON([]byte(`{"profile":"one","profile":"two"}`), "observation") })
	})
}
