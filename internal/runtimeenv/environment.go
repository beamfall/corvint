// Package runtimeenv resolves runtime settings from the CORVINT_ environment namespace.
package runtimeenv

import "os"

// Lookup preserves the distinction between an absent and an explicitly empty value.
type Lookup func(string) (string, bool)

// Resolve reads one setting without changing the environment.
func Resolve(lookup Lookup, suffix string) string {
	value, _ := lookup("CORVINT_" + suffix)
	return value
}

// Value reads a setting from the process environment.
func Value(suffix string) string {
	return Resolve(os.LookupEnv, suffix)
}
