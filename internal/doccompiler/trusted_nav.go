package doccompiler

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

//go:embed trusted_nav_probe.py
var trustedNavProbe string

const TrustedNavRequestProfile = "corvint-trusted-project-navigation-request/0"
const TrustedNavProfile = "corvint-trusted-project-navigation/0"
const TrustedNavMaxInput = 2 << 20

var trustedNavPath = regexp.MustCompile(`^[A-Za-z0-9_./-]+$`)

type TrustedNavEnvironment struct {
	Python          string `json:"python"`
	PythonSHA256    string `json:"python_sha256"`
	InventorySHA256 string `json:"inventory_sha256"`
	Lock            string `json:"lock"`
	LockSHA256      string `json:"lock_sha256"`
}
type TrustedNavEntry struct {
	Title string `json:"title"`
	Path  string `json:"path"`
}
type TrustedNavDocument struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	SHA256  string `json:"sha256"`
}
type TrustedNavRequest struct {
	Profile      string                `json:"profile"`
	Revision     string                `json:"revision"`
	Config       string                `json:"config"`
	ConfigSHA256 string                `json:"config_sha256"`
	Environment  TrustedNavEnvironment `json:"environment"`
	Nav          []TrustedNavEntry     `json:"nav"`
	Documents    []TrustedNavDocument  `json:"documents"`
}
type trustedNavSnapshot struct {
	Owner           string          `json:"owner"`
	ConfigSHA256    string          `json:"config_sha256"`
	DocsDir         string          `json:"docs_dir"`
	Nav             json.RawMessage `json:"nav"`
	NavSHA256       string          `json:"nav_sha256"`
	Start           int             `json:"start_byte"`
	End             int             `json:"end_byte"`
	SpanSHA256      string          `json:"span_sha256"`
	InventorySHA256 string          `json:"inventory_sha256"`
	SiteDirOverride string          `json:"site_dir_override"`
}
type trustedNavSource struct {
	Path   string `json:"path"`
	Blob   string `json:"blob"`
	SHA256 string `json:"sha256"`
}
type trustedNavResult struct {
	Profile           string               `json:"profile"`
	State             string               `json:"state"`
	Revision          string               `json:"revision"`
	Sources           []trustedNavSource   `json:"sources"`
	SourceSHA256      string               `json:"source_sha256"`
	EnvironmentSHA256 string               `json:"environment_sha256"`
	ProbeSHA256       string               `json:"probe_sha256"`
	Snapshot          trustedNavSnapshot   `json:"snapshot"`
	SnapshotSHA256    string               `json:"snapshot_sha256"`
	Reload            trustedNavSnapshot   `json:"reload"`
	ReloadSHA256      string               `json:"reload_sha256"`
	CandidateSHA256   string               `json:"candidate_sha256"`
	Replacement       string               `json:"replacement"`
	ReplacementSHA256 string               `json:"replacement_sha256"`
	ProposalPatch     string               `json:"proposal_patch"`
	PatchSHA256       string               `json:"patch_sha256"`
	Documents         []TrustedNavDocument `json:"documents"`
	BuildStrict       string               `json:"build_strict"`
	OfflineQualified  string               `json:"offline_qualified"`
	Limitations       []string             `json:"limitations"`
}

