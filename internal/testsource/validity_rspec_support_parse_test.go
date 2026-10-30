package testsource

import (
	"context"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// issue1312Spec is the GitHub #1312 including spec: a shared group defined and included in the
// same file.
const issue1312Spec = `RSpec.describe "Thing" do
  shared_examples "sends nothing" do
    it "sends nothing" do
      expect(1).to eq(1)
    end
  end

  context "when off" do
    it_behaves_like "sends nothing"
  end
end
`

// issue1312Support is a support shared context whose helper body is body.
func issue1312Support(body string) string {
	return "RSpec.shared_context \"x\" do\n  def a(e)\n" + body + "  end\nend\n"
}

func issue1312Read(files map[string]string) testvalidity.ExampleSource {
	source, _ := RSpecInclusionSource(testvalidity.RSpecInclusion{
		SpecPath: "spec/a_spec.rb", SpecText: issue1312Spec, IncludeLine: 9,
		SharedPath: "spec/a_spec.rb", SharedText: issue1312Spec, ExampleLine: 3,
		Support: bound(files),
	})
	return source
}

// TestRSpecValueOpenersBalance pins TSA-V0-396: each row of the GitHub #1312 table, and the other
// value positions, reads as one balanced block, so the support file binds and the inclusion is
// read; modifier forms, keyword arguments, string contents and a value block closed on its own
// line open nothing.
func TestRSpecValueOpenersBalance(t *testing.T) {
	cases := []struct{ name, body string }{
		{"assign if (#1312)", "    b = if e\n      1\n    else\n      2\n    end\n"},
		{"assign case (#1312)", "    b = case e\n    when 1 then 1\n    else 2\n    end\n"},
		{"assign begin (#1312)", "    b = begin\n      1\n    rescue StandardError\n      2\n    end\n"},
		{"assign unless (#1312)", "    b = unless e\n      1\n    end\n"},
		{"if statement (#1312)", "    if e\n      1\n    end\n"},
		{"assign while", "    b = while e\n      break 1\n    end\n"},
		{"assign until", "    b = until e\n      break 1\n    end\n"},
		{"or-assign if", "    @b ||= if e\n      1\n    end\n"},
		{"plus-assign case", "    b += case e\n    when 1 then 1\n    end\n"},
		{"argument if", "    f(if e\n      1\n    end)\n"},
		{"second argument begin", "    f(1, begin\n      2\n    end)\n"},
		{"and if", "    e && if e\n      1\n    end\n"},
		{"return case", "    return case e\n    when 1 then 1\n    end\n"},
		{"hash value if", "    h = { k => if e\n      1\n    end }\n"},
		{"one-line value if", "    b = if e then 1 else 2 end\n"},
		{"value begin over lines", "    b = begin; 1\n    end\n"},
		{"modifier if", "    b = 1 if e\n    f(e) unless e\n"},
		{"modifier after return", "    return if e\n    return unless e\n    return while e\n"},
		{"keyword argument", "    f(e, if: true, unless: false)\n"},
		{"string contents", "    s = \"a = if b\"\n    t = '(begin'\n"},
		{"heredoc body", "    text = <<~TEXT\n      x = if y\n    TEXT\n    u = <<-'A'\n      v = begin\n      A\n"},
		{"value if with end method", "    b = if (1..2).end == 2\n      1\n    end\n"},
		{"value if with end symbol", "    b = if options[:end]\n      1\n    end\n"},
		{"value if with end predicate", "    b = if r.end? || o.end!\n      1\n    end\n"},
		{"label key if (V1-2078)", "    h = { k: if e\n      1\n    end }\n"},
		{"append if (V1-2078)", "    a << if e\n      1\n    end\n"},
		{"else if (V1-2078)", "    if e\n      1\n    else if f\n      2\n    end\n    end\n"},
		{"else if one line (V1-2078)", "    if e\n      1\n    else if f then 2 end\n    end\n"},
		{"assign if end predicate (V1-2078)", "    x = if a.end?\n      1\n    end\n"},
		{"label value then modifier (V1-2078)", "    f(k: v) if e\n    a << b if e\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			text := issue1312Support(c.body)
			if reason := newRSpecReader("spec/support/x.rb", text).read(); reason != "" {
				t.Fatalf("support read = %q, want balanced:\n%s", reason, text)
			}
			source := issue1312Read(map[string]string{"spec/support/x.rb": text})
			if source.Read != testvalidity.ExampleSourceLocated || source.Cause != "" {
				t.Errorf("read = %q cause %q, want located", source.Read, source.Cause)
			}
		})
	}
}

