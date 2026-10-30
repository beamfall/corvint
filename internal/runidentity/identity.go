// Package runidentity produces the corvintRunIdentity declaration a Playwright report embeds under
// config.metadata so `corvint runs stability|compare|reusable` can pool its runs
// (docs/specs/playwright-run-evidence-v0.md, PWR-V0-046..PWR-V0-050). The reader never infers
// identity; this producer computes it from the working trees before a run, by one documented rule
// set, so hand-written producers can agree with it. Git is only read, through bounded subprocesses
// with a scrubbed environment; nothing is written.
package runidentity

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

// Profile names the producer's output document.
const Profile = "corvint-run-identity/0"

// DefaultTestMatch is Playwright's default testMatch (`**/*.@(spec|test).?(c|m)[jt]s?(x)`) as a
// regular expression over the testDir-relative slash path (PWR-V0-047).
const DefaultTestMatch = `(^|/)[^/]*\.(spec|test)\.[cm]?[jt]sx?$`

// Refusal reasons (PWR-V0-050).
const (
	ReasonConfigUnreadable      = "CONFIG_UNREADABLE"
	ReasonNotRepository         = "NOT_A_REPOSITORY"
	ReasonTestDirOutside        = "TEST_DIR_OUTSIDE_REPOSITORY"
	ReasonTestMatchInvalid      = "TEST_MATCH_INVALID"
	ReasonRevisionUnresolved    = "TEST_REVISION_UNRESOLVED"
	ReasonApplicationUnresolved = "APPLICATION_REVISION_UNRESOLVED"
	ReasonUnmerged              = "UNMERGED_PATHS"
	ReasonSubmoduleModified     = "SUBMODULE_MODIFIED"
	ReasonPathUnrepresentable   = "PATH_UNREPRESENTABLE"
	ReasonNoTestFiles           = "NO_TEST_FILES"
	ReasonGitReadFailed         = "GIT_READ_FAILED"
	ReasonIndexFlags            = "INDEX_FLAGS_SET"
	ReasonFilterConfigured      = "FILTER_CONFIGURED"
	ReasonProjectTestDirOutside = "PROJECT_TEST_DIR_OUTSIDE_REPOSITORY"
	ReasonClosureInvalid        = "CLOSURE_DECLARATION_INVALID"
	// ClosureRuleDeclared marks caller-declared closure digests, which are not verified (PWR-V0-071).
	ClosureRuleDeclared = "DECLARED"
	// ClosureRuleComputed marks closure digests this producer computed from the Playwright config's
	// static TypeScript graph (PWR-V0-072).
	ClosureRuleComputed = typescript.PlaywrightTestClosureRule
	// LimitClosureUnknown and LimitClosureWidened report a computed closure that could not narrow
	// (PWR-V0-073).
	LimitClosureUnknown           = "TEST_CLOSURE_UNKNOWN"
	LimitClosureWidened           = "TEST_CLOSURE_EXCLUSION_WIDENED"
	LimitProjectTestDirUnresolved = "PROJECT_TEST_DIR_UNRESOLVED"
	LimitProjectTestDirNoTests    = "PROJECT_TEST_DIR_NO_TEST_FILES"
	LimitUntrackedOutsideDigest   = "UNTRACKED_FILES_OUTSIDE_DIGEST"
	LimitApplicationWorktreeDirty = "APPLICATION_WORKTREE_MODIFIED"
	worktreeSuffix                = "+worktree.sha256:"
	listLimit                     = 64 << 20
	operationTimeout              = 2 * time.Minute
)

// Error is a refusal with a named reason.
type Error struct {
	Reason string
	Detail string
}

func (e *Error) Error() string {
	if e.Detail == "" {
		return e.Reason
	}
	return e.Reason + ": " + e.Detail
}

func refuse(reason, format string, args ...any) error {
	return &Error{Reason: reason, Detail: fmt.Sprintf(format, args...)}
}

