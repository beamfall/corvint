package extevidence

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

func executableDigest(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

// The only provider input to the offline build is an authored copy of main.go:
// no Corvint internals, installed binary, generated source, or dependency cache.
func TestProviderKitAuthoredProvider(t *testing.T) {
	t.Run("EEP-V0-016 copyable provider", func(t *testing.T) {
		source, err := os.ReadFile(filepath.Join("..", "..", "examples", "evidence-provider", "v0", "main.go"))
		if err != nil {
			t.Fatal(err)
		}
		source = bytes.ReplaceAll(source, []byte(`"kit-example"`), []byte(`"authored-example"`))
		source = bytes.ReplaceAll(source, []byte(`"0.1.0"`), []byte(`"0.1.1"`))
		dir := t.TempDir()
		writeRecord(t, dir, "main.go", source)
		binary := filepath.Join(dir, "provider")
		goBinary, err := exec.LookPath("go")
		if err != nil {
			t.Fatal(err)
		}
		env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "GOCACHE=" + filepath.Join(dir, "cache"), "GOTOOLCHAIN=local", "GOENV=off", "GOWORK=off", "GO111MODULE=off", "GOPROXY=off", "GOSUMDB=off"}
		build := procgroup.Run(t.Context(), procgroup.Spec{Argv: []string{goBinary, "build", "-trimpath", "-o", binary, "main.go"}, Dir: dir, Env: env, Timeout: 2 * time.Minute, OutputLimit: 1 << 20})
		if build.ExitStatus != 0 || !build.ExitObserved || !build.OwnedProcessGroupCleanup {
			t.Fatalf("offline provider build: %+v", build)
		}
		repo := newRepository(t)
		for _, schema := range []string{Schema, Schema1, Schema2} {
			t.Run(schema, func(t *testing.T) {
				pin := Pin{Schema: schema, ProviderID: "authored-example", ProviderRevision: "0.1.1", RepositoryRevision: repo.head, ExecutableSHA256: executableDigest(t, binary)}
				argv := []string{binary, "--provider-version", "0.1.1", "--profile", schema, "--revision", repo.head, "--path", "pkg/main.go"}
				if schema != Schema {
					pin.RepositoryID, pin.Origin = "app", repo.first
					argv = append(argv, "--origin", repo.first)
				}
				encoded, _ := json.Marshal(argv)
				command, err := ParseCommand(string(encoded))
				if err != nil {
					t.Fatal(err)
				}
				data, err := ReadPinned(t.Context(), repo.root, command, pin)
				if err != nil {
					t.Fatal(err)
				}
				file := writeRecord(t, t.TempDir(), "record.json", data)
				pin.ExecutableSHA256 = ""
				fromFile, err := ReadPinned(t.Context(), repo.root, file, pin)
				if err != nil || !bytes.Equal(data, fromFile) {
					t.Fatalf("file record differs: %v", err)
				}
				section := Section(t.Context(), repo.index(), []string{file}, nil, []string{"pkg/main.go"}, 20)
				if len(section["results"].([]any)) != 1 {
					t.Fatalf("authored relation not composed: %v", section)
				}
				fileSection := canonicalSection(t, section, file)
				commandSection := canonicalSection(t, Section(t.Context(), repo.index(), []string{command}, nil, []string{"pkg/main.go"}, 20), command)
				if !bytes.Equal(fileSection, commandSection) {
					t.Fatal("authored provider differs across file and contained command transports")
				}
				if !bytes.Contains(fileSection, []byte(`"authority":"external-provider"`)) {
					t.Fatal("external authority missing")
				}
			})
		}
	})
}

