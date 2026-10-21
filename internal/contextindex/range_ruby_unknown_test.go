package contextindex

import (
	"context"
	"reflect"
	"testing"
)

// rangeReceiptOf commits each changed path's replacement over files and
// returns the committed range receipt.
func rangeReceiptOf(t *testing.T, files map[string]string, changes map[string]string) map[string]any {
	t.Helper()
	root := impactRepositoryWithFiles(t, files)
	base := testGit(t, root, "rev-parse", "HEAD")
	for changed, replacement := range changes {
		writeTestFile(t, root, changed, replacement)
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "changed")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	ranged, err := RangeImpact(context.Background(), index, base, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	return ranged
}

func rangeUncertainty(receipt map[string]any) []any {
	return anySlice(receipt["coverage"].(map[string]any)["uncertainty"])
}

// TestNGIV0059RubyRangeReportsUnknownVerification: NGI-V0-059. A committed
// range over a Ruby path in a repository that proves no Ruby runner carries the
// same unknown-language line path impact reports for that path; a proved
// runner drops it from both.
func TestNGIV0059RubyRangeReportsUnknownVerification(t *testing.T) {
	t.Parallel()
	const changed = "app/api_clients/foo_client.rb"
	edited := "class FooClient\n  def fetch = :changed\nend\n"
	for _, test := range []struct {
		name        string
		files       map[string]string
		wantUnknown bool
	}{
		{"no Gemfile", rubyVerificationFiles(nil), true},
		{"Minitest without runner", rubyVerificationFiles(map[string]string{"Gemfile": "gem \"minitest\"\n"}), true},
		{"RSpec proved", rubyVerificationFiles(map[string]string{"Gemfile": "gem \"rspec\"\n"}), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, pathUncertainty := impactVerificationOf(t, test.files, changed)
			if got := anyContains(pathUncertainty, unknownLanguageVerification); got != test.wantUnknown {
				t.Fatalf("path impact uncertainty = %v, want unknown-language line %v", pathUncertainty, test.wantUnknown)
			}
			ranged := rangeReceiptOf(t, test.files, map[string]string{changed: edited})
			uncertainty := rangeUncertainty(ranged)
			if got := anyContains(uncertainty, unknownLanguageVerification); got != test.wantUnknown {
				t.Fatalf("range uncertainty = %v (verification %v), want unknown-language line %v",
					uncertainty, ranged["verification"], test.wantUnknown)
			}
		})
	}
}

// TestNGIV0059RangeWithoutRubyPathKeepsUncertainty: NGI-V0-059. A range that
// admits no Ruby path, or whose plan names a Go command, never gains the line:
// Go range receipts keep their exact bytes in a Ruby repository with no runner.
func TestNGIV0059RangeWithoutRubyPathKeepsUncertainty(t *testing.T) {
	t.Parallel()
	goFiles := map[string]string{
		"go.mod":                    "module example.test/widget\n\ngo 1.24\n",
		"internal/widget/widget.go": "package widget\n\nfunc Widget() int { return 1 }\n",
	}
	withRuby := rubyVerificationFiles(nil)
	for name, body := range goFiles {
		withRuby[name] = body
	}
	goEdit := map[string]string{"internal/widget/widget.go": "package widget\n\nfunc Widget() int { return 2 }\n"}
	plain := rangeReceiptOf(t, goFiles, goEdit)
	ruby := rangeReceiptOf(t, withRuby, goEdit)
	for _, receipt := range []map[string]any{plain, ruby} {
		if anyContains(rangeUncertainty(receipt), unknownLanguageVerification) {
			t.Fatalf("Go range gained the unknown-language line: %v", rangeUncertainty(receipt))
		}
	}
	if !reflect.DeepEqual(rangeUncertainty(plain), rangeUncertainty(ruby)) ||
		!reflect.DeepEqual(plain["verification"], ruby["verification"]) {
		t.Fatalf("Go range changed by untouched Ruby files: plain=%v/%v ruby=%v/%v",
			plain["verification"], rangeUncertainty(plain), ruby["verification"], rangeUncertainty(ruby))
	}
	// A Ruby path beside a Go command: the plan names a language command, so
	// path and range impact both omit the line.
	mixed := rangeReceiptOf(t, withRuby, map[string]string{
		"internal/widget/widget.go":     goEdit["internal/widget/widget.go"],
		"app/api_clients/foo_client.rb": "class FooClient\n  def fetch = :changed\nend\n",
	})
	if anyContains(rangeUncertainty(mixed), unknownLanguageVerification) {
		t.Fatalf("mixed Go and Ruby range gained the line: %v (verification %v)", rangeUncertainty(mixed), mixed["verification"])
	}
	// A docs-only range in a Ruby repository admits no Ruby path.
	docs := rangeReceiptOf(t, withRuby, map[string]string{"README.md": "# widget\n"})
	if anyContains(rangeUncertainty(docs), unknownLanguageVerification) {
		t.Fatalf("docs-only range gained the line: %v", rangeUncertainty(docs))
	}
}
