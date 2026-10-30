package contextindex

import (
	"strings"
	"testing"
)

// rubyExpected is a Symbol without the identity every symbol from one fixture
// shares, so a table row reads as the declaration it pins.
type rubyExpected struct {
	kind, name string
	line       int
}

func rubyTestSource() Source {
	return Source{Path: "app/models/thing.rb", BlobHash: "b7"}
}

func assertRubySymbols(t *testing.T, text string, want []rubyExpected) []Symbol {
	t.Helper()
	source := rubyTestSource()
	symbols, notes := rubySymbols(source, text)
	if len(notes) != 0 {
		t.Fatalf("notes = %v, want none for a fully scanned source", notes)
	}
	if len(symbols) != len(want) {
		t.Fatalf("symbols = %+v (%d), want %d", symbols, len(symbols), len(want))
	}
	for index, expected := range want {
		actual := Symbol{Kind: expected.kind, Name: expected.name, Path: source.Path, BlobHash: source.BlobHash, Line: expected.line}
		if symbols[index] != actual {
			t.Errorf("symbol %d = %+v, want %+v", index, symbols[index], actual)
		}
	}
	return symbols
}

// TestRubySymbolsExtractsEachKind pins the declaration vocabulary: the method
// name is what was defined rather than where it was hung, a qualified constant
// stays whole, and a singleton class contributes nothing because it has no name.
func TestRubySymbolsExtractsEachKind(t *testing.T) {
	for _, testCase := range []struct {
		name string
		text string
		want []rubyExpected
	}{
		{
			name: "methods including receiver and suffixed forms",
			text: "def plain\nend\ndef self.build\nend\ndef logger.warn\nend\ndef @io.rewind\nend\ndef $stream.flush\nend\ndef valid?\nend\ndef save!\nend\ndef name=(value)\nend\n",
			want: []rubyExpected{
				{"func", "plain", 1},
				{"func", "build", 3},
				{"func", "warn", 5},
				{"func", "rewind", 7},
				{"func", "flush", 9},
				{"func", "valid?", 11},
				{"func", "save!", 13},
				{"func", "name=", 15},
			},
		},
		{
			name: "operator methods",
			text: "def <=>(other)\nend\ndef [](key)\nend\ndef []=(key, value)\nend\ndef +(other)\nend\ndef ==(other)\nend\ndef <<(item)\nend\n",
			want: []rubyExpected{
				{"func", "<=>", 1},
				{"func", "[]", 3},
				{"func", "[]=", 5},
				{"func", "+", 7},
				{"func", "==", 9},
				{"func", "<<", 11},
			},
		},
		{
			name: "endless method keeps the bare name",
			// `def foo = 42` assigns a body; only `def foo=(v)` names a setter,
			// and the difference is whether the `=` touches the identifier.
			text: "def total = 42\n",
			want: []rubyExpected{{"func", "total", 1}},
		},
		{
			name: "nested modules and classes",
			text: "module Outer\n  module Inner\n    class Thing\n    end\n  end\nend\n",
			want: []rubyExpected{
				{"module", "Outer", 1},
				{"module", "Inner", 2},
				{"type", "Thing", 3},
			},
		},
		{
			name: "qualified names and superclasses",
			text: "class Foo::Bar < Base\nend\nmodule Alpha::Beta\nend\n",
			want: []rubyExpected{
				{"type", "Foo::Bar", 1},
				{"module", "Alpha::Beta", 3},
			},
		},
		{
			name: "singleton class has no name to record",
			text: "class Registry\n  class << self\n    def instance\n    end\n  end\nend\n",
			want: []rubyExpected{
				{"type", "Registry", 1},
				{"func", "instance", 3},
			},
		},
		{
			name: "constant assignment at the start of a line",
			text: "MAX_RETRIES = 3\nDefaults = Struct.new(:host)\nConfig::TIMEOUT = 5\n  INDENTED = 1\n",
			want: []rubyExpected{
				{"var", "MAX_RETRIES", 1},
				{"var", "Defaults", 2},
				{"var", "Config::TIMEOUT", 3},
				{"var", "INDENTED", 4},
			},
		},
		{
			name: "assignment-shaped operators are not declarations",
			text: "FLAG == other\nFLAG => pattern\nFLAG =~ /x/\nFLAG ||= 1\nFLAG += 1\nrecord.CONST = 1\n",
			want: nil,
		},
		{
			name: "locals and attribute macros are not declarations",
			text: "class Thing\n  attr_accessor :name\n  attr_reader :size\n  def initialize\n    @count = 0\n    total = 1\n  end\nend\n",
			want: []rubyExpected{
				{"type", "Thing", 1},
				{"func", "initialize", 4},
			},
		},
		{
			name: "keyword matching is whole-token",
			text: "definitely = true\ndef_something(1)\nclassify(2)\nmodules = []\nrecord.class\n:class\ndef real\nend\n",
			want: []rubyExpected{{"func", "real", 7}},
		},
		{
			name: "several declarations on one line",
			text: "def first; end; def second; end\n",
			want: []rubyExpected{
				{"func", "first", 1},
				{"func", "second", 1},
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			assertRubySymbols(t, testCase.text, testCase.want)
		})
	}
}

