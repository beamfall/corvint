package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const runsClosureA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// runsClosureDeclared is runsDeclared with config digest config and a DECLARED closure.
func runsClosureDeclared(config, blob string) string {
	return `"corvintRunIdentity":{"applicationRevision":"app-1","configDigest":"` + config + `","testFileBlobs":{"cart.spec.ts":"` + blob +
		`"},"testClosureRule":"DECLARED","testClosureDigests":{"cart.spec.ts":"` + runsClosureA + `"}}`
}

// runsSidecar is an identity sidecar binding the declaration to report.
func runsSidecar(report, identity string) string {
	sum := sha256.Sum256([]byte(report))
	return `{"profile":"corvint-run-identity-sidecar/0","reportSha256":"` + hex.EncodeToString(sum[:]) + `",` + identity + `}`
}

// PWR-V0-065, PWR-V0-067: --identity and --identity-sidecars bind sidecars on every runs read and
// state each report's identity source; a bad sidecar refuses naming the report.
func TestPWRV0065_RunsIdentitySidecars(t *testing.T) {
	dir := t.TempDir()
	bare1, bare2 := runsReport("passed", ""), strings.Replace(runsReport("passed", ""), `"duration":10`, `"duration":11`, 1)
	writeRunsReports(t, dir, map[string]string{
		"a.json": bare1, "b.json": bare2, "e.json": runsReport("passed", runsDeclared), "n.json": strings.Replace(runsReport("passed", ""), `"duration":10`, `"duration":12`, 1),
		"a.json.identity.json": runsSidecar(bare1, runsDeclared), "b.json.identity.json": runsSidecar(bare2, runsDeclared),
		"x.identity.json": runsSidecar(bare1, runsDeclared),
	})
	code, out, errs := runsCLI(t, "--root", dir, "runs", "stability", "--runs", "a.json,b.json,e.json,n.json", "--identity-sidecars", "--min-runs", "3")
	var doc struct {
		Runs []struct {
			IdentitySource string `json:"identitySource"`
		} `json:"runs"`
		Tests []struct {
			Verdict string `json:"verdict"`
			Runs    []int  `json:"runs"`
		} `json:"tests"`
	}
	decodeRuns(t, out, &doc)
	if code != 0 || len(doc.Runs) != 4 || doc.Runs[0].IdentitySource != "SIDECAR" || doc.Runs[1].IdentitySource != "SIDECAR" ||
		doc.Runs[2].IdentitySource != "EMBEDDED" || doc.Runs[3].IdentitySource != "NOT_DECLARED" || len(doc.Tests) != 1 || doc.Tests[0].Verdict != "DETERMINISTIC" {
		t.Fatalf("sidecars: %d %s %s", code, out, errs)
	}
	code, out, errs = runsCLI(t, "--root", dir, "runs", "stability", "--runs", "a.json,n.json", "--identity", "n.json=x.identity.json")
	var d runsRefusalDoc
	decodeRuns(t, errs, &d)
	if code != 2 || out != "" || d.Code != "runs-report-unavailable" || d.Reason != "IDENTITY_SIDECAR_MISMATCH" || d.Option != "runs" || d.Run != 2 {
		t.Fatalf("mismatch: %d %s %s", code, out, errs)
	}
	code, out, errs = runsCLI(t, "--root", dir, "runs", "stability", "--runs", "e.json", "--identity", "e.json="+filepath.Join(dir, "x.identity.json"))
	decodeRuns(t, errs, &d)
	if code != 2 || d.Reason != "IDENTITY_SIDECAR_MISMATCH" {
		t.Fatalf("embedded with foreign sidecar: %d %s %s", code, out, errs)
	}
	_, plain, _ := runsCLI(t, "--root", dir, "runs", "stability", "--runs", "a.json,b.json")
	if _, out, _ := runsCLI(t, "--root", dir, "runs", "stability", "--runs", "a.json,b.json"); strings.Contains(out, "identitySource") || plain != out {
		t.Fatalf("unrequested identity source: %s", out)
	}
	for _, c := range []struct {
		args   []string
		option string
	}{
		{[]string{"runs", "stability", "--runs", "a.json", "--identity", "other.json=x.identity.json"}, "identity"},
		{[]string{"runs", "stability", "--runs", "a.json", "--identity", "a.json"}, "identity"},
		{[]string{"runs", "stability", "--runs", "a.json", "--identity", "a.json=x", "--identity", "a.json=y"}, "identity"},
	} {
		code, out, errs := runsCLI(t, append([]string{"--root", dir}, c.args...)...)
		decodeRuns(t, errs, &d)
		if code != 2 || out != "" || d.Code != "runs-invalid-arguments" || d.Reason != "VALUE_INVALID" || d.Option != c.option {
			t.Errorf("%q: %d %s %s", c.args, code, out, errs)
		}
	}
	// The other reads bind sidecars too.
	code, out, errs = runsCLI(t, "--root", dir, "runs", "compare", "--baseline", "a.json", "--candidate", "b.json", "--identity-sidecars")
	if code != 0 || strings.Count(out, `"identitySource":"SIDECAR"`) != 2 {
		t.Fatalf("compare: %d %s %s", code, out, errs)
	}
	code, out, errs = runsCLI(t, "--root", dir, "runs", "redundancy", "--runs", "a.json,n.json", "--identity-sidecars")
	if code != 0 || !strings.Contains(out, `"identitySource":"SIDECAR"`) || !strings.Contains(out, `"identitySource":"NOT_DECLARED"`) {
		t.Fatalf("redundancy: %d %s %s", code, out, errs)
	}
	code, out, errs = runsCLI(t, "--root", dir, "runs", "reusable", "--selection", "a.json", "--prior", "b.json", "--identity", "a.json=a.json.identity.json")
	if code != 0 || !strings.Contains(out, `"identitySource":"SIDECAR"`) || !strings.Contains(out, `"identitySource":"NOT_DECLARED"`) {
		t.Fatalf("reusable: %d %s %s", code, out, errs)
	}
	if err := os.WriteFile(filepath.Join(dir, "n.json.identity.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, errs = runsCLI(t, "--root", dir, "runs", "redundancy", "--runs", "a.json,n.json", "--identity-sidecars")
	decodeRuns(t, errs, &d)
	if code != 2 || d.Reason != "IDENTITY_SIDECAR_INVALID" || d.Option != "runs" || d.Run != 2 {
		t.Fatalf("invalid present sidecar: %d %s", code, errs)
	}
}

// PWR-V0-065, PWR-V0-067: baseline record binds sidecars and states each evidence's source.
func TestPWRV0067_RunsBaselineIdentitySource(t *testing.T) {
	repo := t.TempDir()
	reuseGit(t, repo, "init", "-q")
	reuseGit(t, repo, "config", "user.email", "corvint@example.test")
	reuseGit(t, repo, "config", "user.name", "Corvint Test")
	reuseCommit(t, repo, map[string]string{"a.txt": "1"})
	// The sidecar's applicationRevision is checked against --rev like an embedded one.
	head := strings.TrimSpace(reuseGit(t, repo, "rev-parse", "HEAD"))
	declared := strings.Replace(runsDeclared, "app-1", head, 1)
	full := runsReport("failed", "")
	other := strings.Replace(full, `"duration":10`, `"duration":12`, 1)
	writeRunsReports(t, repo, map[string]string{"full.json": full, "iso.json": runsReport("failed", declared), "full.json.identity.json": runsSidecar(full, declared),
		"other.json": other, "other.json.identity.json": runsSidecar(other, runsDeclared)})
	code, out, errs := runsCLI(t, "--root", repo, "runs", "baseline", "record", "--rev", "HEAD", "--full", "other.json", "--isolated", "iso.json", "--identity-sidecars")
	var d runsRefusalDoc
	decodeRuns(t, errs, &d)
	if code != 2 || out != "" || d.Reason != "APPLICATION_REVISION_DIFFERS" || d.Option != "full" {
		t.Fatalf("sidecar revision not checked: %d %s", code, errs)
	}
	code, out, errs = runsCLI(t, "--root", repo, "runs", "baseline", "record", "--rev", "HEAD", "--full", "full.json", "--isolated", "iso.json", "--identity-sidecars")
	if code != 0 || !strings.Contains(out, `"identitySource":"SIDECAR"`) || !strings.Contains(out, `"identitySource":"EMBEDDED"`) {
		t.Fatalf("record: %d %s %s", code, out, errs)
	}
	if err := os.WriteFile(filepath.Join(repo, "baseline.json"), []byte(out), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errs = runsCLI(t, "--root", repo, "runs", "baseline", "diff", "--baseline", "baseline.json", "--runs", "iso.json", "--identity-sidecars")
	if code != 0 || !strings.Contains(out, `"identitySource":"EMBEDDED"`) {
		t.Fatalf("diff: %d %s %s", code, out, errs)
	}
}

// PWR-V0-069, PWR-V0-070: --pool-by closure pools across a config change and refuses a closure
// claimed over two test-file blobs.
func TestPWRV0069_RunsStabilityPoolByClosure(t *testing.T) {
	dir := t.TempDir()
	writeRunsReports(t, dir, map[string]string{
		"a.json": runsReport("passed", runsClosureDeclared("cfg-1", "blob-1")),
		"b.json": runsReport("passed", runsClosureDeclared("cfg-2", "blob-1")),
		"c.json": runsReport("passed", runsClosureDeclared("cfg-1", "blob-2")),
	})
	code, out, errs := runsCLI(t, "--root", dir, "runs", "stability", "--runs", "a.json,b.json", "--min-runs", "2", "--pool-by", "closure")
	var doc struct {
		PoolBy string `json:"poolBy"`
		Runs   []struct {
			ClosureSource string `json:"closureSource"`
		} `json:"runs"`
		Tests []struct {
			Verdict string `json:"verdict"`
		} `json:"tests"`
	}
	decodeRuns(t, out, &doc)
	if code != 0 || doc.PoolBy != "closure" || len(doc.Tests) != 1 || doc.Tests[0].Verdict != "DETERMINISTIC" || doc.Runs[1].ClosureSource != "DECLARED" {
		t.Fatalf("closure pooling: %d %s %s", code, out, errs)
	}
	code, out, _ = runsCLI(t, "--root", dir, "runs", "stability", "--runs", "a.json,b.json", "--min-runs", "2", "--pool-by", "inputs")
	if code != 0 || strings.Contains(out, "poolBy") || strings.Count(out, `"verdict":"INSUFFICIENT"`) != 2 {
		t.Fatalf("inputs pooling: %d %s", code, out)
	}
	code, out, errs = runsCLI(t, "--root", dir, "runs", "stability", "--runs", "a.json,b.json,c.json", "--pool-by", "closure")
	var d runsRefusalDoc
	decodeRuns(t, errs, &d)
	if code != 2 || out != "" || d.Code != "runs-report-unavailable" || d.Reason != "CLOSURE_DIGEST_CONTRADICTED" || d.Option != "pool-by" || d.Run != 3 ||
		!strings.Contains(errs, `"conflictsWith":1`) {
		t.Fatalf("contradiction: %d %s %s", code, out, errs)
	}
	for _, args := range [][]string{{"--pool-by", "files"}, {"--pool-by", "closure", "--pool-by", "closure"}} {
		code, _, errs := runsCLI(t, append([]string{"--root", dir, "runs", "stability", "--runs", "a.json"}, args...)...)
		decodeRuns(t, errs, &d)
		if code != 2 || d.Reason != "VALUE_INVALID" || d.Option != "pool-by" {
			t.Errorf("%q: %d %s", args, code, errs)
		}
	}
}

// PWR-V0-071: runs identity --closure-digests declares caller closures and refuses a bad file.
func TestPWRV0071_RunsIdentityClosureDigests(t *testing.T) {
	root := t.TempDir()
	reuseGit(t, root, "init", "-q")
	reuseGit(t, root, "config", "user.email", "corvint@example.test")
	reuseGit(t, root, "config", "user.name", "Corvint Test")
	reuseCommit(t, root, map[string]string{"playwright.config.ts": "export default {}\n", "cart.spec.ts": "test('x', () => {})\n"})
	writeRunsReports(t, root, map[string]string{
		"closures.json": `{"cart.spec.ts":"` + runsClosureA + `"}`,
		"unknown.json":  `{"other.spec.ts":"` + runsClosureA + `"}`,
		"bad.json":      `{"cart.spec.ts":"sha256:abc"}`,
	})
	code, out, errs := runsCLI(t, "--root", root, "runs", "identity", "--config", "playwright.config.ts", "--closure-digests", "closures.json")
	if code != 0 || !strings.Contains(out, `"testClosureRule":"DECLARED"`) || !strings.Contains(out, `"testClosureDigests":{"cart.spec.ts":"`+runsClosureA+`"}`) {
		t.Fatalf("declared: %d %s %s", code, out, errs)
	}
	for _, file := range []string{"unknown.json", "bad.json", "absent.json", "."} {
		code, out, errs := runsCLI(t, "--root", root, "runs", "identity", "--config", "playwright.config.ts", "--closure-digests", file)
		var d runsRefusalDoc
		decodeRuns(t, errs, &d)
		if code != 2 || out != "" || d.Code != "runs-identity-unavailable" || d.Reason != "CLOSURE_DECLARATION_INVALID" {
			t.Errorf("%s: %d %s %s", file, code, out, errs)
		}
	}
}

// PWR-V0-072: runs identity --compute-closures prints computed closure digests, and the usage
// names it and the sidecar options on every command that takes them.
func TestPWRV0072_RunsIdentityComputeClosures(t *testing.T) {
	root := t.TempDir()
	reuseGit(t, root, "init", "-q")
	reuseGit(t, root, "config", "user.email", "corvint@example.test")
	reuseGit(t, root, "config", "user.name", "Corvint Test")
	reuseCommit(t, root, map[string]string{
		"playwright.config.ts": "export default defineConfig({ testDir: 'e2e', projects: [{ name: 'main' }] });\n",
		"e2e/cart.spec.ts":     "test('x', () => {})\n",
	})
	code, out, errs := runsCLI(t, "--root", root, "runs", "identity", "--config", "playwright.config.ts", "--test-dir", "e2e", "--compute-closures")
	if code != 0 || !strings.Contains(out, `"testClosureRule":"corvint-test-closure/0"`) || !strings.Contains(out, `"testClosureDigests":{"cart.spec.ts":"sha256:`) ||
		!strings.Contains(out, `"closure":{"source":"COMPUTED"}`) {
		t.Fatalf("computed: %d %s %s", code, out, errs)
	}
	code, out, errs = runsCLI(t, "--root", root, "runs", "identity", "--config", "playwright.config.ts", "--test-dir", "e2e")
	if code != 0 || strings.Contains(out, "closure") {
		t.Fatalf("default output carries a closure: %d %s %s", code, out, errs)
	}
	if got := strings.Count(runsHelp, "[--identity REPORT=FILE...] [--identity-sidecars]"); got != 6 || !strings.Contains(runsHelp, "[--compute-closures]") {
		t.Fatalf("usage names the sidecar options %d times, want 6", got)
	}
}
