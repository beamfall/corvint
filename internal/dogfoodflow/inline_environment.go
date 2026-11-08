// SPDX-License-Identifier: Apache-2.0

package dogfoodflow

import (
	"context"
	"maps"
	"os"
	"runtime"
	"strings"
)

// ChildEnvironmentMatchesProcess admits an inline command only when Exec would
// give its child exactly the environment already visible to this process. Check
// at each invocation: the context may introduce a run-owned stable reuse store.
// Comparing the entire effective environment fails closed if the child contract
// gains another transformation. Never mutate process-global environment here.
func ChildEnvironmentMatchesProcess(ctx context.Context) bool {
	effective := func(entries []string) map[string]string {
		values := make(map[string]string, len(entries))
		for _, entry := range entries {
			key, value, ok := strings.Cut(entry, "=")
			if !ok {
				continue
			}
			if runtime.GOOS == "windows" {
				key = strings.ToUpper(key)
			}
			values[key] = value // exec.Cmd uses the last value of a duplicate.
		}
		return values
	}
	return maps.Equal(effective(os.Environ()), effective(childEnvironment(ctx)))
}

// CitationEnvironment returns the exact settings Exec would give this
// invocation. An inline citation handler must explicitly account for them;
// comparing only the process environment misses the run-owned reuse store.
func CitationEnvironment(ctx context.Context) []string {
	return childEnvironment(ctx)
}
