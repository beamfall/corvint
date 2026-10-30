package lrfrepo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/cem/verify"
)

// StableReuseEnv names the private, run-scoped directory through which one
// dogfood run's OCM prepare, status and aggregate processes share accepted
// stable verdicts, so a pinned CEM is verified once per run rather than once
// per intent scope (OCM-V0-027, V1-2894). Unset, every OCM read and prepare
// verifies in full as before.
const StableReuseEnv = "CORVINT_OCM_STABLE_REUSE_DIR"

const (
	stableReuseProfile  = "corvint-ocm-stable-reuse/0"
	maxStableReuseBytes = 1 << 20
)

// stableReuseKey binds a reusable verdict to the exact CEM bytes, the resolved
// base and target, the repository and artifact root, and the verifier
// executable's digest. Any difference is a miss and a full verification.
type stableReuseKey struct {
	CEMSha256      string `json:"cemSha256"`
	ExpectedBase   string `json:"expectedBase"`
	Target         string `json:"target"`
	Root           string `json:"root"`
	VerifierSha256 string `json:"verifierSha256"`
}

// stableReuseEntry is the part of an accepted stable verdict an OCM read or
// prepare consumes, plus the artifact checks a reuse must reproduce.
type stableReuseEntry struct {
	Profile        string                       `json:"profile"`
	Key            stableReuseKey               `json:"key"`
	TargetRevision string                       `json:"targetRevision"`
	PatchSha256    string                       `json:"patchSha256"`
	MapSha256      string                       `json:"mapSha256"`
	ArtifactChecks []verify.StableArtifactCheck `json:"artifactChecks"`
}

// StableReuse holds the accepted stable verdicts of one run: in memory for
// the process that made it, and in the run's private directory when
// StableReuseEnv names one. It only ever stores ACCEPT verdicts, and a reuse
// re-reads every declared artifact, because the artifact root is a mutable
// worktree no digest in the key covers. A nil *StableReuse never reuses.
type StableReuse struct {
	mu       sync.Mutex
	memory   map[stableReuseKey]stableReuseEntry
	dir      string
	once     sync.Once
	verifier string
}

// NewStableReuse returns the reuse for one run: an in-memory store, backed by
// the StableReuseEnv directory when that names an absolute, private directory.
func NewStableReuse() *StableReuse {
	return &StableReuse{memory: map[stableReuseKey]stableReuseEntry{}, dir: stableReuseDir()}
}

// environmentStableReuse is the reuse a process without a caller-supplied one
// gets: the run's directory store, or nil when no run shares one.
func environmentStableReuse() *StableReuse {
	if stableReuseDir() == "" {
		return nil
	}
	return NewStableReuse()
}

