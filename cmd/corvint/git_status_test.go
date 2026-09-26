//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStandaloneReadsRefuseGitFiltersWithoutMutation(t *testing.T) {
	t.Parallel()
	t.Run("EAF-V0-007", func(t *testing.T) {
		for _, driver := range []string{"clean", "process"} {
			t.Run(driver, func(t *testing.T) {
				root := taskContextRepository(t)
				marker := filepath.Join(root, "filter-executed")
				filter := ": > '" + strings.ReplaceAll(marker, "'", "'\"'\"'") + "'; cat"
				command := exec.Command("git", "-C", root, "config", "filter.hostile."+driver, filter)
				if data, err := command.CombinedOutput(); err != nil {
					t.Fatalf("configure filter: %v %s", err, data)
				}
				if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("*.go filter=hostile\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				source := filepath.Join(root, "cache", "demux.go")
				data, err := os.ReadFile(source)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(source, bytes.ReplaceAll(data, []byte("Split"), []byte("SpLit")), 0o600); err != nil {
					t.Fatal(err)
				}
				old := time.Unix(1700000000, 0)
				if err := os.Chtimes(source, old, old); err != nil {
					t.Fatal(err)
				}
				before := repositoryBytesDigest(t, root)
				for _, test := range []struct {
					args  []string
					input string
				}{
					{[]string{"query", "--task", "find Split", "--limit", "1"}, ""},
					{[]string{"context", "--task", "find Split", "--limit", "1"}, ""},
					{[]string{"feature", "split"}, ""},
					{[]string{"impact", "cache/demux.go"}, ""},
					{[]string{"affected"}, ""},
					{cliArguments(root, "session-start")[2:], `{}`},
					{cliArguments(root, "user-prompt")[2:], `{"task":"find Split"}`},
					{cliArguments(root, "file-change")[2:], `{"paths":["cache/demux.go"]}`},
				} {
					t.Run(strings.Join(test.args, " "), func(t *testing.T) {
						var stdout, stderr bytes.Buffer
						code := runContext(context.Background(), append([]string{"--root", root}, test.args...), strings.NewReader(test.input), &stdout, &stderr)
						var failure struct {
							Code  string `json:"code"`
							Error string `json:"error"`
						}
						if err := json.Unmarshal(stderr.Bytes(), &failure); err != nil {
							t.Fatalf("non-JSON refusal: %d %q %q", code, stdout.String(), stderr.String())
						}
						wantCode := "repository-probe-failed"
						wantError := failure.Error == "Git status cannot safely observe repository metadata: repository config sets filter.hostile."+driver
						if test.args[0] == "affected" {
							wantCode = "unsupported-affected-status"
							wantError = strings.Contains(failure.Error, "affected: worktree status is unavailable")
						}
						if code != 2 || stdout.Len() != 0 || failure.Code != wantCode || !wantError {
							t.Fatalf("unsafe standalone read: %d %q %q", code, stdout.String(), stderr.String())
						}
						if _, err := os.Stat(marker); !os.IsNotExist(err) {
							t.Fatalf("standalone read executed filter: %v", err)
						}
						if after := repositoryBytesDigest(t, root); before != after {
							t.Fatal("refused standalone read mutated repository bytes")
						}
					})
				}
				for _, activation := range []string{"init", "adopt"} {
					t.Run(activation, func(t *testing.T) {
						var stdout, stderr bytes.Buffer
						code := runContext(context.Background(), []string{"--root", root, activation, "--full-receipt"}, strings.NewReader(""), &stdout, &stderr)
						var receipt struct {
							Inventory struct {
								DirtyState string `json:"dirtyState"`
								Gaps       []struct {
									Code string `json:"code"`
								} `json:"gaps"`
							} `json:"inventory"`
						}
						if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
							t.Fatal(err)
						}
						readFailure := false
						for _, gap := range receipt.Inventory.Gaps {
							readFailure = readFailure || gap.Code == "git-read-failed"
						}
						if code != 0 || stderr.Len() != 0 || receipt.Inventory.DirtyState != "UNKNOWN" || !readFailure {
							t.Fatalf("activation lost its bounded status uncertainty: %d %s %s", code, &stdout, &stderr)
						}
						if _, err := os.Stat(marker); !os.IsNotExist(err) {
							t.Fatalf("activation executed filter: %v", err)
						}
						if after := repositoryBytesDigest(t, root); before != after {
							t.Fatal("activation mutated repository bytes")
						}
					})
				}
			})
		}
	})
}

func TestStandaloneReadNamesUnsupportedRepositoryFeature(t *testing.T) {
	t.Parallel()
	t.Run("EAF-V0-011", func(t *testing.T) {
		for _, test := range []struct{ name, reason string }{
			{"gitlink", "index records a submodule (gitlink)"},
			{"include", "repository config uses an include directive (include.*)"},
		} {
			t.Run(test.name, func(t *testing.T) {
				root := taskContextRepository(t)
				secret := filepath.Join(t.TempDir(), "credential-value")
				arguments := []string{"config", "include.path", secret}
				if test.name == "gitlink" {
					oid, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
					if err != nil {
						t.Fatal(err)
					}
					arguments = []string{"update-index", "--add", "--cacheinfo", "160000," + strings.TrimSpace(string(oid)) + ",child"}
				}
				if data, err := exec.Command("git", append([]string{"-C", root}, arguments...)...).CombinedOutput(); err != nil {
					t.Fatalf("configure %s: %v %s", test.name, err, data)
				}
				var stdout, stderr bytes.Buffer
				code := runContext(context.Background(), []string{"--root", root, "context", "--task", "find Split", "--limit", "1"}, strings.NewReader(""), &stdout, &stderr)
				var failure struct {
					Code  string `json:"code"`
					Error string `json:"error"`
				}
				if err := json.Unmarshal(stderr.Bytes(), &failure); err != nil {
					t.Fatalf("non-JSON refusal: %d %q %q", code, stdout.String(), stderr.String())
				}
				want := "Git status cannot safely observe repository metadata: " + test.reason
				if code != 2 || failure.Code != "repository-probe-failed" || failure.Error != want || strings.Contains(stderr.String(), "credential-value") {
					t.Fatalf("refusal did not name its feature: %d %q", code, stderr.String())
				}
			})
		}
	})
}
