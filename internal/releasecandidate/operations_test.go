package releasecandidate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func canonicalTemp(t *testing.T) string {
	t.Helper()
	name, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return name
}

func installFixture(t *testing.T) (string, *VerifiedCandidate) {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("POSIX installer")
	}
	candidate := canonicalTemp(t)
	verified := &VerifiedCandidate{Manifest: Manifest{Version: "0.5.0a1", CorvintVersion: "Corvint 0.5.0a1 (build 9)"}, files: map[string][]byte{}}
	setFixtureProgram(t, verified, "#!/bin/sh\nprintf '%s\\n' 'Corvint 0.5.0a1 (build 9)'\n")
	previous := verifyForInstall
	verifyForInstall = func(context.Context, string) (*VerifiedCandidate, error) { return verified, nil }
	t.Cleanup(func() { verifyForInstall = previous })
	return candidate, verified
}

func setFixtureProgram(t *testing.T, verified *VerifiedCandidate, program string) {
	t.Helper()
	verified.files["core/corvint_"+runtime.GOOS+"_"+runtime.GOARCH+".tar.gz"] = coreScriptArchive(t, runtime.GOOS+"_"+runtime.GOARCH, program)
}

func TestPUBV0025RecoveryLifecycle(t *testing.T) {
	t.Run("PUB-V0-025 upgrade rollback backup reinstall and scoped removal", func(t *testing.T) {
		candidate, verified := installFixture(t)
		store := filepath.Join(canonicalTemp(t), "store")
		old, err := InstallCore(t.Context(), candidate, store)
		if err != nil {
			t.Fatal(err)
		}
		oldBytes, err := os.ReadFile(filepath.Join(old, "corvint"))
		if err != nil {
			t.Fatal(err)
		}
		// A retained archive is the backup; no installed state is repaired in place.
		backup := append([]byte(nil), verified.files["core/corvint_"+runtime.GOOS+"_"+runtime.GOARCH+".tar.gz"]...)
		verified.Manifest.Version, verified.Manifest.CorvintVersion = "0.5.0a2", "Corvint 0.5.0a2 (build 10)"
		setFixtureProgram(t, verified, "#!/bin/sh\nprintf '%s\\n' 'Corvint 0.5.0a2 (build 10)'\n")
		newPath, err := InstallCore(t.Context(), candidate, store)
		if err != nil {
			t.Fatal(err)
		}
		for path, expected := range map[string]string{old: "Corvint 0.5.0a1 (build 9)", newPath: verified.Manifest.CorvintVersion} {
			if err := probeCoreVersion(t.Context(), filepath.Join(path, "corvint"), path, expected); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(newPath, "corvint"), []byte("corrupted"), 0700); err != nil {
			t.Fatal(err)
		}
		if _, err := InstallCore(t.Context(), candidate, store); err == nil {
			t.Fatal("overwrote corrupted install")
		}
		if raw, err := os.ReadFile(filepath.Join(old, "corvint")); err != nil || string(raw) != string(oldBytes) {
			t.Fatalf("rollback bytes changed: %v", err)
		}
		// These removals are deliberately confined to test-owned temporary paths.
		if err := os.RemoveAll(newPath); err != nil {
			t.Fatal(err)
		}
		verified.Manifest.Version, verified.Manifest.CorvintVersion = "0.5.0a1", "Corvint 0.5.0a1 (build 9)"
		verified.files["core/corvint_"+runtime.GOOS+"_"+runtime.GOARCH+".tar.gz"] = backup
		restored, err := InstallCore(t.Context(), candidate, filepath.Join(canonicalTemp(t), "restore"))
		if err != nil {
			t.Fatal(err)
		}
		if raw, err := os.ReadFile(filepath.Join(restored, "corvint")); err != nil || string(raw) != string(oldBytes) {
			t.Fatalf("restore differs: %v", err)
		}
		if _, err := os.Stat(candidate); err != nil {
			t.Fatal("retained candidate removed", err)
		}
		if _, err := os.Stat(old); err != nil {
			t.Fatal("old version removed", err)
		}
		for _, selector := range []string{"current", "latest"} {
			if _, err := os.Lstat(filepath.Join(store, selector)); !os.IsNotExist(err) {
				t.Fatal("selector created")
			}
		}
	})
}