// stableReuseDir is the StableReuseEnv directory when it is absolute, a real
// directory rather than a link, and (outside Windows) private to its owner.
func stableReuseDir() string {
	dir := os.Getenv(StableReuseEnv)
	if dir == "" || !filepath.IsAbs(dir) {
		return ""
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || (runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0) {
		return ""
	}
	return dir
}

// stableReuseProcessStart is when this process initialised its packages. An
// executable path whose file changed at or after it, for example replaced by
// an install's rename, may no longer hold the running image.
var stableReuseProcessStart = time.Now()

// procSelfExe opens the running image itself on Linux, even after the path it
// was started from is replaced.
const procSelfExe = "/proc/self/exe"

// verifierIdentity is the SHA-256 of the running executable, or "" when it
// cannot be read or its file may have been replaced since this process
// started, which disables reuse: a verdict is never stored or reused under
// the digest of bytes other than the verifier that ran.
func (r *StableReuse) verifierIdentity() string {
	r.once.Do(func() {
		path, image := procSelfExe, true
		if _, err := os.Stat(procSelfExe); runtime.GOOS != "linux" || err != nil {
			executable, err := os.Executable()
			if err != nil {
				return
			}
			path, image = executable, false
		}
		file, err := os.Open(path)
		if err != nil {
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || (!image && !fileChangedBefore(info, stableReuseProcessStart)) {
			return
		}
		hash := sha256.New()
		if _, err := io.Copy(hash, file); err != nil {
			return
		}
		r.verifier = hex.EncodeToString(hash.Sum(nil))
	})
	return r.verifier
}

func (r *StableReuse) key(cemRaw []byte, base, target, root string) (stableReuseKey, bool) {
	verifier := r.verifierIdentity()
	return stableReuseKey{sha256Hex(cemRaw), base, target, root, verifier}, verifier != ""
}

func (r *StableReuse) entryPath(key stableReuseKey) string {
	encoded, _ := json.Marshal(key)
	return filepath.Join(r.dir, "stable-"+sha256Hex(encoded)+".json")
}

// lookup returns a stored verdict for exactly key whose declared artifacts
// still read back as the verdict recorded them; anything else is a miss.
func (r *StableReuse) lookup(ctx context.Context, key stableReuseKey, root string, cemRaw []byte) (stableReuseEntry, bool) {
	r.mu.Lock()
	entry, ok := r.memory[key]
	r.mu.Unlock()
	if !ok && r.dir != "" {
		entry, ok = r.readEntry(key)
	}
	if !ok || entry.Profile != stableReuseProfile || entry.Key != key {
		return stableReuseEntry{}, false
	}
	checks, err := verify.RecheckStableArtifacts(ctx, root, cemRaw)
	if err != nil || !sameArtifactChecks(checks, entry.ArtifactChecks) {
		return stableReuseEntry{}, false
	}
	return entry, true
}

func (r *StableReuse) readEntry(key stableReuseKey) (stableReuseEntry, bool) {
	path := r.entryPath(key)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxStableReuseBytes {
		return stableReuseEntry{}, false
	}
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) > maxStableReuseBytes {
		return stableReuseEntry{}, false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var entry stableReuseEntry
	if decoder.Decode(&entry) != nil {
		return stableReuseEntry{}, false
	}
	return entry, true
}

// store keeps an accepted verdict for key; a directory write that fails only
// costs a later process its reuse.
func (r *StableReuse) store(key stableReuseKey, result verify.StableResult) {
	entry := stableReuseEntry{stableReuseProfile, key, *result.TargetRevision, *result.PatchSha256, *result.MapSha256, result.ArtifactChecks}
	r.mu.Lock()
	r.memory[key] = entry
	r.mu.Unlock()
	if r.dir == "" {
		return
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return
	}
	file, err := os.CreateTemp(r.dir, ".stable-*.tmp")
	if err != nil {
		return
	}
	_, writeErr := file.Write(raw)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil || os.Rename(file.Name(), r.entryPath(key)) != nil {
		_ = os.Remove(file.Name())
	}
}

func sameArtifactChecks(left, right []verify.StableArtifactCheck) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// fullStableVerification is acceptedStable; tests replace it to count the
// full verifications a run performs.
var fullStableVerification = acceptedStable

// acceptedStableReused is acceptedStable for an OCM read or prepare: an
// accepted verdict for exactly this key is reused, and otherwise the full
// verification runs and an ACCEPT is stored for the rest of the run.
func acceptedStableReused(ctx context.Context, reuse *StableReuse, cemRaw []byte, options StableUniverseOptions) (target, patch string, err error) {
	var key stableReuseKey
	keyed := false
	if reuse != nil {
		key, keyed = reuse.key(cemRaw, options.ExpectedBase, options.Target, options.Root)
		if keyed && options.ArtifactRoot == options.Root {
			if entry, ok := reuse.lookup(ctx, key, options.ArtifactRoot, cemRaw); ok {
				return entry.TargetRevision, entry.PatchSha256, nil
			}
		}
	}
	result, err := fullStableVerification(ctx, cemRaw, options)
	if err != nil {
		return "", "", err
	}
	if keyed && options.ArtifactRoot == options.Root {
		reuse.store(key, result)
	}
	return *result.TargetRevision, *result.PatchSha256, nil
}
