package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// The preflight result cache (TOL-V0-047..050, GitHub #1037). It lives
// under the user's cache directory, outside every repository, Tasks store
// and trace store; only `preflight --cache` reads it, and a replayed report
// is the same untrusted read result a fresh run returns, so it never
// becomes evidence, a witness, a gate result or authority.
const (
	preflightCacheProfile    = "corvint-tasks-preflight-cache/0"
	preflightCacheMaxEntries = 256
	preflightCacheMaxAge     = 7 * 24 * time.Hour
	preflightCacheMaxBytes   = 4 << 20
	// maxImplementationBytes bounds the executable the implementation
	// digest hashes.
	maxImplementationBytes = 1 << 30
)

// Cache statuses (TOL-V0-048).
const (
	cacheHit     = "HIT"
	cacheMiss    = "MISS"
	cacheFresh   = "FRESH"
	cacheSkipped = "SKIPPED"
)

// preflightCacheHitDetail prefixes the warning a replayed report carries.
const preflightCacheHitDetail = "PREFLIGHT_CACHE_HIT:"

var cacheEntryName = regexp.MustCompile(`^[0-9a-f]{64}\.json$`)

// localNow and implementationDigest are seams: tests age entries and
// change the check implementation without rebuilding the binary.
var (
	localNow             = time.Now
	implementationDigest = executableDigest
	implementationOnce   sync.Once
	implementationSum    string
	implementationErr    error
)

// executableDigest is the sha256 of the running executable: any rebuild of
// the check implementation changes it (TOL-V0-047).
func executableDigest() (string, error) {
	implementationOnce.Do(func() {
		exe, err := os.Executable()
		if err != nil {
			implementationErr = err
			return
		}
		f, err := os.Open(exe)
		if err != nil {
			implementationErr = err
			return
		}
		defer f.Close()
		h := sha256.New()
		n, err := io.Copy(h, io.LimitReader(f, maxImplementationBytes+1))
		if err == nil && n > maxImplementationBytes {
			err = errors.New("the executable exceeds the implementation digest bound")
		}
		implementationSum, implementationErr = hex.EncodeToString(h.Sum(nil)), err
	})
	return implementationSum, implementationErr
}

// localCacheDir is <user cache dir>/corvint-tasks/<sub>, created 0700.
func localCacheDir(sub string) (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "corvint-tasks", sub)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// preflightInputs is everything a preflight report depends on; key()
// digests it (TOL-V0-047).
type preflightInputs struct {
	implementation string
	ticketID       string
	record         wire.Digest
	ledger         wire.Digest
	policy         wire.Digest
	commit         string
	head           string
	worktree       wire.Digest
	prefixes       []string
	plan           string
	deep           bool
	gates          []string
	witnesses      bool
	// candidate is the witness candidate branch and the commit it resolves
	// to, so a repaired candidate is a new input (TOL-V0-940).
	candidate string
}

func (p preflightInputs) key() wire.Digest {
	prefixes := append([]string{}, p.prefixes...)
	sort.Strings(prefixes)
	lines := []string{preflightCacheProfile, "implementation " + p.implementation, "ticket " + strconv.Quote(p.ticketID),
		"record " + string(p.record), "ledger " + string(p.ledger), "policy " + string(p.policy), "commit " + p.commit,
		"head " + p.head, "worktree " + string(p.worktree), "plan " + strconv.Quote(p.plan), "deep " + strconv.FormatBool(p.deep)}
	for _, pre := range prefixes {
		lines = append(lines, "path "+strconv.Quote(pre))
	}
	for _, g := range p.gates {
		lines = append(lines, "gate "+strconv.Quote(g))
	}
	// TOL-V0-940: only a --witnesses run adds a line, so every earlier key
	// keeps its bytes.
	if p.witnesses {
		lines = append(lines, "witnesses true")
		if p.candidate != "" {
			lines = append(lines, "candidate "+strconv.Quote(p.candidate))
		}
	}
	return wire.Sum([]byte(strings.Join(lines, "\n") + "\n"))
}

// cachedReport is one stored preflight report.
type cachedReport struct {
	storedAt time.Time
	outcome  string
	item     wire.Value
	warnings []string
}

