// Package sharedpath checks and mechanically repairs the shared-path change
// policy: changes to shared test infrastructure travel in separate, labelled
// commits that cite a passing affected run (SPC-V0, V1-1591).
package sharedpath

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/gitstatus"
)

// ConfigProfile is the closed shared-path configuration profile (SPC-V0-001).
const ConfigProfile = "corvint-shared-paths/0"

// DefaultConfig is the configuration path relative to the checkout root.
const DefaultConfig = ".corvint/shared-paths.json"

const (
	maxConfigBytes = 64 << 10
	maxNotesBytes  = 1 << 20
	maxGlobs       = 256
	maxGlobBytes   = 512
	maxMarkers     = 32
	maxCommits     = 512
	maxGitBytes    = 32 << 20
	gitTimeout     = 2 * time.Minute
)

// Refusal codes. Each is owned by docs/specs/shared-path-change-policy-v0.md.
const (
	CodeInvalidArguments  = "changes-invalid-arguments"
	CodeGitUnavailable    = "changes-git-unavailable"
	CodeConfigUnavailable = "changes-shared-config-unavailable"
	CodeNotesUnavailable  = "changes-shared-notes-unavailable"
	CodeRangeUnavailable  = "changes-shared-range-unavailable"
	CodeDirtyTree         = "changes-shared-dirty-tree"
	CodeDetachedHead      = "changes-shared-detached-head"
	CodeBranchMismatch    = "changes-shared-branch-mismatch"
	CodeMergeInRange      = "changes-shared-merge-in-range"
	CodeLiveSession       = "changes-shared-live-session"
	CodeHeadMismatch      = "changes-shared-head-mismatch"
	CodeTreeMismatch      = "changes-shared-tree-mismatch"
	CodeShallowMergeBase  = "changes-shared-shallow-merge-base"
)

// Config is the closed shared-path policy configuration.
type Config struct {
	Profile string `json:"profile"`
	// Shared are the globs naming shared test infrastructure.
	Shared []string `json:"shared"`
	// Generated are the globs naming generated files: they are neither shared
	// nor other paths, and merge conflicts confined to them are not findings.
	Generated []string `json:"generated"`
	// Marker is the commit subject prefix of a shared-path commit.
	Marker string `json:"marker"`
	// RunTrailer is the message trailer key that cites the affected run.
	RunTrailer string `json:"runTrailer"`
	// SessionMarkers are checkout-relative paths whose presence means an agent
	// session is live in this checkout.
	SessionMarkers []string `json:"sessionMarkers"`
	// digest is the SHA-256 of the loaded file bytes (SPC-V0-011).
	digest string
}

// Digest is "sha256:" and the hexadecimal SHA-256 of the configuration file
// bytes LoadConfig read, or empty for a configuration built in code.
func (c Config) Digest() string { return c.digest }

// LoadConfig reads and validates a bounded, closed configuration file.
func LoadConfig(file string) (Config, error) {
	var c Config
	raw, err := readRegular(file, maxConfigBytes)
	if err != nil {
		return c, errors.New(CodeConfigUnavailable)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&c) != nil || decoder.More() {
		return Config{}, errors.New(CodeConfigUnavailable)
	}
	if c.RunTrailer == "" {
		c.RunTrailer = "Affected-Run"
	}
	if c.Profile != ConfigProfile || len(c.Shared) == 0 || strings.TrimSpace(c.Marker) == "" || strings.ContainsAny(c.Marker, "\n\r") ||
		!validGlobs(c.Shared) || !validGlobs(c.Generated) || !trailerKey.MatchString(c.RunTrailer) || len(c.SessionMarkers) > maxMarkers {
		return Config{}, errors.New(CodeConfigUnavailable)
	}
	for _, m := range c.SessionMarkers {
		if !relativePath(m) {
			return Config{}, errors.New(CodeConfigUnavailable)
		}
	}
	sum := sha256.Sum256(raw)
	c.digest = "sha256:" + hex.EncodeToString(sum[:])
	return c, nil
}

