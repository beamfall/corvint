package secretscreen

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"regexp"
	"regexp/syntax"
	"strings"
	"sync"
	"testing"
	"unicode"
)

var alternativePatterns = func() []*regexp.Regexp {
	patterns := make([]*regexp.Regexp, len(writerAlternatives))
	for i, a := range writerAlternatives {
		patterns[i] = regexp.MustCompile(`(?i)` + a.expr)
	}
	return patterns
}()

// prefilterDisagreement compares the screened result for an already masked
// text with Pattern itself, and checks each branch's necessary condition
// against that branch alone, so an unsound condition cannot hide behind
// another branch that happens to match the same text.
func prefilterDisagreement(masked string, got [][]int) string {
	if want := Pattern.FindAllStringIndex(masked, -1); !reflect.DeepEqual(got, want) {
		return fmt.Sprintf("matches for %q = %v, Pattern gives %v", masked, got, want)
	}
	if strings.Contains(masked, "\u212a") || strings.Contains(masked, "\u017f") {
		return ""
	}
	lower := []byte(masked)
	for i, c := range lower {
		if c >= 'A' && c <= 'Z' {
			lower[i] = c + ('a' - 'A')
		}
	}
	for i, a := range writerAlternatives {
		if !a.need(lower) && alternativePatterns[i].MatchString(masked) {
			return fmt.Sprintf("branch %d (%s) matches %q but its necessary condition is false", i, a.expr, masked)
		}
	}
	return ""
}

// TestMain checks every text any test in this package screens, so each
// existing positive and negative fixture is also a differential case.
func TestMain(m *testing.M) {
	var mu sync.Mutex
	var disagreements []string
	observeMatches = func(masked string, matches [][]int) {
		if d := prefilterDisagreement(masked, matches); d != "" {
			mu.Lock()
			disagreements = append(disagreements, d)
			mu.Unlock()
		}
	}
	code := m.Run()
	for _, d := range disagreements {
		fmt.Fprintln(os.Stderr, "LTA-V0-014 prefilter disagreement:", d)
	}
	if len(disagreements) != 0 && code == 0 {
		code = 1
	}
	os.Exit(code)
}

func TestLTAV0014PatternSourceIsUnchanged(t *testing.T) {
	const want = "97e4089e0234470bb311b37ffc445e16a5f4ba9fe14114dfb9f982bb9c7e7055"
	if got := fmt.Sprintf("%x", sha256.Sum256([]byte(Pattern.String()))); got != want {
		t.Fatalf("writer pattern digest = %s, want %s; a detector change needs its own requirement and a new digest here", got, want)
	}
	if len(writerAlternatives) > 64 {
		t.Fatalf("%d alternatives do not fit the subset mask", len(writerAlternatives))
	}
}

// differentialCorpus derives texts from the parity fixtures: each alone, in
// upper case, with the two non-ASCII runes that fold to ASCII letters, split
// across a line, inside a JSON string and as base64, plus every ordered pair,
// which exercises compiled subsets no single fixture reaches.
func differentialCorpus(t testing.TB) []string {
	parity := loadParityCorpus(t)
	var fixtures []string
	for _, c := range append(parity.Baseline, parity.WriterOnly...) {
		fixtures = append(fixtures, c.Text)
	}
	if len(fixtures) < 70 {
		t.Fatalf("read %d parity fixtures", len(fixtures))
	}
	corpus := []string{"", "plain text with nothing to find", "--- PASS: TestName (0.01s)"}
	// Boundaries of the necessary conditions that no parity fixture sits on.
	for _, space := range []string{"\t", "\v", "\f", "\r", "\x1c", "\x1d", "\x1e", "\x1f", "\u0085", "\u00a0", "\u3000"} {
		corpus = append(corpus, "token"+space+"=synthetic123", `{"db_password"`+space+`:"synthetic123"}`, "pass"+space+": synthetic123")
	}
	corpus = append(corpus,
		"--password\thunter2", "--db-token\tabc123", "tool --credentials\tabc123",
		"xoxb-0123456789", "xoxoxb-0123456789", "xoxxoxb-0123456789",
		"sk0123456789abcdef0123456789abcdef", "task sk0123456789ABCDEF0123456789abcdef", "sk0123456789abcdef0123456789abcde",
	)
	for _, text := range fixtures {
		corpus = append(corpus, text,
			strings.ToUpper(text),
			strings.NewReplacer("k", "\u212a", "s", "\u017f").Replace(text),
			strings.Replace(text, "=", "\n=", 1),
			text[:len(text)/2]+"\n"+text[len(text)/2:],
			fmt.Sprintf(`{"output":%q,"status":"ok"}`, text),
			base64.StdEncoding.EncodeToString([]byte(text)),
			"--- PASS: TestName (0.01s)\n"+text,
		)
	}
	for _, first := range fixtures {
		for _, second := range fixtures {
			corpus = append(corpus, first+"\n"+second)
		}
	}
	return corpus
}

