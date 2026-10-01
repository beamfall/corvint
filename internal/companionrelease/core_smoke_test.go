package companionrelease

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPUBV0024InstalledCoreDiscoveryWorkflows(t *testing.T) {
	t.Run("PUB-V0-024 installed core discovery workflows", testPUBV0024InstalledCoreDiscoveryWorkflows)
}

func testPUBV0024InstalledCoreDiscoveryWorkflows(t *testing.T) {
	repository, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	binary := filepath.Join(root, "corvint")
	goPath, err := goBinary()
	if err != nil {
		t.Fatal(err)
	}
	if _, stderr, err := runCaptured(context.Background(), repository, closedGoEnv(filepath.Join(root, "go-home"), ""), buildTimeout,
		goPath, "build", "-trimpath", "-buildvcs=false", "-ldflags=-X main.build=1", "-o", binary, "./cmd/corvint"); err != nil {
		t.Fatalf("build installed fixture: %v: %s", err, stderr)
	}
	steps, err := checkCoreDiscoveryWorkflows(context.Background(), binary, filepath.Join(root, "scratch"))
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 5 {
		t.Fatalf("got %d steps, want 5", len(steps))
	}
	for _, observed := range steps {
		if !observed.OK {
			t.Fatalf("%s failed: %s", observed.Name, observed.Detail)
		}
	}
	if _, err := os.Stat(binary); err != nil {
		t.Fatal(err)
	}
}

func TestPUBV0024DocumentationCorpusSearchProfiles(t *testing.T) {
	t.Run("PUB-V0-024 supported search receipts and negative controls", func(t *testing.T) {
		cases := []struct {
			name, raw string
			wantError bool
		}{
			{"legacy", `{"schema":"corvint-corpus-receipt/1","operation":"search","results":[{"id":"release"}]}`, false},
			{"current", `{"schema":"corvint-corpus-receipt/2","operation":"search","results":[{"id":"release"}]}`, false},
			{"unsupported profile", `{"schema":"corvint-corpus-receipt/3","operation":"search","results":[{"id":"release"}]}`, true},
			{"wrong operation", `{"schema":"corvint-corpus-receipt/2","operation":"get","results":[{"id":"release"}]}`, true},
			{"empty results", `{"schema":"corvint-corpus-receipt/2","operation":"search","results":[]}`, true},
			{"malformed JSON", `{`, true},
			{"trailing JSON", `{"schema":"corvint-corpus-receipt/2","operation":"search","results":[{}]} {}`, true},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				err := validateDocumentationCorpusSearch([]byte(tc.raw))
				if (err != nil) != tc.wantError {
					t.Fatalf("validation error = %v, wantError = %v", err, tc.wantError)
				}
			})
		}
	})
}