// TrustedNavigation runs only after an explicit owner trust grant. The fixed
// loader executes pinned project tooling; process groups are not a hostile-code sandbox.
func TrustedNavigation(ctx context.Context, root string, raw []byte, trusted bool) (output []byte, err error) {
	if !trusted {
		return nil, failure("trusted-project-required", "navigation requires explicit project trust")
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return nil, failure("trusted-nav-platform", "trusted navigation requires Darwin or Linux process groups")
	}
	if len(raw) > TrustedNavMaxInput {
		return nil, failure("trusted-nav-limit", "request exceeds 2 MiB")
	}
	if err := VerifyCanonicalJSON(raw); err != nil {
		return nil, err
	}
	var req TrustedNavRequest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return nil, failure("trusted-nav-request", "invalid closed request")
	}
	if req.Profile != TrustedNavRequestProfile || !revisionPattern.MatchString(req.Revision) || (req.Config != "mkdocs.yml" && req.Config != "mkdocs.yaml") || !digestPattern.MatchString(req.ConfigSHA256) || !filepath.IsAbs(req.Environment.Python) || !digestPattern.MatchString(req.Environment.PythonSHA256) || !digestPattern.MatchString(req.Environment.InventorySHA256) || !navPath(req.Environment.Lock) || !digestPattern.MatchString(req.Environment.LockSHA256) || len(req.Nav) == 0 || len(req.Nav) > 64 || req.Documents == nil || len(req.Documents) > 64 {
		return nil, failure("trusted-nav-request", "invalid request pins or limits")
	}
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	temp, err := os.MkdirTemp("", "corvint-trusted-nav-")
	if err != nil {
		return nil, err
	}
	defer finishTrustedNav(temp, &output, &err, os.RemoveAll)
	git, err := exec.LookPath("git")
	if err != nil {
		return nil, err
	}
	gitRun := func(args ...string) (string, error) {
		r, e := runCommand(ctx, git, append([]string{"--no-optional-locks", "-C", root}, args...), temp, append(isolatedEnvironment(git, temp), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_REPLACE_OBJECTS=1", "GIT_NO_LAZY_FETCH=1"), 40<<20, 64<<10)
		return r.stdout, e
	}
	replacements, e := gitRun("for-each-ref", "--format=%(refname)", "refs/replace")
	if e != nil || replacements != "" {
		return nil, failure("trusted-nav-source", "replacement refs are unsupported")
	}
	head, err := gitRun("rev-parse", "HEAD")
	if err != nil || head != req.Revision {
		return nil, failure("trusted-nav-stale", "request revision is not HEAD")
	}
	status, err := gitRun("status", "--porcelain=v1", "--untracked-files=all")
	if err != nil || status != "" {
		return nil, failure("trusted-nav-dirty", "repository must be clean")
	}
	stage := filepath.Join(temp, "source")
	if err = os.Mkdir(stage, 0700); err != nil {
		return nil, err
	}
	tree, err := gitRun("ls-tree", "-r", "-z", req.Revision)
	if err != nil {
		return nil, err
	}
	sources := []trustedNavSource{}
	contents := map[string][]byte{}
	folded := map[string]bool{}
	total := 0
	for _, row := range strings.Split(strings.TrimSuffix(tree, "\x00"), "\x00") {
		fields, name, ok := strings.Cut(row, "\t")
		parts := strings.Fields(fields)
		if !ok || len(parts) != 3 || (parts[0] != "100644" && parts[0] != "100755") || parts[1] != "blob" || !navPath(name) || folded[strings.ToLower(name)] {
			return nil, failure("trusted-nav-source", "unsupported or ambiguous Git manifest")
		}
		for prior := range contents {
			if navCaseAmbiguity(name, prior) {
				return nil, failure("trusted-nav-source", "case-ambiguous source path")
			}
		}
		folded[strings.ToLower(name)] = true
		// Git show output must retain all bytes, unlike the text-oriented command helper.
		data, e := navGitBlob(ctx, git, root, req.Revision+":"+name, temp)
		if e != nil {
			return nil, e
		}
		total += len(data)
		if gitBlobID(data, len(parts[2])) != parts[2] {
			return nil, failure("trusted-nav-source", "Git blob identity mismatch")
		}
		if len(data) > 1<<20 || total > 32<<20 || len(sources) >= 2048 {
			return nil, failure("trusted-nav-limit", "source manifest exceeds bounds")
		}
		contents[name] = data
		sources = append(sources, trustedNavSource{name, parts[2], sha256Hex(data)})
		target := filepath.Join(stage, filepath.FromSlash(name))
		if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
			return nil, e
		}
		if e = os.WriteFile(target, data, 0600); e != nil {
			return nil, e
		}
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].Path < sources[j].Path })
	config, ok := contents[req.Config]
	if !ok || sha256Hex(config) != req.ConfigSHA256 {
		return nil, failure("trusted-nav-stale", "config bytes do not match")
	}
	lock, ok := contents[req.Environment.Lock]
	if !ok || sha256Hex(lock) != req.Environment.LockSHA256 {
		return nil, failure("trusted-nav-environment", "project lock does not match")
	}
	probe := func(configHash string) (trustedNavSnapshot, error) {
		if e := navPythonPin(req.Environment); e != nil {
			return trustedNavSnapshot{}, e
		}
		input := map[string]any{"root": stage, "config": req.Config, "config_sha256": configHash, "inventory_sha256": req.Environment.InventorySHA256, "site_dir": filepath.Join(temp, "site")}
		output, e := navProbe(ctx, req.Environment.Python, input, temp)
		if e != nil {
			return trustedNavSnapshot{}, e
		}
		var s trustedNavSnapshot
		if e = json.Unmarshal(output, &s); e != nil {
			return s, failure("trusted-nav-probe", "invalid snapshot")
		}
		if e = navPythonPin(req.Environment); e != nil {
			return s, e
		}
		return s, nil
	}
	if err = navWorktreeMatches(root, contents); err != nil {
		return nil, err
	}
	original, err := probe(req.ConfigSHA256)
	if err != nil {
		return nil, err
	}
	if !navPath(original.DocsDir) || original.Start < 0 || original.End < original.Start || original.End > len(config) || original.Owner != req.Config || original.ConfigSHA256 != req.ConfigSHA256 || original.InventorySHA256 != req.Environment.InventorySHA256 || original.SpanSHA256 != sha256Hex(config[original.Start:original.End]) {
		return nil, failure("trusted-nav-authority", "invalid authority snapshot")
	}
	spellings := map[string]bool{}
	for name := range contents {
		spellings[name] = true
	}
	docs := append([]TrustedNavDocument{}, req.Documents...)
	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })
	for _, doc := range docs {
		if !navPath(doc.Path) || !strings.HasPrefix(doc.Path, original.DocsDir+"/") || !strings.HasSuffix(doc.Path, ".md") || len(doc.Content) == 0 || len(doc.Content) > 1<<20 || sha256Hex([]byte(doc.Content)) != doc.SHA256 {
			return nil, failure("trusted-nav-document", "invalid proposed document")
		}
		for name := range spellings {
			if foldedPathOverlap(doc.Path, name) || navCaseAmbiguity(doc.Path, name) {
				return nil, failure("trusted-nav-document", "proposed document overlaps source")
			}
		}
		spellings[doc.Path] = true
		total += len(doc.Content)
		if total > 32<<20 {
			return nil, failure("trusted-nav-limit", "candidate exceeds source bound")
		}
		dest := filepath.Join(stage, filepath.FromSlash(doc.Path))
		if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
			return nil, err
		}
		if err = os.WriteFile(dest, []byte(doc.Content), 0600); err != nil {
			return nil, err
		}
	}
	wanted := []map[string]string{}
	seen := map[string]bool{}
	for _, entry := range req.Nav {
		if !validReason(entry.Title) || len(entry.Title) > 256 || !navPath(entry.Path) || !strings.HasSuffix(entry.Path, ".md") || seen[entry.Title] {
			return nil, failure("trusted-nav-entry", "invalid or duplicate nav entry")
		}
		seen[entry.Title] = true
		info, e := os.Stat(filepath.Join(stage, original.DocsDir, filepath.FromSlash(entry.Path)))
		if e != nil || !info.Mode().IsRegular() {
			return nil, failure("trusted-nav-entry", "nav page is missing")
		}
		wanted = append(wanted, map[string]string{entry.Title: entry.Path})
	}
	replacement, err := CanonicalJSON(wanted)
	if err != nil {
		return nil, err
	}
	replacement = bytes.TrimSuffix(replacement, []byte("\n"))
	candidate := append(append(append([]byte{}, config[:original.Start]...), replacement...), config[original.End:]...)
	if err = os.WriteFile(filepath.Join(stage, req.Config), candidate, 0600); err != nil {
		return nil, err
	}
	reloaded, err := probe(sha256Hex(candidate))
	if err != nil {
		return nil, err
	}
	var observed []map[string]string
	if json.Unmarshal(reloaded.Nav, &observed) != nil || !equalJSON(observed, wanted) || reloaded.Owner != original.Owner || reloaded.DocsDir != original.DocsDir || reloaded.ConfigSHA256 != sha256Hex(candidate) {
		return nil, failure("trusted-nav-reload", "candidate effective navigation differs")
	}
	if err = navWorktreeMatches(root, contents); err != nil {
		return nil, err
	}
	after, err := gitRun("status", "--porcelain=v1", "--untracked-files=all")
	if err != nil || after != status {
		return nil, failure("trusted-nav-source-changed", "repository changed during validation")
	}
	after, err = gitRun("rev-parse", "HEAD")
	if err != nil || after != head {
		return nil, failure("trusted-nav-source-changed", "HEAD changed during validation")
	}
	var patch bytes.Buffer
	navReplaceDiff(&patch, req.Config, config, candidate, false)
	for _, doc := range docs {
		navReplaceDiff(&patch, doc.Path, nil, []byte(doc.Content), true)
	}
	result := trustedNavResult{Profile: TrustedNavProfile, State: "NAVIGATION_VALIDATED", Revision: req.Revision, Sources: sources, SourceSHA256: navDigest(sources), EnvironmentSHA256: navDigest(req.Environment), ProbeSHA256: sha256Hex([]byte(trustedNavProbe)), Snapshot: original, SnapshotSHA256: navDigest(original), Reload: reloaded, ReloadSHA256: navDigest(reloaded), CandidateSHA256: sha256Hex(candidate), Replacement: string(replacement), ReplacementSHA256: sha256Hex(replacement), ProposalPatch: patch.String(), PatchSHA256: sha256Hex(patch.Bytes()), Documents: docs, BuildStrict: "NOT_RUN", OfflineQualified: "NOT_OBSERVED", Limitations: []string{"Explicitly trusted pinned project tooling; no hostile-code or escaping-descendant containment.", "Navigation reload only; no build, offline qualification or source apply.", "Initial profile supports one root config, local Markdown navigation and the bundled MkDocs theme."}}
	output, err = CanonicalJSON(result)
	if len(output) > 8<<20 {
		return nil, failure("trusted-nav-limit", "result exceeds 8 MiB")
	}
	return output, err
}

