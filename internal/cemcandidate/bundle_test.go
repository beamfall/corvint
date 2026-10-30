package cemcandidate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/cli"
	"github.com/Beamfall/corvint/internal/cem/verify"
)

// bundleCase is a product repository the bundle command may only read, a
// request built from a cem prepare/cite map produced in a disposable clone of
// it, and an alias directory that is a symlink to the real temporary
// directory, as macOS /var is to /private/var.
type bundleCase struct {
	real, alias, repo, request string
	r                          PlainRequest
}

func newBundleCase(t *testing.T) bundleCase {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	caseRoot := neutralCaseRoot(t)
	temp, repo, base, target := recipeRepository(t, caseRoot)
	clone := filepath.Join(temp, "clone")
	fixtureGit(t, temp, "", "clone", "-q", "--no-hardlinks", repo, clone)
	source, produced := producerMap(t, clone, base, target, map[string]string{"1": "6:6", "2": "6:6"})
	alias := filepath.Join(realTemp(t), "alias")
	if e := os.Symlink(temp, alias); e != nil {
		t.Fatal(e)
	}
	inputs := filepath.Join(temp, "inputs")
	if e := os.Mkdir(inputs, 0o700); e != nil {
		t.Fatal(e)
	}
	ref := func(name string, raw []byte) FileRef {
		pin := save(t, filepath.Join(inputs, name), raw)
		pin.Path = filepath.Join(alias, "inputs", name)
		return pin
	}
	artifact := func(name string) []byte {
		b, e := os.ReadFile(filepath.Join(caseRoot, "artifacts", name))
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	evidence := source.Change.Evidence[0].ID
	r := PlainRequest{Profile: PlainAssemblyProfile, Repository: filepath.Join(alias, "repo"), ExpectedBase: base, Target: target,
		SourceMap: ref("source.json", produced), RunnerPlan: ref("runner-plan.json", artifact("runner-plan.json")), RunnerReceipt: ref("runner-receipt.json", artifact("runner-receipt.json")),
		Criteria: []PlainCriterion{{CriterionEvidenceID: evidence, HunkIDs: []string{source.Change.Hunks[0].ID, source.Change.Hunks[1].ID}, EvidenceIDs: []string{evidence}}}}
	save(t, filepath.Join(inputs, "request.json"), encode(t, r))
	return bundleCase{real: temp, alias: alias, repo: repo, request: filepath.Join(alias, "inputs", "request.json"), r: r}
}

// snapshot records every path under root with its mode and content digest.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	e := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		entry := info.Mode().String()
		if info.Mode().IsRegular() {
			b, e := os.ReadFile(p)
			if e != nil {
				return e
			}
			sum := sha256.Sum256(b)
			entry += " " + hex.EncodeToString(sum[:])
		}
		files[p] = entry
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	return files
}

// readOnly removes write permission from every path under root until the
// test ends, so a write into the product repository fails as well as shows.
func readOnly(t *testing.T, root string) {
	t.Helper()
	var dirs []string
	e := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		if d.IsDir() {
			dirs = append(dirs, p)
		}
		return os.Chmod(p, info.Mode().Perm()&^0o222)
	})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		for _, dir := range dirs {
			_ = os.Chmod(dir, 0o755)
		}
	})
}

type bundleOutput struct {
	OK           bool       `json:"ok"`
	Profile      string     `json:"profile"`
	StablePath   string     `json:"stablePath"`
	StableSha256 string     `json:"stableSha256"`
	NextActions  [][]string `json:"nextActions"`
	Verification struct {
		Outcome string `json:"outcome"`
	} `json:"verification"`
}

func runBundle(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code, handled := RunBundleInvocation(context.Background(), args, &stdout, &stderr)
	if !handled {
		t.Fatalf("%q was not handled", args)
	}
	return code, stdout.String(), stderr.String()
}

