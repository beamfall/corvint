package lrfrepo

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// countFullVerifications counts the full stable verifications until the test
// ends.
func countFullVerifications(t *testing.T) *int {
	t.Helper()
	count := 0
	previous := fullStableVerification
	fullStableVerification = func(ctx context.Context, cemRaw []byte, options StableUniverseOptions) (verify.StableResult, error) {
		count++
		return previous(ctx, cemRaw, options)
	}
	t.Cleanup(func() { fullStableVerification = previous })
	return &count
}

// privateReuseDir names a fresh private run store in StableReuseEnv.
func privateReuseDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "stable-reuse")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv(StableReuseEnv, dir)
	return dir
}

// TestOCMV0027RunVerifiesUnchangedCEMOnce (OCM-V0-027, V1-2894): within one
// run, prepare, status and an aggregate over several scopes verify the same
// CEM bytes once; every later read reuses that ACCEPT through the run store,
// and without the store each read verifies in full as before.
func TestOCMV0027RunVerifiesUnchangedCEMOnce(t *testing.T) {
	root, base, target := makeOCMPrepareRepository(t)
	prepareStableCEM(t, root, base, target)
	dir := privateReuseDir(t)
	count := countFullVerifications(t)
	prepare := PrepareOptions{
		MapPath: DefaultOCMMapPath, CEMPath: wire.ExcludedCEMPath,
		IntentPath: "docs/specs/intent.md", ExpectedBase: base, Target: target,
	}
	if _, err := PrepareOCM(context.Background(), root, prepare); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	read := OCMReadOptions{
		OCMPath: DefaultOCMMapPath, CEMPath: wire.ExcludedCEMPath,
		ExpectedBase: base, Target: target, ExpectedBaseGiven: true, TargetGiven: true,
	}
	aggregate := read
	aggregate.StableReuse = NewStableReuse()
	for _, options := range []OCMReadOptions{read, read, aggregate, aggregate} {
		result, err := ReadOCM(context.Background(), root, options)
		if err != nil || result.State == "invalid" || result.VerificationCause != nil {
			t.Fatalf("status: err=%v result=%+v", err, result)
		}
	}
	if *count != 1 {
		t.Fatalf("full stable verifications = %d, want 1 per run", *count)
	}
	entries, _ := filepath.Glob(filepath.Join(dir, "stable-*.json"))
	if len(entries) != 1 {
		t.Fatalf("run store entries = %v, want one", entries)
	}

	t.Setenv(StableReuseEnv, "")
	if _, err := ReadOCM(context.Background(), root, read); err != nil {
		t.Fatal(err)
	}
	if *count != 2 {
		t.Fatalf("a read outside the run reused a verdict: %d verifications", *count)
	}
}

// TestOCMV0027ChangedDigestForcesReverification (OCM-V0-027): a CEM whose
// bytes differ from the stored verdict's misses the store and is verified in
// full under the same base, target, root and verifier, so its own verdict
// stands: the rebound map is refused for the old target rather than admitted
// by reuse, and accepted only for its own target.
func TestOCMV0027ChangedDigestForcesReverification(t *testing.T) {
	root, base, target := makeOCMPrepareRepository(t)
	raw := prepareStableCEM(t, root, base, target)
	privateReuseDir(t)
	count := countFullVerifications(t)
	options := StableUniverseOptions{Root: root, ExpectedBase: base, Target: target, ArtifactRoot: root}
	reuse := NewStableReuse()
	if _, _, err := acceptedStableReused(context.Background(), reuse, raw, options); err != nil {
		t.Fatal(err)
	}
	if _, _, err := acceptedStableReused(context.Background(), reuse, raw, options); err != nil || *count != 1 {
		t.Fatalf("unchanged digest: err=%v verifications=%d, want 1", err, *count)
	}

	writeOCMPrepareFile(t, root, "internal/example.txt", "after again\n")
	gitOCMPrepare(t, root, "commit", "-qam", "next")
	next := gitOCMPrepare(t, root, "rev-parse", "HEAD")
	prepareCEM(t, root, base, next, true)
	changed, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath)))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(changed, raw) {
		t.Fatal("fixture CEM digest did not change")
	}
	if _, _, err := acceptedStableReused(context.Background(), reuse, changed, options); err == nil {
		t.Fatal("a changed CEM digest was admitted for the old target by reuse")
	}
	if *count != 2 {
		t.Fatalf("full stable verifications = %d, want 2: the changed digest must re-verify", *count)
	}
	options.Target = next
	if got, _, err := acceptedStableReused(context.Background(), reuse, changed, options); err != nil || got != next || *count != 3 {
		t.Fatalf("changed CEM for its own target: target=%s err=%v verifications=%d", got, err, *count)
	}
}

