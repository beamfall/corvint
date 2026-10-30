package testsource

// The environment-contract projection of `corvint tests validity --profile NAME`
// (docs/specs/test-source-analysis-v0.md, TSA-V0-363): the profile's hard-coded-address warnings
// on the named files, each attributed to the test that contains it. The projection is advisory:
// it changes no test's hygiene axis and no exit status.

import "fmt"

// SourceValidityEnv is the profile's warnings on the named files.
type SourceValidityEnv struct {
	Profile  string                     `json:"profile"`
	Reason   string                     `json:"reason,omitempty"`
	Warnings []SourceValidityEnvWarning `json:"warnings"`
}

// SourceValidityEnvWarning is one warning with the `path:line` test id of the innermost test that
// contains it, the id the report's tests use; Test is empty outside every test.
type SourceValidityEnvWarning struct {
	Test string `json:"test,omitempty"`
	EnvWarning
}

func envValidity(r Repo, commit, profile string, files []string) (*SourceValidityEnv, error) {
	texts, err := readEnvTexts(r, commit, profile, newReadBudget())
	if err != nil {
		return nil, err
	}
	report := ReadEnvContract(texts, profile)
	out := &SourceValidityEnv{Profile: profile, Reason: report.Reason, Warnings: []SourceValidityEnvWarning{}}
	named := map[string]bool{}
	for _, name := range files {
		named[name] = true
	}
	scopes := map[string][][2]int{}
	for _, w := range report.Warnings {
		if !named[w.Path] {
			continue
		}
		if _, ok := scopes[w.Path]; !ok {
			scopes[w.Path] = testScopes(w.Path, texts[w.Path])
		}
		warning := SourceValidityEnvWarning{EnvWarning: w}
		if span, ok := innermostTest(scopes[w.Path], w.Line); ok {
			warning.Test = fmt.Sprintf("%s:%d", w.Path, span[0])
		}
		out.Warnings = append(out.Warnings, warning)
	}
	return out, nil
}
