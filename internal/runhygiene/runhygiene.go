// Package runhygiene is the fail-safe test-attempt hygiene (AFU-V1-038) shared by the run-evidence
// adapters (internal/appflows) and the Playwright provider receipt (internal/jstestprovider). It does
// not parse headers: a line that mentions a sensitive name anywhere, in any case or quoting, is
// replaced whole; a request or response body marker anywhere in a line keeps only the text before it
// and ends the detail; and an attachment that may hold cookies, credentials, a request or a response
// is dropped.
package runhygiene

import (
	"path"
	"regexp"
	"strings"
)

// DroppedMarker replaces a dropped failure line.
const DroppedMarker = "[dropped by run-evidence hygiene]"

var (
	// sensitiveLine matches the names inside any word, so `access_token`, `csrfToken` and `sessionId` count.
	sensitiveLine       = regexp.MustCompile(`(?i)cookie|authori[sz]ation|bearer|token|secret|passw(or)?d|api[-_]?key|csrf|session|credential|\bbasic\s+[a-z0-9+/]{4,}`)
	bodyMarker          = regexp.MustCompile(`(?i)\b(request|response)(\s*(body|text|data|payload))?\s*:`)
	sensitiveAttachment = regexp.MustCompile(`(?i)cookie|authori[sz]ation|token|secret|passw(or)?d|session|credential|request|response|body|storage-?state`)
)

// ScrubFailure drops every sensitive line and cuts the detail at the first body marker.
func ScrubFailure(text string) string {
	kept := []string{}
	for _, line := range strings.Split(strings.ReplaceAll(strings.ToValidUTF8(text, "�"), "\x00", ""), "\n") {
		body := bodyMarker.FindStringIndex(line)
		if body != nil {
			line = line[:body[0]]
		}
		if sensitiveLine.MatchString(line) {
			line = DroppedMarker
		}
		if body != nil {
			return strings.Join(append(kept, strings.TrimSuffix(line, DroppedMarker)+DroppedMarker), "\n")
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// KeepAttachment keeps a path pointer only: inline content has no path and is never read, and a name
// or file that may hold cookies, credentials, a request or a response, or a HAR file, is dropped.
func KeepAttachment(name, file string) bool {
	return file != "" && !sensitiveAttachment.MatchString(name+" "+path.Base(file)) && !strings.HasSuffix(strings.ToLower(file), ".har")
}
