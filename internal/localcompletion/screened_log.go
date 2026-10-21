package localcompletion

import (
	"bufio"
	"bytes"
	"strings"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

const (
	// screenedLogMarker opens every secret-screened check log (LCP-V0-031).
	screenedLogMarker = "log-secret-screened\n"
	// maxFailedTests and maxFailedTestName bound the failing test names a
	// verify result carries (LCP-V0-031).
	maxFailedTests    = 16
	maxFailedTestName = 256
)

// screenedLog keeps a secret-screened check log readable (LCP-V0-031): every
// secret-shaped span becomes secretscreen.Placeholder below the
// log-secret-screened marker line, so test names and FAIL lines survive. When
// the redacted text still matches the screen, or would exceed the log bound,
// only the marker is kept, as before.
func screenedLog(raw []byte) []byte {
	redacted, _ := secretscreen.Screen(string(raw))
	if secretscreen.MatchString(redacted) || len(screenedLogMarker)+len(redacted) > maxLogBytes {
		return []byte(screenedLogMarker)
	}
	return []byte(screenedLogMarker + redacted)
}

// failedTestNames returns the distinct Go test names of `--- FAIL: NAME`
// lines in a check's retained logs, in first-seen order and bounded
// (LCP-V0-031). The logs are the retained, already screened bytes, so a name
// never carries a span the screen redacted.
func failedTestNames(logs ...[]byte) []string {
	var names []string
	seen := map[string]bool{}
	for _, log := range logs {
		scanner := bufio.NewScanner(bytes.NewReader(log))
		scanner.Buffer(make([]byte, 0, 64<<10), maxLogBytes)
		for scanner.Scan() {
			line := strings.TrimLeft(scanner.Text(), " \t")
			rest, found := strings.CutPrefix(line, "--- FAIL: ")
			if !found {
				continue
			}
			name, _, _ := strings.Cut(rest, " ")
			if name == "" || len(name) > maxFailedTestName || seen[name] || secretscreen.MatchString(name) {
				continue
			}
			seen[name] = true
			names = append(names, name)
			if len(names) == maxFailedTests {
				return names
			}
		}
	}
	return names
}