func TestProviderKitExactPins(t *testing.T) {
	t.Run("EEP-V0-017 exact compatibility window", func(t *testing.T) {
		repo := newRepository(t)
		data := fixture(t, repo.head)
		record, err := Decode(data)
		if err != nil {
			t.Fatal(err)
		}
		pin := Pin{Schema: Schema, ProviderID: record.Provider.ID, ProviderRevision: record.Provider.Revision, RepositoryRevision: repo.head}
		file := writeRecord(t, t.TempDir(), "provider.json", data)
		for name, edit := range map[string]func(*Pin){
			"schema-upgrade":     func(p *Pin) { p.Schema = Schema1; p.RepositoryID = "app"; p.Origin = repo.first },
			"unsupported":        func(p *Pin) { p.Schema = "external-evidence-provider/3" },
			"provider-id":        func(p *Pin) { p.ProviderID = "replacement" },
			"provider-revision":  func(p *Pin) { p.ProviderRevision = "next" },
			"stale":              func(p *Pin) { p.RepositoryRevision = repo.first },
			"abbreviated-commit": func(p *Pin) { p.RepositoryRevision = repo.head[:8] },
		} {
			t.Run(name, func(t *testing.T) {
				wrong := pin
				edit(&wrong)
				if got, err := ReadPinned(t.Context(), repo.root, file, wrong); err == nil || got != nil {
					t.Fatal("mismatch must return no bytes")
				}
			})
		}
		for name, invalid := range map[string][]byte{
			"malformed":          []byte(`{"schema":`),
			"authority":          mutate(t, data, func(r map[string]any) { r["authority"] = "governing" }),
			"unsupported-record": mutate(t, data, func(r map[string]any) { r["schema"] = "external-evidence-provider/3" }),
			"oversized":          bytes.Repeat([]byte(" "), MaxRecordBytes+1),
		} {
			bad := writeRecord(t, t.TempDir(), name+".json", invalid)
			if got, err := ReadPinned(t.Context(), repo.root, bad, pin); err == nil || got != nil {
				t.Fatalf("%s accepted", name)
			}
		}
		if _, err := ReadPinned(t.Context(), repo.root, t.TempDir(), pin); err == nil {
			t.Fatal("directory accepted")
		}
		p := newPair(t)
		v1 := conformance(t, "two-repository.json", p.values())
		decoded, _ := Decode1(v1)
		v1pin := Pin{Schema: Schema1, ProviderID: decoded.Provider.ID, ProviderRevision: decoded.Provider.Revision, RepositoryRevision: p.app.head, RepositoryID: "application", Origin: p.app.first}
		v1file := writeRecord(t, t.TempDir(), "v1.json", v1)
		if _, err := ReadPinned(t.Context(), p.app.root, v1file, v1pin); err != nil {
			t.Fatal(err)
		}
		v1pin.Schema = Schema2
		if _, err := ReadPinned(t.Context(), p.app.root, v1file, v1pin); err == nil {
			t.Fatal("/1 silently upgraded to /2")
		}
		v1pin.Schema, v1pin.Origin = Schema1, p.e2e.first
		if _, err := ReadPinned(t.Context(), p.app.root, v1file, v1pin); err == nil {
			t.Fatal("wrong repository origin accepted")
		}
	})
}