func navPath(p string) bool {
	if !trustedNavPath.MatchString(p) || path.Clean(p) != p || strings.HasPrefix(p, "/") || p == "." {
		return false
	}
	for _, v := range strings.Split(p, "/") {
		if v == ".." || strings.EqualFold(v, ".git") {
			return false
		}
	}
	return true
}
func navDigest(v any) string { b, _ := CanonicalJSON(v); return sha256Hex(b) }
func equalJSON(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}
func navPythonPin(e TrustedNavEnvironment) error {
	data, err := ReadTrustedNavFile(e.Python, 64<<20)
	if err != nil || sha256Hex(data) != e.PythonSHA256 {
		return failure("trusted-nav-environment", "interpreter pin mismatch or unavailable")
	}
	return nil
}

func navProbe(ctx context.Context, python string, input any, temp string) ([]byte, error) {
	b, _ := json.Marshal(input)
	name := filepath.Join(temp, "probe-input.json")
	if err := os.WriteFile(name, b, 0600); err != nil {
		return nil, err
	}
	env := isolatedEnvironment(python, temp)
	result, err := runCommand(ctx, python, []string{"-I", "-B", "-c", trustedNavProbe, name}, temp, env, 2<<20, 64<<10)
	if err != nil {
		return nil, failure("trusted-nav-probe-refused", "pinned MkDocs probe refused the input or exceeded limits")
	}
	return []byte(result.stdout), nil
}
func navReplaceDiff(out *bytes.Buffer, name string, old, new []byte, create bool) {
	fmt.Fprintf(out, "diff --git a/%[1]s b/%[1]s\n", name)
	if create {
		fmt.Fprintf(out, "new file mode 100644\n--- /dev/null\n+++ b/%s\n", name)
	} else {
		fmt.Fprintf(out, "--- a/%[1]s\n+++ b/%[1]s\n", name)
	}
	a, b := splitLines(old), splitLines(new)
	start := 1
	if len(a) == 0 {
		start = 0
	}
	newStart := 1
	if len(b) == 0 {
		newStart = 0
	}
	fmt.Fprintf(out, "@@ -%s +%s @@\n", hunkRange(start, len(a)), hunkRange(newStart, len(b)))
	for _, entry := range []struct {
		prefix string
		lines  [][]byte
	}{{"-", a}, {"+", b}} {
		for _, line := range entry.lines {
			out.WriteString(entry.prefix)
			out.Write(line)
			if !bytes.HasSuffix(line, []byte("\n")) {
				out.WriteString("\n\\ No newline at end of file\n")
			}
		}
	}
}

