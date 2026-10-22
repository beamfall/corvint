package projectprofile

import (
	"reflect"
	"testing"
)

// TestAFPV0196InlineVerifyLine: AFP-V0-196. An inline **Verify:** line declares
// the code spans that directly follow the label; a later span after other text
// is not a command, a span inside a non-shell fence is not read, and the
// Verify heading's fenced block still declares its commands.
func TestAFPV0196InlineVerifyLine(t *testing.T) {
	t.Parallel()
	body := "# Agents\n\n" +
		"Run `corvint affected`. **Verify:** `make gate` (Tasks gate id `gate`).\n" +
		"Also **verify**: `npm run build`, `npm run typecheck` and `npm test` before merging `main`.\n\n" +
		"```markdown\n**Verify:** `rm -rf /`\n```\n\n" +
		"## Verify\n\n```sh\ngo vet ./... # vet\n```\n"
	got, unrecognized := VerifyCommands(body)
	want := []DeclaredCommand{
		{Command: "make gate", Inline: true},
		{Command: "npm run build", Inline: true},
		{Command: "npm run typecheck", Inline: true},
		{Command: "npm test", Inline: true},
		{Command: "go vet ./..."},
	}
	if !reflect.DeepEqual(got, want) || len(unrecognized) != 0 {
		t.Fatalf("VerifyCommands = %#v, %v; want %#v", got, unrecognized, want)
	}
}

// TestNGIV0060MandatoryVerifyCommands: NGI-V0-060. Mandatory commands exclude
// launchers and commands a Verify scope sentence makes conditional, and are
// deduplicated in order.
func TestNGIV0060MandatoryVerifyCommands(t *testing.T) {
	t.Parallel()
	body := "## Verify\n\nRun `make windows-vet` only when a Go file changed.\n\n" +
		"```sh\nmake gate\nmake windows-vet\nopen report.html\nmake gate\n```\n\n**Verify:** `npm test`\n"
	if got := MandatoryVerifyCommands(body); !reflect.DeepEqual(got, []string{"make gate", "npm test"}) {
		t.Fatalf("MandatoryVerifyCommands = %v", got)
	}
	if got := MandatoryVerifyCommands("# Agents\n\nNo declaration here.\n"); len(got) != 0 {
		t.Fatalf("undeclared MandatoryVerifyCommands = %v", got)
	}
}