// Options name the inputs. Config is required; TestDir defaults to the config's directory (as
// Playwright's testDir does), AppRoot to the test repository, TestMatch to DefaultTestMatch.
// ProjectTestDirs name projects' own testDirs, whose files the report keys relative to TestDir
// (PWR-V0-052).
type Options struct {
	Config          string
	TestDir         string
	AppRoot         string
	TestMatch       string
	ProjectTestDirs []ProjectTestDir
	// ArtifactDigest is the operator-supplied digest of the build the browser will load
	// (PWR-V0-054); "" leaves the served digest unknown. Callers validate it with ValidArtifactDigest.
	ArtifactDigest string
	// ClosureDigests are caller-declared per-test closure digests by test key, from
	// ParseClosureDigests; nil declares none (PWR-V0-071).
	ClosureDigests map[string]string
	// ComputeClosures computes per-test closure digests under ClosureRuleComputed; declared
	// ClosureDigests win when both are given (PWR-V0-072).
	ComputeClosures bool
}

var closureDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// ParseClosureDigests reads a --closure-digests document: one JSON object mapping test keys to
// sha256: digests, without duplicate keys (PWR-V0-071).
func ParseClosureDigests(raw []byte) (map[string]string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	invalid := func(detail string) (map[string]string, error) { return nil, refuse(ReasonClosureInvalid, "%s", detail) }
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return invalid("not a JSON object")
	}
	out := map[string]string{}
	for dec.More() {
		tok, err := dec.Token()
		key, isKey := tok.(string)
		if err != nil || !isKey {
			return invalid("malformed key")
		}
		var digest string
		if dec.Decode(&digest) != nil || !closureDigestPattern.MatchString(digest) {
			return invalid("a value is not sha256: and 64 lowercase hex digits")
		}
		if _, dup := out[key]; dup {
			return invalid("duplicate key")
		}
		out[key] = digest
	}
	if tok, err := dec.Token(); err != nil || tok != json.Delim('}') {
		return invalid("malformed object")
	}
	if _, err := dec.Token(); err != io.EOF {
		return invalid("trailing data")
	}
	return out, nil
}

// Served-digest states of a Document (PWR-V0-054).
const (
	ServedDigestDeclared = "DECLARED"
	ServedDigestUnknown  = "UNKNOWN"
)

var artifactDigestPattern = regexp.MustCompile(`^[A-Za-z0-9._:+/=@-]{1,256}$`)

// ValidArtifactDigest reports whether v is an acceptable --artifact-digest: 1 to 256 bytes of
// letters, digits and ._:+/=@- (for example sha256:<hex> or an image digest) (PWR-V0-054).
func ValidArtifactDigest(v string) bool { return artifactDigestPattern.MatchString(v) }

// ProjectTestDir is one project's own testDir and its testMatch ("" inherits the top-level one).
type ProjectTestDir struct {
	Dir       string
	TestMatch string
}

// ProjectTestDirReport states what one --project-test-dir contributed (PWR-V0-052, PWR-V0-053).
type ProjectTestDirReport struct {
	Dir       string `json:"dir"`
	TestMatch string `json:"testMatch"`
	Status    string `json:"status"`
	Reason    string `json:"reason,omitempty"`
	TestFiles int    `json:"testFiles"`
}

// Declaration is the member to embed as config.metadata.corvintRunIdentity (PWR-V0-046).
type Declaration struct {
	ApplicationRevision string            `json:"applicationRevision"`
	ConfigDigest        string            `json:"configDigest"`
	TestFileBlobs       map[string]string `json:"testFileBlobs"`
	TestRevision        string            `json:"testRevision"`
	TestRootPath        string            `json:"testRootPath"`
	// ServedArtifactDigest names the build the run served, present only when declared (PWR-V0-054).
	ServedArtifactDigest string `json:"servedArtifactDigest,omitempty"`
	// TestClosureDigests and TestClosureRule are present only with --closure-digests (PWR-V0-071).
	TestClosureDigests map[string]string `json:"testClosureDigests,omitempty"`
	TestClosureRule    string            `json:"testClosureRule,omitempty"`
}

// Counts sizes the inputs the declaration covers.
type Counts struct {
	TestFiles         int `json:"testFiles"`
	DigestFiles       int `json:"digestFiles"`
	UntrackedOutside  int `json:"untrackedOutsideDigest"`
	ModifiedDigested  int `json:"modifiedDigestFiles"`
	ModifiedTestFiles int `json:"modifiedTestFiles"`
	// ClosureDigests counts the declared closure digests, absent without --closure-digests.
	ClosureDigests int `json:"closureDigests,omitempty"`
}