// TestOCMV0027ReuseMissesOnVerifierOrArtifactChange (OCM-V0-027): a verdict
// stored under another verifier identity, a store entry whose artifact checks
// no longer read back, a corrupt entry and a non-private store are all misses.
func TestOCMV0027ReuseMissesOnVerifierOrArtifactChange(t *testing.T) {
	root, base, target := makeOCMPrepareRepository(t)
	raw := prepareStableCEM(t, root, base, target)
	dir := privateReuseDir(t)
	count := countFullVerifications(t)
	options := StableUniverseOptions{Root: root, ExpectedBase: base, Target: target, ArtifactRoot: root}
	verifyOnce := func(reuse *StableReuse) {
		t.Helper()
		if _, _, err := acceptedStableReused(context.Background(), reuse, raw, options); err != nil {
			t.Fatal(err)
		}
	}
	first := NewStableReuse()
	verifyOnce(first)
	verifyOnce(NewStableReuse())
	if *count != 1 {
		t.Fatalf("a second process in the run did not reuse: %d", *count)
	}

	other := NewStableReuse()
	other.once.Do(func() {})
	other.verifier = "0000000000000000000000000000000000000000000000000000000000000000"
	verifyOnce(other)
	if *count != 2 {
		t.Fatalf("another verifier identity reused the verdict: %d", *count)
	}

	key, _ := first.key(raw, base, target, root)
	entry := first.memory[key]
	entry.ArtifactChecks = append(entry.ArtifactChecks, verify.StableArtifactCheck{Kind: "log", Path: "gone.txt", Sha256: "sha256:00", ByteLength: 1})
	first.memory[key] = entry
	verifyOnce(first)
	if *count != 3 {
		t.Fatalf("a verdict whose artifacts no longer read back was reused: %d", *count)
	}

	if err := os.WriteFile(first.entryPath(key), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	verifyOnce(NewStableReuse())
	if *count != 4 {
		t.Fatalf("a corrupt store entry was reused: %d", *count)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if NewStableReuse().dir != "" {
			t.Fatal("a store readable by others was admitted")
		}
	}
	t.Setenv(StableReuseEnv, "relative/dir")
	if environmentStableReuse() != nil {
		t.Fatal("a relative store was admitted")
	}
}

// TestOCMV0027ReplacedExecutableHasNoIdentity (OCM-V0-027): an executable
// path whose file changed after the process started, as an install's rename
// onto it does, may not hold the running verifier, so it yields no identity
// and disables reuse; Linux reads the running image itself instead.
func TestOCMV0027ReplacedExecutableHasNoIdentity(t *testing.T) {
	staged := filepath.Join(t.TempDir(), "staged")
	if err := os.WriteFile(staged, []byte("verifier\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	before := time.Now().Add(-time.Hour)
	if err := os.Chtimes(staged, before, before); err != nil {
		t.Fatal(err)
	}
	started := time.Now().Add(-time.Minute)
	installed := filepath.Join(filepath.Dir(staged), "installed")
	if err := os.Rename(staged, installed); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(installed)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "darwin" && fileChangedBefore(info, started) {
		t.Error("a rename after the process start kept the file's identity")
	}
	if !fileChangedBefore(info, time.Now().Add(time.Hour)) {
		t.Error("a file unchanged since before the process start lost its identity")
	}

	previous := stableReuseProcessStart
	t.Cleanup(func() { stableReuseProcessStart = previous })
	stableReuseProcessStart = time.Time{}
	identity := new(StableReuse).verifierIdentity()
	if runtime.GOOS == "linux" {
		if identity == "" {
			t.Error("the running image on Linux yielded no identity")
		}
	} else if identity != "" {
		t.Error("an executable changed after the process start kept its identity")
	}
}
