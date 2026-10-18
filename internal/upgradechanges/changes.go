// Package upgradechanges compiles an experimental, offline upgrade checklist.
package upgradechanges

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

const (
	maxCommits   = 256
	maxMessage   = 64 << 10
	maxGitBytes  = 4 << 20
	maxFiles     = 16
	maxFileBytes = 1 << 20
	maxRows      = 1024
	maxRefs      = 4096
	maxFlagDiff  = 256 << 10
	maxFlags     = 64
)

// Range and time bounds (UCL1142-V0-008). They are variables only so tests can lower them.
var (
	// maxRangeCommits bounds every commit in FROM..TO, counted before any message is read.
	maxRangeCommits int64 = 512
	// compileDeadline is the total wall-time deadline of one Compile call.
	compileDeadline = 30 * time.Second
)

// RangeTooLargeError refuses a FROM..TO range holding more commits than the bound
// (UCL1142-V0-008); its code is changes-range-too-large.
type RangeTooLargeError struct {
	Limit   int64
	Commits int64
}

func (*RangeTooLargeError) Error() string { return "changes-range-too-large" }

// TimeoutError refuses a call that reached its own total deadline (UCL1142-V0-008);
// its code is changes-timeout.
type TimeoutError struct {
	LimitMS   int64
	ElapsedMS int64
}

func (*TimeoutError) Error() string { return "changes-timeout" }

var (
	shaPattern   = regexp.MustCompile(`^[a-fA-F0-9]{7,64}$`)
	mergePattern = regexp.MustCompile(`^Merge pull request #([1-9][0-9]{0,14}) from .+$`)
	issuePattern = regexp.MustCompile(`(?i)\b(closes|fixes|resolves|refs)[ \t]+#([1-9][0-9]{0,14})\b`)
	refPattern   = regexp.MustCompile(`#([1-9][0-9]{0,14})\b`)
	// Flag names declared through the standard flag package or compared as
	// "--name" literals in cmd/corvint Go source (UCL1142-V0-007).
	flagDeclPattern    = regexp.MustCompile(`\.(?:String|Bool|Int|Int64|Uint|Uint64|Float64|Duration|Func|BoolFunc)\(\s*"([A-Za-z0-9][A-Za-z0-9_.-]{0,63})"`)
	flagVarPattern     = regexp.MustCompile(`\.(?:StringVar|BoolVar|IntVar|Int64Var|UintVar|Uint64Var|Float64Var|DurationVar|Var|TextVar)\([^"\n]*?,\s*"([A-Za-z0-9][A-Za-z0-9_.-]{0,63})"`)
	flagLiteralPattern = regexp.MustCompile(`"--([A-Za-z0-9][A-Za-z0-9_.-]{0,63})=?"`)
)

const (
	notObserved     = "NOT_OBSERVED"
	flagSourceBasis = "cmd-corvint-go-source-diff"
)

type Request struct {
	Root, From, To string
	Files          []string
	// IssueStateFile is an optional operator-supplied issue-state export
	// (UCL1142-V0-006); Corvint never queries GitHub itself.
	IssueStateFile string
}
type PullRequest struct {
	Number     int64    `json:"number"`
	Merge      string   `json:"merge"`
	Summary    string   `json:"summary"`
	FlagsBasis string   `json:"new_flags_basis"`
	NewFlags   []string `json:"new_flags"`
	Commands   string   `json:"new_commands"`
}
type Issue struct {
	Number      int64  `json:"number"`
	PR          int64  `json:"pr"`
	Commit      string `json:"commit"`
	Evidence    string `json:"evidence"`
	State       string `json:"state"`
	GitHubState string `json:"github_state"`
	Summary     string `json:"summary"`
}
type Hint struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Issue       int64  `json:"issue"`
	ClosingPR   int64  `json:"closing_pr"`
	GitHubState string `json:"github_state"`
	Message     string `json:"message"`
}
type Report struct {
	Profile   string        `json:"profile"`
	From      string        `json:"from"`
	To        string        `json:"to"`
	PRs       []PullRequest `json:"prs"`
	Issues    []Issue       `json:"issues"`
	Hints     []Hint        `json:"hints"`
	Unknowns  []string      `json:"unknowns"`
	Truncated bool          `json:"truncated"`
}

