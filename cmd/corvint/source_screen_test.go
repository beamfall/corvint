package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// The secret-bearing fixture lines are assembled at run time so this file
// itself carries no screen match.
var (
	screenFixtureSecret = "correct-horse-battery-" + "staple"
	screenFixtureSource = "package auth\n" +
		"var pass" + "word = \"" + screenFixtureSecret + "\"\r\n" +
		"// clean line\n" +
		"-----BEGIN RSA " + "PRIVATE KEY-----\n" +
		"MIIEpAIBAAKCAQEA3Tz2mr7SZiAMfQyuvBjM9OiJjRazXBZ1BjP5CE\n" +
		"-----END RSA " + "PRIVATE KEY-----\n" +
		"func Done() {}\n"
)

// ESV-V0-012: every line a secret-screen match touches, including the clean
// looking body of a multi-line key block, becomes the placeholder with its
// line ending kept; other lines stay byte-exact.
func TestESVV0012ScreenSourceLinesRedactsSecretLines(t *testing.T) {
	t.Parallel()
	source := []byte(screenFixtureSource)
	lines := bytes.SplitAfter(source, []byte{'\n'})
	lines = lines[:len(lines)-1]
	text, redacted := screenSourceLines(source, lines, 2, 7)
	want := secretscreen.Placeholder + "\r\n// clean line\n" + strings.Repeat(secretscreen.Placeholder+"\n", 3) + "func Done() {}\n"
	if text != want || !reflect.DeepEqual(redacted, []int{2, 4, 5, 6}) {
		t.Fatalf("screened = %q %v, want %q", text, redacted, want)
	}
	if text, redacted := screenSourceLines(source, lines, 3, 3); text != "// clean line\n" || redacted != nil {
		t.Fatalf("clean range = %q %v", text, redacted)
	}
}

func TestESVV0012SourceViewRedactsSecretLines(t *testing.T) {
	t.Parallel()
	view, err := extractSourceView([]byte(screenFixtureSource), nil, "1:3", "", 4096)
	if err != nil {
		t.Fatal(err)
	}
	text := view["text"].(string)
	if strings.Contains(text, screenFixtureSecret) || !strings.Contains(text, secretscreen.Placeholder) || !reflect.DeepEqual(view["redacted_lines"], []int{2}) {
		t.Fatalf("source view = %v", view)
	}
}

// ESV-V0-012 through the CLI: `context --expand` never prints a secret line,
// discloses which lines it replaced, and its sha256 still names the exact
// pinned bytes; the context packet carries no file text at all.
func TestESVV0012ContextExpandAndPacketScreenSecretLines(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSummaryFixture(t, root, "go.mod", "module example.test/auth\n\ngo 1.27.0\n")
	writeSummaryFixture(t, root, "auth/client.go", screenFixtureSource)
	gitFixture(t, root, "init", "-q")
	gitFixture(t, root, "add", "-A")
	gitFixture(t, root, "-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "fixture")
	tree := gitFixture(t, root, "rev-parse", "HEAD^{tree}")
	blob := gitFixture(t, root, "rev-parse", "HEAD:auth/client.go")
	code, stdout, stderr := runContextCommand(t, "--root", root, "context", "--expand", "cv1:"+tree+":"+blob+":all:auth/client.go")
	if code != 0 {
		t.Fatalf("expand: exit %d: %s", code, stderr)
	}
	selection := decodeObject(t, stdout)["selection"].(map[string]any)
	exact := sha256.Sum256([]byte(screenFixtureSource))
	if strings.Contains(string(stdout), screenFixtureSecret) || selection["sha256"] != hex.EncodeToString(exact[:]) || !reflect.DeepEqual(selection["redacted_lines"], []any{2.0, 4.0, 5.0, 6.0}) {
		t.Fatalf("expand selection = %v", selection)
	}
	code, stdout, stderr = runContextCommand(t, "--root", root, "context", "--task", "auth client password")
	if code != 0 || strings.Contains(string(stdout), screenFixtureSecret) {
		t.Fatalf("packet: exit %d leaks or fails: %s %s", code, stdout, stderr)
	}
}
