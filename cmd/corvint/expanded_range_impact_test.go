package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandedRangeImpactCLIExplicitReadOnly(t *testing.T) {
	t.Parallel()
	t.Run("ERI-V0-005 explicit-read-only", func(t *testing.T) {
		root := impactCLIRepository(t)
		base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
		for index := 0; index < 101; index++ {
			if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("capacity-%03d.txt", index)), []byte(fmt.Sprintf("range member %d\n", index)), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		affectedGit(t, root, "add", ".")
		affectedGit(t, root, "commit", "-qm", "expanded range target")
		before := repositoryBytesDigest(t, root)
		reader := &forbiddenImpactReader{}
		var stdout, stderr bytes.Buffer
		exit := run([]string{"--root", root, "impact", "--base", base, "--range-profile", "expanded-256", "--limit", "20"}, reader, &stdout, &stderr)
		if exit != 0 || stderr.Len() != 0 || reader.reads != 0 {
			t.Fatalf("exit=%d stdout=%s stderr=%s reads=%d", exit, &stdout, &stderr, reader.reads)
		}
		var envelope struct {
			Context struct {
				Profile string `json:"profile"`
				Range   struct {
					ChangedPathCount int `json:"changedPathCount"`
				} `json:"range"`
				Omissions struct {
					Count int `json:"count"`
				} `json:"omissions"`
			} `json:"context"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Context.Profile != "corvint-range-impact-expanded/experimental" || envelope.Context.Range.ChangedPathCount != 101 || envelope.Context.Omissions.Count != 101 {
			t.Fatalf("incomplete or unlabelled experimental receipt: %s", &stdout)
		}
		if after := repositoryBytesDigest(t, root); after != before {
			t.Fatal("expanded range success mutated repository bytes")
		}
		stdout.Reset()
		stderr.Reset()
		exit = run([]string{"--root", root, "impact", "--base", base}, reader, &stdout, &stderr)
		if exit != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), `"code": "unsupported-impact-range"`) || !strings.Contains(stderr.String(), "100-path bound") {
			t.Fatalf("default range silently widened: exit=%d stdout=%s stderr=%s", exit, &stdout, &stderr)
		}
	})
}

func TestExpandedRangeImpactCLIClosedSelection(t *testing.T) {
	t.Parallel()
	t.Run("ERI-V0-001 closed-profile-selection", func(t *testing.T) {
		root := impactCLIRepository(t)
		base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
		for _, arguments := range [][]string{
			{"--range-profile"},
			{"--range-profile="},
			{"--range-profile", "256", "--base", base},
			{"--range-profile", "expanded-800", "--base", base},
			{"--range-profile", "expanded-256"},
			{"--range-profile", "expanded-256", "pkg/main.go"},
			{"--range-profile", "expanded-256", "--working-tree-untracked", "pkg/main.go"},
			{"--base", base, "--range-profile", "expanded-256", "pkg/main.go"},
			{"--base", base, "--range-profile", "expanded-256", "--working-tree-untracked"},
			{"--base", base, "--range-profile", "expanded-256", "--range-profile=expanded-256"},
		} {
			var stdout, stderr bytes.Buffer
			reader := &forbiddenImpactReader{}
			argv := append([]string{"--root", root, "impact"}, arguments...)
			if exit := run(argv, reader, &stdout, &stderr); exit != 2 || stdout.Len() != 0 || reader.reads != 0 || !strings.Contains(stderr.String(), `"code": "invalid-arguments"`) {
				t.Fatalf("args=%v exit=%d stdout=%s stderr=%s reads=%d", arguments, exit, &stdout, &stderr, reader.reads)
			}
		}
		var stdout, stderr bytes.Buffer
		if exit := run([]string{"--root", root, "impact", "--range-profile=expanded-256", "--base", base}, &forbiddenImpactReader{}, &stdout, &stderr); exit != 0 || stderr.Len() != 0 || !bytes.Contains(stdout.Bytes(), []byte(`"rangeProfile":"expanded-256"`)) {
			t.Fatalf("explicit inline selector failed: exit=%d stdout=%s stderr=%s", exit, &stdout, &stderr)
		}
	})
}

func TestExpandedRangeImpactProveRejectsSelector(t *testing.T) {
	t.Parallel()
	t.Run("ERI-V0-004 prove-retains-default-profile", func(t *testing.T) {
		root := impactCLIRepository(t)
		base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
		before := repositoryBytesDigest(t, root)
		for _, selector := range [][]string{{"--range-profile", "expanded-256"}, {"--range-profile=expanded-256"}} {
			var stdout, stderr bytes.Buffer
			reader := &forbiddenImpactReader{}
			arguments := append([]string{"--root", root, "prove", "--base", base}, selector...)
			if exit := run(arguments, reader, &stdout, &stderr); exit != 2 || stdout.Len() != 0 || reader.reads != 0 || !strings.Contains(stderr.String(), `"code": "invalid-arguments"`) || !strings.Contains(stderr.String(), "prove does not support --range-profile") {
				t.Fatalf("selector=%v exit=%d stdout=%s stderr=%s reads=%d", selector, exit, &stdout, &stderr, reader.reads)
			}
		}
		if after := repositoryBytesDigest(t, root); after != before {
			t.Fatal("rejected prove selector mutated repository bytes")
		}
	})
}