func TestLTAV0014PrefilterReturnsPatternMatches(t *testing.T) {
	livePatterns.Lock()
	livePatterns.bySubset = nil
	livePatterns.Unlock()
	positives := 0
	for _, text := range differentialCorpus(t) {
		got := writerMatches(text)
		if len(got) != 0 {
			positives++
		}
		// writerMatches masks before screening; unmasked texts are compared
		// here directly, masked ones by TestMain's observer.
		if !strings.Contains(text, "--- PASS: ") {
			if d := prefilterDisagreement(text, got); d != "" {
				t.Fatal(d)
			}
		}
	}
	if positives < 5000 {
		t.Fatalf("only %d corpus texts matched; the corpus no longer exercises the detector", positives)
	}
	t.Run("past the subset cache bound", func(t *testing.T) {
		livePatterns.Lock()
		subsets := len(livePatterns.bySubset)
		livePatterns.Unlock()
		if subsets != maxLivePatterns {
			t.Fatalf("corpus compiled %d subsets, want the bound %d reached so the Pattern fallback is exercised", subsets, maxLivePatterns)
		}
	})
}

func TestLTAV0014TextWithoutCandidatesSkipsThePattern(t *testing.T) {
	for _, text := range []string{"", "ordinary prose, nothing shaped like a credential", receiptShapedDocument(4 << 10)} {
		if livePattern(text) == Pattern {
			t.Fatalf("text of %d bytes fell back to the complete pattern", len(text))
		}
	}
	if livePattern("to\u212aen=abcdefghijklmnopqrstuvwxyz") != Pattern {
		t.Fatal("a text holding a non-ASCII rune that folds to an ASCII letter must use the complete pattern")
	}
}

func FuzzLTAV0014PrefilterReturnsPatternMatches(f *testing.F) {
	for i, text := range differentialCorpus(f) {
		if i < 600 {
			f.Add(text)
		}
	}
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 4<<10 {
			t.Skip()
		}
		masked, got := "", [][]int(nil)
		previous := observeMatches
		observeMatches = func(m string, matches [][]int) { masked, got = m, matches }
		writerMatches(text)
		observeMatches = previous
		if d := prefilterDisagreement(masked, got); d != "" {
			t.Fatal(d)
		}
	})
}

// receiptShapedDocument is a deterministic stand-in for an encoded evidence
// receipt: JSON whose bulk is base64 byte fields and hexadecimal digests.
func receiptShapedDocument(size int) string {
	var doc strings.Builder
	doc.WriteString(`{"schema":"corvint-browser-behavior-evidence/1","raw_attempts":[`)
	seed := sha256.Sum256([]byte("receipt"))
	for i := 0; doc.Len() < size; i++ {
		var field []byte
		for len(field) < 3<<10 {
			seed = sha256.Sum256(seed[:])
			field = append(field, seed[:]...)
		}
		if i > 0 {
			doc.WriteByte(',')
		}
		fmt.Fprintf(&doc, `{"ordinal":%d,"control_id":"control-%d","attempt":1,"hook_sha256":"sha256:%s","hook_bytes":"%s"}`,
			i, i, hex.EncodeToString(seed[:]), base64.StdEncoding.EncodeToString(field))
	}
	doc.WriteString(`],"fallback":"full-relevant-suite"}`)
	return doc.String()
}

