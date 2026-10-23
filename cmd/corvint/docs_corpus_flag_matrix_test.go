package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// corpusFlagExpectation is one operation's admitted options, written out literally from the
// parser at a0e56f3c1 (its allowed and required maps) rather than read from corpusOperations, so
// the shared table cannot vouch for itself. import-cases and search-cases come from the
// claude/issues-812-813-legacy-cases parser (DCP-V1-070..079), and discover-legacy from the
// claude/issue-811-legacy-inventory parser (DCP-V1-060..069); both predate the table.
type corpusFlagExpectation struct{ required, optional []string }

var corpusReadOptional = []string{"--limit", "--offset", "--retirement", "--caller-revision"} // DCP-V1-332

var corpusFlagExpectations = map[string]corpusFlagExpectation{
	"manifest":            {required: []string{"--revision", "--scope", "--timestamp"}, optional: []string{"--schema"}},
	"build":               {required: []string{"--manifest"}},
	"render":              {required: []string{"--artifact"}},
	"maintain":            {required: []string{"--artifact", "--page"}, optional: []string{"--apply"}},
	"cem":                 {required: []string{"--artifact", "--cem"}, optional: []string{"--id"}},
	"behavior-adapter":    {required: []string{"--input"}, optional: []string{"--previous", "--check"}},
	"behavior-provider":   {required: []string{"--input"}},
	"discover-playwright": {required: []string{"--migration", "--config", "--playwright-list"}, optional: []string{"--receipt"}},
	"witness-playwright":  {required: []string{"--input", "--receipt-input", "--report"}},
	"discover-legacy":     {required: []string{"--runner", "--revision", "--path"}, optional: []string{"--input"}},
	"import-cases":        {required: []string{"--format", "--input"}, optional: []string{"--content"}},
	"search-cases":        {required: []string{"--format", "--input"}, optional: []string{"--query", "--id", "--match", "--limit"}},
	"eval":                {required: []string{"--artifact", "--cases"}, optional: []string{"--baseline", "--format"}},
	"audit-sample":        {required: []string{"--artifact", "--seed", "--size"}, optional: []string{"--stratify", "--scope"}},
	"audit-report":        {required: []string{"--artifact", "--sample"}, optional: []string{"--format"}},
	"delta":               {required: []string{"--previous"}, optional: []string{"--artifact", "--manifest", "--limit", "--offset", "--uncovered", "--anchorless", "--format"}},
	"carry-forward":       {required: []string{"--previous", "--provider"}, optional: []string{"--artifact", "--manifest"}},
	"info":                {required: []string{"--artifact"}, optional: corpusReadOptional},
	"validate":            {required: []string{"--artifact"}, optional: corpusReadOptional},
	"inventory":           {required: []string{"--artifact"}, optional: corpusReadOptional},
	"coverage":            {required: []string{"--artifact"}, optional: corpusReadOptional},
	"gaps":                {required: []string{"--artifact"}, optional: append([]string{"--id", "--kind"}, corpusReadOptional...)},                 // DCP-V1-321
	"search":              {required: []string{"--artifact", "--query"}, optional: append([]string{"--include", "--kind"}, corpusReadOptional...)}, // DCP-V1-328, DCP-V1-331
	"concept":             {required: []string{"--artifact", "--query"}, optional: corpusReadOptional},
	"vocabulary":          {required: []string{"--artifact", "--query"}, optional: corpusReadOptional},
	"locate":              {required: []string{"--artifact", "--path"}, optional: append([]string{"--repository"}, corpusReadOptional...)},
	"claims":              {required: []string{"--artifact"}, optional: append([]string{"--path", "--query", "--state", "--qualifier", "--run"}, corpusReadOptional...)}, // DCP-V1-313, DCP-V1-318: exactly one form, checked after parsing
	"recommend-tests":     {required: []string{"--artifact", "--path"}, optional: append([]string{"--repository"}, corpusReadOptional...)},
	"get":                 {required: []string{"--artifact", "--id"}, optional: corpusReadOptional},
	"trace":               {required: []string{"--artifact", "--id"}, optional: corpusReadOptional},
	"related":             {required: []string{"--artifact", "--id"}, optional: corpusReadOptional},
	"journey":             {required: []string{"--artifact", "--id"}, optional: corpusReadOptional},
	"stability":           {required: []string{"--artifact", "--id"}, optional: corpusReadOptional},
	"flow":                {required: []string{"--artifact", "--id"}, optional: append([]string{"--section", "--include"}, corpusReadOptional...)},
	"dependencies":        {required: []string{"--artifact", "--id"}, optional: corpusReadOptional},
	"navigation":          {required: []string{"--artifact", "--id"}, optional: corpusReadOptional},
	"intent":              {required: []string{"--artifact"}, optional: append([]string{"--id"}, corpusReadOptional...)}, // DCP-V1-319
}

var corpusAllFlags = []string{"--revision", "--scope", "--timestamp", "--manifest", "--artifact", "--page", "--apply",
	"--cem", "--id", "--input", "--previous", "--check", "--migration", "--config", "--playwright-list", "--receipt",
	"--receipt-input", "--report", "--query", "--path", "--limit", "--offset", "--retirement", "--format", "--content", "--runner", "--match", "--schema", "--section", "--include", "--cases", "--baseline", "--seed", "--size", "--stratify", "--sample", "--repository", "--kind", "--caller-revision"}

