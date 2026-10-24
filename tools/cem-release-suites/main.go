// Command cem-release-suites runs the sealed protocol/cem-1.0 stable suites
// against two built release binaries, corvint and cem01-go, and refuses when
// the two were not built from the same clean commit
// (docs/specs/cem-stable-v1.md CEM-V1-572, V1-2323).
//
// Usage:
//
//	cem-release-suites --corvint BIN --cem01 BIN [--root DIR]
//
// Each binary must carry Go build information naming one vcs.revision with
// vcs.modified=false, the two revisions must be equal, and the checkout at
// --root must be at that revision with protocol/cem-1.0 unmodified, so the
// sealed suites read are the ones the binaries were built with. The tamper
// matrix, citation vectors, neutral criterion and receipt authentication cases
// then run through `corvint cem verify-stable` and
// `cem01-go verify-stable-canonical`; each result is checked against the
// sealed expectation and the two binaries' result lines, runtime aside, must
// be byte-identical. The repository-envelope cells are prose recipes rebuilt
// by the in-process S0E harnesses, so this command does not run them; the
// Makefile target runs those suites at source level and says so.
package main

import (
	"bytes"
	"debug/buildinfo"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"
)

const (
	stableDir     = "protocol/cem-1.0/stable"
	corvintModule = "github.com/Beamfall/corvint"
	corvintMain   = "github.com/Beamfall/corvint/cmd/corvint"
	cem01Main     = "github.com/Beamfall/corvint/interop/cem01-go"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("cem-release-suites", flag.ContinueOnError)
	flags.SetOutput(stderr)
	corvint := flags.String("corvint", "", "built corvint binary")
	cem01 := flags.String("cem01", "", "built cem01-go binary")
	root := flags.String("root", ".", "repository checkout the binaries were built from")
	if err := flags.Parse(args); err != nil || *corvint == "" || *cem01 == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: cem-release-suites --corvint BIN --cem01 BIN [--root DIR]")
		return 2
	}
	absRoot, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintln(stderr, "cem-release-suites:", err)
		return 2
	}
	revision, err := releaseBuild(*corvint, *cem01, absRoot)
	if err != nil {
		fmt.Fprintln(stderr, "cem-release-suites: refused:", err)
		return 1
	}
	fmt.Fprintf(stdout, "cem-release-suites: both binaries built from clean %s\n", revision)
	r := &runner{root: absRoot, bins: [2]binary{{name: "corvint", argv: []string{mustAbs(*corvint), "cem", "verify-stable"}}, {name: "cem01-go", argv: []string{mustAbs(*cem01), "verify-stable-canonical"}}}}
	tmp, err := os.MkdirTemp("", "cem-release-suites-")
	if err != nil {
		fmt.Fprintln(stderr, "cem-release-suites:", err)
		return 2
	}
	defer os.RemoveAll(tmp)
	r.tmp = tmp
	for _, s := range []struct {
		name string
		fn   func(string) error
	}{{"tamper-matrix", r.tamperMatrix}, {"citation-vectors", r.citationVectors}, {"neutral-criterion", r.neutralCriterion}, {"receipt-authentication", r.receiptAuthentication}} {
		before := r.runs
		if err := s.fn(filepath.Join(absRoot, stableDir, s.name)); err != nil {
			fmt.Fprintf(stderr, "cem-release-suites: %s: FAIL: %v\n", s.name, err)
			return 1
		}
		fmt.Fprintf(stdout, "cem-release-suites: %s: PASS (%d cases on both binaries)\n", s.name, r.runs-before)
	}
	fmt.Fprintln(stdout, "cem-release-suites: repository-envelope: NOT_RUN against binaries (prose recipes; run at source level by make cem-release-suites)")
	return 0
}

func mustAbs(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}