// InspectTrustedNavigation observes pins only after explicit trust; it does not
// install packages or claim the observed toolchain is independently trustworthy.
func InspectTrustedNavigation(ctx context.Context, python string, trusted bool) (output []byte, err error) {
	if !trusted {
		return nil, failure("trusted-project-required", "environment inspection requires explicit project trust")
	}
	if !filepath.IsAbs(python) || (runtime.GOOS != "darwin" && runtime.GOOS != "linux") {
		return nil, failure("trusted-nav-environment", "unsupported interpreter or platform")
	}
	data, err := ReadTrustedNavFile(python, 64<<20)
	if err != nil {
		return nil, failure("trusted-nav-environment", "interpreter unavailable or exceeds bound")
	}
	pin := sha256Hex(data)
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	temp, err := os.MkdirTemp("", "corvint-trusted-nav-inspect-")
	if err != nil {
		return nil, err
	}
	defer finishTrustedNav(temp, &output, &err, os.RemoveAll)
	raw, err := navProbe(ctx, python, map[string]any{"mode": "inventory"}, temp)
	if err != nil {
		return nil, err
	}
	var inventory struct {
		SHA256 string `json:"inventory_sha256"`
	}
	if json.Unmarshal(raw, &inventory) != nil || !digestPattern.MatchString(inventory.SHA256) {
		return nil, failure("trusted-nav-environment", "invalid package inventory")
	}
	if err = navPythonPin(TrustedNavEnvironment{Python: python, PythonSHA256: pin}); err != nil {
		return nil, err
	}
	return CanonicalJSON(map[string]any{"profile": "corvint-trusted-project-navigation-environment/0", "python": python, "python_sha256": pin, "inventory_sha256": inventory.SHA256, "probe_sha256": sha256Hex([]byte(trustedNavProbe)), "trust": "caller-owned-toolchain"})
}