// lookupPreflightCache returns the entry stored under key, or ok false
// when there is none, it is older than the age bound, or it cannot be read
// as an entry for this key (problem names why).
func lookupPreflightCache(dir string, key wire.Digest) (c cachedReport, ok bool, problem string) {
	p := filepath.Join(dir, string(key)+".json")
	st, err := os.Lstat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return c, false, ""
	}
	if err != nil || !st.Mode().IsRegular() || st.Size() > preflightCacheMaxBytes {
		return c, false, "the cache entry is not a readable regular file within the bound"
	}
	raw, err := os.ReadFile(p)
	if err != nil || len(raw) > preflightCacheMaxBytes {
		return c, false, "the cache entry could not be read"
	}
	v, err := wire.Parse(raw)
	if err != nil {
		return c, false, "the cache entry is not a canonical document"
	}
	r := wire.NewReader(v, "/")
	r.Closed("profile", "key", "storedAt", "outcome", "item", "warnings")
	_ = r.Profile(preflightCacheProfile)
	gotKey := r.Field("key").String()
	at := r.Field("storedAt").String()
	outcome := r.Field("outcome").Enum(wire.OutcomeOK, wire.OutcomeRefused)
	item := r.Field("item").Value()
	warns := r.Field("warnings").Value()
	if r.Err() != nil || gotKey != string(key) || item.Kind != wire.KindObject || warns.Kind != wire.KindArray {
		return c, false, "the cache entry does not match its key or profile"
	}
	stored, err := time.Parse(time.RFC3339, at)
	if err != nil {
		return c, false, "the cache entry has no valid storedAt"
	}
	if age := localNow().Sub(stored); age > preflightCacheMaxAge || age < -time.Minute {
		return c, false, ""
	}
	for _, w := range warns.Arr {
		if w.Kind != wire.KindString {
			return c, false, "the cache entry's warnings are not strings"
		}
		c.warnings = append(c.warnings, w.Str)
	}
	c.storedAt, c.outcome, c.item = stored, outcome, item
	return c, true, ""
}

// storePreflightCache writes the entry atomically (a 0600 temporary file
// renamed into place), then prunes the directory to the age and count
// bounds (TOL-V0-049).
func storePreflightCache(dir string, key wire.Digest, c cachedReport) error {
	warns := make([]wire.Value, 0, len(c.warnings))
	for _, w := range c.warnings {
		warns = append(warns, wire.String(w))
	}
	doc := wire.ObjectValue(wire.NewObject().Set("profile", wire.String(preflightCacheProfile)).Set("key", wire.String(string(key))).
		Set("storedAt", wire.String(c.storedAt.UTC().Format(time.RFC3339))).Set("outcome", wire.String(c.outcome)).
		Set("item", c.item).Set("warnings", wire.Array(warns...)))
	raw := wire.EncodeFile(doc)
	if len(raw) > preflightCacheMaxBytes {
		return wire.Errorf(wire.CodeLimitExceeded, "cache", "the report exceeds the %d-byte cache entry bound", preflightCacheMaxBytes)
	}
	f, err := os.CreateTemp(dir, ".entry-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, werr := f.Write(raw)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Rename(tmp, filepath.Join(dir, string(key)+".json"))
	}
	if werr != nil {
		_ = os.Remove(tmp)
		return werr
	}
	pruneCacheDir(dir, preflightCacheMaxEntries, preflightCacheMaxAge)
	return nil
}

// pruneCacheDir removes entries older than maxAge, then the oldest entries
// beyond maxEntries, by modification time. It touches only file names the
// cache writes, and leftover temporary files older than an hour.
func pruneCacheDir(dir string, maxEntries int, maxAge time.Duration) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type kept struct {
		name string
		mod  time.Time
	}
	var live []kept
	now := localNow()
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		name := e.Name()
		switch {
		case strings.HasPrefix(name, ".entry-"):
			if now.Sub(info.ModTime()) > time.Hour {
				_ = os.Remove(filepath.Join(dir, name))
			}
		case cacheEntryName.MatchString(name):
			if now.Sub(info.ModTime()) > maxAge {
				_ = os.Remove(filepath.Join(dir, name))
				continue
			}
			live = append(live, kept{name, info.ModTime()})
		}
	}
	if len(live) <= maxEntries {
		return
	}
	sort.Slice(live, func(i, j int) bool {
		if !live[i].mod.Equal(live[j].mod) {
			return live[i].mod.Before(live[j].mod)
		}
		return live[i].name < live[j].name
	})
	for _, k := range live[:len(live)-maxEntries] {
		_ = os.Remove(filepath.Join(dir, k.name))
	}
}