// TestRubySymbolsIgnoresCommentsAndStrings is the negative half of the
// extractor's contract. Every fixture here contains text that a line scanner
// would report as a declaration; each one must contribute nothing, and the real
// declaration after it must still be found, which proves the scanner resumed
// rather than gave up.
func TestRubySymbolsIgnoresCommentsAndStrings(t *testing.T) {
	for _, testCase := range []struct {
		name string
		text string
		want []rubyExpected
	}{
		{
			name: "line comments",
			text: "# def commented\nvalue = 1 # class Commented\ndef real\nend\n",
			want: []rubyExpected{{"func", "real", 3}},
		},
		{
			name: "block comments",
			text: "=begin\ndef blocked\nclass Blocked\n=end\ndef real\nend\n",
			want: []rubyExpected{{"func", "real", 5}},
		},
		{
			name: "block comment markers only count at column 0",
			text: "  =begin\ndef real\nend\n  =end\n",
			want: []rubyExpected{{"func", "real", 2}},
		},
		{
			name: "single-quoted string",
			text: "puts 'def hidden'\nputs 'class Hidden'\ndef real\nend\n",
			want: []rubyExpected{{"func", "real", 3}},
		},
		{
			name: "double-quoted string with interpolation",
			text: "puts \"def hidden #{name}\"\nLABEL = \"#{row[\"class Hidden\"]}\"\ndef real\nend\n",
			want: []rubyExpected{
				{"var", "LABEL", 2},
				{"func", "real", 3},
			},
		},
		{
			name: "percent literals",
			text: "NAMES = %w[def class module]\nSYMS = %i(def class)\nTEXT = %q{def hidden}\nNEST = %w[a[def b]c]\ndef real\nend\n",
			want: []rubyExpected{
				{"var", "NAMES", 1},
				{"var", "SYMS", 2},
				{"var", "TEXT", 3},
				{"var", "NEST", 4},
				{"func", "real", 5},
			},
		},
		{
			name: "escaped quote does not end the string early",
			text: "puts 'it\\'s def hidden'\ndef real\nend\n",
			want: []rubyExpected{{"func", "real", 2}},
		},
		{
			name: "hash inside a string is not a comment",
			text: "COLOR = '#fff' # trailing\ndef real\nend\n",
			want: []rubyExpected{
				{"var", "COLOR", 1},
				{"func", "real", 2},
			},
		},
		{
			name: "character literal quote does not open a string",
			text: "quote = ?\"\ndef real\nend\n",
			want: []rubyExpected{{"func", "real", 2}},
		},
		{
			name: "multi-line string swallows its contents",
			text: "BANNER = \"line one\ndef hidden\nline three\"\ndef real\nend\n",
			want: []rubyExpected{
				{"var", "BANNER", 1},
				{"func", "real", 4},
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			assertRubySymbols(t, testCase.text, testCase.want)
		})
	}
}

// TestRubySymbolsIgnoresHeredocBodies is the highest-value negative case. A
// heredoc body is arbitrary text that routinely contains SQL, YAML, or Ruby
// samples; reporting `def` and `class` from one is the failure this extractor
// exists to avoid.
func TestRubySymbolsIgnoresHeredocBodies(t *testing.T) {
	for _, testCase := range []struct {
		name string
		text string
		want []rubyExpected
	}{
		{
			name: "squiggly heredoc",
			text: "def real\n  sample = <<~RUBY\n    def fake_method\n    class FakeClass\n  RUBY\nend\n",
			want: []rubyExpected{{"func", "real", 1}},
		},
		{
			name: "dash heredoc with indented terminator",
			text: "text = <<-EOS\n  def fake_method\n  EOS\ndef real\nend\n",
			want: []rubyExpected{{"func", "real", 4}},
		},
		{
			name: "plain heredoc requires a column-zero terminator",
			text: "text = <<EOS\ndef fake_method\n  EOS\nclass FakeClass\nEOS\ndef real\nend\n",
			want: []rubyExpected{{"func", "real", 6}},
		},
		{
			name: "quoted and interpolating tags",
			text: "raw = <<~'RAW'\n  def fake_raw\nRAW\nlive = <<~\"LIVE\"\n  class FakeLive\nLIVE\ndef real\nend\n",
			want: []rubyExpected{{"func", "real", 7}},
		},
		{
			name: "two heredocs opened on one line run their bodies in order",
			text: "pair = [<<~ONE, <<~TWO]\n  def fake_one\nONE\n  class FakeTwo\nTWO\ndef real\nend\n",
			want: []rubyExpected{{"func", "real", 6}},
		},
		{
			name: "append operator is not a heredoc",
			text: "handlers << Handler\nqueue << item\nclass Real\nend\n",
			want: []rubyExpected{{"type", "Real", 3}},
		},
		{
			name: "code after the heredoc opener is still scanned",
			text: "SQL = <<~TEXT.strip\n  def fake_method\nTEXT\ndef real\nend\n",
			want: []rubyExpected{
				{"var", "SQL", 1},
				{"func", "real", 4},
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			assertRubySymbols(t, testCase.text, testCase.want)
		})
	}
}