// cappedOutput keeps subprocess output bounded even if a local object is hostile.
type cappedOutput struct {
	buffer   bytes.Buffer
	limit    int
	exceeded bool
	cancel   context.CancelFunc
}

func (b *cappedOutput) Write(p []byte) (int, error) {
	n := len(p)
	if len(p) > b.limit-b.buffer.Len() {
		p = p[:b.limit-b.buffer.Len()]
		b.exceeded = true
		if b.cancel != nil {
			b.cancel()
		}
	}
	_, _ = b.buffer.Write(p)
	return n, nil
}

type gitReader struct {
	ctx       context.Context
	root      string
	remaining int
}

func (g *gitReader) read(limit int, args ...string) ([]byte, bool, error) {
	if g.remaining == 0 {
		return nil, true, nil
	}
	if limit > g.remaining {
		limit = g.remaining
	}
	ctx, cancel := context.WithCancel(g.ctx)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"--no-replace-objects", "-C", g.root}, args...)...)
	// Do not inherit object-directory, config-injection or worktree overrides. No
	// command invokes hooks, a pager, a diff driver, or a network operation.
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, "GIT_") {
			cmd.Env = append(cmd.Env, v)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_NO_LAZY_FETCH=1", "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_PAGER=cat")
	out := &cappedOutput{limit: limit, cancel: cancel}
	cmd.Stdout = out
	cmd.Stderr = io.Discard
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	g.remaining -= out.buffer.Len()
	if g.ctx.Err() != nil || (err != nil && !out.exceeded) {
		return nil, false, errors.New("changes-git-unavailable")
	}
	return out.buffer.Bytes(), out.exceeded, nil
}
func (g *gitReader) resolve(value string) (string, error) {
	if !shaPattern.MatchString(value) {
		return "", errors.New("changes-invalid-arguments")
	}
	raw, cut, err := g.read(128, "rev-parse", "--verify", "--end-of-options", value+"^{commit}")
	if g.ctx.Err() != nil {
		return "", errors.New("changes-git-unavailable")
	}
	if err != nil || cut {
		return "", errors.New("changes-build-unavailable")
	}
	id := strings.TrimSpace(string(raw))
	if !shaPattern.MatchString(id) {
		return "", errors.New("changes-build-unavailable")
	}
	return id, nil
}
func (g *gitReader) ids(args ...string) ([]string, bool, error) {
	raw, cut, err := g.read((maxCommits+1)*66, args...)
	if err != nil {
		return nil, false, err
	}
	// Only complete lines are usable after a byte cap.
	if cut {
		if end := bytes.LastIndexByte(raw, '\n'); end >= 0 {
			raw = raw[:end+1]
		} else {
			raw = nil
		}
	}
	ids := strings.Fields(string(raw))
	if len(ids) > maxCommits {
		ids = ids[:maxCommits]
		cut = true
	}
	return ids, cut, nil
}
func (g *gitReader) message(id string) (string, bool, error) {
	raw, cut, err := g.read(maxMessage, "show", "--no-patch", "--no-show-signature", "--encoding=UTF-8", "--format=%B", "--no-notes", id, "--")
	if cut {
		return "", true, err
	}
	return string(raw), false, err
}
func safeText(s string, limit int) string {
	s, _ = secretscreen.Screen(s)
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > limit {
		s = s[:limit]
		for !utf8.ValidString(s) {
			s = s[:len(s)-1]
		}
	}
	return s
}

// errCompileDeadline is the cancellation cause of Compile's own deadline.
var errCompileDeadline = errors.New("changes-timeout")

// Compile builds the checklist within compileDeadline. Reaching that deadline refuses with
// TimeoutError rather than changes-git-unavailable, which stays reserved for Git that cannot run
// or a repository that cannot be read; a caller's own cancellation keeps changes-git-unavailable
// (UCL1142-V0-008). The classification reads the cancellation cause, which the first cancellation
// fixes, so a caller cancelling after the deadline fired cannot turn it into git-unavailable.
func Compile(parent context.Context, req Request) (Report, error) {
	start := time.Now()
	ctx, cancel := context.WithTimeoutCause(parent, compileDeadline, errCompileDeadline)
	defer cancel()
	r, err := compile(ctx, req)
	if err != nil && err.Error() == "changes-git-unavailable" && errors.Is(context.Cause(ctx), errCompileDeadline) {
		return r, &TimeoutError{LimitMS: compileDeadline.Milliseconds(), ElapsedMS: time.Since(start).Milliseconds()}
	}
	return r, err
}

