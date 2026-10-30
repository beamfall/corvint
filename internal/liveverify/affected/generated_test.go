package affected

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AFP-V0-444: the generated-path declaration follows the documentation entry rules, and an
// absent declaration is no declaration rather than an error.
func TestReadGeneratedDeclarationAFPV0444(t *testing.T) {
	root := t.TempDir()
	if sum, entries, err := ReadGeneratedDeclaration(root); err != nil || sum != "" || entries != nil {
		t.Fatalf("absent declaration = %q %v %v", sum, entries, err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, GeneratedPathsPath), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	valid := `{"profile":"corvint-generated-paths/0","paths":["public/frontend/","vendor/app.min.js"]}`
	write(valid)
	sum, entries, err := ReadGeneratedDeclaration(root)
	want := sha256.Sum256([]byte(valid))
	if err != nil || sum != hex.EncodeToString(want[:]) || strings.Join(entries, ",") != "public/frontend/,vendor/app.min.js" {
		t.Fatalf("valid declaration = %q %v %v", sum, entries, err)
	}
	for _, invalid := range []string{
		`{"profile":"corvint-documentation-paths/0","paths":[]}`,
		`{"profile":"corvint-generated-paths/0"}`,
		`{"profile":"corvint-generated-paths/0","paths":["b","a"]}`,
		`{"profile":"corvint-generated-paths/0","paths":[".corvint/"]}`,
		`{"profile":"corvint-generated-paths/0","paths":["../x"]}`,
		`{"profile":"corvint-generated-paths/0","paths":[],"extra":1}`,
	} {
		write(invalid)
		if _, _, err := ReadGeneratedDeclaration(root); err == nil {
			t.Errorf("accepted invalid declaration %s", invalid)
		}
	}
}

// AFP-V0-444: an over-bound file gets a streamed content identity, and ReadSource's refusal names
// the fixed bound.
func TestHashOverBoundSourceAFPV0444(t *testing.T) {
	root := t.TempDir()
	body := strings.Repeat("x", MaxSourceBytes+1)
	if err := os.WriteFile(filepath.Join(root, "bundle.js"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadSource(root, "bundle.js")
	if !errors.Is(err, ErrWalkLimit) || !strings.Contains(err.Error(), "4194304-byte per-file source bound") {
		t.Fatalf("ReadSource over-bound error = %v", err)
	}
	sum, err := HashOverBoundSource(root, "bundle.js")
	want := sha256.Sum256([]byte(body))
	if err != nil || sum != hex.EncodeToString(want[:]) {
		t.Fatalf("HashOverBoundSource = %q %v", sum, err)
	}
	if err := os.Symlink("bundle.js", filepath.Join(root, "link.js")); err == nil {
		if _, err := HashOverBoundSource(root, "link.js"); !errors.Is(err, ErrInvalidUnit) {
			t.Fatalf("symlink hashed: %v", err)
		}
	}
	if !strings.Contains(SourceBoundRemedy, GeneratedPathsPath) || !strings.Contains(SourceBoundRemedy, "4194304") {
		t.Fatalf("remedy does not name the bound and the declaration: %s", SourceBoundRemedy)
	}
}