// verifyStable runs the bundle's verify-stable next action and returns its
// exit status, accept and verdictSha256.
func verifyStable(t *testing.T, next []string) (int, bool, string) {
	t.Helper()
	if len(next) < 3 || next[0] != "corvint" || next[1] != "cem" || next[2] != "verify-stable" {
		t.Fatalf("next action %q is not cem verify-stable", next)
	}
	var stdout, stderr bytes.Buffer
	code, handled := cli.RunStableInvocation(context.Background(), next[1:], &stdout, &stderr)
	if !handled {
		t.Fatalf("%q was not handled", next)
	}
	var result struct {
		Accept        *bool  `json:"accept"`
		VerdictSha256 string `json:"verdictSha256"`
	}
	if e := json.Unmarshal(stdout.Bytes(), &result); e != nil {
		t.Fatalf("verify-stable output %q: %v", &stdout, e)
	}
	return code, result.Accept != nil && *result.Accept, result.VerdictSha256
}

// TestCSBV0001To003BundleFromDisposableCloneVerifiesOutOfTree is the
// end-to-end route of #1767: cem prepare and cem cite run in a disposable
// clone, cem bundle assembles a stable bundle for the read-only product
// repository, and cem verify-stable, run as the printed next action, accepts
// it with the same verdictSha256 on two independent bundles, while a
// tampered bundle is refused and the product repository is unchanged.
func TestCSBV0001To003BundleFromDisposableCloneVerifiesOutOfTree(t *testing.T) {
	c := newBundleCase(t)
	readOnly(t, c.repo)
	before := snapshot(t, c.repo)
	verdicts := map[string]bool{}
	stables := map[string]bool{}
	var outs []string
	for _, name := range []string{"bundle-1", "bundle-2"} {
		out := filepath.Join(c.alias, name)
		code, stdout, stderr := runBundle(t, "cem", "bundle", "--request", c.request, "--out", out)
		if code != 0 || stderr != "" {
			t.Fatalf("cem bundle: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !strings.HasPrefix(stdout, `{"ok":true,"profile":"cem-stable-plain-assembly-result/1",`) {
			t.Fatalf("cem bundle output %q", stdout)
		}
		var result bundleOutput
		if e := json.Unmarshal([]byte(stdout), &result); e != nil {
			t.Fatal(e)
		}
		realOut := filepath.Join(c.real, name)
		if result.Verification.Outcome != "ACCEPT" || result.StablePath != filepath.Join(realOut, "stable.json") || len(result.NextActions) != 1 {
			t.Fatalf("cem bundle result %+v", result)
		}
		want := []string{"corvint", "cem", "verify-stable", "--repository", c.repo, "--map", result.StablePath, "--expected-base", c.r.ExpectedBase, "--target", c.r.Target, "--artifacts", realOut}
		if !reflect.DeepEqual(result.NextActions[0], want) {
			t.Fatalf("next action %q, want %q", result.NextActions[0], want)
		}
		code, accept, verdict := verifyStable(t, result.NextActions[0])
		if code != 0 || !accept || verdict == "" {
			t.Fatalf("verify-stable: exit=%d accept=%v verdict=%q", code, accept, verdict)
		}
		verdicts[verdict] = true
		stables[result.StableSha256] = true
		outs = append(outs, realOut)
	}
	if len(verdicts) != 1 || len(stables) != 1 {
		t.Fatalf("two bundles gave verdicts %v and stable digests %v, want one each", verdicts, stables)
	}
	// A changed artifact byte, and a map whose receipt record names other
	// bytes, are each refused.
	receipt := c.r.RunnerReceipt.Sha256
	other := strings.Repeat("0", len(receipt))
	for tamper, change := range map[string]func([]byte) []byte{
		"artifacts/runner-receipt.json": func(b []byte) []byte { return append(b, ' ') },
		"stable.json":                   func(b []byte) []byte { return bytes.Replace(b, []byte(receipt), []byte(other), 1) },
	} {
		path := filepath.Join(outs[1], tamper)
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		changed := change(bytes.Clone(b))
		if bytes.Equal(changed, b) {
			t.Fatalf("tamper of %s changed nothing", tamper)
		}
		if e := os.WriteFile(path, changed, 0o600); e != nil {
			t.Fatal(e)
		}
		next := []string{"corvint", "cem", "verify-stable", "--repository", c.repo, "--map", filepath.Join(outs[1], "stable.json"), "--expected-base", c.r.ExpectedBase, "--target", c.r.Target, "--artifacts", outs[1]}
		if code, accept, _ := verifyStable(t, next); code == 0 || accept {
			t.Fatalf("tampered %s: exit=%d accept=%v, want a refusal", tamper, code, accept)
		}
		if e := os.WriteFile(path, b, 0o600); e != nil {
			t.Fatal(e)
		}
	}
	after := snapshot(t, c.repo)
	for p, entry := range after {
		if before[p] != entry {
			t.Errorf("product repository path %s changed: %q -> %q", p, before[p], entry)
		}
	}
	for p := range before {
		if _, ok := after[p]; !ok {
			t.Errorf("product repository path %s was removed", p)
		}
	}
}

// TestCSBV0004BundlePathRules proves the realpath rule: a symlinked final
// element is still refused and names the path, an unresolvable ancestor is
// refused as bundle-path-unresolved naming the path, and no refusal writes a
// map.
func TestCSBV0004BundlePathRules(t *testing.T) {
	c := newBundleCase(t)
	link := filepath.Join(c.real, "inputs", "request-link.json")
	if e := os.Symlink(filepath.Join(c.real, "inputs", "request.json"), link); e != nil {
		t.Fatal(e)
	}
	missing := filepath.Join(c.alias, "no-such-dir", "request.json")
	for name, tc := range map[string]struct {
		request, out, code, names string
	}{
		"symlinked-leaf":       {link, filepath.Join(c.alias, "out-leaf"), "candidate-input", strings.TrimPrefix(link, "/")},
		"missing-request-dir":  {missing, filepath.Join(c.alias, "out-missing"), "bundle-path-unresolved", missing},
		"missing-out-ancestor": {c.request, filepath.Join(c.alias, "no-such-dir", "out"), "bundle-path-unresolved", filepath.Join(c.alias, "no-such-dir", "out")},
	} {
		t.Run(name, func(t *testing.T) {
			code, stdout, stderr := runBundle(t, "cem", "bundle", "--request", tc.request, "--out", tc.out)
			if code != 1 || stdout != "" || !strings.HasPrefix(stderr, `{"code": "`+tc.code+`", `) || !strings.Contains(stderr, tc.names) {
				t.Fatalf("exit=%d stdout=%q stderr=%q, want %s naming %s", code, stdout, stderr, tc.code, tc.names)
			}
			if _, e := os.Lstat(filepath.Join(tc.out, "stable.json")); !os.IsNotExist(e) {
				t.Fatalf("refusal wrote stable.json: %v", e)
			}
		})
	}
	pinned := c.r
	pinned.SourceMap.Path = filepath.Join(c.alias, "no-such-dir", "source.json")
	request := save(t, filepath.Join(c.real, "inputs", "request-missing-pin.json"), encode(t, pinned)).Path
	code, _, stderr := runBundle(t, "cem", "bundle", "--request", request, "--out", filepath.Join(c.alias, "out-pin"))
	if code != 1 || !strings.HasPrefix(stderr, `{"code": "bundle-path-unresolved", `) || !strings.Contains(stderr, pinned.SourceMap.Path) {
		t.Fatalf("unresolvable pin: exit=%d stderr=%q", code, stderr)
	}
	// A verifier REJECT is typed and names its issue codes (CSB-V0-005).
	swapped := c.r
	swapped.ExpectedBase, swapped.Target = c.r.Target, c.r.ExpectedBase
	request = save(t, filepath.Join(c.real, "inputs", "request-swapped.json"), encode(t, swapped)).Path
	code, stdout, stderr := runBundle(t, "cem", "bundle", "--request", request, "--out", filepath.Join(c.alias, "out-swapped"))
	if code != 1 || stdout != "" || !strings.HasPrefix(stderr, `{"code": "bundle-verification-rejected", `) || !strings.Contains(stderr, "base-revision-mismatch") {
		t.Fatalf("verifier refusal: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

// TestCSBV0002And005BundleRefusals proves the closed argv and the plain-only
// profile: each refusal is one typed stderr line and stdout stays empty.
func TestCSBV0002And005BundleRefusals(t *testing.T) {
	temp := realTemp(t)
	tasks := save(t, filepath.Join(temp, "tasks.json"), []byte(`{"profile":"`+StableAssemblyProfile+`"}`)).Path
	notJSON := save(t, filepath.Join(temp, "bad.json"), []byte(`[`)).Path
	out := filepath.Join(temp, "out")
	for name, tc := range map[string]struct {
		args []string
		exit int
		code string
	}{
		"global-root":      {[]string{"--root", temp, "cem", "bundle", "--request", tasks, "--out", out}, 2, "invalid-arguments"},
		"missing-out":      {[]string{"cem", "bundle", "--request", tasks}, 2, "invalid-arguments"},
		"repeated-request": {[]string{"cem", "bundle", "--request", tasks, "--request", tasks, "--out", out}, 2, "invalid-arguments"},
		"unknown-option":   {[]string{"cem", "bundle", "--request", tasks, "--out", out, "--experimental"}, 2, "invalid-arguments"},
		"value-missing":    {[]string{"cem", "bundle", "--request", "--out", out}, 2, "invalid-arguments"},
		"relative-out":     {[]string{"cem", "bundle", "--request", tasks, "--out", "out"}, 2, "invalid-arguments"},
		"unclean-request":  {[]string{"cem", "bundle", "--request", temp + "/./tasks.json", "--out", out}, 2, "invalid-arguments"},
		"tasks-profile":    {[]string{"cem", "bundle", "--request", tasks, "--out", out}, 1, "bundle-profile-unsupported"},
		"not-json":         {[]string{"cem", "bundle", "--request", notJSON, "--out", out}, 1, "bundle-request-invalid"},
	} {
		t.Run(name, func(t *testing.T) {
			code, stdout, stderr := runBundle(t, tc.args...)
			if code != tc.exit || stdout != "" || !strings.HasPrefix(stderr, `{"code": "`+tc.code+`", `) || !strings.HasSuffix(stderr, "\"ok\": false}\n") {
				t.Fatalf("exit=%d stdout=%q stderr=%q, want exit %d %s", code, stdout, stderr, tc.exit, tc.code)
			}
		})
	}
	if _, e := os.Lstat(out); !os.IsNotExist(e) {
		t.Fatalf("a refusal created the bundle directory: %v", e)
	}
	var stdout, stderr bytes.Buffer
	for _, args := range [][]string{{"cem", "verify-stable"}, {"ocm", "x", "cem", "bundle"}, {"cem"}} {
		if _, handled := RunBundleInvocation(context.Background(), args, &stdout, &stderr); handled {
			t.Fatalf("%q was handled as cem bundle", args)
		}
	}
}

// TestCSBV0005VerifierRefusalKeepsItsCode proves an untyped assembly error is
// typed from the verification result it carries: the verifier's own code
// first, then bundle-verification-rejected for a REJECT, else unchanged.
func TestCSBV0005VerifierRefusalKeepsItsCode(t *testing.T) {
	untyped := errors.New("stable verification refused at artifacts: ")
	code := "unsupported-spec"
	for name, tc := range map[string]struct {
		v    verify.StableResult
		e    error
		want string
	}{
		"verifier-code": {verify.StableResult{Outcome: "UNSUPPORTED", Code: &code}, untyped, code},
		"reject":        {verify.StableResult{Outcome: "REJECT", IssueCodes: []string{"a", "b"}}, untyped, "bundle-verification-rejected"},
		"not-verified":  {verify.NewStableResult(verify.StableOptions{}), untyped, ""},
		"already-typed": {verify.StableResult{Code: &code}, cemcode.New("candidate-input", "x"), "candidate-input"},
	} {
		got := verifierRefusal(tc.v, tc.e)
		if cemcode.CodeOf(got) != tc.want {
			t.Errorf("%s: code %q, want %q (%v)", name, cemcode.CodeOf(got), tc.want, got)
		}
	}
}