// Document is the `corvint runs identity` output.
type Document struct {
	Profile            string      `json:"profile"`
	CorvintRunIdentity Declaration `json:"corvintRunIdentity"`
	Config             string      `json:"config"`
	TestDir            string      `json:"testDir"`
	TestMatch          string      `json:"testMatch"`
	Counts             Counts      `json:"counts"`
	Limits             []string    `json:"limits"`
	// ServedDigest is DECLARED when --artifact-digest named the served build, else UNKNOWN
	// (PWR-V0-054).
	ServedDigest string `json:"servedDigest"`
	// ProjectTestDirs is present only when a project testDir was named.
	ProjectTestDirs []*ProjectTestDirReport `json:"projectTestDirs,omitempty"`
	// Closure is present only with --compute-closures (PWR-V0-072, PWR-V0-073).
	Closure *ClosureReport `json:"closure,omitempty"`
}

// Closure sources of a ClosureReport (PWR-V0-072).
const (
	ClosureSourceComputed = "COMPUTED"
	ClosureSourceDeclared = "DECLARED"
)

// ClosureReport states where the closure digests came from and, for a computed closure, which
// test keys have none and why (PWR-V0-073). Widened is non-empty when no file could be excluded
// from any test's digest.
type ClosureReport struct {
	Source  string            `json:"source"`
	Unknown map[string]string `json:"unknown,omitempty"`
	// SharedCauses states each reason that leaves two or more tests unknown once, with the count
	// (PWR-V0-178), so a config-level cause is not read off as one finding per test.
	SharedCauses []SharedClosureCause `json:"sharedCauses,omitempty"`
	Widened      string               `json:"widened,omitempty"`
}

// SharedClosureCause is one reason several tests have no computed closure digest.
type SharedClosureCause struct {
	Reason string `json:"reason"`
	Tests  int    `json:"tests"`
}

func sharedClosureCauses(unknown map[string]string) []SharedClosureCause {
	counts := map[string]int{}
	for _, reason := range unknown {
		counts[reason]++
	}
	var shared []SharedClosureCause
	for reason, n := range counts {
		if n > 1 {
			shared = append(shared, SharedClosureCause{Reason: reason, Tests: n})
		}
	}
	sort.Slice(shared, func(i, j int) bool {
		if shared[i].Tests != shared[j].Tests {
			return shared[i].Tests > shared[j].Tests
		}
		return shared[i].Reason < shared[j].Reason
	})
	return shared
}

type entry struct {
	mode, blob, path string
}

// scope is one directory whose matching files are test files.
type scope struct {
	root    string
	matcher *regexp.Regexp
	report  *ProjectTestDirReport
}

type producer struct {
	ctx    context.Context
	budget *gitrun.Budget
}

