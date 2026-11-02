package behaviorfalsify

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/execfile"
)

// BBF-V0-014: actual staged hook observes safe transport; arbitrary standalone
// inheritance cannot add TMPDIR to the previously closed hook environment.
func TestBBFV0014TempRootTransport(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(root, "hook")
	if err := execfile.WriteFile(hook, []byte("#!/bin/sh\n[ -z \"$GITHUB_TOKEN\" ] && [ -z \"$NODE_OPTIONS\" ] || exit 7\nprintf '%s\\n%s\\n' \"$TMPDIR\" \"$0\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(hook)
	if err != nil {
		t.Fatal(err)
	}
	command := Command{Path: hook, ExecutableSHA256: digestBytes(data)}
	saved := os.Environ()
	os.Clearenv()
	defer func() {
		os.Clearenv()
		for _, entry := range saved {
			p := strings.SplitN(entry, "=", 2)
			os.Setenv(p[0], p[1])
		}
	}()
	for key, value := range map[string]string{"PATH": "/opt/homebrew/bin:/usr/bin:/bin", "LANG": "C", "LC_ALL": "C", "TMPDIR": root} {
		os.Setenv(key, value)
	}
	output, evidence := runPinned(context.Background(), command, root, nil, "control", nil, 3*time.Second, 1024)
	paths := strings.Split(strings.TrimSpace(string(output)), "\n")
	if !processSucceeded(evidence) || len(paths) != 2 || paths[0] != root || filepath.Dir(filepath.Dir(paths[1])) != root || !strings.HasPrefix(filepath.Base(filepath.Dir(paths[1])), "corvint-behavior-control-") {
		t.Fatalf("escaped hook: %q %+v", output, evidence)
	}
	if _, err := os.Stat(filepath.Dir(paths[1])); !os.IsNotExist(err) {
		t.Fatalf("staging remains: %v", err)
	}
	os.Setenv("TMPDIR", "/tmp") // fixed-tmp: asserts the default environment ignores TMPDIR; nothing is written
	if slices.Contains(commandEnvironment(root, nil, "control"), "TMPDIR=/tmp") {
		t.Fatal("default hook environment changed")
	}
	os.Setenv("TMPDIR", root)
	os.Setenv("NODE_OPTIONS", "sentinel")
	if slices.Contains(commandEnvironment(root, nil, "control"), "TMPDIR="+root) {
		t.Fatal("arbitrary inheritance admitted")
	}
}

// BBF-V0-014: cancellation is injected only after the staged hook actually
// creates its marker; a fixture/start failure cannot masquerade as cleanup proof.
func TestBBFV0014TempRootInterruptedStaging(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(root, "hook")
	script := "#!/bin/sh\nprintf '%s' \"$0\" > \"$TMPDIR/started\"\ntrap '' TERM\nsleep 30\n"
	if err := execfile.WriteFile(hook, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	saved := os.Environ()
	os.Clearenv()
	defer func() {
		os.Clearenv()
		for _, entry := range saved {
			p := strings.SplitN(entry, "=", 2)
			os.Setenv(p[0], p[1])
		}
	}()
	for key, value := range map[string]string{"PATH": "/opt/homebrew/bin:/usr/bin:/bin", "LANG": "C", "LC_ALL": "C", "TMPDIR": root} {
		os.Setenv(key, value)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan ProcessEvidence, 1)
	go func() {
		_, e := runPinned(ctx, Command{Path: hook, ExecutableSHA256: digestBytes([]byte(script))}, root, nil, "control", nil, 5*time.Second, 1024)
		done <- e
	}()
	var staged []byte
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		staged, err = os.ReadFile(filepath.Join(root, "started"))
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	evidence := <-done
	if len(staged) == 0 {
		t.Fatalf("staged fixture not reached: %+v", evidence)
	}
	if filepath.Dir(filepath.Dir(string(staged))) != root {
		t.Fatalf("escaped staging: %q", staged)
	}
	if !evidence.Cancelled || !evidence.Completed || !evidence.OwnedCleanup || !evidence.DescendantsGone {
		t.Fatalf("interruption not retired: %+v", evidence)
	}
	if _, err := os.Stat(filepath.Dir(string(staged))); !os.IsNotExist(err) {
		t.Fatalf("staging residue: %v", err)
	}
}
