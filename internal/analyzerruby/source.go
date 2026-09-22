package analyzerruby

import (
	"bytes"
	"strings"
)

// sourceObservation is the closed result of scanning one `.rb` input.
//
// The matrix admits `require "rails"`, `RSpec.describe`, and `Cucumber` as the
// only recognizable static tokens, but it gives `require "rails"` nothing to
// populate: there is no framework fact kind at all, and `classifies` already
// defaults to `source`. Detecting it could therefore only change a
// classification by overriding the test-path convention, which would misread a
// `spec/rails_helper.rb` that requires Rails as production source. The spec's
// "may yield" is permissive and its "never a framework-version fact" is a
// prohibition, so emitting nothing for that token satisfies both -- and
// scanning for it would be computation charged against ACP-009 that no fact
// can consume.
type sourceObservation struct {
	rspec    bool
	cucumber bool
}

// classification is the `ruby.source` value: exactly `source` or `test`.
func (o sourceObservation) classification(path string) string {
	if o.rspec || o.cucumber || testPath(path) {
		return "test"
	}
	return "source"
}

// scanSource masks the file, then reads the closed token set out of the code
// view only.
func scanSource(path string, body []byte) (sourceObservation, string) {
	if !strings.HasSuffix(path, ".rb") {
		return sourceObservation{}, "UNSUPPORTED_SCHEMA"
	}
	if overBound(len(body), MaxSourceBytes) {
		return sourceObservation{}, "LIMIT_EXCEEDED"
	}
	masked, why := maskRuby(body)
	if why != "" {
		return sourceObservation{}, why
	}
	return sourceObservation{
		rspec:    containsToken(masked, "RSpec.describe"),
		cucumber: containsToken(masked, "Cucumber"),
	}, ""
}

// containsToken reports an identifier-bounded occurrence in the masked code
// view. Masked spans are spaces, so a token inside a comment, string, heredoc,
// percent literal, regex, or data section can never match.
func containsToken(masked []byte, token string) bool {
	needle := []byte(token)
	for at := 0; ; {
		found := bytes.Index(masked[at:], needle)
		if found < 0 {
			return false
		}
		found += at
		before := found == 0 || !identifierByte(masked[found-1])
		after := found+len(needle) >= len(masked) || !identifierByte(masked[found+len(needle)])
		if before && after {
			return true
		}
		at = found + 1
	}
}

// testPath is the conventional Ruby test layout, carried over from the
// reference implementation: a `_test.rb`/`_spec.rb` basename, or any
// `test`/`tests`/`spec` path segment.
func testPath(path string) bool {
	segments := strings.Split(path, "/")
	name := strings.ToLower(segments[len(segments)-1])
	if strings.HasSuffix(name, "_test.rb") || strings.HasSuffix(name, "_spec.rb") {
		return true
	}
	for _, segment := range segments {
		switch strings.ToLower(segment) {
		case "test", "tests", "spec":
			return true
		}
	}
	return false
}