// Produce computes the declaration by the PWR-V0-047..PWR-V0-049 rules.
func Produce(ctx context.Context, opts Options) (Document, error) {
	p := &producer{ctx: ctx, budget: gitrun.NewBudget(4096, 10*time.Minute)}
	match := opts.TestMatch
	if match == "" {
		match = DefaultTestMatch
	}
	matcher, err := regexp.Compile(match)
	if err != nil {
		return Document{}, refuse(ReasonTestMatchInvalid, "%v", err)
	}
	config, err := filepath.Abs(opts.Config)
	if err != nil {
		return Document{}, refuse(ReasonConfigUnreadable, "%s", opts.Config)
	}
	if info, err := os.Stat(config); err != nil || !info.Mode().IsRegular() {
		return Document{}, refuse(ReasonConfigUnreadable, "%s", opts.Config)
	}
	top, err := p.toplevel(filepath.Dir(config))
	if err != nil {
		return Document{}, err
	}
	testDir := opts.TestDir
	if testDir == "" {
		testDir = filepath.Dir(config)
	}
	testRoot, ok := within(top, testDir)
	if !ok {
		return Document{}, refuse(ReasonTestDirOutside, "%s", testDir)
	}
	configRel, ok := within(top, config)
	if !ok {
		return Document{}, refuse(ReasonTestDirOutside, "%s", opts.Config)
	}
	testRevision, err := p.revision(top, ReasonRevisionUnresolved)
	if err != nil {
		return Document{}, err
	}
	tracked, changed, err := p.checkout(top)
	if err != nil {
		return Document{}, err
	}
	untracked, err := p.nulList(top, "ls-files", "-z", "-o", "--exclude-standard")
	if err != nil {
		return Document{}, err
	}
	doc := Document{Profile: Profile, Config: configRel, TestDir: testRoot, TestMatch: match, Limits: []string{}}
	scopes, err := projectScopes(top, match, opts.ProjectTestDirs, &doc)
	if err != nil {
		return Document{}, err
	}
	scopes = append([]scope{{root: testRoot, matcher: matcher}}, scopes...)
	// A test file is keyed relative to the top-level testDir, as the reporter spells it, so a
	// project testDir outside it yields "../" keys (PWR-V0-052).
	isTest := func(repoPath string) (string, bool) {
		if hasSegment(repoPath, "node_modules") {
			return "", false
		}
		for _, s := range scopes {
			if rel, ok := under(s.root, repoPath); ok && s.matcher.MatchString(rel) {
				return reportKey(testRoot, repoPath), true
			}
		}
		return "", false
	}
	blobs := map[string]string{}
	digest := []entry{}
	rehash := []string{}
	configTracked := false
	for _, e := range tracked {
		configTracked = configTracked || e.path == configRel
		if _, test := isTest(e.path); test {
			continue
		}
		digest = append(digest, e)
	}
	isChanged := map[string]bool{}
	for _, name := range changed {
		isChanged[name] = true
	}
	type spec struct{ blob, repoPath, key string }
	specs := []spec{}
	for _, e := range tracked {
		if key, test := isTest(e.path); test {
			if _, err := os.Lstat(filepath.Join(top, filepath.FromSlash(e.path))); err != nil {
				continue // deleted from the working tree: Playwright cannot run it
			}
			specs = append(specs, spec{blob: e.blob, repoPath: e.path, key: key})
			if isChanged[e.path] {
				rehash = append(rehash, e.path)
			}
		}
	}
	for _, name := range untracked {
		if key, test := isTest(name); test {
			specs = append(specs, spec{repoPath: name, key: key})
			rehash = append(rehash, name)
			continue
		}
		if name == configRel {
			continue
		}
		doc.Counts.UntrackedOutside++
	}
	if !configTracked {
		if _, test := isTest(configRel); !test {
			digest = append(digest, entry{mode: "100644", path: configRel})
			rehash = append(rehash, configRel)
		}
	}
	for _, e := range digest {
		if isChanged[e.path] {
			rehash = append(rehash, e.path)
		}
	}
	hashed, err := p.worktreeBlobs(top, rehash)
	if err != nil {
		return Document{}, err
	}
	// The selected config is bound by name, so two configs over one tree never share a digest.
	lines := make([]string, 0, len(digest)+1)
	lines = append(lines, "config\t"+configRel+"\n")
	lineOf := make(map[string]string, len(digest)+len(specs))
	for _, e := range digest {
		blob := e.blob
		if h, ok := hashed[e.path]; ok && h != blob {
			blob = h
			doc.Counts.ModifiedDigested++
		}
		lines = append(lines, e.mode+" "+blob+"\t"+e.path+"\n")
		lineOf[e.path] = lines[len(lines)-1]
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "")))
	for _, s := range specs {
		blob := s.blob
		if h, ok := hashed[s.repoPath]; ok && h != blob {
			if blob != "" {
				doc.Counts.ModifiedTestFiles++
			}
			blob = h
		}
		blobs[s.key] = blob
		lineOf[s.repoPath] = "test " + blob + "\t" + s.repoPath + "\n"
		for _, sc := range scopes[1:] {
			if rel, ok := under(sc.root, s.repoPath); ok && sc.matcher.MatchString(rel) {
				sc.report.TestFiles++
			}
		}
	}
	for _, sc := range scopes[1:] {
		if sc.report.TestFiles == 0 {
			sc.report.Status, sc.report.Reason = "UNRESOLVED", LimitProjectTestDirNoTests
			doc.Limits = append(doc.Limits, LimitProjectTestDirNoTests)
		}
	}
	if len(blobs) == 0 {
		return Document{}, refuse(ReasonNoTestFiles, "no file under %s matches %s", testRoot, match)
	}
	// A declared closure must name a declared test key (PWR-V0-071).
	for key := range opts.ClosureDigests {
		if _, ok := blobs[key]; !ok {
			return Document{}, refuse(ReasonClosureInvalid, "a closure key is not a declared test key")
		}
	}
	app := top
	if opts.AppRoot != "" {
		if app, err = p.toplevel(opts.AppRoot); err != nil {
			var coded *Error
			if errors.As(err, &coded) && coded.Reason == ReasonNotRepository {
				return Document{}, refuse(ReasonApplicationUnresolved, "%s is not a Git checkout", opts.AppRoot)
			}
			return Document{}, err
		}
		if app != top {
			if _, _, err := p.checkout(app); err != nil {
				return Document{}, err
			}
		}
	}
	appRevision, dirty, err := p.applicationRevision(app)
	if err != nil {
		return Document{}, err
	}
	if dirty {
		doc.Limits = append(doc.Limits, LimitApplicationWorktreeDirty)
	}
	if doc.Counts.UntrackedOutside > 0 {
		doc.Limits = append(doc.Limits, LimitUntrackedOutsideDigest)
	}
	sort.Strings(doc.Limits)
	doc.Limits = slices.Compact(doc.Limits)
	doc.Counts.TestFiles, doc.Counts.DigestFiles = len(blobs), len(digest)
	doc.CorvintRunIdentity = Declaration{ApplicationRevision: appRevision, ConfigDigest: "sha256:" + hex.EncodeToString(sum[:]),
		TestFileBlobs: blobs, TestRevision: testRevision, TestRootPath: testRoot, ServedArtifactDigest: opts.ArtifactDigest}
	if len(opts.ClosureDigests) > 0 {
		doc.CorvintRunIdentity.TestClosureDigests, doc.CorvintRunIdentity.TestClosureRule = opts.ClosureDigests, ClosureRuleDeclared
		doc.Counts.ClosureDigests = len(opts.ClosureDigests)
		if opts.ComputeClosures {
			doc.Closure = &ClosureReport{Source: ClosureSourceDeclared}
		}
	} else if opts.ComputeClosures {
		keys := make(map[string]string, len(specs))
		for _, s := range specs {
			keys[s.repoPath] = s.key
		}
		computed, report := computeClosures(top, configRel, lineOf, keys, isChanged)
		doc.Closure = report
		doc.CorvintRunIdentity.TestClosureRule = ClosureRuleComputed
		if len(computed) > 0 {
			doc.CorvintRunIdentity.TestClosureDigests, doc.Counts.ClosureDigests = computed, len(computed)
		}
		if len(report.Unknown) > 0 {
			doc.Limits = append(doc.Limits, LimitClosureUnknown)
		}
		if report.Widened != "" {
			doc.Limits = append(doc.Limits, LimitClosureWidened)
		}
		sort.Strings(doc.Limits)
	}
	doc.ServedDigest = ServedDigestUnknown
	if opts.ArtifactDigest != "" {
		doc.ServedDigest = ServedDigestDeclared
	}
	return doc, nil
}