func compile(ctx context.Context, req Request) (Report, error) {
	issueStateUnknown := "GitHub issue state: NOT_OBSERVED"
	if req.IssueStateFile != "" {
		issueStateUnknown = "GitHub issue state: operator-supplied file, not verified by Corvint"
	}
	r := Report{Profile: "corvint-upgrade-checklist/0", PRs: []PullRequest{}, Issues: []Issue{}, Hints: []Hint{}, Unknowns: []string{
		issueStateUnknown,
		"reopen/revert semantics: NOT_OBSERVED",
		"new commands per PR, and flags not declared by a literal in cmd/corvint Go source: NOT_OBSERVED",
		"issue state without closing keywords, unassociated commits and unsupported merge forms: NOT_OBSERVED",
		"closures before FROM and references outside this source repository: NOT_OBSERVED",
	}}
	if len(req.Files) > maxFiles {
		return r, errors.New("changes-invalid-arguments")
	}
	githubStates := map[int64]string{}
	if req.IssueStateFile != "" {
		var err error
		if githubStates, err = readIssueStates(ctx, req.Root, req.IssueStateFile); err != nil {
			return r, err
		}
	}
	g := gitReader{ctx: ctx, root: req.Root, remaining: maxGitBytes}
	var err error
	if r.From, err = g.resolve(req.From); err != nil {
		return r, err
	}
	if r.To, err = g.resolve(req.To); err != nil {
		return r, err
	}
	// Reject reversed or unrelated builds without querying a remote.
	_, cut, err := g.read(128, "merge-base", "--is-ancestor", r.From, r.To)
	if ctx.Err() != nil {
		return r, errors.New("changes-git-unavailable")
	}
	if err != nil || cut {
		return r, errors.New("changes-range-unavailable")
	}
	// A shallow checkout silently omits carried commits; refuse rather than report a complete range.
	shallow, _, err := g.read(64, "rev-parse", "--is-shallow-repository")
	if ctx.Err() != nil {
		return r, errors.New("changes-git-unavailable")
	}
	if err != nil || strings.TrimSpace(string(shallow)) != "false" {
		return r, errors.New("changes-range-unavailable")
	}
	// Count the whole range before reading any message, so a range over the bound refuses at once
	// with the bound and the count instead of spending the deadline (UCL1142-V0-008).
	counted, cut, err := g.read(32, "rev-list", "--count", r.From+".."+r.To)
	if err != nil {
		return r, err
	}
	commits, perr := strconv.ParseInt(strings.TrimSpace(string(counted)), 10, 64)
	if cut || perr != nil || commits < 0 {
		return r, errors.New("changes-git-unavailable")
	}
	if commits > maxRangeCommits {
		return r, &RangeTooLargeError{Limit: maxRangeCommits, Commits: commits}
	}
	ids, cut, err := g.ids("rev-list", "--first-parent", "--max-count=257", r.From+".."+r.To)
	if err != nil {
		return r, err
	}
	r.Truncated = cut
	issueRows := map[string]Issue{}
	for _, id := range ids {
		if ctx.Err() != nil {
			return r, errors.New("changes-git-unavailable")
		}
		msg, cut, e := g.message(id)
		if e != nil {
			return r, e
		}
		if cut {
			r.Truncated = true
			continue
		}
		subject, body, _ := strings.Cut(msg, "\n")
		match := mergePattern.FindStringSubmatch(subject)
		if match == nil {
			continue
		}
		parents, pcut, e := g.read(256, "show", "--no-patch", "--no-show-signature", "--encoding=UTF-8", "--no-notes", "--format=%P", id, "--")
		if e != nil {
			return r, e
		}
		if pcut {
			r.Truncated = true
			continue
		}
		pp := strings.Fields(string(parents))
		if len(pp) != 2 {
			continue
		}
		number, _ := strconv.ParseInt(match[1], 10, 64)
		summary := "Merge pull request #" + match[1]
		for _, line := range strings.Split(body, "\n") {
			if strings.TrimSpace(line) != "" {
				summary = safeText(line, 240)
				break
			}
		}
		flags, basis, e := g.newFlags(pp[0], id)
		if e != nil {
			return r, e
		}
		if basis == notObserved {
			r.Truncated = true
		}
		r.PRs = append(r.PRs, PullRequest{number, id, summary, basis, flags, notObserved})
		add := func(commit, message string) {
			for _, m := range issuePattern.FindAllStringSubmatch(message, -1) {
				num, _ := strconv.ParseInt(m[2], 10, 64)
				keyword := strings.ToLower(m[1])
				state := "closed_by_commit_message"
				if keyword == "refs" {
					state = "unknown"
				}
				key := strconv.FormatInt(number, 10) + ":" + m[2]
				old, exists := issueRows[key]
				if exists && (old.State == "closed_by_commit_message" || state == "unknown") {
					continue
				}
				if !exists && len(issueRows) >= maxRows {
					r.Truncated = true
					continue
				}
				subject, _, _ := strings.Cut(message, "\n")
				issueRows[key] = Issue{num, number, commit, keyword, state, githubState(githubStates, num), safeText(subject, 240)}
			}
		}
		add(id, msg)
		carried, ccut, e := g.ids("rev-list", "--topo-order", "--max-count=257", id, "^"+pp[0], "^"+r.From)
		if e != nil {
			return r, e
		}
		if ccut {
			r.Truncated = true
		}
		for _, commit := range carried {
			if commit == id {
				continue
			}
			message, mcut, e := g.message(commit)
			if e != nil {
				return r, e
			}
			if mcut {
				r.Truncated = true
				continue
			}
			add(commit, message)
		}
	}
	for _, row := range issueRows {
		r.Issues = append(r.Issues, row)
	}
	sort.Slice(r.PRs, func(i, j int) bool {
		if r.PRs[i].Number != r.PRs[j].Number {
			return r.PRs[i].Number < r.PRs[j].Number
		}
		return r.PRs[i].Merge < r.PRs[j].Merge
	})
	sort.Slice(r.Issues, func(i, j int) bool {
		if r.Issues[i].Number != r.Issues[j].Number {
			return r.Issues[i].Number < r.Issues[j].Number
		}
		return r.Issues[i].PR < r.Issues[j].PR
	})
	closed := map[int64][]Issue{}
	for _, issue := range r.Issues {
		if issue.State == "closed_by_commit_message" {
			closed[issue.Number] = append(closed[issue.Number], issue)
		}
	}
	files := append([]string(nil), req.Files...)
	sort.Strings(files)
	for i, name := range files {
		if ctx.Err() != nil {
			return r, errors.New("changes-git-unavailable")
		}
		if i > 0 && name == files[i-1] {
			continue
		}
		if len(name) > 4096 {
			return r, errors.New("changes-refs-unavailable")
		}
		path := name
		if !filepath.IsAbs(path) {
			path = filepath.Join(req.Root, path)
		}
		path, err = filepath.Abs(path)
		if err != nil || len(path) > 4096 {
			return r, errors.New("changes-refs-unavailable")
		}
		data, cut, e := readRefsContext(ctx, path)
		if ctx.Err() != nil {
			return r, errors.New("changes-git-unavailable")
		}
		if e != nil {
			return r, errors.New("changes-refs-unavailable")
		}
		if cut {
			r.Truncated = true
		}
		// Omit an incomplete final line after truncation; it may end mid-number.
		if cut {
			if end := bytes.LastIndexByte(data, '\n'); end >= 0 {
				data = data[:end+1]
			} else {
				data = nil
			}
		}
		matches := refPattern.FindAllSubmatchIndex(data, maxRefs+1)
		if len(matches) > maxRefs {
			matches = matches[:maxRefs]
			r.Truncated = true
		}
		line, offset := 1, 0
		screenedName, _ := secretscreen.Screen(name)
	scanRefs:
		for _, m := range matches {
			if ctx.Err() != nil {
				return r, errors.New("changes-git-unavailable")
			}
			line += bytes.Count(data[offset:m[0]], []byte{'\n'})
			offset = m[0]
			num, _ := strconv.ParseInt(string(data[m[2]:m[3]]), 10, 64)
			for _, issue := range closed[num] {
				if len(r.Hints) >= maxRows {
					r.Truncated = true
					break scanRefs
				}
				message := "workaround may be removable"
				if issue.GitHubState == "OPEN" {
					message = "closing evidence, but the supplied issue state is OPEN; review"
				}
				r.Hints = append(r.Hints, Hint{screenedName, line, num, issue.PR, issue.GitHubState, message})
			}
		}
	}
	sort.SliceStable(r.Hints, func(i, j int) bool {
		a, b := r.Hints[i], r.Hints[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Issue != b.Issue {
			return a.Issue < b.Issue
		}
		return a.ClosingPR < b.ClosingPR
	})
	if ctx.Err() != nil {
		return r, errors.New("changes-git-unavailable")
	}
	return r, nil
}

// newFlags reports flag names added by one merge relative to its first parent
// in non-test cmd/corvint Go source (UCL1142-V0-007). A name that the same diff
// also removes is a move, not a new flag. Partial diff output is NOT_OBSERVED,
// never a complete empty list.
func (g *gitReader) newFlags(parent, merge string) ([]string, string, error) {
	raw, cut, err := g.read(maxFlagDiff, "diff", "--text", "--no-ext-diff", "--no-textconv", "--no-color", "--no-renames", "--no-relative",
		"--unified=0", "--src-prefix=a/", "--dst-prefix=b/", parent, merge, "--", "cmd/corvint")
	if err != nil {
		return nil, "", err
	}
	if cut {
		return []string{}, notObserved, nil
	}
	added, removed := map[string]bool{}, map[string]bool{}
	oldSource, newSource, inHunk := false, false, false
	for _, line := range strings.Split(string(raw), "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			oldSource, newSource, inHunk = false, false, false
		case !inHunk && (strings.HasPrefix(line, "Binary files ") || line == "GIT binary patch"):
			// --text should prevent this; any binary patch is unparsed evidence.
			return []string{}, notObserved, nil
		case !inHunk && strings.HasPrefix(line, "--- "):
			name, ok := patchPath(line[4:], "a/")
			if !ok {
				return []string{}, notObserved, nil
			}
			oldSource = flagSourceFile(name)
		case !inHunk && strings.HasPrefix(line, "+++ "):
			name, ok := patchPath(line[4:], "b/")
			if !ok {
				return []string{}, notObserved, nil
			}
			newSource = flagSourceFile(name)
		case strings.HasPrefix(line, "@@"):
			inHunk = true
		case inHunk && newSource && strings.HasPrefix(line, "+"):
			collectFlags(added, line[1:])
		case inHunk && oldSource && strings.HasPrefix(line, "-"):
			collectFlags(removed, line[1:])
		}
	}
	flags := []string{}
	for name := range added {
		if !removed[name] {
			flags = append(flags, "--"+name)
		}
	}
	sort.Strings(flags)
	if len(flags) > maxFlags {
		return []string{}, notObserved, nil
	}
	return flags, flagSourceBasis, nil
}

