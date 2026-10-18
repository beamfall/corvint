package companionrelease

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRubyProviderBundleProfileThree drives CRB-V0-020: the /3 bundle carries
// corvint-ruby-test-provider as a tenth binary with its manifest row, SHA256SUMS
// line and usage smoke step, and a reader refuses any /3 inventory that lacks one
// of them, carries the provider under /2, or binds it to a different source.
func TestRubyProviderBundleProfileThree(t *testing.T) {
	t.Run("CRB-V0-020 ruby provider bundle profile", func(t *testing.T) {
		dir := rubyRetainedFixture(t, nil)
		v, err := VerifyRetainedBundle(dir)
		if err != nil {
			t.Fatal(err)
		}
		if string(v.Manifest.Profile) != `"corvint-companion-bundle/3"` || len(v.Manifest.Components) != 10 || len(v.SmokeSteps) != 34 {
			t.Fatalf("wrong /3 inventory: %s %d components %d steps", v.Manifest.Profile, len(v.Manifest.Components), len(v.SmokeSteps))
		}
		var sums string
		for _, e := range v.entries {
			if e.Path == "SHA256SUMS" {
				sums = string(e.Data)
			}
		}
		rubyDigest := sha256Hex([]byte("ruby provider fixture\n"))
		if !strings.Contains(sums, rubyDigest+"  bin/corvint-ruby-test-provider\n") {
			t.Fatalf("SHA256SUMS lacks the Ruby provider:\n%s", sums)
		}
		dest := filepath.Join(t.TempDir(), "extract")
		if err := v.Extract(dest); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(filepath.Join(dest, "bin", "corvint-ruby-test-provider"))
		if err != nil || info.Mode().Perm()&0o100 == 0 {
			t.Fatalf("extracted provider not executable: %v %v", info, err)
		}
	})
	t.Run("CRB-V0-020 ruby inventory refusals", func(t *testing.T) {
		withoutComponent := func(m map[string]json.RawMessage, e *[]ArchiveEntry) {
			var components []ComponentManifest
			if err := json.Unmarshal(m["components"], &components); err != nil {
				t.Fatal(err)
			}
			components = components[:len(components)-1]
			body, err := json.Marshal(components)
			if err != nil {
				t.Fatal(err)
			}
			m["components"] = body
		}
		for name, mutate := range map[string]func(map[string]json.RawMessage, *[]ArchiveEntry){
			"ruby under /2": func(m map[string]json.RawMessage, _ *[]ArchiveEntry) {
				m["profile"] = json.RawMessage(`"corvint-companion-bundle/2"`)
			},
			"unknown /4": func(m map[string]json.RawMessage, _ *[]ArchiveEntry) {
				m["profile"] = json.RawMessage(`"corvint-companion-bundle/4"`)
			},
			"missing manifest row": withoutComponent,
			"tampered binary": func(_ map[string]json.RawMessage, e *[]ArchiveEntry) {
				for j := range *e {
					if (*e)[j].Path == "bin/corvint-ruby-test-provider" {
						(*e)[j].Data = []byte("other bytes\n")
					}
				}
			},
			"missing binary": func(_ map[string]json.RawMessage, e *[]ArchiveEntry) {
				var kept []ArchiveEntry
				for _, entry := range *e {
					if entry.Path != "bin/corvint-ruby-test-provider" {
						kept = append(kept, entry)
					}
				}
				*e = kept
			},
		} {
			t.Run(name, func(t *testing.T) {
				if _, err := VerifyRetainedBundle(rubyRetainedFixture(t, mutate)); err == nil {
					t.Fatal("accepted invalid /3 inventory")
				}
			})
		}
		for name, edit := range map[string]func([]SmokeStep) []SmokeStep{
			"missing smoke step": func(steps []SmokeStep) []SmokeStep {
				var kept []SmokeStep
				for _, s := range steps {
					if s.Name != "corvint-ruby-test-provider-help" {
						kept = append(kept, s)
					}
				}
				return kept
			},
			"step bound to another binary": func(steps []SmokeStep) []SmokeStep {
				for j := range steps {
					if steps[j].Name == "corvint-ruby-test-provider-help" {
						steps[j].ComponentSHA256 = strings.Repeat("0", 64)
					}
				}
				return steps
			},
		} {
			t.Run(name, func(t *testing.T) {
				dir := rubyRetainedFixture(t, nil)
				rewriteFixtureSmoke(t, dir, edit)
				if _, err := VerifyRetainedBundle(dir); err == nil {
					t.Fatal("accepted invalid /3 smoke inventory")
				}
			})
		}
		dir := coreRetainedFixture(t, nil)
		rewriteFixtureSmoke(t, dir, func(steps []SmokeStep) []SmokeStep {
			return append(steps, SmokeStep{Name: "corvint-ruby-test-provider-help", OK: true, ComponentSHA256: steps[0].ComponentSHA256, SourceCommit: "c", SourceTree: "t", InvokedPath: "/x"})
		})
		if _, err := VerifyRetainedBundle(dir); err == nil {
			t.Fatal("/2 accepted a Ruby smoke step")
		}
	})
	t.Run("CRB-V0-020 installed identity closed per profile", func(t *testing.T) {
		digest := strings.Repeat("a", 64)
		nine := []string{"bin/corvint", "bin/corvint-console", "bin/corvint-dashboard-snapshot", "bin/corvint-docs-mcp", "bin/corvint-go-test-provider", "bin/corvint-js-test-provider", "bin/corvint-mcp", "bin/corvint-tasks", "bin/corvint-test-validity-mcp"}
		for name, binaries := range map[string][]string{
			"/2 nine":  nine,
			"/3 ten":   append(append([]string(nil), nine...), "bin/corvint-ruby-test-provider"),
			"extra":    append(append([]string(nil), nine...), "bin/other"),
			"no tasks": append(append([]string(nil), nine[:7]...), "bin/corvint-test-validity-mcp", "bin/corvint-ruby-test-provider"),
		} {
			t.Run(name, func(t *testing.T) {
				i := coreIdentity{Profile: "corvint-core-installed-identity/0", UnchangedAfter: true, ArchiveSHA256: digest, SourceCommit: strings.Repeat("b", 40), SourceTree: strings.Repeat("c", 40), TasksCommit: strings.Repeat("d", 40),
					ChecksumsSHA256: digest, SmokeSHA256: digest, NPMCacheSHA256: digest, BrowserCacheSHA256: digest, GoAuthoritySHA256: digest}
				i.Platform.OS, i.Platform.Arch = "darwin", "arm64"
				for _, b := range binaries {
					i.Binaries = append(i.Binaries, coreNamedDigest{Name: b, SHA256: digest})
				}
				err := validateCoreIdentity(i, CoreInstalledReport{BundleSHA256: digest, SourceCommit: i.SourceCommit, SourceTree: i.SourceTree})
				refused := err != nil && err.Error() == "core binary inventory differs"
				if want := name == "extra" || name == "no tasks"; refused != want {
					t.Fatalf("binary inventory refusal = %v (%v), want %v", refused, err, want)
				}
			})
		}
	})
}