func (p *producer) git(dir string, stdin []byte, args ...string) ([]byte, error) {
	out, err := gitrun.Run(p.ctx, p.budget, gitrun.Options{Dir: dir, Env: environment(), Stdin: stdin, StdoutLimit: listLimit, PerOpTimeout: operationTimeout}, args...)
	if err != nil {
		return nil, refuse(ReasonGitReadFailed, "git %s: %v", args[0], err)
	}
	return out, nil
}

func (p *producer) toplevel(dir string) (string, error) {
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", refuse(ReasonNotRepository, "%s", dir)
	}
	out, err := gitrun.Run(p.ctx, p.budget, gitrun.Options{Dir: dir, Env: environment(), StdoutLimit: 8192, PerOpTimeout: operationTimeout}, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", refuse(ReasonNotRepository, "%s", dir)
	}
	top := strings.TrimRight(string(out), "\r\n")
	if resolved, err := filepath.EvalSymlinks(top); err == nil {
		top = resolved
	}
	return filepath.Clean(top), nil
}

func (p *producer) revision(dir, reason string) (string, error) {
	out, err := gitrun.Run(p.ctx, p.budget, gitrun.Options{Dir: dir, Env: environment(), StdoutLimit: 8192, PerOpTimeout: operationTimeout}, "rev-parse", "--verify", "--end-of-options", "HEAD^{commit}")
	if err != nil {
		return "", refuse(reason, "%s has no HEAD commit", dir)
	}
	return strings.TrimSpace(string(out)), nil
}

