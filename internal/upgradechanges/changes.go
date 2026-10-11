// Package upgradechanges compiles an experimental, offline upgrade checklist.
package upgradechanges

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
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
)

var (
	shaPattern   = regexp.MustCompile(`^[a-fA-F0-9]{7,64}$`)
	mergePattern = regexp.MustCompile(`^Merge pull request #([1-9][0-9]{0,14}) from .+$`)
	issuePattern = regexp.MustCompile(`(?i)\b(closes|fixes|resolves|refs)[ \t]+#([1-9][0-9]{0,14})\b`)
	refPattern   = regexp.MustCompile(`#([1-9][0-9]{0,14})\b`)
)

type Request struct {
	Root, From, To string
	Files          []string
}
type PullRequest struct {
	Number   int64  `json:"number"`
	Merge    string `json:"merge"`
	Summary  string `json:"summary"`
	Commands string `json:"new_flags_commands"`
}
type Issue struct {
	Number   int64  `json:"number"`
	PR       int64  `json:"pr"`
	Commit   string `json:"commit"`
	Evidence string `json:"evidence"`
	State    string `json:"state"`
	Summary  string `json:"summary"`
}
type Hint struct {
	File      string `json:"file"`
	Line      int    `json:"line"`
	Issue     int64  `json:"issue"`
	ClosingPR int64  `json:"closing_pr"`
	Message   string `json:"message"`
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

func Compile(ctx context.Context, req Request) (Report, error) {
	r := Report{Profile: "corvint-upgrade-checklist/0", PRs: []PullRequest{}, Issues: []Issue{}, Hints: []Hint{}, Unknowns: []string{
		"GitHub issue state, reopen/revert semantics: NOT_OBSERVED",
		"new flags/commands per PR: NOT_OBSERVED",
		"issue state without closing keywords, unassociated commits and unsupported merge forms: NOT_OBSERVED",
		"closures before FROM and references outside this source repository: NOT_OBSERVED",
	}}
	if len(req.Files) > maxFiles {
		return r, errors.New("changes-invalid-arguments")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
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
		r.PRs = append(r.PRs, PullRequest{number, id, summary, "NOT_OBSERVED"})
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
				issueRows[key] = Issue{num, number, commit, keyword, state, safeText(subject, 240)}
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
				r.Hints = append(r.Hints, Hint{screenedName, line, num, issue.PR, "workaround may be removable"})
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

// readRefsFile is replaceable so a test can model a stalled filesystem.
var readRefsFile = readRefs

type refsRead struct {
	data []byte
	cut  bool
	err  error
}

// readRefsContext returns when ctx ends even if the underlying read is stalled
// (for example a regular file on an unresponsive remote filesystem). The
// abandoned read goroutine holds only its descriptor until the process exits.
func readRefsContext(ctx context.Context, path string) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	done := make(chan refsRead, 1)
	read := readRefsFile
	go func() {
		data, cut, err := read(path)
		done <- refsRead{data, cut, err}
	}()
	select {
	case r := <-done:
		return r.data, r.cut, r.err
	case <-ctx.Done():
		return nil, false, ctx.Err()
	}
}
