package gateselect

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// memoryScanMemo keeps the memo body in memory and stamps a file by its size
// and modification time alone, so a chmod leaves the stamp in place.
func memoryScanMemo(identity string, body *[]byte) *ScanMemo {
	return &ScanMemo{
		Identity: identity,
		Load:     func() []byte { return *body },
		Store:    func(stored []byte) { *body = stored },
		Stamp: func(info fs.FileInfo) (string, bool) {
			return fmt.Sprintf("%d %d", info.Size(), info.ModTime().UnixNano()), true
		},
	}
}

// TestAFPV0822ScanMemoIndexMatchesTheFullScan: an index through a cold or a
// warm memo equals the full scan; a warm memo reads no pinned file, and a
// changed file, another program or another module is scanned again.
func TestAFPV0822ScanMemoIndexMatchesTheFullScan(t *testing.T) {
	root := writeFixture(t, parallelFixture())
	full, err := indexRepository(root, fixtureModule)
	if err != nil {
		t.Fatal(err)
	}
	var body []byte
	cold, err := indexRepositoryMemo(root, fixtureModule, memoryScanMemo("program", &body))
	if err != nil || !reflect.DeepEqual(full, cold) || body == nil {
		t.Fatalf("cold memo index differs or stored nothing: err %v, stored %d bytes", err, len(body))
	}
	stored := body
	// An unreadable file proves the warm index recalls it rather than reading.
	unreadable := filepath.Join(root, "p07", "p_test.go")
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatal(err)
	}
	warm, err := indexRepositoryMemo(root, fixtureModule, memoryScanMemo("program", &body))
	if err != nil || !reflect.DeepEqual(full, warm) {
		t.Fatalf("warm memo index differs from the full scan: err %v", err)
	}
	if &body[0] != &stored[0] {
		t.Fatal("an index that recalled every file rewrote the memo")
	}
	if _, err := indexRepositoryMemo(root, fixtureModule, memoryScanMemo("another program", &body)); err == nil {
		t.Fatal("a memo of another program was trusted")
	}
	if _, err := indexRepositoryMemo(root, fixtureModule+"/other", memoryScanMemo("program", &body)); err == nil {
		t.Fatal("a memo of another module was trusted")
	}
	if err := os.Chmod(unreadable, 0o644); err != nil {
		t.Fatal(err)
	}
	// A changed file is scanned again and the memo is rewritten.
	changed := filepath.Join(root, "p03", "p.go")
	if err := os.WriteFile(changed, []byte("package p03\n\nimport \"os\"\n\nvar _, _ = os.Getwd()\nvar a = \"docs/specs/changed-name.md\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	full, err = indexRepository(root, fixtureModule)
	if err != nil {
		t.Fatal(err)
	}
	rescanned, err := indexRepositoryMemo(root, fixtureModule, memoryScanMemo("program", &body))
	if err != nil || !reflect.DeepEqual(full, rescanned) {
		t.Fatalf("index after a change differs from the full scan: err %v", err)
	}
	if reflect.DeepEqual(body, stored) {
		t.Fatal("the memo was not rewritten after a change")
	}
}

// TestAFPV0822UnpinnedFilesAreNeverRecalled: a file the stamp cannot pin is
// read on every index and is not stored.
func TestAFPV0822UnpinnedFilesAreNeverRecalled(t *testing.T) {
	root := writeFixture(t, parallelFixture())
	var body []byte
	memo := memoryScanMemo("program", &body)
	pin := memo.Stamp
	memo.Stamp = func(info fs.FileInfo) (string, bool) {
		if info.Name() == "p_test.go" {
			return "", false
		}
		return pin(info)
	}
	if _, err := indexRepositoryMemo(root, fixtureModule, memo); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "p07", "p_test.go"), 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, "p07", "p_test.go"), 0o644) })
	if _, err := indexRepositoryMemo(root, fixtureModule, memo); err == nil {
		t.Fatal("an unpinned file was recalled instead of read")
	}
}