// patchPath decodes one ---/+++ header path: /dev/null, a plain path with an
// optional trailing tab (Git adds one when the name has a space), or a C-quoted
// path. It reports false only for a header it cannot decode, so the caller
// abstains instead of silently skipping a source file.
func patchPath(header, prefix string) (string, bool) {
	header = strings.TrimSuffix(header, "\t")
	if header == "/dev/null" {
		return "", true
	}
	if strings.HasPrefix(header, `"`) {
		unquoted, err := strconv.Unquote(header)
		if err != nil {
			return "", false
		}
		header = unquoted
	}
	name, ok := strings.CutPrefix(header, prefix)
	return name, ok
}

func flagSourceFile(name string) bool {
	rest, ok := strings.CutPrefix(name, "cmd/corvint/")
	return ok && !strings.Contains(rest, "/") && strings.HasSuffix(rest, ".go") && !strings.HasSuffix(rest, "_test.go")
}

func collectFlags(into map[string]bool, line string) {
	for _, pattern := range []*regexp.Regexp{flagDeclPattern, flagVarPattern, flagLiteralPattern} {
		for _, m := range pattern.FindAllStringSubmatch(line, -1) {
			into[m[1]] = true
		}
	}
}

// uniqueMembers decodes one JSON object, refusing a repeated member name so a
// row such as {"state":"OPEN","state":"CLOSED"} cannot hide a conflict.
func uniqueMembers(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if token, err := dec.Token(); err != nil || token != json.Delim('{') {
		return nil, false
	}
	row := map[string]json.RawMessage{}
	for dec.More() {
		token, err := dec.Token()
		key, isKey := token.(string)
		if err != nil || !isKey {
			return nil, false
		}
		if _, seen := row[key]; seen {
			return nil, false
		}
		var value json.RawMessage
		if dec.Decode(&value) != nil {
			return nil, false
		}
		row[key] = value
	}
	return row, true
}