// readRegular reads at most limit bytes of a regular file. The open does not
// block on a FIFO or device, and the descriptor is checked before any read.
func readRegular(file string, limit int) ([]byte, error) {
	f, err := openInput(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	raw, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil || len(raw) > limit {
		return nil, errors.New("unreadable or oversized")
	}
	if raw == nil {
		raw = []byte{}
	}
	return raw, nil
}

var trailerKey = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]{0,63}$`)

func validGlobs(globs []string) bool {
	if len(globs) > maxGlobs {
		return false
	}
	for _, g := range globs {
		if len(g) == 0 || len(g) > maxGlobBytes || !relativePath(g) {
			return false
		}
		for _, segment := range strings.Split(g, "/") {
			if _, err := path.Match(segment, ""); err != nil {
				return false
			}
		}
	}
	return true
}

func relativePath(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") || strings.ContainsRune(p, 0) {
		return false
	}
	for _, segment := range strings.Split(p, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

// Match reports whether the slash-separated path matches glob, where a `**`
// segment matches zero or more whole segments and every other segment uses
// path.Match.
func Match(glob, name string) bool {
	return matchSegments(strings.Split(glob, "/"), strings.Split(name, "/"))
}

func matchSegments(glob, name []string) bool {
	for len(glob) > 0 {
		if glob[0] == "**" {
			for skip := 0; skip <= len(name); skip++ {
				if matchSegments(glob[1:], name[skip:]) {
					return true
				}
			}
			return false
		}
		if len(name) == 0 {
			return false
		}
		if ok, _ := path.Match(glob[0], name[0]); !ok {
			return false
		}
		glob, name = glob[1:], name[1:]
	}
	return len(name) == 0
}

func matchAny(globs []string, name string) bool {
	for _, g := range globs {
		if Match(g, name) {
			return true
		}
	}
	return false
}

// Class is a changed path's policy class: generated wins over shared.
func (c Config) Class(name string) string {
	switch {
	case matchAny(c.Generated, name):
		return "generated"
	case matchAny(c.Shared, name):
		return "shared"
	default:
		return "other"
	}
}

// git runs one bounded Git command in root with the caller's Git overrides
// removed; extra environment entries are appended afterwards. It never runs a
// network operation, hook, pager or editor.
type git struct {
	ctx  context.Context
	root string
}

func (g git) run(stdin []byte, extra []string, args ...string) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(g.ctx, gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, gitstatus.Executable(), append([]string{"--no-replace-objects", "-c", "core.hooksPath=" + os.DevNull, "-C", g.root}, args...)...)
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, "GIT_") {
			cmd.Env = append(cmd.Env, v)
		}
	}
	// GIT_OPTIONAL_LOCKS=0 keeps reads such as `status` from refreshing the
	// index (SPC-V0-004).
	cmd.Env = append(cmd.Env, "GIT_NO_LAZY_FETCH=1", "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_PAGER=cat", "GIT_EDITOR=false")
	cmd.Env = append(cmd.Env, extra...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var out bytes.Buffer
	cmd.Stdout = &limited{buffer: &out, remaining: maxGitBytes}
	cmd.Stderr = io.Discard
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil, -1, errors.New(CodeGitUnavailable)
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return out.Bytes(), exit.ExitCode(), nil
	}
	if err != nil {
		return nil, -1, errors.New(CodeGitUnavailable)
	}
	return out.Bytes(), 0, nil
}

// ok runs a command that must succeed.
func (g git) ok(args ...string) ([]byte, error) {
	out, code, err := g.run(nil, nil, args...)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, errors.New(CodeGitUnavailable)
	}
	return out, nil
}

type limited struct {
	buffer    *bytes.Buffer
	remaining int
}

func (l *limited) Write(p []byte) (int, error) {
	if len(p) > l.remaining {
		return 0, errors.New("git output limit")
	}
	l.remaining -= len(p)
	return l.buffer.Write(p)
}

var fullSHA = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

func (g git) commit(rev string) (string, error) {
	if rev == "" || strings.HasPrefix(rev, "-") || strings.ContainsAny(rev, " \t\n\r\x00") {
		return "", errors.New(CodeInvalidArguments)
	}
	out, code, err := g.run(nil, nil, "rev-parse", "--verify", "--quiet", "--end-of-options", rev+"^{commit}")
	if err != nil {
		return "", err
	}
	sha := strings.TrimSpace(string(out))
	if code != 0 || !fullSHA.MatchString(sha) {
		return "", errors.New(CodeRangeUnavailable)
	}
	return sha, nil
}

// rangeCommit is one commit of base..head, oldest first.
type rangeCommit struct {
	sha     string
	parents []string
}

func (g git) commits(base, head string) ([]rangeCommit, error) {
	out, err := g.ok("rev-list", "--reverse", "--topo-order", "--parents", "--end-of-options", head, "^"+base)
	if err != nil {
		return nil, err
	}
	var rows []rangeCommit
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		rows = append(rows, rangeCommit{sha: fields[0], parents: fields[1:]})
		if len(rows) > maxCommits {
			return nil, errors.New(CodeRangeUnavailable)
		}
	}
	return rows, nil
}

// change is one path a commit changed against its first parent.
type change struct {
	mode, object, path string
	deleted            bool
}

func (g git) changes(sha string) ([]change, error) {
	out, err := g.ok("diff-tree", "-r", "--no-renames", "--no-commit-id", "--root", "--raw", "-z", sha)
	if err != nil {
		return nil, err
	}
	var rows []change
	fields := bytes.Split(out, []byte{0})
	for i := 0; i+1 < len(fields); i += 2 {
		meta := strings.Fields(strings.TrimPrefix(string(fields[i]), ":"))
		if len(meta) != 5 {
			return nil, errors.New(CodeGitUnavailable)
		}
		rows = append(rows, change{mode: meta[1], object: meta[3], path: string(fields[i+1]), deleted: meta[4] == "D"})
	}
	return rows, nil
}

// message is a parsed commit object.
type message struct {
	author, committer, encoding string
	body                        []byte
}

func (g git) message(sha string) (message, error) {
	raw, err := g.ok("cat-file", "commit", sha)
	if err != nil {
		return message{}, err
	}
	header, body, found := bytes.Cut(raw, []byte("\n\n"))
	if !found {
		header, body = raw, nil
	}
	var m message
	m.body = body
	for _, line := range strings.Split(string(header), "\n") {
		key, value, _ := strings.Cut(line, " ")
		switch key {
		case "author":
			m.author = value
		case "committer":
			m.committer = value
		case "encoding":
			m.encoding = value
		}
	}
	return m, nil
}

func (m message) subject() string {
	first, _, _ := strings.Cut(string(m.body), "\n")
	return first
}

// runCitation returns the value of the last RunTrailer line in the message.
func (m message) runCitation(key string) (string, bool) {
	value, found := "", false
	for _, line := range strings.Split(string(m.body), "\n") {
		if v, ok := strings.CutPrefix(line, key+":"); ok {
			value, found = strings.TrimSpace(v), true
		}
	}
	return value, found
}
