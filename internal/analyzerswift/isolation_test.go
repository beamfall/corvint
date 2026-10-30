package analyzerswift

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeHasNoAmbientWriteOrEnvironmentDependency(t *testing.T) {
	directory := t.TempDir()
	home := filepath.Join(directory, "home")
	cwd := filepath.Join(directory, "cwd")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(cwd, 0o700); err != nil {
		t.Fatal(err)
	}
	request := fixtureRequest(t, "ambient-isolation")
	wire := requestBytes(t, request)
	before := snapshotDirectory(t, directory)
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	t.Setenv("HOME", home)
	t.Setenv("PATH", filepath.Join(directory, "no-tools"))
	got := Analyze(bytes.NewReader(wire))
	if !bytes.Contains(got, []byte(`"status":"CANDIDATE"`)) {
		t.Fatalf("result=%s", got)
	}
	if after := snapshotDirectory(t, directory); !sameSnapshot(before, after) {
		t.Fatalf("ambient mutation before=%v after=%v", before, after)
	}
	control := filepath.Join(home, "control")
	if err := os.WriteFile(control, []byte("detected"), 0o600); err != nil {
		t.Fatal(err)
	}
	if sameSnapshot(before, snapshotDirectory(t, directory)) {
		t.Fatal("ambient-write spy failed its hard negative control")
	}
}

func snapshotDirectory(t testing.TB, directory string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == directory {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		result[path] = info.Mode().String() + ":" + info.ModTime().UTC().String()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func sameSnapshot(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for path, value := range left {
		if right[path] != value {
			return false
		}
	}
	return true
}
