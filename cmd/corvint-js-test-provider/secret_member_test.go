package main

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

// PWP-V3-012: a secret-shaped refusal adds one pointer line after the unchanged refusal line;
// other errors add nothing.
func TestPWPV3012ReportSecretShapedMember(t *testing.T) {
	var out bytes.Buffer
	refusal := &jstestprovider.QualifiedSecretError{Pointer: "/receipt/tests/0/failureMessage"}
	reportSecretShapedMember(&out, fmt.Errorf("wrapped: %w", refusal))
	if got := out.String(); got != "secret-shaped member: /receipt/tests/0/failureMessage\n" {
		t.Fatalf("diagnostic %q", got)
	}
	out.Reset()
	reportSecretShapedMember(&out, &jstestprovider.QualifiedSecretError{})
	if got := out.String(); got != "secret-shaped member: (document root)\n" {
		t.Fatalf("root diagnostic %q", got)
	}
	out.Reset()
	reportSecretShapedMember(&out, errors.New("qualified-document-output-overflow"))
	if out.Len() != 0 {
		t.Fatalf("unrelated error printed %q", out.String())
	}
}

// PWP-V3-016: a report-identity-unknown refusal adds one line naming the tests without an
// identity by pointer, never by title; other refusals add nothing.
func TestPWPV3016ReportProjectLocationUnknown(t *testing.T) {
	test := func(browser string) jstestprovider.TestOutcome {
		return jstestprovider.TestOutcome{Name: "secret-title", Anchor: &jstestprovider.Anchor{File: "/repo/a.spec.ts", Line: 1}, Project: &jstestprovider.ProjectIdentity{Name: "one", Browser: browser}}
	}
	receipt := jstestprovider.Receipt{Profile: jstestprovider.ExternalProfile, Tests: []jstestprovider.TestOutcome{test("chromium"), test("")}, Infrastructure: &jstestprovider.InfrastructureFailure{Reason: "report-identity-unknown", Detail: "project-location-unknown"}}
	var out bytes.Buffer
	reportProjectLocationUnknown(&out, receipt)
	if got := out.String(); got != "project-location-unknown tests: /receipt/tests/1\n" {
		t.Fatalf("diagnostic %q", got)
	}
	out.Reset()
	receipt.Infrastructure.Reason = "reporter"
	reportProjectLocationUnknown(&out, receipt)
	if out.Len() != 0 {
		t.Fatalf("unrelated refusal printed %q", out.String())
	}
}
