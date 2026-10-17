// Package evidencereach is the read-only evidence-freshness check behind `corvint runs fresh` and
// the FRESH_PASS reuse condition of `corvint runs reusable` (docs/specs/playwright-run-evidence-v0.md,
// PWR-V0-038 and PWR-V0-039). Evidence produced at an OLD commit is FRESH at a NEW commit when no
// path inside its declared reach changed between the two: the paths the evidence cites plus the
// shared-code patterns the repository declares in .corvint/evidence-reach.json at NEW. Anything that
// cannot be resolved is UNRESOLVED, never FRESH. Git is only read, through bounded subprocesses with
// a scrubbed environment; nothing is written.
package evidencereach

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

// Profiles, location and bounds (PWR-V0-038, PWR-V0-039).
const (
	ConfigProfile = "corvint-evidence-reach/0"
	FreshProfile  = "corvint-evidence-fresh/0"
	ConfigPath    = ".corvint/evidence-reach.json"

	MaxConfigBytes   = 64 << 10
	MaxPatterns      = 256
	MaxPatternBytes  = 1024
	MaxDoubleStars   = 4
	MaxCited         = 64
	MaxDiffBytes     = 16 << 20
	MaxDecidingPaths = 256

	gitOperationTimeout = 60 * time.Second
	maxRevisionBytes    = 256
)

// Verdicts.
const (
	Fresh      = "FRESH"
	Stale      = "STALE"
	Unresolved = "UNRESOLVED"
)

// Unresolved reasons.
const (
	ReasonRevisionUnresolved = "REVISION_UNRESOLVED"
	ReasonReachUndeclared    = "REACH_UNDECLARED"
	ReasonReachInvalid       = "REACH_INVALID"
	ReasonReachUnreadable    = "REACH_UNREADABLE"
	ReasonDiffFailed         = "DIFF_FAILED"
	ReasonDiffBoundExceeded  = "DIFF_BOUND_EXCEEDED"
	ReasonCitedPathInvalid   = "CITED_PATH_INVALID"
)

// Why a changed path is in reach.
const (
	ByCited       = "cited"
	ByShared      = "shared"
	ByReachConfig = "reach-config"
)

// Config is the repository's reach declaration.
type Config struct {
	Profile string   `json:"profile"`
	Shared  []string `json:"shared"`
}

var errInvalid = errors.New(ReasonReachInvalid)

// ParseConfig admits one strict reach declaration: one JSON object with exactly the profile and a
// shared array of at most MaxPatterns valid patterns.
func ParseConfig(raw []byte) (Config, error) {
	var cfg Config
	if len(raw) > MaxConfigBytes {
		return cfg, errInvalid
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&cfg) != nil {
		return Config{}, errInvalid
	}
	if _, err := dec.Token(); err != io.EOF {
		return Config{}, errInvalid
	}
	if cfg.Profile != ConfigProfile || cfg.Shared == nil || len(cfg.Shared) > MaxPatterns {
		return Config{}, errInvalid
	}
	for _, p := range cfg.Shared {
		if !ValidPattern(p) {
			return Config{}, errInvalid
		}
	}
	return cfg, nil
}

// ValidPath reports whether p is a clean repository-relative slash path.
func ValidPath(p string) bool {
	if p == "" || len(p) > MaxPatternBytes || strings.ContainsAny(p, "\\\x00") || strings.HasPrefix(p, "/") {
		return false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// ValidPattern reports whether p is a reach pattern: a repository-relative slash path whose
// segments are path.Match globs, where a whole `**` segment spans any number of segments (at most
// MaxDoubleStars of them) and a trailing `/` covers everything under a directory.
func ValidPattern(p string) bool {
	body := strings.TrimSuffix(p, "/")
	if !ValidPath(body) {
		return false
	}
	stars := 0
	for _, seg := range strings.Split(body, "/") {
		if seg == "**" {
			stars++
			continue
		}
		if strings.Contains(seg, "**") {
			return false
		}
		if _, err := path.Match(seg, ""); err != nil {
			return false
		}
	}
	return stars <= MaxDoubleStars
}

// Match reports whether the repository path p matches the valid pattern.
func Match(pattern, p string) bool {
	if strings.HasSuffix(pattern, "/") {
		pattern += "**"
	}
	return matchSegments(strings.Split(pattern, "/"), strings.Split(p, "/"))
}

func matchSegments(pat, segs []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			for i := 0; i <= len(segs); i++ {
				if matchSegments(pat[1:], segs[i:]) {
					return true
				}
			}
			return false
		}
		if len(segs) == 0 {
			return false
		}
		if ok, _ := path.Match(pat[0], segs[0]); !ok {
			return false
		}
		pat, segs = pat[1:], segs[1:]
	}
	return len(segs) == 0
}

