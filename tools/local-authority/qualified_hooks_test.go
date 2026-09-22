package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func hookBundleFixture(t *testing.T) (string, string, releaseManifest) {
	t.Helper()
	base := t.TempDir()
	bundle := filepath.Join(base, "bundle")
	if err := os.Mkdir(bundle, 0700); err != nil {
		t.Fatal(err)
	}
	template, err := os.ReadFile("../../integrations/codex/plugins/corvint/scripts/authority-hook.json")
	if err != nil {
		t.Fatal(err)
	}
	consumer := []byte("test-only-consumer")
	m := releaseManifest{Profile: "corvint-authority-release/1", SourceRevision: strings.Repeat("a", 40), AdapterTemplateSHA256: digest(template), Files: []releaseFile{{"bin/git", strings.Repeat("b", 64), "0555"}, {"corvint", digest(consumer), "0555"}, {"go/bin/go", strings.Repeat("c", 64), "0555"}, {"local-authority", strings.Repeat("d", 64), "0555"}}}
	m.ReleaseID = sourceReleaseID(m)
	adapter, err := renderAdapter(template, "/Library/CorvintAuthority/versions/"+m.ReleaseID+"/corvint", digest(consumer))
	if err != nil {
		t.Fatal(err)
	}
	m.Files = append([]releaseFile{{"authority-hook.json", digest(adapter), "0444"}}, m.Files...)
	for name, raw := range map[string][]byte{"manifest.json": mustJSONLine(m), "corvint": consumer, "authority-hook.json": adapter} {
		if err := os.WriteFile(filepath.Join(bundle, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	hooks, err := os.ReadFile("../../integrations/codex/plugins/corvint/hooks/qualified-hooks.proposed.json")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(base, "template.json")
	if err := os.WriteFile(path, hooks, 0600); err != nil {
		t.Fatal(err)
	}
	return bundle, path, m
}
func TestQualifiedHooksPrepareExactSidecar(t *testing.T) {
	t.Run("QLF-V0-007 fixed four event packaging", func(t *testing.T) {
		bundle, template, m := hookBundleFixture(t)
		out := filepath.Join(t.TempDir(), "sidecar")
		if err := prepareQualifiedHooks(bundle, template, out); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join(out, "hooks.json"))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(raw, []byte("RELEASE_SHA256")) || bytes.Count(raw, []byte(m.ReleaseID)) != 4 || bytes.Count(raw, []byte("--qualified-lifecycle")) != 4 {
			t.Fatalf("incorrect routes %s", raw)
		}
		receipt, err := os.ReadFile(filepath.Join(out, "preparation.json"))
		if err != nil {
			t.Fatal(err)
		}
		var r qualifiedHooksReceipt
		if strictDecode(receipt, &r) != nil || r.Authority != "NONE" || r.HooksSHA256 != digest(raw) || r.ReleaseID != m.ReleaseID {
			t.Fatal("invalid sidecar receipt")
		}
		if prepareQualifiedHooks(bundle, template, out) == nil {
			t.Fatal("existing output overwritten")
		}
	})
}
func TestQualifiedHooksRejectTemplateAndBundleDrift(t *testing.T) {
	for _, mode := range []string{"consumer", "adapter", "pin", "manifest", "duplicate", "missing", "extra", "mode", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			bundle, template, _ := hookBundleFixture(t)
			out := filepath.Join(t.TempDir(), "sidecar")
			switch mode {
			case "consumer", "adapter":
				name := "corvint"
				if mode == "adapter" {
					name = "authority-hook.json"
				}
				os.WriteFile(filepath.Join(bundle, name), []byte("drift"), 0600)
			case "pin":
				raw, _ := os.ReadFile(filepath.Join(bundle, "authority-hook.json"))
				raw = bytes.Replace(raw, []byte(`/Library/CorvintAuthority/versions/`), []byte(`/wrong/`), 1)
				os.WriteFile(filepath.Join(bundle, "authority-hook.json"), raw, 0600)
				manifestRaw, _ := os.ReadFile(filepath.Join(bundle, "manifest.json"))
				var manifest releaseManifest
				if strictDecode(manifestRaw, &manifest) != nil {
					t.Fatal("fixture manifest")
				}
				for i := range manifest.Files {
					if manifest.Files[i].Path == "authority-hook.json" {
						manifest.Files[i].SHA256 = digest(raw)
					}
				}
				os.WriteFile(filepath.Join(bundle, "manifest.json"), mustJSONLine(manifest), 0600)
			case "manifest":
				raw, _ := os.ReadFile(filepath.Join(bundle, "manifest.json"))
				os.WriteFile(filepath.Join(bundle, "manifest.json"), append(raw, ' '), 0600)
			case "symlink":
				os.Remove(filepath.Join(bundle, "corvint"))
				os.Symlink(template, filepath.Join(bundle, "corvint"))
			default:
				raw, _ := os.ReadFile(template)
				switch mode {
				case "duplicate":
					raw = bytes.Replace(raw, []byte(`"timeout": 2`), []byte(`"timeout": 2, "timeout": 2`), 1)
				case "missing":
					raw = bytes.Replace(raw, []byte(`"SessionEnd"`), []byte(`"Unknown"`), 1)
				case "extra":
					raw = bytes.Replace(raw, []byte(`"type": "command"`), []byte(`"type": "command", "extra": true`), 1)
				case "mode":
					raw = bytes.Replace(raw, []byte("--qualified-lifecycle"), []byte("--other"), 1)
				}
				os.WriteFile(template, raw, 0600)
			}
			if prepareQualifiedHooks(bundle, template, out) == nil {
				t.Fatal("invalid input accepted")
			}
			if _, err := os.Lstat(out); !os.IsNotExist(err) {
				t.Fatal("invalid input left sidecar")
			}
		})
	}
}

