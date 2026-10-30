package testvalidity

import "strings"

// Stub-decided assertions and feature-flag conditions of RSpec examples
// (docs/specs/test-source-analysis-v0.md, TSA-V0-430..442). internal/testsource reads them from
// committed spec source; this file holds the vocabulary.

// Stub and flag rule IDs.
const (
	// RuleRBStubDecidesAssertion: a negated change matcher observes a receiver whose operation or
	// observed method a stub in the example or its hooks replaces, so the stub decides the
	// assertion. Reachability is not read, so the example abstains (TSA-V0-431).
	RuleRBStubDecidesAssertion = "RB-STUB-DECIDES-ASSERTION"
	// RuleRBMessageChainPrefixOnly: a receive_message_chain stub is asserted only on a prefix of
	// its chain, and its final message is never asserted (TSA-V0-433).
	RuleRBMessageChainPrefixOnly = "RB-MESSAGE-CHAIN-PREFIX-ONLY"
	// RuleRBFlagConditional: the example forces a feature-flag or configuration lookup through a
	// stub, so what it proves is conditional on that flag value; advisory (TSA-V0-436).
	RuleRBFlagConditional = "RB-FLAG-CONDITIONAL"
	// RuleRBFlagForcedNonDefault: the forced flag value differs from the project-declared default;
	// advisory (TSA-V0-438).
	RuleRBFlagForcedNonDefault = "RB-FLAG-FORCED-NON-DEFAULT"
)

const (
	// FlagLookupsSchema names the committed flag lookup declaration (TSA-V0-439).
	FlagLookupsSchema = "corvint-flag-lookups/0"
	// FlagLookupsPath is the declaration's repository path.
	FlagLookupsPath = ".corvint/flag-lookups.json"
	// FlagAnyKey is the key of a flag stub without a `.with` constraint: every key is forced.
	FlagAnyKey = "*"
	// flagUnresolved stands for a key or forced value the reader cannot read as a literal.
	flagUnresolved = "<unresolved>"
)

// FlagLookup is one feature-flag or configuration lookup: a constant receiver path and a method.
type FlagLookup struct {
	Receiver string `json:"receiver"`
	Method   string `json:"method"`
}

// String is the lookup's `Receiver.method` form.
func (l FlagLookup) String() string { return l.Receiver + "." + l.Method }

// FlagLookups is the lookup list stubs are classified against and the project-declared code
// defaults by flag key. Source is the declaration's path, or "" for the built-in list.
type FlagLookups struct {
	Schema   string            `json:"schema"`
	Lookups  []FlagLookup      `json:"lookups"`
	Defaults map[string]string `json:"defaults,omitempty"`
	Source   string            `json:"-"`
}

// BuiltinFlagLookups is the list used when no declaration is committed (TSA-V0-435): the common
// flag clients' enabled? (and FeatureToggle.active?) and ENV.fetch. It declares no defaults.
func BuiltinFlagLookups() FlagLookups {
	lookups := []FlagLookup{}
	for _, receiver := range []string{"Feature", "FeatureFlag", "FeatureFlags", "FeatureToggle", "Features", "Flipper"} {
		lookups = append(lookups, FlagLookup{Receiver: receiver, Method: "enabled?"})
	}
	lookups = append(lookups, FlagLookup{Receiver: "FeatureToggle", Method: "active?"}, FlagLookup{Receiver: "ENV", Method: "fetch"})
	return FlagLookups{Schema: FlagLookupsSchema, Lookups: lookups}
}

// Match returns the lookup a stub of method on the class or constant receiver is, if any.
func (f FlagLookups) Match(receiver, method string) (FlagLookup, bool) {
	receiver = strings.TrimPrefix(receiver, "::")
	for _, l := range f.Lookups {
		if l.Receiver == receiver && l.Method == method {
			return l, true
		}
	}
	return FlagLookup{}, false
}

// FlagCondition is one flag value an example forces (TSA-V0-437). Key is the flag key, FlagAnyKey
// when the stub constrains no argument, or "" when it is not a literal; Forced is the forced value,
// or "" when it is not one literal. Default and DefaultFrom are the project-declared default and
// the file declaring it, when declared. Qualifier is "conditional on flag KEY=VALUE".
type FlagCondition struct {
	Lookup      string `json:"lookup"`
	Key         string `json:"key"`
	Forced      string `json:"forced"`
	Default     string `json:"default,omitempty"`
	DefaultFrom string `json:"defaultFrom,omitempty"`
	Location    string `json:"location"`
	Qualifier   string `json:"qualifier"`
}

// FlagQualifier is the condition's qualifier text; an unread key or value is "<unresolved>" and
// a stub on every key names the lookup.
func FlagQualifier(lookup, key, forced string) string {
	switch key {
	case "":
		key = flagUnresolved
	case FlagAnyKey:
		key = lookup + "(*)"
	}
	if forced == "" {
		forced = flagUnresolved
	}
	return "conditional on flag " + key + "=" + forced
}