func TestProviderKitCommandPins(t *testing.T) {
	t.Run("EEP-TR-011 pinned contained command", func(t *testing.T) {
		repo := newRepository(t)
		data := fixture(t, repo.head)
		record, _ := Decode(data)
		pin := Pin{Schema: Schema, ProviderID: record.Provider.ID, ProviderRevision: record.Provider.Revision, RepositoryRevision: repo.head}
		file := writeRecord(t, t.TempDir(), "record.json", data)
		command := providerCommand(t, "serve", file)
		if _, err := ReadPinned(t.Context(), repo.root, command, pin); err == nil {
			t.Fatal("missing executable digest accepted")
		}
		pin.ExecutableSHA256 = strings.Repeat("0", 64)
		if _, err := ReadPinned(t.Context(), repo.root, command, pin); err == nil || err.Error() != "executable SHA256 differs from pin" {
			t.Fatalf("mismatch did not fail before launch: %v", err)
		}
		binary, _ := os.Executable()
		pin.ExecutableSHA256 = executableDigest(t, binary)
		got, err := ReadPinned(t.Context(), repo.root, command, pin)
		if err != nil || !bytes.Equal(got, data) {
			t.Fatalf("pinned command failed: %v", err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if got, err := ReadPinned(ctx, repo.root, providerCommand(t, "partial-sleep", file), pin); err == nil || got != nil {
			t.Fatal("cancelled command returned bytes")
		}
	})
}

func TestProviderKitFixtureConformance(t *testing.T) {
	t.Run("EEP-V0-018 reuse conformance", func(t *testing.T) {
		transportConformance(t, func(file string) string { return providerCommand(t, "serve", file) })
	})
}

// Each labelled fixture class has one pinned kit-consumer outcome; "" accepts.
// Core's own outcomes for the same classes stay with their existing witnesses.
func TestProviderKitProfileReasons(t *testing.T) {
	t.Run("EEP-V0-020 distinct profile reasons", func(t *testing.T) {
		p := newPair(t)
		v0 := fixture(t, p.app.head)
		record, _ := Decode(v0)
		pin := Pin{Schema: Schema, ProviderID: record.Provider.ID, ProviderRevision: record.Provider.Revision, RepositoryRevision: p.app.head}
		v1 := conformance(t, "two-repository.json", p.values())
		decoded, _ := Decode1(v1)
		pin1 := Pin{Schema: Schema1, ProviderID: decoded.Provider.ID, ProviderRevision: decoded.Provider.Revision, RepositoryRevision: p.app.head, RepositoryID: "application", Origin: p.app.first}
		atAncestor := pin
		atAncestor.RepositoryRevision = p.app.first
		mismatched := pin1
		mismatched.Origin = p.e2e.first
		repeatedID := bytes.Replace(v0, []byte(`"id": "mockdocs"`), []byte(`"id": "mockdocs", "id": "other"`), 1)
		deep := []byte(strings.Repeat("[", maxPinnedDepth+2) + strings.Repeat("]", maxPinnedDepth+2))
		for _, c := range []struct {
			name string
			data []byte
			pin  Pin
			want string
		}{
			{"valid /0", v0, pin, ""},
			{"valid /1", v1, pin1, ""},
			{"stale pinned at head", fixture(t, p.app.first), pin, "repository revision differs from pin"},
			{"stale pinned at its ancestor", fixture(t, p.app.first), atAncestor, ""},
			{"malformed", []byte(`{"schema":`), pin, "record is not a strict JSON document: unexpected EOF"},
			{"too deep", deep, pin, "record is not a strict JSON document: nesting exceeds 32"},
			{"ambiguous profile", append([]byte(`{"schema":"external-evidence-provider/3",`), v0[1:]...), pin, "ambiguous record profile: repeated schema member"},
			{"ambiguous identity", repeatedID, pin, `ambiguous record: repeated member "provider.id"`},
			{"ambiguous repository", conformance(t, "ambiguous.json", p.values()), pin1, "ambiguous pinned repository origin"},
			{"repository mismatch", v1, mismatched, "repository origin differs from pin"},
			{"unsupported profile", mutate(t, v0, func(r map[string]any) { r["schema"] = "external-evidence-provider/3" }), pin, "unsupported record profile"},
			{"absent profile", mutate(t, v0, func(r map[string]any) { delete(r, "schema") }), pin, "unsupported record profile"},
			{"other supported profile", v1, Pin{Schema: Schema, ProviderID: pin1.ProviderID, ProviderRevision: pin1.ProviderRevision, RepositoryRevision: p.app.head}, "record schema differs from pin"},
		} {
			t.Run(c.name, func(t *testing.T) {
				file := writeRecord(t, t.TempDir(), "record.json", c.data)
				got, err := ReadPinned(t.Context(), p.app.root, file, c.pin)
				if c.want == "" && (err != nil || !bytes.Equal(got, c.data)) {
					t.Fatalf("want accepted: %v", err)
				}
				if c.want != "" && (err == nil || err.Error() != c.want || got != nil) {
					t.Fatalf("want %q with no bytes, got %v", c.want, err)
				}
			})
		}
	})
}
