package affected

import (
	"slices"
	"testing"
)

// AZU-V0-8927: the word of a value-supplying parameter expansion is a path
// token of the driver's text, in make's `$$` spelling and the shell's.
func TestDriverPathTokensReadValueWords_AZU_V0_8927(t *testing.T) {
	for _, c := range []struct {
		name, text string
		want       []string
	}{
		{"make-colon-dash", "cat $${F:-data/x.txt}", []string{"data/x.txt"}},
		{"shell-colon-dash", "cat ${F:-data/x.txt}", []string{"data/x.txt"}},
		{"dash", "cat $${F-data/x.txt}", []string{"data/x.txt"}},
		{"colon-equals", "cat $${F:=data/x.txt}", []string{"data/x.txt"}},
		{"equals", "cat $${F=data/x.txt}", []string{"data/x.txt"}},
		{"colon-plus", "cat $${F:+data/x.txt}", []string{"data/x.txt"}},
		{"plus", "cat $${F+data/x.txt}", []string{"data/x.txt"}},
		{"nested", "cat $${F:-$${G:-data/y.txt}}", []string{"data/y.txt"}},
		{"nested-joined", "cat $${F:-$${ROOT:-.}/data/y.txt}", []string{"data/y.txt"}},
		{"nested-joined-root", "cat $${F:-$${ROOT:-conf}/data/y.txt}", []string{"data/y.txt", "conf/data/y.txt"}},
		{"siblings", "cat $${F:-data/x.txt} $${G:-data/y.txt}", []string{"data/x.txt", "data/y.txt"}},
		{"suffix-joined", "cat $${DIR:-data}/x.txt", []string{"data/x.txt"}},
		{"unclosed", "cat $${F:-data/x.txt", []string{"data/x.txt"}},
		{"make-block", "check:\n\t@f=$${1:-data/x.txt}; \\\n\tcat \"$$f\" script/run.sh\n", []string{"script/run.sh", "data/x.txt"}},
		// Parameters the shared lexer does not take are read flat: the
		// default is kept, never dropped.
		{"indirect", "cat ${!F:-data/x.txt}", []string{"data/x.txt"}},
		{"subscript", "cat ${F[0]:-data/x.txt}", []string{"data/x.txt"}},
		{"special", "cat ${#:-data/x.txt}", []string{"data/x.txt"}},
		{"indirect-in-word", "cat ${F:-${!G:-data/y.txt}}", []string{"data/y.txt"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := driverPathTokens(c.text)
			for _, want := range c.want {
				if !slices.Contains(got, want) {
					t.Errorf("driverPathTokens(%q) = %q, missing %q", c.text, got, want)
				}
			}
		})
	}
}

// AZU-V0-8927: `${N:?WORD}` is an error message and every other operator is
// no value, so their words stay out of the tokens as in the shell plugin
// (AZU-V0-8920): the rule adds the defaults and nothing else.
func TestDriverPathTokensKeepNonValueFormsBlank_AZU_V0_8927(t *testing.T) {
	for _, text := range []string{
		"cat $${G:?d/seven.txt}",
		"cat $${H#d/eight.txt}",
		"cat $${H%d/nine.txt}",
		"cat $${H/d/ten.txt}",
	} {
		if got, old := driverPathTokens(text), driverBlankedPathTokens(text); !slices.Equal(got, old) {
			t.Errorf("driverPathTokens(%q) = %q, want the blanked tokens %q", text, got, old)
		}
	}
}

// AZU-V0-8927: a default nested deeper than the shared lexer follows is read
// flat, so it is still a token.
func TestDriverPathTokensReadPastTheDepthBound_AZU_V0_8927(t *testing.T) {
	text := "data/deep.txt"
	for level := 0; level < 3*MaxValueWordDepth; level++ {
		text = "$${L" + string(rune('A'+level)) + ":-" + text + "}"
	}
	if got := driverPathTokens("cat " + text); !slices.Contains(got, "data/deep.txt") {
		t.Fatalf("driverPathTokens dropped a default %d levels deep: %q", 3*MaxValueWordDepth, got)
	}
}

// AZU-V0-8927: the value-word rule only adds tokens. For every text built
// from the pieces below, balanced or not, in make's spelling and the
// shell's, the tokens the driver read before the rule are a prefix of those
// it reads now, unchanged and in order, so no driver unit loses a reader and
// no selection narrows. The last four lines are the recipe lines of this
// repository's Makefile that use a default form.
func TestDriverPathTokensOnlyAdd_AZU_V0_8927(t *testing.T) {
	check := func(text string) (added int) {
		old, got := driverBlankedPathTokens(text), driverPathTokens(text)
		if len(got) < len(old) || !slices.Equal(got[:len(old)], old) {
			t.Fatalf("driverPathTokens removed or reordered a token\n text %q\n was  %q\n is   %q", text, old, got)
		}
		return len(got) - len(old)
	}
	pieces := []string{
		"${F:-", "$${1:-", "${ROOT:-.}", "$${A:+", "${B=", "${C:?", "${D#", "${!E:-", "${G[0]:-", "${", "$$", "$", "}", "}}",
		"$(DIR)", "$tmp", "\\", "\\}", "\"", " ", "\n\t",
		"/data/x.txt", "docs/a.md", "x/y", ".", "-",
	}
	texts, grew := 0, 0
	var build func(prefix string, left int)
	build = func(prefix string, left int) {
		texts++
		if check(prefix) > 0 {
			grew++
		}
		if left == 0 {
			return
		}
		for _, piece := range pieces {
			build(prefix+piece, left-1)
		}
	}
	build("", 4)
	if texts < 400000 || grew == 0 {
		t.Fatalf("%d texts, %d with an added token: the pieces exercise too little", texts, grew)
	}
	for _, text := range []string{
		"elif [ -z \"$${BASE_SHA-}\" ] && [ \"$${GITHUB_EVENT_NAME-}\" = pull_request ]; then \\",
		"elif [ -z \"$${BASE_SHA-}\" ]; then \\",
		"@if [ \"$${CORVINT_CI_CADENCE-}\" = DEFER ]; then echo 'selection-baseline-check: NOT_RUN'; exit 0; fi; \\",
		"@work=$$(mktemp -d \"$${TMPDIR:-/tmp}/corvint-install.XXXXXX\") && \\",
	} {
		check(text)
	}
	t.Logf("%d texts, %d with an added token", texts, grew)
}