// releaseBuild refuses unless both binaries were built from the same clean
// commit and the checkout at root is that commit with the packet unmodified.
func releaseBuild(corvint, cem01, root string) (string, error) {
	a, err := buildinfo.ReadFile(corvint)
	if err != nil {
		return "", fmt.Errorf("corvint build information: %v", err)
	}
	b, err := buildinfo.ReadFile(cem01)
	if err != nil {
		return "", fmt.Errorf("cem01-go build information: %v", err)
	}
	revision, err := sameBuild(a, b)
	if err != nil {
		return "", err
	}
	head, err := gitOutput(root, "", nil, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	if head != revision {
		return "", fmt.Errorf("checkout HEAD %s is not the build revision %s; check out the build commit", head, revision)
	}
	if dirty, err := gitOutput(root, "", nil, "status", "--porcelain", "--", "protocol/cem-1.0"); err != nil || dirty != "" {
		return "", fmt.Errorf("protocol/cem-1.0 differs from the build revision: %v%s", err, dirty)
	}
	return revision, nil
}

// sameBuild returns the shared vcs.revision of a corvint and a cem01-go build,
// refusing a wrong main package, a missing or modified revision, or two
// different revisions.
func sameBuild(corvint, cem01 *debug.BuildInfo) (string, error) {
	if corvint.Path != corvintMain || corvint.Main.Path != corvintModule {
		return "", fmt.Errorf("corvint binary is %s (module %s), not %s", corvint.Path, corvint.Main.Path, corvintMain)
	}
	if cem01.Path != cem01Main {
		return "", fmt.Errorf("cem01-go binary is %s, not %s", cem01.Path, cem01Main)
	}
	revisions := [2]string{}
	for i, info := range []*debug.BuildInfo{corvint, cem01} {
		settings := map[string]string{}
		for _, s := range info.Settings {
			settings[s.Key] = s.Value
		}
		name := [2]string{"corvint", "cem01-go"}[i]
		if settings["vcs"] != "git" || settings["vcs.revision"] == "" {
			return "", fmt.Errorf("%s carries no Git vcs.revision; build it from a Git checkout without -buildvcs=false", name)
		}
		if settings["vcs.modified"] != "false" {
			return "", fmt.Errorf("%s was built from a modified worktree at %s", name, settings["vcs.revision"])
		}
		revisions[i] = settings["vcs.revision"]
	}
	if revisions[0] != revisions[1] {
		return "", fmt.Errorf("builds differ: corvint %s, cem01-go %s", revisions[0], revisions[1])
	}
	return revisions[0], nil
}

type binary struct {
	name string
	argv []string
}

type runner struct {
	root, tmp string
	bins      [2]binary
	runs      int
}

type stableRun struct {
	exit   int
	line   []byte
	result map[string]any
}

// verify runs one case through both binaries and requires their result lines,
// runtime aside, to be byte-identical; it returns the corvint run.
func (r *runner) verify(repo string, mapBytes []byte, base, target, artifacts, trustedKeys string) (stableRun, error) {
	r.runs++
	mapPath := filepath.Join(r.tmp, fmt.Sprintf("map-%d.json", r.runs))
	if err := os.WriteFile(mapPath, mapBytes, 0o600); err != nil {
		return stableRun{}, err
	}
	var runs [2]stableRun
	for i, b := range r.bins {
		argv := append(append([]string(nil), b.argv[1:]...), "--repository", repo, "--map", mapPath, "--expected-base", base, "--target", target, "--artifacts", artifacts)
		if trustedKeys != "" {
			argv = append(argv, "--trusted-keys", trustedKeys)
		}
		cmd := exec.Command(b.argv[0], argv...)
		cmd.Dir = r.tmp
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		var exitErr *exec.ExitError
		switch {
		case err == nil:
		case errors.As(err, &exitErr):
			runs[i].exit = exitErr.ExitCode()
		default:
			return stableRun{}, fmt.Errorf("%s: %v", b.name, err)
		}
		runs[i].line = bytes.TrimSuffix(out.Bytes(), []byte("\n"))
		if err := json.Unmarshal(runs[i].line, &runs[i].result); err != nil {
			return stableRun{}, fmt.Errorf("%s printed no result line: %v: %s", b.name, err, out.Bytes())
		}
	}
	a, b := withoutRuntime(runs[0].line), withoutRuntime(runs[1].line)
	if runs[0].exit != runs[1].exit || !bytes.Equal(a, b) {
		return stableRun{}, fmt.Errorf("consumers differ: corvint exit %d %s; cem01-go exit %d %s", runs[0].exit, a, runs[1].exit, b)
	}
	return runs[0], nil
}

// withoutRuntime removes the runtime member, whose goVersion names the build
// toolchain, from one result line without reordering anything else.
func withoutRuntime(line []byte) []byte {
	start := bytes.Index(line, []byte(`,"runtime":{`))
	if start < 0 {
		return line
	}
	end := bytes.IndexByte(line[start+len(`,"runtime":{`):], '}')
	if end < 0 {
		return line
	}
	return append(append([]byte(nil), line[:start]...), line[start+len(`,"runtime":{`)+end+1:]...)
}

func resultExceptRuntime(m map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		if k != "runtime" {
			out[k] = v
		}
	}
	return out
}