// TestRSpecInclusionUnreadSupportIsolated pins TSA-V0-397: a support file that still does not read
// abstains only the inclusions whose shared group it could define, literally or through a
// non-literal name, and the abstention anchors that file; an inclusion of a group defined
// elsewhere still binds.
func TestRSpecInclusionUnreadSupportIsolated(t *testing.T) {
	broken := func(name string) string {
		return "RSpec.shared_examples " + name + " do\n  it 'y' do\n    expect(1).to eq(1)\n  end\nend\nend\n"
	}
	cases := []struct {
		name, support, want, cause string
	}{
		{"defines another group", broken(`"x"`), testvalidity.ExampleSourceLocated, ""},
		{"defines no group", "def a\nend\nend\n", testvalidity.ExampleSourceLocated, ""},
		{"defines the included group", broken(`"sends nothing"`), testvalidity.SharedGroupUnbound, "spec/support/broken.rb"},
		{"defines a non-literal group", broken(`NAME`), testvalidity.SharedGroupUnbound, "spec/support/broken.rb"},
		{"defines the included group second on a line", "RSpec.shared_examples('other') {}; RSpec.shared_examples('sends nothing') {}\nend\n",
			testvalidity.SharedGroupUnbound, "spec/support/broken.rb"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if newRSpecReader("spec/support/broken.rb", c.support).read() == "" {
				t.Fatal("support file read; the case needs one that does not")
			}
			source := issue1312Read(map[string]string{"spec/support/broken.rb": c.support})
			if source.Read != c.want || source.Cause != c.cause {
				t.Fatalf("read = %q cause %q, want %q cause %q", source.Read, source.Cause, c.want, c.cause)
			}
			if c.cause == "" {
				return
			}
			p := project(source, "spec/a_spec.rb:9>same:3")
			if want := []string{"spec/a_spec.rb:9>same:3", c.cause}; p.Hygiene.State != testvalidity.HygieneAbstained || !reflect.DeepEqual(p.Hygiene.Anchors, want) {
				t.Errorf("hygiene = %+v, want ABSTAINED anchored %q", p.Hygiene, want)
			}
		})
	}
}

// TestSourceValidityQualifiedSupportParse pins TSA-V0-396 and TSA-V0-397 through the command: the
// GitHub #1312 reproduction is ELIGIBLE, and an unreadable support file defining the included
// group abstains with that file among the anchors.
func TestSourceValidityQualifiedSupportParse(t *testing.T) {
	ref := "spec/a_spec.rb:9>same:3"
	for _, c := range []struct {
		name, support string
		want          SourceValidityTest
	}{
		{"issue 1312", issue1312Support("    b = if e\n      1\n    else\n      2\n    end\n"),
			SourceValidityTest{Test: ref, Hygiene: testvalidity.Axis{State: testvalidity.HygieneEligible, Reason: hygieneReasonSourceRead}}},
		{"unreadable definer", "RSpec.shared_examples 'sends nothing' do\nend\nend\n",
			SourceValidityTest{Test: ref, Hygiene: testvalidity.Axis{State: testvalidity.HygieneAbstained, Reason: testvalidity.SharedGroupUnbound, Anchors: []string{ref, "spec/support/x.rb"}}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			commitFiles(t, root, map[string]string{"spec/a_spec.rb": issue1312Spec, "spec/support/x.rb": c.support})
			report, err := SourceValidity(OpenRepo(context.Background(), root), SourceValidityOptions{Files: []string{ref}})
			if err != nil {
				t.Fatal(err)
			}
			got := report.Tests[0]
			if got.Test != c.want.Test || got.Hygiene.State != c.want.Hygiene.State || got.Hygiene.Reason != c.want.Hygiene.Reason ||
				c.want.Hygiene.Anchors != nil && !reflect.DeepEqual(got.Hygiene.Anchors, c.want.Hygiene.Anchors) {
				t.Errorf("test = %+v, want %+v", got, c.want)
			}
		})
	}
}
