package affected

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"
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
		// Parameters the shared lexer does not take are joined in place: the
		// default is kept, never dropped.
		{"indirect", "cat ${!F:-data/x.txt}", []string{"data/x.txt"}},
		{"subscript", "cat ${F[0]:-data/x.txt}", []string{"data/x.txt"}},
		{"special", "cat ${#:-data/x.txt}", []string{"data/x.txt"}},
		{"indirect-in-word", "cat ${F:-${!G:-data/y.txt}}", []string{"data/y.txt"}},
		// A default the shared lexer does not take is joined into the path
		// around it, as the shell reads it when the default applies.
		{"indirect-in-path", "cat data/${!S:-x}.txt", []string{"data/x.txt"}},
		{"subscript-in-path", "cat data/${A[0]:-x}.txt", []string{"data/x.txt"}},
		{"indirect-around-nested", "cat ${!F:-data/${S:-x}.txt}", []string{"data/x.txt"}},
		{"indirect-nested-in-path", "cat ${F:-data/${!S:-x}.txt}", []string{"data/x.txt"}},
		{"make-indirect-in-path", "cat data/$${!S:-x}.txt", []string{"data/x.txt"}},
		{"plain-in-path", "cat ${F:-data}/${G:-x}.txt", []string{"data/x.txt"}},
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
// (AZU-V0-8920): the rule adds the defaults and nothing else. That holds
// beside a default the shared lexer leaves unjoined, which turns on the
// joined read, and a default nested in such a form is still read alone, as
// the shared lexer reads it, never joined to the form's own text.
func TestDriverPathTokensKeepNonValueFormsBlank_AZU_V0_8927(t *testing.T) {
	for _, c := range []struct {
		text  string
		added []string // the tokens after the blanked ones, in order
	}{
		{"cat $${G:?d/seven.txt}", nil},
		{"cat $${H#d/eight.txt}", nil},
		{"cat $${H%d/nine.txt}", nil},
		{"cat $${H/d/ten.txt}", nil},
		// Beside an unjoined default.
		{"cat ${!S:-x} ${X:?data/msg.txt}", nil},
		{"cat ${!S:-data/x.txt} ${G:?data/err.txt}", []string{"data/x.txt"}},
		{"cat ${!S:-x} ${X%data/pct.txt}", nil},
		{"cat ${!S:-x} ${X#data/hash.txt}", nil},
		{"cat ${!S:-x} ${X/data/sub.txt}", nil},
		{"cat ${!S:-x} ${X:?a/${Y}/msg.txt}", nil},
		{"cat ${!S:-x} ${X%${Y:+data/b.txt}/c.txt} ${X%%x}", []string{"data/b.txt"}},
		{"cat ${!S:-x} ${X:?unclosed data/msg.txt", nil},
		// A default nested in a non-value form.
		{"cat ${!S:-x} ${X%${Y:-data/a.txt}}", []string{"data/a.txt"}},
		{"cat ${!S:-x} ${X%pre/${!Y:-a}.txt}", nil},
		{"cat ${!S:-x} ${X%${!Y:-data/a.txt}}", []string{"data/a.txt"}},
		{"cat ${!S:-data/${X#${Y:-p}q/r}.txt}", nil},
	} {
		old := driverBlankedPathTokens(c.text)
		want := slices.Clone(old)
		for _, token := range c.added {
			if !slices.Contains(want, token) {
				want = append(want, token)
			}
		}
		if got := driverPathTokens(c.text); !slices.Equal(got, want) {
			t.Errorf("driverPathTokens(%q) = %q, want %q (the blanked tokens %q and the defaults)", c.text, got, want, old)
		}
	}
}

// AZU-V0-8927: the joined read is one pass. A run of unclosed subscripted
// openers, each asking for the `]` that ends its subscript, reads each byte
// of the text at most once for that search, where a search per opener read
// the rest of the text each time.
func TestDriverJoinedReadIsLinear_AZU_V0_8927(t *testing.T) {
	for _, text := range []string{
		strings.Repeat("${A[", 20000),
		strings.Repeat("${A[", 20000) + "]",
		"cat ${!S:-x} " + strings.Repeat("${A[x}", 20000),
		"cat ${!S:-x} " + strings.Repeat("${!A[", 20000) + "data/x.txt",
	} {
		unjoined := newDriverScanText(text)
		driverUnjoinedValueForm(unjoined)
		joined := newDriverScanText(text)
		driverJoinedValueWords(joined)
		for name, scan := range map[string]*driverScanText{"driverUnjoinedValueForm": unjoined, "driverJoinedValueWords": joined} {
			if scan.scanned > len(text) {
				t.Errorf("%s read %d bytes for `]` in a text of %d", name, scan.scanned, len(text))
			}
		}
	}
	done := make(chan []string, 1)
	go func() { done <- driverPathTokens("cat ${!S:-x} " + strings.Repeat("${A[", 20000)) }()
	select {
	case <-done:
	case <-time.After(time.Minute):
		t.Fatal("driverPathTokens took over a minute on 20000 unclosed `${A[` openers")
	}
}