// applicationRevision is the checkout's HEAD commit, suffixed with the sha256 of its tracked
// uncommitted diff of tracked files when that diff is non-empty (PWR-V0-049).
func (p *producer) applicationRevision(dir string) (string, bool, error) {
	head, err := p.revision(dir, ReasonApplicationUnresolved)
	if err != nil {
		return "", false, err
	}
	h := &countingHash{Hash: sha256.New()}
	if err := gitrun.RunStream(p.ctx, p.budget, gitrun.Options{Dir: dir, Env: environment(), PerOpTimeout: operationTimeout}, h,
		"diff", "--binary", "--no-ext-diff", "--no-textconv", "--no-color", "--full-index", "--ignore-submodules=none", "HEAD", "--"); err != nil {
		return "", false, refuse(ReasonGitReadFailed, "git diff: %v", err)
	}
	if h.n == 0 {
		return head, false, nil // stat-only differences produce no diff bytes
	}
	return head + worktreeSuffix + hex.EncodeToString(h.Sum(nil)), true, nil
}

// countingHash records whether the streamed diff had any content.
type countingHash struct {
	hash.Hash
	n int64
}

func (c *countingHash) Write(b []byte) (int, error) {
	c.n += int64(len(b))
	return c.Hash.Write(b)
}

// checkout validates one checkout before any content is hashed and returns its index entries and
// the paths diff-files reports changed. It refuses a repository-configured clean or process filter
// (which would run a command), an unmerged entry, an assume-unchanged or skip-worktree entry (whose
// working-tree change Git would not report) and a modified submodule (whose inner edits no blob id
// names).
func (p *producer) checkout(dir string) ([]entry, []string, error) {
	cfg, err := p.git(dir, nil, "config", "-z", "--list")
	if err != nil {
		return nil, nil, err
	}
	for _, rec := range strings.Split(string(cfg), "\x00") {
		key, _, _ := strings.Cut(rec, "\n")
		key = strings.ToLower(key)
		if strings.HasPrefix(key, "filter.") && (strings.HasSuffix(key, ".clean") || strings.HasSuffix(key, ".process")) {
			return nil, nil, refuse(ReasonFilterConfigured, "%s", key)
		}
	}
	out, err := p.git(dir, nil, "ls-files", "-z", "-s", "-v")
	if err != nil {
		return nil, nil, err
	}
	entries := []entry{}
	for _, rec := range strings.Split(string(out), "\x00") {
		if rec == "" {
			continue
		}
		meta, name, ok := strings.Cut(rec, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 4 {
			return nil, nil, refuse(ReasonGitReadFailed, "unexpected ls-files record")
		}
		if fields[3] != "0" {
			return nil, nil, refuse(ReasonUnmerged, "%s", name)
		}
		if fields[0] != "H" {
			return nil, nil, refuse(ReasonIndexFlags, "%s (%s)", name, fields[0])
		}
		entries = append(entries, entry{mode: fields[1], blob: fields[2], path: name})
	}
	changed, err := p.nulList(dir, "diff-files", "-z", "--name-only", "--ignore-submodules=none")
	if err != nil {
		return nil, nil, err
	}
	isChanged := make(map[string]bool, len(changed))
	for _, name := range changed {
		isChanged[name] = true
	}
	for _, e := range entries {
		if e.mode == "160000" && isChanged[e.path] {
			return nil, nil, refuse(ReasonSubmoduleModified, "%s", e.path)
		}
	}
	return entries, changed, nil
}

func (p *producer) nulList(dir string, args ...string) ([]string, error) {
	out, err := p.git(dir, nil, args...)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, name := range strings.Split(string(out), "\x00") {
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

// worktreeBlobs hashes the named repository paths' working-tree content as Git blob ids without
// writing objects: regular files in one `hash-object --stdin-paths` batch, symlinks from their
// target text. A path that no longer exists hashes to "deleted".
func (p *producer) worktreeBlobs(top string, names []string) (map[string]string, error) {
	out := map[string]string{}
	regular := []string{}
	for _, name := range names {
		if _, done := out[name]; done {
			continue
		}
		info, err := os.Lstat(filepath.Join(top, filepath.FromSlash(name)))
		switch {
		case err != nil:
			out[name] = "deleted"
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(filepath.Join(top, filepath.FromSlash(name)))
			if err != nil {
				return nil, refuse(ReasonGitReadFailed, "readlink %s", name)
			}
			blob, err := p.git(top, []byte(filepath.ToSlash(target)), "hash-object", "--stdin")
			if err != nil {
				return nil, err
			}
			out[name] = strings.TrimSpace(string(blob))
		case info.Mode().IsRegular():
			if strings.ContainsAny(name, "\n\r") || strings.HasPrefix(name, "\"") {
				return nil, refuse(ReasonPathUnrepresentable, "%q", name)
			}
			out[name] = ""
			regular = append(regular, name)
		default:
			out[name] = "special"
		}
	}
	if len(regular) == 0 {
		return out, nil
	}
	raw, err := p.git(top, []byte(strings.Join(regular, "\n")+"\n"), "hash-object", "--stdin-paths")
	if err != nil {
		return nil, err
	}
	ids := strings.Fields(string(raw))
	if len(ids) != len(regular) {
		return nil, refuse(ReasonGitReadFailed, "hash-object returned %d ids for %d paths", len(ids), len(regular))
	}
	for i, name := range regular {
		out[name] = ids[i]
	}
	return out, nil
}

// projectScopes resolves each --project-test-dir (PWR-V0-052, PWR-V0-053). A directory outside the
// repository refuses before anything under it is read; one that does not exist as a directory is
// reported UNRESOLVED under a limit and contributes no files, never a guessed location.
func projectScopes(top, inherited string, dirs []ProjectTestDir, doc *Document) ([]scope, error) {
	scopes := []scope{}
	for _, d := range dirs {
		match := d.TestMatch
		if match == "" {
			match = inherited
		}
		matcher, err := regexp.Compile(match)
		if err != nil {
			return nil, refuse(ReasonTestMatchInvalid, "%s: %v", d.Dir, err)
		}
		report := &ProjectTestDirReport{Dir: filepath.ToSlash(d.Dir), TestMatch: match, Status: "RESOLVED"}
		doc.ProjectTestDirs = append(doc.ProjectTestDirs, report)
		root, ok := within(top, d.Dir)
		if !ok || danglingEscape(top, d.Dir, 0) {
			return nil, refuse(ReasonProjectTestDirOutside, "%s", d.Dir)
		}
		report.Dir = root
		if info, err := os.Stat(filepath.Join(top, filepath.FromSlash(root))); err != nil || !info.IsDir() {
			report.Status, report.Reason = "UNRESOLVED", LimitProjectTestDirUnresolved
			doc.Limits = append(doc.Limits, LimitProjectTestDirUnresolved)
			continue
		}
		scopes = append(scopes, scope{root: root, matcher: matcher, report: report})
	}
	return scopes, nil
}

// danglingEscape reports whether dir does not resolve because its deepest existing component is a
// symlink whose target (missing or not) lies outside top: within() alone would accept its lexical
// path. It reads link text only, never the target's content.
func danglingEscape(top, dir string, depth int) bool {
	abs, err := filepath.Abs(dir)
	if err != nil || depth > 40 {
		return true
	}
	if _, err := filepath.EvalSymlinks(abs); err == nil {
		return false
	}
	for p := abs; p != filepath.Dir(p); p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return false
		}
		target, err := os.Readlink(p)
		if err != nil {
			return true
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(p), target)
		}
		rest, err := filepath.Rel(p, abs)
		if err != nil {
			return true
		}
		next := filepath.Join(target, rest)
		if _, ok := within(top, next); !ok {
			return true
		}
		return danglingEscape(top, next, depth+1)
	}
	return false
}