// Deciding is one changed path inside the reach and why it is there.
type Deciding struct {
	Path string `json:"path"`
	By   string `json:"by"`
}

// Classify decides freshness from the paths that changed between OLD and NEW (PWR-V0-039): STALE
// when any changed path is cited, matches a shared pattern, or is the reach declaration itself;
// otherwise FRESH. Deciding paths are sorted and capped at MaxDecidingPaths; truncated counts the
// rest.
func Classify(changed, cited []string, cfg Config) (verdict string, deciding []Deciding, truncated int) {
	citedSet := map[string]bool{}
	for _, c := range cited {
		citedSet[c] = true
	}
	deciding = []Deciding{}
	sorted := append([]string{}, changed...)
	sort.Strings(sorted)
	for _, p := range sorted {
		by := ""
		switch {
		case citedSet[p]:
			by = ByCited
		case p == ConfigPath:
			by = ByReachConfig
		default:
			for _, pattern := range cfg.Shared {
				if Match(pattern, p) {
					by = ByShared
					break
				}
			}
		}
		if by == "" {
			continue
		}
		if len(deciding) >= MaxDecidingPaths {
			truncated++
			continue
		}
		deciding = append(deciding, Deciding{Path: p, By: by})
	}
	if len(deciding) > 0 || truncated > 0 {
		return Stale, deciding, truncated
	}
	return Fresh, deciding, 0
}

// Result is the `corvint runs fresh` document. Old and New are the resolved commit ids, or the
// arguments as given when they did not resolve.
type Result struct {
	Profile           string     `json:"profile"`
	Old               string     `json:"old"`
	New               string     `json:"new"`
	Verdict           string     `json:"verdict"`
	Reason            string     `json:"reason"`
	ReachConfig       string     `json:"reachConfig"`
	Cited             []string   `json:"cited,omitempty"`
	Shared            []string   `json:"shared,omitempty"`
	ChangedPaths      int        `json:"changedPaths"`
	Deciding          []Deciding `json:"deciding,omitempty"`
	DecidingTruncated int        `json:"decidingTruncated"`
}

// Summary is the compact form: the verdict and counts without the reach and path lists.
func (r Result) Summary() Result {
	r.Cited, r.Shared, r.Deciding = nil, nil, nil
	return r
}

// Reader runs the bounded Git reads for one repository and memoizes them, so one reuse read
// resolves each revision, reach declaration and revision-pair diff once.
type Reader struct {
	Root  string
	revs  map[string]revision
	reach map[string]reachRead
	diffs map[[2]string]diffRead
}

type revision struct {
	id string
	ok bool
}

type reachRead struct {
	cfg    Config
	reason string
}

type diffRead struct {
	paths  []string
	reason string
}

// NewReader returns a reader rooted at root (the working directory when empty).
func NewReader(root string) *Reader {
	return &Reader{Root: root, revs: map[string]revision{}, reach: map[string]reachRead{}, diffs: map[[2]string]diffRead{}}
}

// Check is the freshness of evidence produced at old, read at new, for the cited paths
// (PWR-V0-039). Every failure is UNRESOLVED with a reason, never FRESH.
func (r *Reader) Check(ctx context.Context, old, new string, cited []string) Result {
	res := Result{Profile: FreshProfile, Old: old, New: new, ReachConfig: ConfigPath, Cited: append([]string{}, cited...), Deciding: []Deciding{}}
	if len(cited) > MaxCited {
		return unresolved(res, ReasonCitedPathInvalid)
	}
	for _, c := range cited {
		if !ValidPath(c) {
			return unresolved(res, ReasonCitedPathInvalid)
		}
	}
	oldID, okOld := r.resolve(ctx, old)
	newID, okNew := r.resolve(ctx, new)
	if !okOld || !okNew {
		return unresolved(res, ReasonRevisionUnresolved)
	}
	res.Old, res.New = oldID, newID
	reach := r.reachAt(ctx, newID)
	if reach.reason != "" {
		return unresolved(res, reach.reason)
	}
	res.Shared = append([]string{}, reach.cfg.Shared...)
	diff := r.diff(ctx, oldID, newID)
	if diff.reason != "" {
		return unresolved(res, diff.reason)
	}
	res.ChangedPaths = len(diff.paths)
	res.Verdict, res.Deciding, res.DecidingTruncated = Classify(diff.paths, cited, reach.cfg)
	return res
}