// cacheValue is the `cache` member of a preflight report run with --cache
// or --fresh.
func cacheValue(status string, key wire.Digest, storedAt *time.Time, reason string) wire.Value {
	o := wire.NewObject().Set("status", wire.String(status))
	if key == "" {
		o.Set("key", wire.Null())
	} else {
		o.Set("key", wire.String(string(key)))
	}
	if storedAt == nil {
		o.Set("storedAt", wire.Null())
	} else {
		o.Set("storedAt", wire.String(storedAt.UTC().Format(time.RFC3339)))
	}
	o.Set("reason", optionalProse(reason))
	return wire.ObjectValue(o)
}

// preflightCacheRun is one preflight's use of the cache.
type preflightCacheRun struct {
	dir    string
	key    wire.Digest
	fresh  bool
	reason string // non-empty: the inputs could not be keyed, so nothing is replayed or stored
}

// startPreflightCache opens the cache for the preflightInputKey result. A
// failure to key the inputs (no HEAD, a bound, an unreadable executable) or
// an unavailable cache directory skips the cache with the reason, and the
// checks run (TOL-V0-048).
func startPreflightCache(key wire.Digest, reason string, fresh bool) *preflightCacheRun {
	c := &preflightCacheRun{fresh: fresh}
	if reason != "" {
		c.reason = reason
		return c
	}
	dir, err := localCacheDir("preflight")
	if err != nil {
		c.reason = "the cache directory is unavailable: " + err.Error()
		return c
	}
	c.dir, c.key = dir, key
	return c
}

// preflightInputKey completes the inputs with the implementation digest
// and the working-tree state and digests them, or returns why it cannot
// (TOL-V0-047; the TOL-V0-051 input digest of preflight).
func preflightInputKey(root string, in preflightInputs) (wire.Digest, string) {
	impl, err := implementationDigest()
	if err != nil {
		return "", "the check implementation digest is unavailable: " + err.Error()
	}
	head, tree, err := store.WorkingTreeState(root)
	if err != nil {
		return "", "the working tree could not be keyed: " + err.Error()
	}
	in.implementation, in.head, in.worktree = impl, head, tree
	return in.key(), ""
}

// replay returns the stored report for an identical input key, or nil.
func (c *preflightCacheRun) replay(cmd []string, rc *readCtx) *wire.Result {
	if c.reason != "" || c.fresh {
		return nil
	}
	hit, ok, problem := lookupPreflightCache(c.dir, c.key)
	if !ok {
		if problem != "" {
			c.reason = problem
		}
		return nil
	}
	res := success(cmd, rc)
	res.Untrusted = true
	hit.item.Obj.Set("cache", cacheValue(cacheHit, c.key, &hit.storedAt, ""))
	res.Items = []wire.Value{hit.item}
	res.Warnings = append(res.Warnings, hit.warnings...)
	res.Warnings = append(res.Warnings, prose(preflightCacheHitDetail+" replayed the report stored at "+
		hit.storedAt.UTC().Format(time.RFC3339)+" for identical inputs (key "+string(c.key)+"); pass --fresh to run the checks"))
	if hit.outcome != wire.OutcomeOK {
		res.Outcome = hit.outcome
	}
	return res
}

// finish stores a completed report under the key and adds the cache member.
func (c *preflightCacheRun) finish(res *wire.Result, item *wire.Object) {
	status := cacheMiss
	if c.fresh {
		status = cacheFresh
	}
	if c.reason != "" && c.key == "" {
		item.Set("cache", cacheValue(cacheSkipped, "", nil, c.reason))
		res.Warnings = append(res.Warnings, prose("preflight cache skipped: "+c.reason))
		return
	}
	stored := localNow().UTC().Truncate(time.Second)
	entry := cachedReport{storedAt: stored, outcome: res.Outcome, item: wire.ObjectValue(item), warnings: append([]string{}, res.Warnings...)}
	if err := storePreflightCache(c.dir, c.key, entry); err != nil {
		item.Set("cache", cacheValue(status, c.key, nil, "the report was not stored: "+err.Error()))
		res.Warnings = append(res.Warnings, prose("preflight cache: the report was not stored: "+err.Error()))
		return
	}
	item.Set("cache", cacheValue(status, c.key, &stored, c.reason))
}