// AZU-V0-8927: a `${` that opens no parameter (a quoted, commented or
// escaped literal) or whose form never closes blanks only its own two bytes
// in the joined read, so the defaults after it are still read composed with
// their text, as the direct form is. A form blanked whole needs a parameter
// after its opener and a matching `}`.
func TestDriverJoinedReadKeepsTextAfterAnUnclosedOpener_AZU_V0_8927(t *testing.T) {
	for _, c := range []struct {
		text string
		want []string
	}{
		{"echo '${'; cat data/${!S:-x}.txt", []string{"data/x.txt"}},
		{"# a literal ${ here\ncat data/${!S:-x}.txt", []string{"data/x.txt"}},
		{"echo '${'; cat data/${A[0]:-x}.txt", []string{"data/x.txt"}},
		{"sed 's/\\${/{/' in.txt\ncat data/${!S:-x}.txt", []string{"data/x.txt"}},
		{"echo '${'; cat data/${S:-x}.txt", []string{"data/x.txt"}},
		{"echo \"${\"; cat data/${!S:-x}.txt", []string{"data/x.txt"}},
		{"echo \\${; cat data/${!S:-x}.txt", []string{"data/x.txt"}},
		{"echo '${' '${'\ncat data/${!S:-x}.txt", []string{"data/x.txt"}},
		{"echo '${X%'; cat data/${!S:-x}.txt", []string{"data/x.txt"}},
		{"cat data/${!S:-x}.txt; echo ${", []string{"data/x.txt"}},
		{"cat ${!S:-'${'}; cat data/${!T:-y}.txt", []string{"data/y.txt"}},
		{"cat ${!S:-data/${X%a}}; cat data/${!T:-y}.txt", []string{"data/y.txt"}},
		// A literal `${` and a later `}` that closes no expansion.
		{"echo '${'; cat data/${!S:-x}.txt; f() { :; }", []string{"data/x.txt"}},
		{"# a ${ b\ncat data/${!S:-x}.txt\nawk '{print}' f", []string{"data/x.txt"}},
		// A subscript inside a form, and a directory form before a default.
		{"cat ${!S:-x} ${A[${i}]:-data/y.txt}", []string{"data/y.txt"}},
		{"cat ${D}/${!S:-x}.txt", []string{"x.txt"}},
	} {
		got := driverPathTokens(c.text)
		for _, token := range c.want {
			if !slices.Contains(got, token) {
				t.Errorf("driverPathTokens(%q) = %q, want %q in it", c.text, got, token)
			}
		}
	}
	// The `]` of a subscript is searched for inside its own form only.
	if got := driverPathTokens("cat ${!S:-x} ${A[x} ${X:?a]-data/cross.txt}"); slices.Contains(got, "data/cross.txt") {
		t.Errorf("driverPathTokens read a subscript past its form's `}`: %q", got)
	}
}

// AZU-V0-8927: a line with a literal `${` before a text never takes a token
// from the text's joined read: the joined read of the lines after it is the
// text's own, byte for byte. (The blanked tokens the driver read before the
// rule end a braced expansion at its first `}`, unclosed or not, and stay as
// they were.)
func TestDriverJoinedReadIgnoresAnEarlierLiteralOpener_AZU_V0_8927(t *testing.T) {
	random := rand.New(rand.NewPCG(8927, 5628))
	pieces := []string{
		"${F:-", "${!E:-", "${G[0]:-", "${X%", "${X:?", "${D}/", "${", "}", "\\", "'", "\"", " ", "\n", ";",
		"data/", "x.txt", "${!S:-x}", "${A[0]:-y}", "data/${!S:-x}.txt", "f() { :; }",
	}
	prefixes := []string{"${\n", "echo '${'\n", "# a literal ${ here\n", "sed 's/\\${/{/' in.txt\n", "echo \\${; ", "echo '${' '${'\n"}
	for range 20000 {
		var text strings.Builder
		for range 1 + random.IntN(8) {
			text.WriteString(pieces[random.IntN(len(pieces))])
		}
		alone := driverJoinedValueWords(newDriverScanText(text.String()))
		for _, prefix := range prefixes {
			if got := driverJoinedValueWords(newDriverScanText(prefix + text.String())); !strings.HasSuffix(got, alone) {
				t.Fatalf("the joined read of %q is %q, which does not end with the text's own %q", prefix+text.String(), got, alone)
			}
		}
	}
}

// AZU-V0-8927: a default nested deeper than the shared lexer follows is
// joined in place, so it is still a token, alone and composed with the text
// after it.
func TestDriverPathTokensReadPastTheDepthBound_AZU_V0_8927(t *testing.T) {
	for _, levels := range []int{MaxValueWordDepth + 2, 3 * MaxValueWordDepth} {
		nest := func(word string) string {
			for level := 0; level < levels; level++ {
				word = "$${L" + string(rune('A'+level)) + ":-" + word + "}"
			}
			return word
		}
		if got := driverPathTokens("cat " + nest("data/deep.txt")); !slices.Contains(got, "data/deep.txt") {
			t.Errorf("driverPathTokens dropped a default %d levels deep: %q", levels, got)
		}
		if got := driverPathTokens("cat " + nest("data") + "/deep.txt"); !slices.Contains(got, "data/deep.txt") {
			t.Errorf("driverPathTokens dropped a default %d levels deep composed with its suffix: %q", levels, got)
		}
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