func TestPUBV0025HostileStore(t *testing.T) {
	t.Run("PUB-V0-025 static symlink aliases and overlaps refused before effects", func(t *testing.T) {
		candidate, _ := installFixture(t)
		for _, test := range []string{"store-link", "ancestor-link", "version-link", "case-alias", "platform-alias", "inside-candidate", "contains-candidate", "unclean", "file"} {
			t.Run(test, func(t *testing.T) {
				root, outside := canonicalTemp(t), canonicalTemp(t)
				store := filepath.Join(root, "store")
				mustMkdir := func(p string) {
					t.Helper()
					if err := os.MkdirAll(p, 0700); err != nil {
						t.Fatal(err)
					}
				}
				mustLink := func(target, p string) {
					t.Helper()
					if err := os.Symlink(target, p); err != nil {
						t.Fatal(err)
					}
				}
				switch test {
				case "store-link":
					mustLink(outside, store)
				case "ancestor-link":
					mustLink(outside, filepath.Join(root, "link"))
					store = filepath.Join(root, "link", "store")
				case "version-link":
					mustMkdir(filepath.Join(store, "corvint"))
					mustLink(outside, filepath.Join(store, "corvint", "0.5.0a1"))
				case "case-alias":
					mustMkdir(filepath.Join(store, "Corvint"))
				case "platform-alias":
					mustMkdir(filepath.Join(store, "corvint", "0.5.0a1", strings.ToUpper(runtime.GOOS+"-"+runtime.GOARCH)))
				case "inside-candidate":
					store = filepath.Join(candidate, "store")
				case "contains-candidate":
					store = filepath.Dir(candidate)
				case "unclean":
					store = root + "/./store"
				case "file":
					if err := os.WriteFile(store, []byte("preserve"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := InstallCore(t.Context(), candidate, store); err == nil {
					t.Fatal("hostile store accepted")
				}
				entries, err := os.ReadDir(outside)
				if err != nil || len(entries) != 0 {
					t.Fatalf("outside modified: %v %v", entries, err)
				}
			})
		}
	})
}

func TestPUBV0026ProbeFailureCleansInstall(t *testing.T) {
	t.Run("PUB-V0-026 version probe failures preserve old installations", func(t *testing.T) {
		for _, program := range []string{"#!/bin/sh\nprintf '%s\\n' 'Corvint 0.5.0a1 (build 9)'\nexit 1\n", "#!/bin/sh\nprintf 'private-secret-value' >&2\nexit 1\n", "#!/bin/sh\nprintf wrong\n", "#!/bin/sh\nwhile :; do printf 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'; done\n"} {
			candidate, verified := installFixture(t)
			store := filepath.Join(canonicalTemp(t), "store")
			old, err := InstallCore(t.Context(), candidate, store)
			if err != nil {
				t.Fatal(err)
			}
			oldBytes, err := os.ReadFile(filepath.Join(old, "corvint"))
			if err != nil {
				t.Fatal(err)
			}
			verified.Manifest.Version = "0.5.0a2"
			setFixtureProgram(t, verified, program)
			if _, err := InstallCore(t.Context(), candidate, store); err == nil || strings.Contains(err.Error(), "private-secret-value") {
				t.Fatalf("probe result: %v", err)
			}
			if raw, err := os.ReadFile(filepath.Join(old, "corvint")); err != nil || string(raw) != string(oldBytes) {
				t.Fatalf("old install changed: %v", err)
			}
			entries, err := os.ReadDir(filepath.Join(store, "corvint", verified.Manifest.Version))
			if err != nil || len(entries) != 0 {
				t.Fatalf("retained failed stage: %v %v", entries, err)
			}
		}
	})
}

func TestPUBV0026CandidateInventoryBounds(t *testing.T) {
	t.Run("PUB-V0-026 bounded inventory before materialization", func(t *testing.T) {
		for _, kind := range []string{"files", "directories", "depth", "sparse", "symlink", "cancelled"} {
			t.Run(kind, func(t *testing.T) {
				root := canonicalTemp(t)
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				switch kind {
				case "files":
					for i := 0; i < 100; i++ {
						if err := os.WriteFile(filepath.Join(root, fmt.Sprint(i)), nil, 0600); err != nil {
							t.Fatal(err)
						}
					}
				case "directories":
					for i := 0; i < 5; i++ {
						if err := os.Mkdir(filepath.Join(root, fmt.Sprint(i)), 0700); err != nil {
							t.Fatal(err)
						}
					}
				case "depth":
					if err := os.MkdirAll(filepath.Join(root, "a", "b"), 0700); err != nil {
						t.Fatal(err)
					}
				case "sparse":
					f, err := os.Create(filepath.Join(root, "large"))
					if err != nil {
						t.Fatal(err)
					}
					err = f.Truncate(maxInputBytes + 1)
					f.Close()
					if err != nil {
						t.Fatal(err)
					}
				case "symlink":
					if err := os.Symlink(canonicalTemp(t), filepath.Join(root, "link")); err != nil {
						t.Fatal(err)
					}
				case "cancelled":
					cancel()
				}
				if _, _, err := readCandidateFiles(ctx, root); err == nil {
					t.Fatal("unbounded inventory accepted")
				}
			})
		}
	})
}

func TestPUBV0026CancelledInstallRetainsNothing(t *testing.T) {
	t.Run("PUB-V0-026 cancellation before verification has no effects", func(t *testing.T) {
		candidate, _ := installFixture(t)
		store := filepath.Join(canonicalTemp(t), "store")
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := InstallCore(ctx, candidate, store); !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel: %v", err)
		}
		if _, err := os.Lstat(store); !os.IsNotExist(err) {
			t.Fatalf("created cancelled store: %v", err)
		}
	})
}

// Used only by the POSIX interruption fixture.
func assertFixtureProcessGone(t *testing.T, pid string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/bin/ps", "-p", pid, "-o", "stat=").Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
			t.Fatalf("cannot observe descendant: %v", err)
		}
	}
	if err == nil && strings.TrimSpace(string(out)) != "" && !strings.HasPrefix(strings.TrimSpace(string(out)), "Z") {
		t.Fatalf("descendant %s survives: %s", pid, out)
	}
}
