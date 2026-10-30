package localcompletion

import (
	"bufio"
	"bytes"
	"regexp"
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
// the redacted text still matches the screen, would exceed the log bound, or
// the raw log carries a private-key block header, only the marker is kept, as
// before. A header without its footer matches the screen as the header alone,
// so redaction would keep the key body; any key header therefore fails closed.
func screenedLog(raw []byte) []byte {
	if privateKeyHeader.Match(raw) {
		return []byte(screenedLogMarker)
	}
	redacted, _ := secretscreen.Screen(string(raw))
	if secretscreen.MatchString(redacted) || len(screenedLogMarker)+len(redacted) > maxLogBytes {
		return []byte(screenedLogMarker)
	}
	return []byte(screenedLogMarker + redacted)
}

// privateKeyHeader matches the PEM and PGP private-key block headers the
// screen recognizes (internal/secretscreen), complete block or not.
var privateKeyHeader = regexp.MustCompile(`(?i)-----BEGIN [A-Z ]*PRIV[A]TE KEY(?: BLOCK)?-----`)

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