func readJSON(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

// recipe builds a public case's recipe repository with Git plumbing and the
// recipe's fixed identity, as each case README specifies.
func (r *runner) recipe(dir string) (repo, base, target string, err error) {
	var recipe struct {
		ObjectFormat string                             `json:"objectFormat"`
		Identity     struct{ Name, Email, Date string } `json:"identity"`
		Commits      []struct {
			Name    string            `json:"name"`
			Message string            `json:"message"`
			Files   map[string]string `json:"files"`
		} `json:"commits"`
	}
	if err = readJSON(filepath.Join(dir, "repository.recipe.json"), &recipe); err != nil {
		return
	}
	if recipe.ObjectFormat != "sha1" {
		return "", "", "", fmt.Errorf("recipe object format %q is not sha1", recipe.ObjectFormat)
	}
	repo = filepath.Join(r.tmp, "repo-"+filepath.Base(dir))
	if err = os.Mkdir(repo, 0o755); err != nil {
		return
	}
	env := []string{"HOME=/nonexistent-cem-release-suites-home", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "LC_ALL=C",
		"GIT_AUTHOR_NAME=" + recipe.Identity.Name, "GIT_AUTHOR_EMAIL=" + recipe.Identity.Email, "GIT_AUTHOR_DATE=" + recipe.Identity.Date,
		"GIT_COMMITTER_NAME=" + recipe.Identity.Name, "GIT_COMMITTER_EMAIL=" + recipe.Identity.Email, "GIT_COMMITTER_DATE=" + recipe.Identity.Date}
	git := func(stdin string, args ...string) string {
		if err != nil {
			return ""
		}
		var out string
		out, err = gitOutput(repo, stdin, env, args...)
		return out
	}
	git("", "init", "-q", "--object-format=sha1")
	for _, commit := range recipe.Commits {
		names := make([]string, 0, len(commit.Files))
		for name := range commit.Files {
			names = append(names, name)
		}
		sort.Strings(names)
		var tree strings.Builder
		for _, name := range names {
			tree.WriteString("100644 blob " + git(commit.Files[name], "hash-object", "-w", "--stdin") + "\t" + name + "\n")
		}
		args := []string{"commit-tree", git(tree.String(), "mktree"), "-m", commit.Message}
		if target != "" {
			args = append(args, "-p", target)
		}
		target = git("", args...)
		if commit.Name == "base" {
			base = target
		}
	}
	git("", "update-ref", "refs/heads/main", target)
	git("", "symbolic-ref", "HEAD", "refs/heads/main")
	return
}

func gitOutput(dir, stdin string, env []string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-C", dir}, args...)...)
	if env != nil {
		cmd.Env = append([]string{"PATH=" + os.Getenv("PATH")}, env...)
	}
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

func (r *runner) caseRepository(dir, wantBase, wantTarget string) (string, error) {
	repo, base, target, err := r.recipe(dir)
	if err != nil {
		return "", err
	}
	if base != wantBase || target != wantTarget {
		return "", fmt.Errorf("recipe commits %s..%s, want %s..%s", base, target, wantBase, wantTarget)
	}
	return repo, nil
}

func (r *runner) tamperMatrix(dir string) error {
	var want struct {
		Profile        string `json:"profile"`
		ArtifactRoot   string `json:"artifactRoot"`
		BaseRevision   string `json:"baseRevision"`
		TargetRevision string `json:"targetRevision"`
		Cases          []struct {
			Name       string         `json:"name"`
			Map        string         `json:"map"`
			Exit       int            `json:"exit"`
			Outcome    string         `json:"outcome"`
			Stage      string         `json:"stage"`
			Code       *string        `json:"code"`
			IssueCodes []string       `json:"issueCodes"`
			Citation   map[string]any `json:"citation"`
		} `json:"cases"`
	}
	if err := readJSON(filepath.Join(dir, "EXPECTED.json"), &want); err != nil {
		return err
	}
	if want.Profile != "cem-stable-tamper-matrix/1" || len(want.Cases) == 0 {
		return fmt.Errorf("EXPECTED.json profile %q with %d cases", want.Profile, len(want.Cases))
	}
	repo, err := r.caseRepository(dir, want.BaseRevision, want.TargetRevision)
	if err != nil {
		return err
	}
	for _, c := range want.Cases {
		raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(c.Map)))
		if err != nil {
			return err
		}
		got, err := r.verify(repo, raw, want.BaseRevision, want.TargetRevision, filepath.Join(dir, want.ArtifactRoot), "")
		if err != nil {
			return fmt.Errorf("%s: %v", c.Name, err)
		}
		var citation map[string]any
		if v, ok := got.result["citation"].(map[string]any); ok {
			citation = v
		}
		var code *string
		if s, ok := got.result["code"].(string); ok {
			code = &s
		}
		if got.exit != c.Exit || got.result["outcome"] != c.Outcome || got.result["stage"] != c.Stage || !reflect.DeepEqual(code, c.Code) ||
			fmt.Sprint(got.result["issueCodes"]) != fmt.Sprint(stringsAny(c.IssueCodes)) || !reflect.DeepEqual(citation, c.Citation) {
			return fmt.Errorf("%s: exit %d %s, want the sealed verdict", c.Name, got.exit, got.line)
		}
	}
	return nil
}