func unresolved(res Result, reason string) Result {
	res.Verdict, res.Reason, res.Deciding = Unresolved, reason, []Deciding{}
	return res
}

func (r *Reader) resolve(ctx context.Context, rev string) (string, bool) {
	if v, ok := r.revs[rev]; ok {
		return v.id, v.ok
	}
	v := revision{}
	if rev != "" && len(rev) <= maxRevisionBytes && !strings.HasPrefix(rev, "-") && !strings.ContainsAny(rev, "\x00\n") {
		out, err := r.git(ctx, 4096, "rev-parse", "--verify", "--quiet", "--end-of-options", rev+"^{commit}")
		id := strings.TrimSpace(string(out))
		v = revision{id: id, ok: err == nil && (len(id) == 40 || len(id) == 64)}
	}
	r.revs[rev] = v
	return v.id, v.ok
}

// reachAt reads the reach declaration from the NEW commit's tree: absent is REACH_UNDECLARED, a
// non-regular entry or an invalid document is REACH_INVALID, and a failed read is REACH_UNREADABLE.
func (r *Reader) reachAt(ctx context.Context, commit string) reachRead {
	if v, ok := r.reach[commit]; ok {
		return v
	}
	v := r.readReach(ctx, commit)
	r.reach[commit] = v
	return v
}

func (r *Reader) readReach(ctx context.Context, commit string) reachRead {
	entry, err := r.git(ctx, 64<<10, "ls-tree", "-z", "--full-tree", commit, "--", ConfigPath)
	if err != nil {
		return reachRead{reason: ReasonReachUnreadable}
	}
	if len(entry) == 0 {
		return reachRead{reason: ReasonReachUndeclared}
	}
	meta, _, found := strings.Cut(string(entry), "\t")
	fields := strings.Fields(meta)
	if !found || len(fields) != 3 || fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") {
		return reachRead{reason: ReasonReachInvalid}
	}
	raw, err := r.git(ctx, MaxConfigBytes+1, "cat-file", "blob", fields[2])
	if outputExceeded(err) {
		return reachRead{reason: ReasonReachInvalid}
	}
	if err != nil {
		return reachRead{reason: ReasonReachUnreadable}
	}
	cfg, err := ParseConfig(raw)
	if err != nil {
		return reachRead{reason: ReasonReachInvalid}
	}
	return reachRead{cfg: cfg}
}

func (r *Reader) diff(ctx context.Context, old, new string) diffRead {
	key := [2]string{old, new}
	if v, ok := r.diffs[key]; ok {
		return v
	}
	v := diffRead{paths: []string{}}
	if old != new {
		out, err := r.git(ctx, MaxDiffBytes, "diff-tree", "-r", "-z", "--no-renames", "--name-only", "--no-commit-id", old, new, "--")
		switch {
		case outputExceeded(err):
			v = diffRead{reason: ReasonDiffBoundExceeded}
		case err != nil:
			v = diffRead{reason: ReasonDiffFailed}
		default:
			for _, p := range strings.Split(string(out), "\x00") {
				if p != "" {
					v.paths = append(v.paths, p)
				}
			}
		}
	}
	r.diffs[key] = v
	return v
}

func outputExceeded(err error) bool {
	var coded *cemcode.Error
	return errors.As(err, &coded) && coded.Code == cemcode.GitOutputExceeded
}

func (r *Reader) git(ctx context.Context, limit int, args ...string) ([]byte, error) {
	options := gitrun.Options{Dir: r.Root, Env: environment(), StdoutLimit: limit, PerOpTimeout: gitOperationTimeout}
	return gitrun.RunReserved(ctx, gitOperationTimeout, options, args...)
}

// environment is the scrubbed child environment other gitrun callers pass: what Git needs to run
// plus settings that keep the read deterministic and non-interactive.
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
	)
}