func githubState(states map[int64]string, issue int64) string {
	if state, ok := states[issue]; ok {
		return state
	}
	return notObserved
}

// readIssueStates reads an operator-supplied JSON array such as the output of
// `gh issue list --state all --json number,state`. Only the issue number and
// OPEN/CLOSED state are used; other members are ignored and never emitted. The
// file is read with the same no-follow bounded reader as --refs-file
// (UCL1142-V0-006).
func readIssueStates(ctx context.Context, root, name string) (map[int64]string, error) {
	refuse := errors.New("changes-issue-state-unavailable")
	path := name
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path, err := filepath.Abs(path)
	if err != nil || len(name) > 4096 || len(path) > 4096 {
		return nil, refuse
	}
	data, cut, err := readRefsContext(ctx, path)
	if ctx.Err() != nil {
		return nil, errors.New("changes-git-unavailable")
	}
	if err != nil || cut {
		return nil, refuse
	}
	var raws []json.RawMessage
	if json.Unmarshal(data, &raws) != nil || raws == nil || len(raws) > maxRows*4 {
		return nil, refuse
	}
	states := map[int64]string{}
	for _, raw := range raws {
		row, ok := uniqueMembers(raw)
		if !ok {
			return nil, refuse
		}
		var number int64
		var state string
		if json.Unmarshal(row["number"], &number) != nil || number <= 0 || json.Unmarshal(row["state"], &state) != nil {
			return nil, refuse
		}
		state = strings.ToUpper(state)
		if state != "OPEN" && state != "CLOSED" {
			return nil, refuse
		}
		if old, ok := states[number]; ok && old != state {
			return nil, refuse
		}
		states[number] = state
	}
	return states, nil
}

