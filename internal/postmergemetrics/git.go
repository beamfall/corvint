package postmergemetrics

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/procgroup"
)

type GitMeasurer struct{ executable string }

func NewGitMeasurer() (*GitMeasurer, error) {
	p, e := exec.LookPath("git")
	if e != nil {
		return nil, ErrGit
	}
	p, e = filepath.Abs(p)
	if e != nil {
		return nil, ErrGit
	}
	return &GitMeasurer{executable: p}, nil
}
func (g *GitMeasurer) command(ctx context.Context, p GitPair, input []byte, args ...string) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ErrCancelled
	}
	argv := []string{g.executable, "--no-pager", "--no-replace-objects", "--literal-pathspecs", "--attr-source=" + p.Approved,
		"-c", "core.attributesFile=/dev/null", "-c", "core.bigFileThreshold=512m", "-c", "core.fsmonitor=false", "-c", "diff.algorithm=myers", "-c", "diff.renames=false", "-c", "diff.ignoreSubmodules=none", "-c", "protocol.allow=never", "-C", p.Root}
	argv = append(argv, args...)
	env := []string{"PATH=/usr/bin:/bin", "LC_ALL=C", "LANG=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_ATTR_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1", "GIT_OPTIONAL_LOCKS=0"}
	out := procgroup.Run(ctx, procgroup.Spec{Argv: argv, Dir: p.Root, Env: env, Stdin: input, Timeout: 10 * time.Second, OutputLimit: 4 << 20, StderrLimit: 64 << 10, InputLimit: 8 << 20, ObserveDescendants: true})
	if ctx.Err() != nil || out.Cancelled || out.TimedOut {
		return nil, ErrCancelled
	}
	if out.Err != nil || !out.ExitObserved || out.ExitStatus != 0 || !out.OwnedProcessGroupCleanup {
		return nil, ErrGit
	}
	return out.Stdout, nil
}
func (g *GitMeasurer) Measure(ctx context.Context, p GitPair) (Measurement, error) {
	var m Measurement
	if !filepath.IsAbs(p.Root) || !oid.MatchString(p.Bot) || !oid.MatchString(p.Approved) || len(p.Bot) != len(p.Approved) {
		return m, ErrGit
	}
	root, e := filepath.EvalSymlinks(p.Root)
	if e != nil {
		return m, ErrGit
	}
	p.Root = root
	top, e := g.command(ctx, p, nil, "rev-parse", "--show-toplevel")
	if e != nil {
		return m, e
	}
	if strings.TrimSuffix(string(top), "\n") != root {
		return m, ErrGit
	}
	// A local attributes override is mutable and outranks the pinned commit's attributes.
	attrs, e := g.command(ctx, p, nil, "rev-parse", "--path-format=absolute", "--git-path", "info/attributes")
	if e != nil {
		return m, e
	}
	attrPath := strings.TrimSuffix(string(attrs), "\n")
	if !filepath.IsAbs(attrPath) {
		return m, ErrGit
	}
	if _, e = os.Lstat(attrPath); !os.IsNotExist(e) {
		return m, ErrGit
	}
	version, e := g.command(ctx, p, nil, "--version")
	if e != nil {
		return m, e
	}
	if len(version) > 256 || !strings.HasPrefix(string(version), "git version ") {
		return m, ErrGit
	}
	for _, commit := range []string{p.Bot, p.Approved} {
		v, e := g.command(ctx, p, nil, "cat-file", "-t", commit)
		if e != nil {
			return m, e
		}
		if string(v) != "commit\n" {
			return m, ErrGit
		}
	}
	if _, e = g.command(ctx, p, nil, "merge-base", "--is-ancestor", p.Bot, p.Approved); e != nil {
		return m, e
	}
	bt, e := g.command(ctx, p, nil, "rev-parse", "--verify", p.Bot+"^{tree}")
	if e != nil {
		return m, e
	}
	at, e := g.command(ctx, p, nil, "rev-parse", "--verify", p.Approved+"^{tree}")
	if e != nil {
		return m, e
	}
	raw, e := g.command(ctx, p, nil, "diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--no-color", "--no-relative", "--ignore-submodules=none", "--diff-algorithm=myers", "--numstat", "-z", p.Bot, p.Approved, "--")
	if e != nil {
		return m, e
	}
	m, paths, e := parseNumstat(raw)
	if e != nil {
		return Measurement{}, e
	}
	if len(paths) > 0 {
		rawModes, err := g.command(ctx, p, nil, "diff", "--raw", "-z", "--no-abbrev", "--no-renames", "--no-ext-diff", "--no-textconv", "--no-relative", "--ignore-submodules=none", p.Bot, p.Approved, "--")
		if err != nil {
			return Measurement{}, err
		}
		if err = excludeGitlinks(&m, raw, rawModes, paths); err != nil {
			return Measurement{}, err
		}
		// Named attribute drivers may carry mutable diff.<name>.binary configuration.
		// Refuse them even though no external driver or textconv is ever executed.
		var input bytes.Buffer
		for _, path := range paths {
			input.WriteString(path)
			input.WriteByte(0)
		}
		attr, e := g.command(ctx, p, input.Bytes(), "check-attr", "--source="+p.Approved, "-z", "--stdin", "diff")
		if e != nil {
			return Measurement{}, e
		}
		fields := bytes.Split(attr, []byte{0})
		if len(fields) != len(paths)*3+1 || len(fields[len(fields)-1]) != 0 {
			return Measurement{}, ErrGit
		}
		for i, path := range paths {
			if string(fields[3*i]) != path || string(fields[3*i+1]) != "diff" {
				return Measurement{}, ErrGit
			}
			switch string(fields[3*i+2]) {
			case "set", "unset", "unspecified":
			default:
				return Measurement{}, ErrGit
			}
		}
	}
	if _, e = os.Lstat(attrPath); !os.IsNotExist(e) {
		return Measurement{}, ErrGit
	}
	m.Bot = p.Bot
	m.Approved = p.Approved
	m.BotTree = strings.TrimSuffix(string(bt), "\n")
	m.ApprovedTree = strings.TrimSuffix(string(at), "\n")
	m.GitVersion = strings.TrimSuffix(string(version), "\n")
	m.Convention = "no-renames-delete-plus-add"
	if !validMeasurement(p, m) {
		return Measurement{}, ErrGit
	}
	return m, nil
}
func parseNumstat(raw []byte) (Measurement, []string, error) {
	m := Measurement{}
	paths := []string{}
	seen := map[string]bool{}
	if len(raw) > 4<<20 {
		return m, nil, ErrGit
	}
	rows := bytes.Split(raw, []byte{0})
	if len(rows[len(rows)-1]) != 0 {
		return m, nil, ErrGit
	}
	for _, row := range rows[:len(rows)-1] {
		cols := bytes.SplitN(row, []byte{'\t'}, 3)
		if len(cols) != 3 {
			return m, nil, ErrGit
		}
		path := string(cols[2])
		if !validPath(path) || seen[path] || len(paths) >= 10000 {
			return m, nil, ErrGit
		}
		seen[path] = true
		paths = append(paths, path)
		m.Files++
		if string(cols[0]) == "-" && string(cols[1]) == "-" {
			m.BinaryFiles++
			continue
		}
		a, e := strconv.ParseInt(string(cols[0]), 10, 64)
		d, f := strconv.ParseInt(string(cols[1]), 10, 64)
		if e != nil || f != nil || a < 0 || d < 0 || a > 1e12 || d > 1e12 {
			return m, nil, ErrGit
		}
		m.TextAdded += a
		m.TextDeleted += d
		if m.TextAdded > 1e12 || m.TextDeleted > 1e12 {
			return m, nil, ErrGit
		}
	}
	if m.BinaryFiles == 0 {
		a, d := m.TextAdded, m.TextDeleted
		m.Added = &a
		m.Deleted = &d
	}
	return m, paths, nil
}
func validPath(p string) bool {
	if len(p) == 0 || len(p) > 512 || !utf8.ValidString(p) || strings.HasPrefix(p, "/") {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

// Gitlink numstat lines are synthetic commit-pointer lines, not edits of text blobs.
func excludeGitlinks(m *Measurement, numstat, raw []byte, paths []string) error {
	fields := bytes.Split(raw, []byte{0})
	if len(fields) != len(paths)*2+1 || len(fields[len(fields)-1]) != 0 {
		return ErrGit
	}
	rows := bytes.Split(numstat, []byte{0})
	for i, path := range paths {
		meta := strings.Fields(string(fields[2*i]))
		if len(meta) != 5 || string(fields[2*i+1]) != path || len(meta[0]) != 7 || meta[0][0] != ':' || len(meta[1]) != 6 || !oid.MatchString(meta[2]) || !oid.MatchString(meta[3]) {
			return ErrGit
		}
		if meta[0][1:] != "160000" && meta[1] != "160000" {
			continue
		}
		m.GitlinkFiles++
		cols := bytes.SplitN(rows[i], []byte{'\t'}, 3)
		if string(cols[0]) == "-" {
			m.BinaryFiles--
		} else {
			a, _ := strconv.ParseInt(string(cols[0]), 10, 64)
			d, _ := strconv.ParseInt(string(cols[1]), 10, 64)
			m.TextAdded -= a
			m.TextDeleted -= d
		}
	}
	if m.BinaryFiles+m.GitlinkFiles > 0 {
		m.Added = nil
		m.Deleted = nil
	}
	return nil
}