// TestRubySymbolsReportsSymbolCap covers the truncation the index cannot
// otherwise observe: a capped file is still indexed and still searchable, so
// without the note its short symbol list is indistinguishable from a file that
// genuinely declares that many methods.
func TestRubySymbolsReportsSymbolCap(t *testing.T) {
	source := rubyTestSource()
	symbols, notes := rubySymbols(source, strings.Repeat("def method_name\nend\n", maxSymbolsPerSource+1))
	if len(symbols) != maxSymbolsPerSource {
		t.Fatalf("symbols = %d, want exactly %d", len(symbols), maxSymbolsPerSource)
	}
	if len(notes) != 1 || notes[0] != (ExtractionNote{source.Path, noteSymbolCap}) {
		t.Fatalf("notes = %v, want one %q note", notes, noteSymbolCap)
	}
}

// TestRubySymbolsReportsLineCap pins the same guarantee for the scan length:
// declarations past the cap were never examined, and the note says so.
func TestRubySymbolsReportsLineCap(t *testing.T) {
	source := rubyTestSource()
	text := strings.Repeat("# pad\n", maxSourceLines) + "def past_the_cap\nend\n"
	symbols, notes := rubySymbols(source, text)
	if len(symbols) != 0 {
		t.Fatalf("symbols = %+v, want none past the line cap", symbols)
	}
	if len(notes) != 1 || notes[0] != (ExtractionNote{source.Path, noteLineCap}) {
		t.Fatalf("notes = %v, want one %q note", notes, noteLineCap)
	}
}

// TestRubySymbolsReportsUnterminatedComment covers a file that ends inside
// `=begin`. Every line after the opener was read as comment text, so any
// declaration among them was skipped and the result is a partial walk.
func TestRubySymbolsReportsUnterminatedComment(t *testing.T) {
	source := rubyTestSource()
	symbols, notes := rubySymbols(source, "def real\nend\n=begin\ndef never_seen\n")
	if len(symbols) != 1 || symbols[0].Name != "real" {
		t.Fatalf("symbols = %+v, want the one declaration before the opener", symbols)
	}
	if len(notes) != 1 || notes[0] != (ExtractionNote{source.Path, noteUnterminatedComment}) {
		t.Fatalf("notes = %v, want one %q note", notes, noteUnterminatedComment)
	}
}

// TestRubySymbolsReportsUnterminatedHeredoc covers a file that ends inside a
// heredoc body. Returning the symbols found before the opener is correct;
// returning them without the note would be the silent-truncation defect.
func TestRubySymbolsReportsUnterminatedHeredoc(t *testing.T) {
	source := rubyTestSource()
	symbols, notes := rubySymbols(source, "QUERY = <<~SQL\n  select 1\n  def never_seen\n")
	if len(symbols) != 1 || symbols[0].Name != "QUERY" {
		t.Fatalf("symbols = %+v, want the one declaration before the opener", symbols)
	}
	if len(notes) != 1 || notes[0] != (ExtractionNote{source.Path, noteUnterminatedString}) {
		t.Fatalf("notes = %v, want one %q note", notes, noteUnterminatedString)
	}
}

// TestRubySymbolsReportsUnterminatedLiteral covers the same end state reached
// through a percent literal rather than a heredoc.
func TestRubySymbolsReportsUnterminatedLiteral(t *testing.T) {
	source := rubyTestSource()
	symbols, notes := rubySymbols(source, "NAMES = %w[\n  alpha\n  def never_seen\n")
	if len(symbols) != 1 || symbols[0].Name != "NAMES" {
		t.Fatalf("symbols = %+v, want the one declaration before the opener", symbols)
	}
	if len(notes) != 1 || notes[0] != (ExtractionNote{source.Path, noteUnterminatedString}) {
		t.Fatalf("notes = %v, want one %q note", notes, noteUnterminatedString)
	}
}

// TestRubySymbolsHandlesEmptyAndNewlineOnlySources pins that a file with
// nothing to declare returns no symbols AND no notes: a note means the walk was
// partial, so emitting one here would make every trivial file look broken.
func TestRubySymbolsHandlesEmptyAndNewlineOnlySources(t *testing.T) {
	for _, text := range []string{"", "\n", "\n\n\n", "# only a comment\n", "def trailing_no_newline"} {
		symbols, notes := rubySymbols(rubyTestSource(), text)
		if len(notes) != 0 {
			t.Errorf("notes for %q = %v, want none", text, notes)
		}
		if text == "def trailing_no_newline" {
			if len(symbols) != 1 || symbols[0].Line != 1 {
				t.Errorf("symbols for %q = %+v, want one at line 1", text, symbols)
			}
			continue
		}
		if len(symbols) != 0 {
			t.Errorf("symbols for %q = %+v, want none", text, symbols)
		}
	}
}