func stringsAny(s []string) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}

func (r *runner) citationVectors(dir string) error {
	var want struct {
		Profile        string `json:"profile"`
		ArtifactRoot   string `json:"artifactRoot"`
		BaseRevision   string `json:"baseRevision"`
		TargetRevision string `json:"targetRevision"`
		Cases          []struct {
			Name string `json:"name"`
			Map  string `json:"map"`
			Exit int    `json:"exit"`
		} `json:"cases"`
	}
	if err := readJSON(filepath.Join(dir, "EXPECTED.json"), &want); err != nil {
		return err
	}
	if want.Profile != "cem-stable-citation-vectors/1" || len(want.Cases) == 0 {
		return fmt.Errorf("EXPECTED.json profile %q with %d cases", want.Profile, len(want.Cases))
	}
	repo, err := r.caseRepository(dir, want.BaseRevision, want.TargetRevision)
	if err != nil {
		return err
	}
	for _, c := range want.Cases {
		raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(c.Map)))
		if err != nil {
			return err
		}
		sealed, err := os.ReadFile(filepath.Join(dir, "results", c.Name+".json"))
		if err != nil {
			return err
		}
		got, err := r.verify(repo, raw, want.BaseRevision, want.TargetRevision, filepath.Join(dir, want.ArtifactRoot), "")
		if err != nil {
			return fmt.Errorf("%s: %v", c.Name, err)
		}
		if got.exit != c.Exit || !bytes.Equal(withoutRuntime(got.line), withoutRuntime(bytes.TrimSuffix(sealed, []byte("\n")))) {
			return fmt.Errorf("%s: exit %d %s, want exit %d and the sealed result line", c.Name, got.exit, got.line, c.Exit)
		}
	}
	return nil
}

func (r *runner) neutralCriterion(dir string) error {
	var want struct {
		Profile        string `json:"profile"`
		ArtifactRoot   string `json:"artifactRoot"`
		Map            string `json:"map"`
		BaseRevision   string `json:"baseRevision"`
		TargetRevision string `json:"targetRevision"`
		Accept         struct {
			Exit   int            `json:"exit"`
			Result map[string]any `json:"resultExceptRuntime"`
		} `json:"accept"`
		Refusals []struct {
			Name       string          `json:"name"`
			Criterion  json.RawMessage `json:"criterion"`
			RebindLink bool            `json:"rebindLink"`
			Exit       int             `json:"exit"`
			Outcome    string          `json:"outcome"`
			Stage      string          `json:"stage"`
			Code       string          `json:"code"`
		} `json:"refusals"`
	}
	if err := readJSON(filepath.Join(dir, "EXPECTED.json"), &want); err != nil {
		return err
	}
	repo, err := r.caseRepository(dir, want.BaseRevision, want.TargetRevision)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(dir, want.Map))
	if err != nil {
		return err
	}
	artifacts := filepath.Join(dir, want.ArtifactRoot)
	got, err := r.verify(repo, raw, want.BaseRevision, want.TargetRevision, artifacts, "")
	if err != nil {
		return fmt.Errorf("accept: %v", err)
	}
	if got.exit != want.Accept.Exit || !reflect.DeepEqual(resultExceptRuntime(got.result), want.Accept.Result) {
		return fmt.Errorf("accept: exit %d %s, want the sealed ACCEPT", got.exit, got.line)
	}
	if len(want.Refusals) == 0 {
		return fmt.Errorf("EXPECTED.json has no refusals")
	}
	for _, c := range want.Refusals {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			return err
		}
		var criterion any
		if err := json.Unmarshal(c.Criterion, &criterion); err != nil {
			return err
		}
		bindings, _ := m["criterionBindings"].([]any)
		links, _ := m["criterionLinks"].([]any)
		if len(bindings) != 1 || len(links) != 1 {
			return fmt.Errorf("%s: the map has %d bindings and %d links, want one each", c.Name, len(bindings), len(links))
		}
		bindings[0] = criterion
		if c.RebindLink {
			id, _ := criterion.(map[string]any)["id"]
			links[0].(map[string]any)["criterionId"] = id
		}
		mutated, err := json.Marshal(m)
		if err != nil {
			return err
		}
		got, err := r.verify(repo, mutated, want.BaseRevision, want.TargetRevision, artifacts, "")
		if err != nil {
			return fmt.Errorf("%s: %v", c.Name, err)
		}
		if code, _ := got.result["code"].(string); got.exit != c.Exit || got.result["outcome"] != c.Outcome || got.result["stage"] != c.Stage || code != c.Code {
			return fmt.Errorf("%s: exit %d %s, want exit %d %s %s %s", c.Name, got.exit, got.line, c.Exit, c.Outcome, c.Stage, c.Code)
		}
	}
	return nil
}