func TestQualifiedHooksPartialFailureLeavesNoReceipt(t *testing.T) {
	out := filepath.Join(t.TempDir(), "sidecar")
	err := writeQualifiedHooksSidecar(out, []byte("hooks"), []byte("receipt"), func() error {
		if _, err := os.Stat(filepath.Join(out, "hooks.json")); err != nil {
			t.Fatal("did not exercise partial publication")
		}
		return errors.New("test-only input drift")
	})
	if err == nil {
		t.Fatal("partial success")
	}
	if _, err := os.Lstat(out); !os.IsNotExist(err) {
		t.Fatal("partial sidecar survived")
	}
}

func TestQualifiedDirectHooksExactExecRoute(t *testing.T) {
	bundle, _, manifest := hookBundleFixture(t)
	raw, err := os.ReadFile("../../integrations/codex/plugins/corvint/hooks/qualified-direct-hooks.proposed.json")
	if err != nil {
		t.Fatal(err)
	}
	template := filepath.Join(t.TempDir(), "direct.json")
	if err = os.WriteFile(template, raw, 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "sidecar")
	if err = prepareQualifiedHooksProfile(bundle, template, out, true); err != nil {
		t.Fatal(err)
	}
	rendered, err := os.ReadFile(filepath.Join(out, "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(rendered, []byte("exec /Library/CorvintAuthority/versions/"+manifest.ReleaseID)) != 4 || bytes.Count(rendered, []byte("--qualified-direct-lifecycle")) != 4 {
		t.Fatal("direct hooks lost exec/version boundary")
	}
	if _, err = renderQualifiedHooks(raw, manifest.ReleaseID); err == nil {
		t.Fatal("legacy preparation accepted direct template")
	}
	bad := bytes.ReplaceAll(raw, []byte("exec /Library/"), []byte("/Library/"))
	if _, err = renderQualifiedHooksCommand(bad, manifest.ReleaseID, directQualifiedHookCommand); err == nil {
		t.Fatal("non-exec direct template accepted")
	}
}