func navCaseAmbiguity(a, b string) bool {
	x, y := strings.Split(a, "/"), strings.Split(b, "/")
	for i := 0; i < len(x) && i < len(y); i++ {
		if !strings.EqualFold(x[i], y[i]) {
			return false
		}
		if x[i] != y[i] {
			return true
		}
	}
	return false
}
func navWorktreeMatches(root string, contents map[string][]byte) error {
	for name, want := range contents {
		current := root
		for _, part := range strings.Split(name, "/") {
			current = filepath.Join(current, part)
			st, err := os.Lstat(current)
			if err != nil || st.Mode()&os.ModeSymlink != 0 {
				return failure("trusted-nav-source-changed", "worktree path differs from committed source")
			}
		}
		st, err := os.Stat(current)
		if err != nil || !st.Mode().IsRegular() || st.Size() != int64(len(want)) {
			return failure("trusted-nav-source-changed", "worktree file differs from committed source")
		}
		f, err := os.Open(current)
		if err != nil {
			return failure("trusted-nav-source-changed", "cannot verify worktree file")
		}
		got, e := io.ReadAll(io.LimitReader(f, int64(len(want))+1))
		f.Close()
		if e != nil || !bytes.Equal(got, want) {
			return failure("trusted-nav-source-changed", "worktree bytes differ from committed source")
		}
	}
	return nil
}

// ReadTrustedNavFile refuses nonregular inputs before opening them. Concurrent
// malicious path replacement is outside this explicit trusted-host profile.
func ReadTrustedNavFile(name string, limit int64) ([]byte, error) {
	st, err := os.Stat(name)
	if err != nil || !st.Mode().IsRegular() || st.Size() > limit {
		return nil, failure("trusted-nav-input", "input must be a bounded regular file")
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, failure("trusted-nav-input", "input unavailable")
	}
	defer f.Close()
	current, err := f.Stat()
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(st, current) {
		return nil, failure("trusted-nav-input", "input changed during acquisition")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, failure("trusted-nav-input", "input read exceeded bound")
	}
	return data, nil
}
func finishTrustedNav(temp string, output *[]byte, result *error, remove func(string) error) {
	if err := remove(temp); err != nil {
		*output = nil
		*result = failure("trusted-nav-cleanup", "temporary state could not be removed; retained at %s", temp)
	}
}