func (r *runner) receiptAuthentication(dir string) error {
	type mutation struct {
		Name           string                     `json:"name"`
		Set            map[string]json.RawMessage `json:"set"`
		TrustedKeys    string                     `json:"trustedKeys"`
		Exit           int                        `json:"exit"`
		Outcome        string                     `json:"outcome"`
		Stage          string                     `json:"stage"`
		Code           string                     `json:"code"`
		IssueCode      string                     `json:"issueCode"`
		Authentication string                     `json:"authentication"`
	}
	var want struct {
		ArtifactRoot   string `json:"artifactRoot"`
		Map            string `json:"map"`
		BaseRevision   string `json:"baseRevision"`
		TargetRevision string `json:"targetRevision"`
		Accept         struct {
			TrustedKeys string         `json:"trustedKeys"`
			Exit        int            `json:"exit"`
			Result      map[string]any `json:"resultExceptRuntime"`
		} `json:"accept"`
		AxisCases []mutation `json:"axisCases"`
		Refusals  []mutation `json:"refusals"`
	}
	if err := readJSON(filepath.Join(dir, "EXPECTED.json"), &want); err != nil {
		return err
	}
	repo, err := r.caseRepository(dir, want.BaseRevision, want.TargetRevision)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(dir, want.Map))
	if err != nil {
		return err
	}
	artifacts := filepath.Join(dir, want.ArtifactRoot)
	keys := func(name string) string {
		if name == "" {
			return ""
		}
		return filepath.Join(dir, name)
	}
	got, err := r.verify(repo, raw, want.BaseRevision, want.TargetRevision, artifacts, keys(want.Accept.TrustedKeys))
	if err != nil {
		return fmt.Errorf("accept: %v", err)
	}
	if got.exit != want.Accept.Exit || !reflect.DeepEqual(resultExceptRuntime(got.result), want.Accept.Result) {
		return fmt.Errorf("accept: exit %d %s, want the sealed ACCEPT", got.exit, got.line)
	}
	cases := append(append([]mutation(nil), want.AxisCases...), want.Refusals...)
	if len(want.AxisCases) == 0 || len(want.Refusals) == 0 {
		return fmt.Errorf("EXPECTED.json has %d axis cases and %d refusals", len(want.AxisCases), len(want.Refusals))
	}
	for _, c := range cases {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			return err
		}
		for k, v := range c.Set {
			m[k] = v
		}
		mutated, err := json.Marshal(m)
		if err != nil {
			return err
		}
		got, err := r.verify(repo, mutated, want.BaseRevision, want.TargetRevision, artifacts, keys(c.TrustedKeys))
		if err != nil {
			return fmt.Errorf("%s: %v", c.Name, err)
		}
		axes, _ := got.result["axes"].(map[string]any)
		code, _ := got.result["code"].(string)
		issue := ""
		if issues, _ := got.result["issueCodes"].([]any); len(issues) > 0 {
			issue, _ = issues[0].(string)
		}
		if got.exit != c.Exit || got.result["outcome"] != c.Outcome || axes["authentication"] != c.Authentication || code != c.Code || issue != c.IssueCode || c.Stage != "" && got.result["stage"] != c.Stage {
			return fmt.Errorf("%s: exit %d %s, want exit %d %s code %q issue %q authentication %s", c.Name, got.exit, got.line, c.Exit, c.Outcome, c.Code, c.IssueCode, c.Authentication)
		}
	}
	return nil
}
