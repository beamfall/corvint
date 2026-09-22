package contextindex

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/runtimeenv"
)

// TCP-V0-019 is an opt-in query-side experiment. The field weights are frozen
// before the development-fold runs; they are not fitted to repository labels.
const contextExactTermWeight = 3.0

var contextCodeStem = regexp.MustCompile(`([A-Za-z0-9_\-]+)\.(?:go|py|rs|ts|tsx|js|java|kt|rb|cs|cpp|c|h)\b`)

type contextTermSelection struct {
	weights     map[string]float64
	identifiers []taskIdentifier
}

func configureContextTerms(compiler *taskContextCompiler) *taskContextCompiler {
	if runtimeenv.Value("CONTEXT_TERMS") != "ident" {
		return compiler
	}
	text := contextQueryValues(compiler.task)
	compiler.selectedTerms = selectContextTerms(text)
	compiler.terms = sortedStringKeys(compiler.selectedTerms.weights)
	return compiler
}

// A valid JSON envelope contributes string values, never field names. Invalid
// JSON stays plain text; no benchmark field or label vocabulary is recognised.
func contextQueryValues(task string) string {
	var value any
	if json.Unmarshal([]byte(task), &value) != nil {
		return task
	}
	var values []string
	appendContextQueryValues(value, &values)
	return strings.Join(values, "\n")
}

func appendContextQueryValues(value any, values *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range sortedStringKeys(typed) {
			appendContextQueryValues(typed[key], values)
		}
	case []any:
		for _, child := range typed {
			appendContextQueryValues(child, values)
		}
	case string:
		*values = append(*values, typed)
	}
}

func selectContextTerms(text string) *contextTermSelection {
	selected := &contextTermSelection{weights: map[string]float64{}}
	names := map[string]bool{}
	for _, token := range contextIdentifier.FindAllString(text, -1) {
		if len(token) < 4 {
			continue
		}
		if hasInnerCamelBoundary(token) || strings.Contains(strings.Trim(token, "_"), "_") {
			names[token] = true
		}
	}
	for _, match := range contextCodeStem.FindAllStringSubmatch(text, -1) {
		if len(match[1]) >= 4 {
			names[match[1]] = true
		}
	}
	for _, name := range sortedStringKeys(names) {
		selected.identifiers = append(selected.identifiers, taskIdentifier{name: name, weight: 1})
		for _, term := range lexicalTerms(name) {
			selected.weights[term] = max(selected.weights[term], 1)
		}
		selected.weights[strings.ToLower(name)] = contextExactTermWeight
	}
	if len(names) == 0 {
		for _, term := range taskLexicalTerms(text) {
			selected.weights[term] = 1
		}
	}
	return selected
}

func (compiler *taskContextCompiler) lexicalIdentifiers() []taskIdentifier {
	if compiler.selectedTerms != nil {
		return compiler.selectedTerms.identifiers
	}
	return compiler.identifiers
}

func (compiler *taskContextCompiler) queryTermGain(field int, term string, gain float64) float64 {
	if compiler.selectedTerms == nil {
		return gain
	}
	if field == 2 {
		return gain * contextExactTermWeight
	}
	return gain * compiler.selectedTerms.weights[term]
}

func sortedStringKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
