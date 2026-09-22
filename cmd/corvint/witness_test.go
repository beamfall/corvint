package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
)

func TestParseWitnessInvocationInterceptsAheadOfTheSharedParser(t *testing.T) {
	t.Parallel()
	repository := leaseRoot(t)
	cases := []struct {
		name    string
		input   []string
		match   bool
		rest    []string
		rootSet bool
	}{
		{"bare verb", []string{"witness", "--base", "abc"}, true, []string{"--base", "abc"}, false},
		{"separate root", []string{"--root", repository, "witness", "--base", "abc"}, true, []string{"--base", "abc"}, true},
		{"inline root", []string{"--root=" + repository, "witness"}, true, []string{}, true},
		{"another verb", []string{"impact", "--base", "abc"}, false, nil, false},
		{"verb not first", []string{"impact", "witness"}, false, nil, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root, rest, matched, err := parseWitnessInvocation(testCase.input)
			if matched != testCase.match {
				t.Fatalf("matched = %t, want %t", matched, testCase.match)
			}
			if !matched {
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.Join(rest, " ") != strings.Join(testCase.rest, " ") {
				t.Errorf("rest = %v, want %v", rest, testCase.rest)
			}
			// normalizeRoot resolves symlinks, so the temporary root may arrive under /private.
			if testCase.rootSet && !strings.HasSuffix(root, repository) {
				t.Errorf("root = %q, want a resolved %q", root, repository)
			}
		})
	}
}

func TestParseWitnessOptions(t *testing.T) {
	t.Parallel()
	options, err := parseWitnessOptions([]string{"--base", "abc", "--head=def", "--cem", "x.json", "--json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if options.base != "abc" || options.head != "def" || options.cem != "x.json" || !options.json {
		t.Fatalf("options = %+v", options)
	}
}

// A bare `--json` must be accepted and only an actual value refused. lane-plan
// once got this wrong by reporting a bare flag as inline; both parsers now
// treat the boolean as "an inline value was supplied" and nothing else.
func TestParseWitnessOptionsAcceptsBareJSON(t *testing.T) {
	t.Parallel()
	options, err := parseWitnessOptions([]string{"--base", "abc", "--json"})
	if err != nil {
		t.Fatalf("--json must be accepted without a value: %v", err)
	}
	if !options.json {
		t.Error("--json did not set the JSON flag")
	}
	if _, err := parseWitnessOptions([]string{"--base", "abc", "--json=1"}); err == nil {
		t.Error("--json=1 must be refused")
	}
}

func TestParseWitnessOptionsRejections(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		"missing base":      {"--json"},
		"unknown option":    {"--base", "abc", "--depth", "2"},
		"missing value":     {"--base"},
		"missing cem value": {"--base", "abc", "--cem"},
	}
	for name, arguments := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseWitnessOptions(arguments); err == nil {
				t.Errorf("%v must be refused", arguments)
			}
		})
	}
}

func TestRunWitnessRefusesAHeadOtherThanTheCapturedRevision(t *testing.T) {
	t.Parallel()
	root := cliRepository(t)
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	code := runWitness(t.Context(), root, []string{"--base", strings.Repeat("a", 40), "--head", strings.Repeat("b", 40)}, stdout, stderr)
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "must name the captured HEAD commit") {
		t.Errorf("stderr = %s", stderr.String())
	}
}

func TestRunWitnessRefusesAnUnknownBase(t *testing.T) {
	t.Parallel()
	root := cliRepository(t)
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := runWitness(t.Context(), root, []string{"--base", strings.Repeat("a", 40)}, stdout, stderr); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("a refused range must print no report, got %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "available commit object") {
		t.Errorf("stderr = %s", stderr.String())
	}
}

// An unknown option is named as unknown even when no value follows it.
func TestParseWitnessOptionsNamesAnUnknownOptionBeforeItsValue(t *testing.T) {
	t.Parallel()
	for _, arguments := range [][]string{{"--base", "abc", "--bogus"}, {"--bogus", "--json"}, {"--bogus", "x"}} {
		_, err := parseWitnessOptions(arguments)
		if err == nil || err.Error() != "unrecognized witness option: --bogus" {
			t.Errorf("%v: err = %v", arguments, err)
		}
	}
}

// AGW-V0-001: an explicit --root that is not a Git repository is refused as an
// argument error before the index builds, as the sibling native verbs do.
func TestParseWitnessInvocationRefusesARootThatIsNotARepository(t *testing.T) {
	t.Parallel()
	_, _, matched, err := parseWitnessInvocation([]string{"--root", filepath.Join(t.TempDir(), "missing"), "witness", "--base", "abc"})
	var kernelError *gokernel.Error
	if !matched || !errors.As(err, &kernelError) || kernelError.Code != "invalid-arguments" ||
		!strings.HasPrefix(kernelError.Message, "not a Git repository: ") {
		t.Fatalf("matched = %t err = %v", matched, err)
	}
}
