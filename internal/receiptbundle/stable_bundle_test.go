package receiptbundle

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
)

// stablePacket is the public S0E repository-envelope packet: one sha1
// repository, its six role artifacts and its sealed positive cem/1.0 map, whose
// target commits the map at the sidecar path.
const stablePacket = "../../protocol/cem-1.0/stable/repository-envelope-packet"

const stableMapPath = "change.cem.json"

type stableFixture struct {
	dir, repo, artifacts string
	base, target         string
	mapBytes             []byte
}

func stableCode(err error) string {
	var coded *cemcode.Error
	if errors.As(err, &coded) {
		return coded.Code
	}
	return ""
}

// requireStableRuntime skips with NOT_RUN where the stable verifier cannot
// accept: its owned Git lifecycle is observed on Darwin and Linux only, and
// the sealed positive keeps a structural hunk only admitted Go releases prove.
func requireStableRuntime(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("NOT_RUN: the stable verifier's owned Git lifecycle is observed on Darwin and Linux only")
	}
	if v := runtime.Version(); v != "go1.27.1" && v != "go1.27.2" && v != "go1.27.3" {
		t.Skipf("NOT_RUN: %s is not an admitted structural tuple", v)
	}
}

// newStableFixture materializes the packet's repository, writes its artifacts
// to an artifact root outside it and the sealed map into the worktree.
func newStableFixture(t *testing.T) stableFixture {
	t.Helper()
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join(stablePacket, name))
		if err != nil {
			t.Fatalf("public packet: %v", err)
		}
		return data
	}
	var cases struct {
		Cases []struct {
			ID             string         `json:"id"`
			ExpectedResult map[string]any `json:"expectedResult"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(read("FULL-RESULT-CASES.json"), &cases); err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := stableFixture{dir: dir, repo: filepath.Join(dir, "repo"), artifacts: filepath.Join(dir, "artifacts")}
	for _, c := range cases.Cases {
		if c.ID == "normal-exit-lingering-descendant-contained" {
			f.base, _ = c.ExpectedResult["expectedBase"].(string)
			f.target, _ = c.ExpectedResult["targetRevision"].(string)
		}
	}
	if f.base == "" || f.target == "" {
		t.Fatal("public packet has no complete positive case")
	}
	command := exec.Command("git", "init", "-q", "--object-format=sha1", f.repo)
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	for _, packName := range []string{"inherited/fixtures/FIXTURES.pack.json", "inherited/fixtures/FIXTURES-ledger-boundary.pack.json"} {
		var pack struct {
			Records []struct{ Path, Type, Sha256, Base64 string } `json:"records"`
		}
		if err := json.Unmarshal(read(packName), &pack); err != nil {
			t.Fatal(err)
		}
		for _, record := range pack.Records {
			if record.Type != "regular" {
				continue
			}
			data, err := base64.StdEncoding.DecodeString(record.Base64)
			if digest := sha256.Sum256(data); err != nil || hex.EncodeToString(digest[:]) != record.Sha256 {
				t.Fatalf("fixture record %s does not match its digest", record.Path)
			}
			destination := ""
			switch {
			case strings.HasPrefix(record.Path, "fixtures/git-objects/sha1/"):
				oid := strings.TrimSuffix(filepath.Base(record.Path), ".object")
				if sum := sha1.Sum(data); hex.EncodeToString(sum[:]) != oid {
					t.Fatalf("object record %s is not the raw object of its name", record.Path)
				}
				var compressed bytes.Buffer
				writer := zlib.NewWriter(&compressed)
				writer.Write(data)
				writer.Close()
				destination, data = filepath.Join(f.repo, ".git", "objects", oid[:2], oid[2:]), compressed.Bytes()
			case strings.HasPrefix(record.Path, "fixtures/artifacts/"):
				destination = filepath.Join(f.artifacts, filepath.Base(record.Path))
			default:
				continue
			}
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(destination, data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	f.mapBytes = read("inherited/fixtures/maps/sha1-positive-sealed.json")
	if err := os.WriteFile(filepath.Join(f.repo, stableMapPath), f.mapBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f stableFixture) options(output string) Options {
	return Options{
		MapPath: stableMapPath, ExpectedBase: f.base, Target: f.target, Output: output,
		Repository: f.repo, Artifacts: f.artifacts, ArtifactsGiven: true,
	}
}

func stableEnvelopeResult(t *testing.T, envelope map[string]any) workflow.StableVerification {
	t.Helper()
	verification, ok := envelope["stableVerification"].(workflow.StableVerification)
	if !ok {
		t.Fatalf("envelope carries no stable verification: %v", envelope)
	}
	return verification
}

// CEM-V1-060, CEM-V1-061, CEM-V1-062: a cem/1.0 export carries the exact map and
// all six role artifacts, including the snapshot-head receipt, under their
// declared paths; moved to another root, the bundle verifies with the same
// result as the source, through the API and through `cem verify-stable`.
func TestStableExportRelocatesWithSameVerification(t *testing.T) {
	requireStableRuntime(t)
	f := newStableFixture(t)
	output := filepath.Join(f.dir, "out", "bundle")
	if err := os.Mkdir(filepath.Dir(output), 0o755); err != nil {
		t.Fatal(err)
	}
	envelope, err := Export(context.Background(), f.repo, f.options(output))
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	exported := stableEnvelopeResult(t, envelope)
	if envelope["ok"] != true || envelope["mutates"] != false || envelope["profile"] != StableProfile ||
		exported.Exit != 0 || exported.Result.Outcome != "ACCEPT" || exported.Result.Sidecar != "EXACT" {
		t.Fatalf("envelope %#v", envelope)
	}
	source, exit := verify.Stable(context.Background(), f.mapBytes, verify.StableOptions{
		Repository: f.repo, ExpectedBase: f.base, Target: f.target, ArtifactRoot: f.artifacts,
	})
	if exit != 0 || !reflect.DeepEqual(source, exported.Result) {
		t.Fatalf("exported result differs from the source verification")
	}
	manifest, err := os.ReadFile(filepath.Join(output, ManifestName))
	if err != nil || envelope["manifestSha256"] != digest(manifest) {
		t.Fatalf("manifest digest: %v", err)
	}

	moved := filepath.Join(f.dir, "elsewhere", "deeper", "moved")
	if err := os.MkdirAll(filepath.Dir(moved), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(output, moved); err != nil {
		t.Fatal(err)
	}
	result, exit, bundle, err := VerifyStable(context.Background(), moved, f.repo, f.base, f.target)
	if err != nil || exit != 0 || !reflect.DeepEqual(result, source) {
		t.Fatalf("relocated bundle: exit %d err %v", exit, err)
	}
	if !bytes.Equal(bundle.Map, f.mapBytes) {
		t.Fatal("the bundle rewrote the map")
	}
	kinds := []string{}
	for _, a := range bundle.Artifacts {
		original, err := os.ReadFile(filepath.Join(f.artifacts, a.Path))
		if err != nil || !bytes.Equal(original, a.Bytes()) {
			t.Fatalf("artifact %s is not an exact copy", a.Path)
		}
		kinds = append(kinds, a.Kind)
	}
	slices.Sort(kinds)
	if want := []string{"runner-plan", "runner-receipt", "tasks-capture", "tasks-claimed-ticket", "tasks-snapshot-head-receipt", "tasks-verification"}; !slices.Equal(kinds, want) {
		t.Fatalf("roles %v", kinds)
	}
	checked := []string{}
	for _, check := range result.ArtifactChecks {
		checked = append(checked, check.Kind)
	}
	if !slices.Contains(checked, "tasks-snapshot-head-receipt") || len(checked) != 6 {
		t.Fatalf("artifact checks %v", checked)
	}

	// The existing stable verifier reads the relocated bundle as it stands.
	raw, err := os.ReadFile(filepath.Join(moved, StableMapName))
	if err != nil {
		t.Fatal(err)
	}
	direct, exit := verify.Stable(context.Background(), raw, verify.StableOptions{
		Repository: f.repo, ExpectedBase: f.base, Target: f.target, ArtifactRoot: filepath.Join(moved, StableArtifactDir),
	})
	if exit != 0 || !reflect.DeepEqual(direct, source) {
		t.Fatalf("verify-stable over the moved bundle: exit %d", exit)
	}
}

// CEM-V1-062: a bundle written from a map whose declared artifact paths are
// nested keeps every path and digest when its root moves.
func TestStableBundleKeepsNestedPathsAcrossRoots(t *testing.T) {
	f := newStableFixture(t)
	document, err := wire.ParseStable(f.mapBytes)
	if err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(f.dir, "nested")
	for i := range document.Artifacts {
		a := &document.Artifacts[i]
		data, err := os.ReadFile(filepath.Join(f.artifacts, a.Path))
		if err != nil {
			t.Fatal(err)
		}
		a.Path = "roles/" + strings.ReplaceAll(a.Kind, "-", "/") + "/" + a.Path
		if err := os.MkdirAll(filepath.Join(nested, filepath.Dir(filepath.FromSlash(a.Path))), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(nested, filepath.FromSlash(a.Path)), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mapBytes, err := wire.EncodeStable(document)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := CollectStable(mapBytes, f.base, f.target, nested)
	if err != nil {
		t.Fatal(err)
	}
	first := writeStableAt(t, filepath.Join(f.dir, "first"), bundle)
	second := filepath.Join(f.dir, "second", "root")
	if err := os.MkdirAll(filepath.Dir(second), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(first, second); err != nil {
		t.Fatal(err)
	}
	read, err := ReadStable(second)
	if err != nil {
		t.Fatalf("read moved bundle: %v", err)
	}
	if !reflect.DeepEqual(read, bundle) || !bytes.Equal(read.Manifest(), bundle.Manifest()) {
		t.Fatal("the moved bundle differs from the written one")
	}
	for _, a := range document.Artifacts {
		data, err := os.ReadFile(filepath.Join(second, StableArtifactDir, filepath.FromSlash(a.Path)))
		if err != nil || digest(data) != a.Sha256 {
			t.Fatalf("artifact %s lost its path or digest", a.Path)
		}
	}
}

func writeStableAt(t *testing.T, dir string, bundle *StableBundle) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	parent, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	if err := WriteStable(parent, "bundle", bundle); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "bundle")
}

// CEM-V1-063: a bundle with a tampered, omitted, unused or escaping entry, or
// an inexact manifest, is refused by the bundle reader before any verifier
// runs; a bundle that records another base or target is refused by
// VerifyStable.
func TestStableBundleRefusesIncompleteOrTamperedEntries(t *testing.T) {
	f := newStableFixture(t)
	bundle, err := CollectStable(f.mapBytes, f.base, f.target, f.artifacts)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := ""
	for _, a := range bundle.Artifacts {
		if a.Kind == "tasks-snapshot-head-receipt" {
			snapshot = filepath.Join(StableArtifactDir, a.Path)
		}
	}
	outside := filepath.Join(f.dir, "outside.json")
	cases := []struct {
		name, code string
		mutate     func(t *testing.T, dir string)
	}{
		{"tampered snapshot-head byte", CodeStableEntryDigest, func(t *testing.T, dir string) {
			path := filepath.Join(dir, snapshot)
			data, _ := os.ReadFile(path)
			data[0] ^= 1
			os.Chmod(path, 0o600)
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"tampered map byte", CodeStableEntryDigest, func(t *testing.T, dir string) {
			path := filepath.Join(dir, StableMapName)
			data, _ := os.ReadFile(path)
			if err := os.WriteFile(path, append(data, ' '), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"omitted artifact", CodeStableEntryMissing, func(t *testing.T, dir string) {
			if err := os.Remove(filepath.Join(dir, snapshot)); err != nil {
				t.Fatal(err)
			}
		}},
		{"omitted map", CodeStableEntryMissing, func(t *testing.T, dir string) {
			if err := os.Remove(filepath.Join(dir, StableMapName)); err != nil {
				t.Fatal(err)
			}
		}},
		{"unused file", CodeStableEntryUnused, func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, StableArtifactDir, "extra.json"), []byte("{}"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"unused directory", CodeStableEntryUnused, func(t *testing.T, dir string) {
			if err := os.Mkdir(filepath.Join(dir, "spare"), 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		{"escaping symlink with the same bytes", CodeStableEntryEscaping, func(t *testing.T, dir string) {
			path := filepath.Join(dir, snapshot)
			data, _ := os.ReadFile(path)
			if err := os.WriteFile(outside, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, path); err != nil {
				t.Fatal(err)
			}
		}},
		{"escaping artifacts directory", CodeStableEntryEscaping, func(t *testing.T, dir string) {
			if err := os.Rename(filepath.Join(dir, StableArtifactDir), filepath.Join(f.dir, "moved-artifacts")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(f.dir, "moved-artifacts"), filepath.Join(dir, StableArtifactDir)); err != nil {
				t.Fatal(err)
			}
		}},
		{"inexact manifest", CodeStableManifest, func(t *testing.T, dir string) {
			path := filepath.Join(dir, ManifestName)
			data, _ := os.ReadFile(path)
			if err := os.WriteFile(path, append([]byte(" "), data...), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"manifest with an unknown field", CodeStableManifest, func(t *testing.T, dir string) {
			path := filepath.Join(dir, ManifestName)
			data, _ := os.ReadFile(path)
			if err := os.WriteFile(path, bytes.Replace(data, []byte(`{"profile"`), []byte(`{"extra":1,"profile"`), 1), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := writeStableAt(t, filepath.Join(f.dir, "case", strings.Repeat("x", i+1)), bundle)
			if _, err := ReadStable(dir); err != nil {
				t.Fatalf("intact bundle refused: %v", err)
			}
			c.mutate(t, dir)
			if _, err := ReadStable(dir); stableCode(err) != c.code {
				t.Fatalf("got %v, want %s", err, c.code)
			}
			if _, exit, _, err := VerifyStable(context.Background(), dir, f.repo, f.base, f.target); exit != 2 || stableCode(err) != c.code {
				t.Fatalf("VerifyStable: exit %d %v", exit, err)
			}
		})
	}

	other := *bundle
	other.Target = strings.Repeat("0", len(f.target))
	dir := writeStableAt(t, filepath.Join(f.dir, "other-target"), &other)
	if _, err := ReadStable(dir); err != nil {
		t.Fatal(err)
	}
	if _, exit, _, err := VerifyStable(context.Background(), dir, f.repo, f.base, f.target); exit != 2 || stableCode(err) != CodeStableRevision {
		t.Fatalf("other target: exit %d %v", exit, err)
	}
}

// CEM-V1-063: a stale or tampered source publishes nothing; the export returns
// the verifier's complete result and exit code.
func TestStableExportPublishesNothingForARefusedSource(t *testing.T) {
	requireStableRuntime(t)
	f := newStableFixture(t)
	snapshot := filepath.Join(f.artifacts, "tasks-snapshot-head-receipt.json")
	cases := []struct {
		name, outcome string
		exit          int
		mutate        func(t *testing.T, o *Options)
	}{
		{"stale base", "REJECT", 1, func(t *testing.T, o *Options) { o.ExpectedBase = f.target }},
		{"tampered artifact", "REJECT", 1, func(t *testing.T, o *Options) {
			data, _ := os.ReadFile(snapshot)
			tampered := filepath.Join(f.dir, "tampered")
			copyTree(t, f.artifacts, tampered)
			os.Chmod(filepath.Join(tampered, filepath.Base(snapshot)), 0o600)
			data[0] ^= 1
			if err := os.WriteFile(filepath.Join(tampered, filepath.Base(snapshot)), data, 0o600); err != nil {
				t.Fatal(err)
			}
			o.Artifacts = tampered
		}},
		{"omitted artifact", "UNSUPPORTED", 2, func(t *testing.T, o *Options) {
			omitted := filepath.Join(f.dir, "omitted")
			copyTree(t, f.artifacts, omitted)
			if err := os.Remove(filepath.Join(omitted, filepath.Base(snapshot))); err != nil {
				t.Fatal(err)
			}
			o.Artifacts = omitted
		}},
		{"no artifact root", "UNSUPPORTED", 2, func(t *testing.T, o *Options) { o.Artifacts, o.ArtifactsGiven = "", false }},
		{"relative repository", "UNSUPPORTED", 2, func(t *testing.T, o *Options) { o.Repository = "repo" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			output := filepath.Join(f.dir, "refused-"+strings.ReplaceAll(c.name, " ", "-"))
			options := f.options(output)
			c.mutate(t, &options)
			envelope, err := Export(context.Background(), f.repo, options)
			if err != nil {
				t.Fatalf("export: %v", err)
			}
			verification := stableEnvelopeResult(t, envelope)
			if envelope["ok"] != false || verification.Exit != c.exit || verification.Result.Outcome != c.outcome {
				t.Fatalf("envelope %#v", envelope)
			}
			if filepath.IsAbs(options.Repository) {
				want, exit := verify.Stable(context.Background(), f.mapBytes, verify.StableOptions{
					Repository: options.Repository, ExpectedBase: options.ExpectedBase, Target: options.Target, ArtifactRoot: options.Artifacts,
				})
				if exit != verification.Exit || !reflect.DeepEqual(want, verification.Result) {
					t.Fatalf("export result differs from verify.Stable over the same inputs")
				}
			}
			if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("a refused export published %s", output)
			}
		})
	}
}

func copyTree(t *testing.T, from, to string) {
	t.Helper()
	if err := os.CopyFS(to, os.DirFS(from)); err != nil {
		t.Fatal(err)
	}
}

// CEM-V1-064: the stable route refuses --witness and an output inside the
// worktree, after verification and before writing; a retired 0.x map
// refuses with the retired-spec fix (CEM-V1-210).
func TestStableExportArgumentRefusals(t *testing.T) {
	requireStableRuntime(t)
	f := newStableFixture(t)
	options := f.options(filepath.Join(f.dir, "with-witness"))
	options.Witness, options.WitnessGiven = filepath.Join(f.dir, "witness.json"), true
	if _, err := Export(context.Background(), f.repo, options); stableCode(err) != cemcode.InvalidArguments {
		t.Fatalf("--witness: %v", err)
	}
	inside := filepath.Join(f.repo, "bundle")
	if _, err := Export(context.Background(), f.repo, f.options(inside)); stableCode(err) != cemcode.BundleOutputRefused {
		t.Fatalf("in-worktree output: %v", err)
	}
	for _, path := range []string{options.Output, inside} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("a refused export published %s", path)
		}
	}

	legacy := newFixture(t)
	writeFixture(t, filepath.Join(legacy.root, mapPath), `{"spec":"cem/0.2"}`)
	_, err := Export(context.Background(), legacy.root, Options{
		MapPath: mapPath, ExpectedBase: legacy.base, Target: legacy.target,
		Output: filepath.Join(t.TempDir(), "bundle"), Artifacts: f.artifacts, ArtifactsGiven: true,
	})
	if stableCode(err) != cemcode.UnsupportedSpec || !strings.Contains(err.Error(), wire.RetiredSpecFix) {
		t.Fatalf("0.x map: %v", err)
	}
}

// CEM-V1-061, CEM-V1-063: WriteStable writes only the map's exact declared
// closure; an altered path, role or byte is refused before anything is
// created, so the output parent stays empty.
func TestWriteStableRefusesAnAlteredClosure(t *testing.T) {
	f := newStableFixture(t)
	for _, c := range []struct {
		name, code string
		alter      func(b *StableBundle)
	}{
		{"escaping path", CodeStableManifest, func(b *StableBundle) { b.Artifacts[0].Path = "../../leaked" }},
		{"duplicated path", CodeStableManifest, func(b *StableBundle) { b.Artifacts[1].Path = b.Artifacts[0].Path }},
		{"added artifact", CodeStableManifest, func(b *StableBundle) {
			b.Artifacts[0].Path = "../../leaked"
			b.Artifacts = append(b.Artifacts, b.Artifacts[0])
		}},
		{"relabelled role", CodeStableManifest, func(b *StableBundle) { b.Artifacts[0].Kind = b.Artifacts[1].Kind }},
		{"omitted role", CodeStableManifest, func(b *StableBundle) { b.Artifacts = b.Artifacts[1:] }},
		{"altered bytes", CodeStableEntryDigest, func(b *StableBundle) { b.Artifacts[0].data = append(b.Artifacts[0].Bytes(), ' ') }},
		{"other base", CodeStableManifest, func(b *StableBundle) { b.Base = f.target }},
	} {
		t.Run(c.name, func(t *testing.T) {
			bundle, err := CollectStable(f.mapBytes, f.base, f.target, f.artifacts)
			if err != nil {
				t.Fatal(err)
			}
			c.alter(bundle)
			dir := t.TempDir()
			parent, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer parent.Close()
			if err := WriteStable(parent, "bundle", bundle); stableCode(err) != c.code {
				t.Fatalf("got %v, want %s", err, c.code)
			}
			if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
				t.Fatalf("a refused write left %d entries", len(entries))
			}
		})
	}
}

// CEM-V1-063: a bundle holding more entries than the walk bound is refused.
func TestReadStableRefusesAnOversizedTree(t *testing.T) {
	f := newStableFixture(t)
	bundle, err := CollectStable(f.mapBytes, f.base, f.target, f.artifacts)
	if err != nil {
		t.Fatal(err)
	}
	dir := writeStableAt(t, filepath.Join(f.dir, "oversized"), bundle)
	spare := filepath.Join(dir, "spare")
	if err := os.Mkdir(spare, 0o700); err != nil {
		t.Fatal(err)
	}
	for i := range maxStableEntries {
		if err := os.WriteFile(filepath.Join(spare, strconv.Itoa(i)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ReadStable(dir); stableCode(err) != CodeStableEntryUnused || !strings.Contains(err.Error(), "more than") {
		t.Fatalf("oversized tree: %v", err)
	}
}

// CEM-V1-063: a bundle changed between an entry's check and its read is
// refused: an entry swapped for an in-root symlink to an undeclared sibling
// (whose bytes still match), a file added after the walk, or, during the
// re-reads, the artifacts directory or the bundle itself moved aside and
// replaced by a symlink to it.
func TestReadStableRefusesABundleChangedDuringTheRead(t *testing.T) {
	f := newStableFixture(t)
	bundle, err := CollectStable(f.mapBytes, f.base, f.target, f.artifacts)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(StableArtifactDir, bundle.Artifacts[0].Path)
	for i, c := range []struct {
		name   string
		call   int // the target read during which the bundle changes
		change func(dir string) error
	}{
		{"symlinked sibling", 1, func(dir string) error {
			if err := os.Rename(filepath.Join(dir, target), filepath.Join(dir, target+".moved")); err != nil {
				return err
			}
			return os.Symlink(filepath.Base(target)+".moved", filepath.Join(dir, target))
		}},
		{"rewritten after its read", 1, func(dir string) error {
			return os.WriteFile(filepath.Join(dir, StableMapName), []byte("{}"), 0o600)
		}},
		{"added file", 1, func(dir string) error {
			return os.WriteFile(filepath.Join(dir, StableArtifactDir, "late.json"), nil, 0o600)
		}},
		{"artifacts directory symlinked during the re-read", 2, func(dir string) error {
			if err := os.Rename(filepath.Join(dir, StableArtifactDir), filepath.Join(dir, StableArtifactDir+".moved")); err != nil {
				return err
			}
			return os.Symlink(StableArtifactDir+".moved", filepath.Join(dir, StableArtifactDir))
		}},
		{"bundle root symlinked during the re-read", 2, func(dir string) error {
			if err := os.Rename(dir, dir+".moved"); err != nil {
				return err
			}
			return os.Symlink(filepath.Base(dir)+".moved", dir)
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := writeStableAt(t, filepath.Join(f.dir, "changed", strconv.Itoa(i)), bundle)
			changed, calls := false, 0
			stableReadHook = func(name string) {
				if name != target {
					return
				}
				if calls++; calls == c.call {
					changed = true
					if err := c.change(dir); err != nil {
						t.Error(err)
					}
				}
			}
			defer func() { stableReadHook = nil }()
			if _, err := ReadStable(dir); stableCode(err) != CodeStableEntryEscaping || !changed {
				t.Fatalf("changed bundle: %v (changed %v)", err, changed)
			}
		})
	}
}

// CEM-V1-061, CEM-V1-063: WriteStable writes only into the directory it
// created and its cleanup removes only what it wrote. A created directory
// replaced before its identity check by a non-empty sibling, or before its
// open by a symlink to that sibling or by another real directory, is refused
// with nothing written or removed; a write that fails part way removes the
// bundle it started.
func TestWriteStableRefusesASwappedBundleDirectory(t *testing.T) {
	f := newStableFixture(t)
	bundle, err := CollectStable(f.mapBytes, f.base, f.target, f.artifacts)
	if err != nil {
		t.Fatal(err)
	}
	moveAside := func(dir string) error {
		return os.Rename(filepath.Join(dir, "bundle"), filepath.Join(dir, "created"))
	}
	for _, c := range []struct {
		name, stage string
		want        []string // every regular file left under the output parent
		change      func(dir string) error
	}{
		{"non-empty sibling moved into place", "mkdir", []string{"bundle/keep"}, func(dir string) error {
			if err := moveAside(dir); err != nil {
				return err
			}
			return os.Rename(filepath.Join(dir, "sibling"), filepath.Join(dir, "bundle"))
		}},
		{"symlink to a sibling", "open", []string{"sibling/keep"}, func(dir string) error {
			if err := moveAside(dir); err != nil {
				return err
			}
			return os.Symlink("sibling", filepath.Join(dir, "bundle"))
		}},
		{"another real directory", "open", []string{"sibling/keep"}, func(dir string) error {
			if err := moveAside(dir); err != nil {
				return err
			}
			return os.Mkdir(filepath.Join(dir, "bundle"), 0o700)
		}},
		{"write failing after the map", StableMapName, []string{"sibling/keep"}, func(dir string) error {
			return os.Chmod(filepath.Join(dir, "bundle", StableArtifactDir), 0o500)
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "sibling"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "sibling", "keep"), []byte("keep"), 0o600); err != nil {
				t.Fatal(err)
			}
			parent, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer parent.Close()
			stableWriteHook = func(stage string) {
				if stage == c.stage {
					if err := c.change(dir); err != nil {
						t.Error(err)
					}
				}
			}
			defer func() { stableWriteHook = nil }()
			if err := WriteStable(parent, "bundle", bundle); stableCode(err) != cemcode.PublishFailed {
				t.Fatalf("changed bundle directory: %v", err)
			}
			var left []string
			filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
				if err == nil && entry.Type().IsRegular() {
					rel, _ := filepath.Rel(dir, path)
					left = append(left, filepath.ToSlash(rel))
				}
				return nil
			})
			if !slices.Equal(left, c.want) {
				t.Fatalf("files left %v, want %v", left, c.want)
			}
			if c.stage == StableMapName {
				if _, err := os.Lstat(filepath.Join(dir, "bundle")); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("a failed write left its bundle directory: %v", err)
				}
			}
		})
	}
}

// CEM-V1-063: a declared file rewritten in place after its own re-read, while
// another file is re-read, is refused by the final walk's comparison of
// identity, size, modification time and change time, including when the
// rewrite keeps the length and restores the modification time.
func TestReadStableRefusesAFileRewrittenAfterItsReread(t *testing.T) {
	f := newStableFixture(t)
	bundle, err := CollectStable(f.mapBytes, f.base, f.target, f.artifacts)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range []struct {
		name    string
		restore bool // set the original modification time back after the rewrite
	}{
		{"modification time moved", false},
		{"modification time restored", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := writeStableAt(t, filepath.Join(f.dir, "rewritten", strconv.Itoa(i)), bundle)
			calls := map[string]int{}
			reread, rewritten := "", false
			stableReadHook = func(name string) {
				calls[name]++
				if calls[name] != 2 || rewritten {
					return
				}
				if reread == "" {
					reread = name // its re-read completes before the next hook call
					return
				}
				rewritten = true
				path := filepath.Join(dir, reread)
				info, err := os.Stat(path)
				if err != nil {
					t.Error(err)
					return
				}
				data, err := os.ReadFile(path)
				if err == nil {
					data[0] ^= 1
					err = os.WriteFile(path, data, 0o600)
				}
				when := info.ModTime()
				if !c.restore {
					when = when.Add(time.Hour)
				}
				if err == nil {
					err = os.Chtimes(path, when, when)
				}
				if err != nil {
					t.Error(err)
				}
			}
			defer func() { stableReadHook = nil }()
			if _, err := ReadStable(dir); stableCode(err) != CodeStableEntryEscaping || !rewritten {
				t.Fatalf("file rewritten after its re-read: %v (rewritten %v)", err, rewritten)
			}
		})
	}
}