// nativeReportShapedDocument is a deterministic stand-in for a runner's
// native report: JSON of file paths, digests and short status strings.
func nativeReportShapedDocument(size int) string {
	var doc strings.Builder
	doc.WriteString(`{"receipt":{"freshness":{"files":{`)
	seed := sha256.Sum256([]byte("native"))
	for i := 0; doc.Len() < size; i++ {
		seed = sha256.Sum256(seed[:])
		if i > 0 {
			doc.WriteByte(',')
		}
		fmt.Fprintf(&doc, `"/cache/node_modules/playwright/lib/impl-%d.js":"%s"`, i, hex.EncodeToString(seed[:]))
	}
	doc.WriteString(`}},"result":{"status":"passed","duration":12,"title":"counter one","project":"chromium"}}}`)
	return doc.String()
}

// BenchmarkScreen retains the before and after of V1-0722 side by side:
// "pattern" is the complete detector on its own, "screen" is MatchString.
func BenchmarkScreen(b *testing.B) {
	// TestMain's observer reruns the complete pattern; time without it.
	observer := observeMatches
	observeMatches = nil
	defer func() { observeMatches = observer }()
	for _, input := range []struct {
		name string
		text string
	}{
		{"receipt-87KB", receiptShapedDocument(87 << 10)},
		{"native-report-13KB", nativeReportShapedDocument(13 << 10)},
	} {
		b.Run(input.name+"/pattern", func(b *testing.B) {
			b.SetBytes(int64(len(input.text)))
			for b.Loop() {
				if Pattern.MatchString(input.text) {
					b.Fatal("benchmark input matched")
				}
			}
		})
		b.Run(input.name+"/screen", func(b *testing.B) {
			b.SetBytes(int64(len(input.text)))
			for b.Loop() {
				if MatchString(input.text) {
					b.Fatal("benchmark input matched")
				}
			}
		})
	}
}

var generatorRunes = []rune{' ', '\t', '\n', '\v', '\f', '\r', 0x1c, 0x1f, 0x85, 0xa0, 0x1680, 0x2000, 0x2028, 0x3000,
	'"', '\'', '\\', '=', ':', '@', '/', '.', '-', '_', '$', '?', '#', 'a', 'Z', '0', '9', 'é', 0xfffd, 0x130, 0x131}

