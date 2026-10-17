package mutation_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
)

// TestKHNV0029_ValueScreenMatchesDeclaredValuesAndOptInClasses: the screen
// compares every declared value of at least MinDenyValueBytes (plain or
// env-style lines, comments and blanks ignored) and only the opt-in
// personal-data classes; an undeclared class never matches.
func TestKHNV0029_ValueScreenMatchesDeclaredValuesAndOptInClasses(t *testing.T) {
	deny := []byte("# adopter values\n\nexport API_TOKEN=\"tok-7f3a9c2e1b\"\nshort\nplain-value-123456\r\n")
	if got := mutation.ParseDenyValues(deny); len(got) != 2 || got[0] != "tok-7f3a9c2e1b" || got[1] != "plain-value-123456" {
		t.Fatalf("ParseDenyValues: %q", got)
	}
	none := mutation.NewValueScreen(deny, nil)
	// A padded base64 line is not KEY=VALUE with VALUE "=": it is compared
	// whole, quoted or not.
	b64 := []byte("c2VjcmV0MTIzNDU2Nzg5MA==\n\"Zm9vYmFyYmF6cXV4=\"\nFOO=bar\n")
	if got := mutation.ParseDenyValues(b64); len(got) != 2 || got[0] != "c2VjcmV0MTIzNDU2Nzg5MA==" || got[1] != "Zm9vYmFyYmF6cXV4=" {
		t.Fatalf("ParseDenyValues base64: %q", got)
	}
	padded := mutation.NewValueScreen(b64, nil)
	all := mutation.NewValueScreen(nil, []string{intent.ScreenClassCard, intent.ScreenClassEmail, intent.ScreenClassPhone})
	cases := []struct {
		screen *mutation.ValueScreen
		text   string
		want   string
	}{
		{none, "rerun with tok-7f3a9c2e1b set", intent.ScreenClassDenyValue},
		{padded, "token c2VjcmV0MTIzNDU2Nzg5MA== leaked", intent.ScreenClassDenyValue},
		{padded, "token Zm9vYmFyYmF6cXV4= leaked", intent.ScreenClassDenyValue},
		{none, "see plain-value-123456", intent.ScreenClassDenyValue},
		{none, "short is not compared", ""},
		{none, "mail ops@example.com", ""},
		{all, "mail ops@example.com", intent.ScreenClassEmail},
		{all, "call +44 20 7946 0958 now", intent.ScreenClassPhone},
		{all, "call (555) 123-4567", intent.ScreenClassPhone},
		{all, "card 4111 1111 1111 1111 on file", intent.ScreenClassCard},
		{all, "build 4111111111111112 failed", ""},
		{all, "commit 1234567890123456789012345", ""},
		{all, "release v0.7.0 at 2026-10-17", ""},
		{nil, "anything", ""},
	}
	for _, c := range cases {
		if got := c.screen.Match(c.text); got != c.want {
			t.Errorf("Match(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}