func corpusFlagArgs(op, flag string) []string {
	switch flag {
	case "--apply", "--check":
		return []string{flag}
	case "--limit", "--offset":
		return []string{flag, "1"}
	case "--schema":
		return []string{flag, "2"}
	case "--section":
		return []string{flag, "readme"}
	case "--caller-revision":
		return []string{flag, strings.Repeat("a", 40)}
	case "--include":
		if op == "search" {
			return []string{flag, "claims"}
		}
		return []string{flag, "paragraphs"}
	}
	return []string{flag, "x.json"}
}

func corpusParseError(args []string) string {
	// The package directory sits two levels under the repository root, which the parser
	// resolves only after every option check.
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	_, handled, err := parseCorpusInvocation(append([]string{"--root", root, "docs", "corpus"}, args...))
	switch {
	case !handled:
		return "not handled"
	case err != nil:
		return err.Error()
	}
	return ""
}

// DCP-V1-056: an independent accept/reject matrix over every corpus operation. Each documented
// option is admitted, each other option is refused, each required option is enforced, and the
// refusal precedence holds: an unknown option, then an unknown operation, then an option the
// operation does not take, then a missing required option.
func TestDocsCorpusOperationFlagMatrix(t *testing.T) {
	t.Parallel()
	if len(corpusFlagExpectations) != len(issue805CorpusOperations) {
		t.Fatalf("expectations cover %d operations, want %d", len(corpusFlagExpectations), len(issue805CorpusOperations))
	}
	for _, op := range issue805CorpusOperations {
		want, ok := corpusFlagExpectations[op]
		if !ok {
			t.Fatalf("no expectation for %s", op)
		}
		base := []string{op}
		for _, flag := range want.required {
			base = append(base, corpusFlagArgs(op, flag)...)
		}
		if got := corpusParseError(base); got != "" {
			t.Errorf("%s with required options refused: %s", op, got)
		}
		for _, flag := range corpusAllFlags {
			if slices.Contains(want.required, flag) {
				continue
			}
			got := corpusParseError(append(slices.Clone(base), corpusFlagArgs(op, flag)...))
			if slices.Contains(want.optional, flag) {
				if got != "" {
					t.Errorf("%s %s refused: %s", op, flag, got)
				}
			} else if got != "option does not apply to corpus operation" {
				t.Errorf("%s %s = %q, want option does not apply", op, flag, got)
			}
		}
		for index, flag := range want.required {
			args := []string{op}
			for other, kept := range want.required {
				if other != index {
					args = append(args, corpusFlagArgs(op, kept)...)
				}
			}
			if got := corpusParseError(args); got != "missing required corpus option" {
				t.Errorf("%s without %s = %q", op, flag, got)
			}
			// An inapplicable option is refused before the missing required one.
			stray := "--revision"
			if op == "manifest" || op == "discover-legacy" {
				stray = "--artifact"
			}
			if got := corpusParseError(append(args, corpusFlagArgs(op, stray)...)); got != "option does not apply to corpus operation" {
				t.Errorf("%s without %s plus %s = %q", op, flag, stray, got)
			}
		}
	}
	for _, check := range []struct {
		args []string
		want string
	}{
		{[]string{"nope", "--bogus", "x"}, "unsupported corpus option"},
		{[]string{"nope", "--help"}, "unsupported corpus option"},
		{[]string{"nope", "--artifact", "x.json"}, "unsupported corpus operation"},
		{[]string{"nope"}, "unsupported corpus operation"},
		{[]string{"search", "--bogus", "x"}, "unsupported corpus option"},
		{[]string{"search", "--query", "a", "--query", "b"}, "duplicate corpus option"},
		{[]string{"search", "--artifact", "x.json", "--query", " "}, "missing required corpus option"},
		{[]string{}, "docs corpus requires an operation"},
	} {
		if got := corpusParseError(check.args); got != check.want {
			t.Errorf("%q = %q, want %q", check.args, got, check.want)
		}
	}
}

// DCP-V1-174: the flow filters parse as comma-separated lists and a malformed
// filter is an argument error before any artifact is read.
func TestDCPV1174CorpusFlowFilterOptions(t *testing.T) {
	t.Parallel()
	base := []string{"flow", "--artifact", "x.json", "--id", "adapter:flow"}
	for _, extra := range [][]string{{"--section", "readme,entities"}, {"--include", "claims,relations"}, {"--section=readme", "--include=paragraphs,claims"}} {
		if got := corpusParseError(append(slices.Clone(base), extra...)); got != "" {
			t.Errorf("%v refused: %s", extra, got)
		}
	}
	for _, extra := range [][]string{{"--include", "subjects"}, {"--include", "claims,claims"}, {"--section", "readme,"}, {"--section", "readme", "--include", "claims"}} {
		if got := corpusParseError(append(slices.Clone(base), extra...)); got != "invalid flow section or include filter" {
			t.Errorf("%v = %q, want invalid filter", extra, got)
		}
	}
}