// reportKey is repoPath relative to the top-level testDir, "../" included, which is how the JSON
// reporter spells a file under a project testDir outside it.
func reportKey(testRoot, repoPath string) string {
	if rel, ok := under(testRoot, repoPath); ok {
		return rel
	}
	rel, err := filepath.Rel(filepath.FromSlash(testRoot), filepath.FromSlash(repoPath))
	if err != nil {
		return repoPath
	}
	return filepath.ToSlash(rel)
}

// within is target's repository-relative slash path under top ("." for top itself).
func within(top, target string) (string, bool) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", false
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	} else if dir, derr := filepath.EvalSymlinks(filepath.Dir(abs)); derr == nil {
		abs = filepath.Join(dir, filepath.Base(abs))
	}
	rel, err := filepath.Rel(top, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// under is repoPath relative to root, when it lies below it.
func under(root, repoPath string) (string, bool) {
	if root == "." {
		return repoPath, true
	}
	if !strings.HasPrefix(repoPath, root+"/") {
		return "", false
	}
	return strings.TrimPrefix(repoPath, root+"/"), true
}

func hasSegment(p, segment string) bool {
	for _, part := range strings.Split(path.Clean(p), "/") {
		if part == segment {
			return true
		}
	}
	return false
}

// environment is the scrubbed child environment: what Git needs to run plus settings that keep the
// read deterministic, non-interactive and free of optional index writes.
func environment() []string {
	env := make([]string, 0, 16)
	for _, name := range []string{"PATH", "SystemRoot", "TMPDIR", "TEMP", "TMP", "USERPROFILE"} {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	return append(env,
		"LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0",
		"GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1", "GCM_INTERACTIVE=never", "GIT_ASKPASS=",
		// No fsmonitor hook runs on behalf of a read.
		"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.fsmonitor", "GIT_CONFIG_VALUE_0=false",
	)
}

// computeClosures digests each test's computed closure (PWR-V0-072): the config-digest lines and
// the test-file lines, less the files the closure proves the test cannot load. A modified file is
// never excluded, and a test whose closure is unknown, whose config admits it nowhere or whose
// closure holds a file outside the digest gets no digest and a reason, so it never pools on a
// narrowed identity (PWR-V0-073).
func computeClosures(top, configRel string, lineOf, keys map[string]string, changed map[string]bool) (map[string]string, *ClosureReport) {
	report := &ClosureReport{Source: ClosureSourceComputed, Unknown: map[string]string{}}
	closures, err := typescript.PlaywrightTestClosures(top, configRel)
	if err != nil {
		for _, key := range keys {
			report.Unknown[key] = "closures were not computed: " + err.Error()
		}
		report.SharedCauses = sharedClosureCauses(report.Unknown)
		return nil, report
	}
	byTest := make(map[string]typescript.PlaywrightTestClosure, len(closures))
	for _, closure := range closures {
		byTest[closure.Test] = closure
	}
	paths := make([]string, 0, len(lineOf))
	for path := range lineOf {
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool { return lineOf[paths[i]] < lineOf[paths[j]] })
	digests := map[string]string{}
	for repoPath, key := range keys {
		closure, admitted := byTest[repoPath]
		switch {
		case !admitted:
			report.Unknown[key] = "no project of the Playwright config admits " + repoPath
		case closure.Unknown != "":
			report.Unknown[key] = closure.Unknown
		}
		for _, file := range closure.Files {
			if report.Unknown[key] == "" && lineOf[file] == "" {
				report.Unknown[key] = "closure file " + file + " is outside the digest (untracked or ignored)"
			}
		}
		if report.Unknown[key] != "" {
			continue
		}
		if closure.Widened != "" && (report.Widened == "" || closure.Widened < report.Widened) {
			report.Widened = closure.Widened
		}
		excluded := make(map[string]bool, len(closure.Excludable))
		for _, file := range closure.Excludable {
			excluded[file] = !changed[file]
		}
		sum := sha256.New()
		io.WriteString(sum, "rule\t"+ClosureRuleComputed+"\nconfig\t"+configRel+"\n")
		for _, path := range paths {
			if !excluded[path] {
				io.WriteString(sum, lineOf[path])
			}
		}
		digests[key] = "sha256:" + hex.EncodeToString(sum.Sum(nil))
	}
	if len(report.Unknown) == 0 {
		report.Unknown = nil
	}
	report.SharedCauses = sharedClosureCauses(report.Unknown)
	return digests, report
}