// TestRubyProviderUsageAdmitsTheBuiltProvider builds cmd/corvint-ruby-test-provider
// and runs the /3 smoke's usage check on it, so a usage-line change fails here
// before the nightly companion-release gate (CRB-V0-020).
func TestRubyProviderUsageAdmitsTheBuiltProvider(t *testing.T) {
	repository, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	binary := filepath.Join(root, "corvint-ruby-test-provider")
	goPath, err := goBinary()
	if err != nil {
		t.Fatal(err)
	}
	if _, stderr, err := runCaptured(context.Background(), repository, closedGoEnv(filepath.Join(root, "go-home"), ""), buildTimeout,
		goPath, "build", "-trimpath", "-buildvcs=false", "-o", binary, "./cmd/corvint-ruby-test-provider"); err != nil {
		t.Fatalf("build provider: %v: %s", err, stderr)
	}
	scratch := filepath.Join(root, "scratch")
	if err := os.Mkdir(scratch, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := checkRubyProviderUsage(context.Background(), binary, "corvint-ruby-test-provider", scratch); err != nil {
		t.Fatal(err)
	}
}

func rewriteFixtureSmoke(t *testing.T, dir string, edit func([]SmokeStep) []SmokeStep) {
	t.Helper()
	path := filepath.Join(dir, "fixture.smoke.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var smoke struct {
		BundleSHA256 string      `json:"bundleSha256"`
		Steps        []SmokeStep `json:"steps"`
	}
	if err := json.Unmarshal(body, &smoke); err != nil {
		t.Fatal(err)
	}
	smoke.Steps = edit(smoke.Steps)
	if body, err = json.Marshal(smoke); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}