var generatorFiller = []string{"", " ", "\n", "x", "-", "--", `"`, "'", "=", ":", "\xff", "\xe2\x84", "é", "a.b", "0123456789", "@", "://", "--- PASS: T (1s)\n", "\t", `\`}

// generateMatch writes a string drawn from the language of re, preferring
// boundary runes, so each branch's condition meets texts built from that
// branch's own expression and not only from the hand-written fixtures.
func generateMatch(r *rand.Rand, re *syntax.Regexp, b *strings.Builder, depth int) {
	switch re.Op {
	case syntax.OpLiteral:
		for _, c := range re.Rune {
			if re.Flags&syntax.FoldCase != 0 && r.Intn(2) == 0 {
				orbit := []rune{c}
				for f := unicode.SimpleFold(c); f != c; f = unicode.SimpleFold(f) {
					orbit = append(orbit, f)
				}
				c = orbit[r.Intn(len(orbit))]
			}
			b.WriteRune(c)
		}
	case syntax.OpCharClass:
		var preferred []rune
		for _, c := range generatorRunes {
			for i := 0; i < len(re.Rune); i += 2 {
				if c >= re.Rune[i] && c <= re.Rune[i+1] {
					preferred = append(preferred, c)
				}
			}
		}
		if len(preferred) != 0 && r.Intn(3) == 0 {
			b.WriteRune(preferred[r.Intn(len(preferred))])
			return
		}
		i := r.Intn(len(re.Rune)/2) * 2
		lo, hi := re.Rune[i], re.Rune[i+1]
		if hi-lo > 300 {
			hi = lo + 300
		}
		c := lo + rune(r.Intn(int(hi-lo+1)))
		if c >= 0xd800 && c <= 0xdfff {
			c = 'a'
		}
		b.WriteRune(c)
	case syntax.OpAnyCharNotNL, syntax.OpAnyChar:
		b.WriteRune(generatorRunes[r.Intn(len(generatorRunes))])
	case syntax.OpCapture:
		generateMatch(r, re.Sub[0], b, depth)
	case syntax.OpConcat:
		for _, sub := range re.Sub {
			generateMatch(r, sub, b, depth)
		}
	case syntax.OpAlternate:
		generateMatch(r, re.Sub[r.Intn(len(re.Sub))], b, depth)
	case syntax.OpStar, syntax.OpPlus, syntax.OpQuest, syntax.OpRepeat:
		least, most := 0, 3
		switch re.Op {
		case syntax.OpPlus:
			least = 1
		case syntax.OpQuest:
			most = 1
		case syntax.OpRepeat:
			least, most = re.Min, re.Max
			if most < 0 {
				most = least + 3
			}
		}
		n := least
		if most > least && depth <= 6 && r.Intn(3) != 0 {
			n = least + r.Intn(most-least+1)
		}
		for i := 0; i < n; i++ {
			generateMatch(r, re.Sub[0], b, depth+1)
		}
	}
}

// TestLTAV0014GeneratedBranchTextsKeepTheirBranchLive draws texts from each
// branch's own expression. The fixed seed keeps it deterministic; the subset
// cache is emptied as it goes so the comparison never settles on the
// complete-pattern fallback.
func TestLTAV0014GeneratedBranchTextsKeepTheirBranchLive(t *testing.T) {
	parsed := make([]*syntax.Regexp, len(writerAlternatives))
	for i, a := range writerAlternatives {
		re, err := syntax.Parse(`(?i)`+a.expr, syntax.Perl)
		if err != nil {
			t.Fatal(err)
		}
		parsed[i] = re.Simplify()
	}
	iterations := 12000
	if testing.Short() {
		iterations = 3000
	}
	r := rand.New(rand.NewSource(20261004))
	hits := make([]int, len(parsed))
	for n := 0; n < iterations; n++ {
		if n%100 == 0 {
			livePatterns.Lock()
			livePatterns.bySubset = nil
			livePatterns.Unlock()
		}
		var b strings.Builder
		for parts := 1 + r.Intn(3); parts > 0; parts-- {
			b.WriteString(generatorFiller[r.Intn(len(generatorFiller))])
			generateMatch(r, parsed[(n+parts)%len(parsed)], &b, 0)
			b.WriteString(generatorFiller[r.Intn(len(generatorFiller))])
		}
		text := b.String()
		// Case folding puts U+212A and U+017F in most classes, and a text
		// holding either bypasses the prefilter; keep that to one text in ten.
		if n%10 != 0 {
			text = strings.NewReplacer("\u212a", "k", "\u017f", "s").Replace(text)
		}
		for i, pattern := range alternativePatterns {
			if pattern.MatchString(text) {
				hits[i]++
			}
		}
		var got [][]int
		if live := livePattern(text); live != nil {
			got = live.FindAllStringIndex(text, -1)
		}
		if d := prefilterDisagreement(text, got); d != "" {
			t.Fatal(d)
		}
	}
	for i, n := range hits {
		if n < iterations/400 {
			t.Errorf("branch %d (%s) matched only %d generated texts", i, writerAlternatives[i].expr, n)
		}
	}
}