// openRefsFile is replaceable so a test can model a stalled filesystem.
var openRefsFile = func(path string) (io.ReadCloser, error) {
	file, err := openRefs(path)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// beforeRefsWait is replaceable so a test can hold readRefsContext until both
// the read result and the cancellation are ready (V1-1411).
var beforeRefsWait = func(<-chan refsRead) {}

type refsRead struct {
	data []byte
	cut  bool
	err  error
}

// readRefsContext returns when ctx ends even if the underlying read is stalled
// (for example a regular file on an unresponsive remote filesystem). On
// cancellation it closes the open descriptor, which ends any read the platform
// can interrupt, so the reading goroutine exits instead of living until process
// exit. A read the kernel cannot interrupt still holds that goroutine until the
// system call itself returns; nothing waits for it (V1-1155).
func readRefsContext(ctx context.Context, path string) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	var mu sync.Mutex
	var open io.Closer
	cancelled := false
	stop := context.AfterFunc(ctx, func() {
		mu.Lock()
		defer mu.Unlock()
		cancelled = true
		if open != nil {
			_ = open.Close()
		}
	})
	// stop runs only after a completed read. On cancellation the callback stays
	// armed: ctx.Done can close before the callback is dispatched, and stopping
	// it then would leave the descriptor open and its reader stalled.
	done := make(chan refsRead, 1)
	openFile := openRefsFile
	go func() {
		file, err := openFile(path)
		if err != nil {
			done <- refsRead{err: err}
			return
		}
		mu.Lock()
		if cancelled {
			mu.Unlock()
			_ = file.Close()
			done <- refsRead{err: context.Canceled}
			return
		}
		open = file
		mu.Unlock()
		raw, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
		mu.Lock()
		open = nil
		mu.Unlock()
		_ = file.Close()
		if len(raw) > maxFileBytes {
			done <- refsRead{raw[:maxFileBytes], true, err}
			return
		}
		done <- refsRead{raw, false, err}
	}()
	beforeRefsWait(done)
	select {
	case r := <-done:
		stop()
		// A read failed by the cancellation close (os.ErrClosed) can win the
		// select against ctx.Done; report the cancellation, not the close (V1-1411).
		if r.err != nil {
			if err := ctx.Err(); err != nil {
				return nil, false, err
			}
		}
		return r.data, r.cut, r.err
	case <-ctx.Done():
		return nil, false, ctx.Err()
	}
}
