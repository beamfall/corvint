package roadmap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDocStateResolvesKnownEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docs-state.json")
	content := `{"IPR-10": {"path":"docs/generated/ipr-10.md","sha256":"abc123","state":"READY"}}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	state := LoadDocState(path, "IPR-10")
	if state.State != "READY" || state.Path != "docs/generated/ipr-10.md" || state.SHA256 != "abc123" {
		t.Fatalf("state = %+v", state)
	}
}

func TestLoadDocStateNotObservedWithoutPath(t *testing.T) {
	state := LoadDocState("", "IPR-10")
	if state.State != notObserved || state.Reason == "" {
		t.Fatalf("state = %+v", state)
	}
}

func TestLoadDocStateNotObservedForMissingEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docs-state.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	state := LoadDocState(path, "IPR-10")
	if state.State != notObserved || state.Reason == "" {
		t.Fatalf("state = %+v", state)
	}
}

func TestLoadDocStateNotObservedOnMalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docs-state.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	state := LoadDocState(path, "IPR-10")
	if state.State != notObserved || state.Reason == "" {
		t.Fatalf("state = %+v", state)
	}
}
