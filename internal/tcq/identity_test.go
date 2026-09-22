package tcq

import (
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestCanonicalCodecMatchesOracle pins the shared `canonical-json-value`
// primitive against the oracle's encoding: sorted keys, minimal separators, the
// frozen escape profile, and exactly one terminal LF on a whole artifact.
func TestCanonicalCodecMatchesOracle(t *testing.T) {
	vectors := loadVectors(t)
	escapes := jsonObject(
		member{"quote", jsonString("a\"b")},
		member{"backslash", jsonString(`a\b`)},
		member{"tab", jsonString("a\tb")},
		member{"newline", jsonString("a\nb")},
		member{"control", jsonString("a\x01b")},
		member{"unicode", jsonString("café ✓")},
		member{"del", jsonString("a\x7fb")},
	)
	if got := string(canonicalValue(escapes)); got != vectors.Canonical["escapes"] {
		t.Errorf("escapes = %q, oracle %q", got, vectors.Canonical["escapes"])
	}
	sorted := jsonObject(
		member{"b", jsonInt(1)},
		member{"A", jsonInt(2)},
		member{"a", jsonInt(3)},
		member{"", jsonInt(4)},
	)
	if got := string(canonicalValue(sorted)); got != vectors.Canonical["sortedKeys"] {
		t.Errorf("sorted keys = %q, oracle %q", got, vectors.Canonical["sortedKeys"])
	}
	nested := jsonObject(
		member{"z", jsonArray([]wire.Value{
			jsonInt(1),
			jsonObject(member{"y", jsonNull()}, member{"x", wire.Value{Kind: wire.KindBool, Bool: true}}),
		})},
		member{"a", jsonInt(-5)},
	)
	if got := string(canonicalJSON(nested)); got != vectors.Canonical["nested"] {
		t.Errorf("nested = %q, oracle %q", got, vectors.Canonical["nested"])
	}
}

// TestExecutionKeysMatchOracle pins TCQ-V0-021's derivation, including a
// non-ASCII path and runtime name.
func TestExecutionKeysMatchOracle(t *testing.T) {
	vectors := loadVectors(t)
	cases := map[string][2]string{
		"pkg/sample_test.go|TestAlpha":            {"pkg/sample_test.go", "TestAlpha"},
		"pkg/sample_test.go|TestTable/alpha-case": {"pkg/sample_test.go", "TestTable/alpha-case"},
		"pkg/test_simple.py|test_alpha":           {"pkg/test_simple.py", "test_alpha"},
		"unicode":                                 {"pkg/café_test.go", "TestCafé"},
	}
	for name, inputs := range cases {
		if got := executionKey(inputs[0], inputs[1]); got != vectors.ExecutionKeys[name] {
			t.Errorf("%s execution key = %s, oracle %s", name, got, vectors.ExecutionKeys[name])
		}
	}
}

// TestSelectorFragmentMatchesOracle pins the extractor selector grammar TCQ
// re-derives against for `go-table-case/1`.
func TestSelectorFragmentMatchesOracle(t *testing.T) {
	vectors := loadVectors(t)
	for value, want := range vectors.SelectorFragments {
		if got := selectorFragment(value); got != want {
			t.Errorf("selectorFragment(%q) = %q, oracle %q", value, got, want)
		}
	}
}

// TestNormalPathMatchesOracle pins TCQ-V0-028's path normalization. An empty
// oracle value stands for the oracle's None: the path is unkeyed.
func TestNormalPathMatchesOracle(t *testing.T) {
	vectors := loadVectors(t)
	for value, want := range vectors.NormalPaths {
		got, ok := normalPath(value, 512)
		if !ok {
			got = ""
		}
		if got != want {
			t.Errorf("normalPath(%q) = %q/%v, oracle %q", value, got, ok, want)
		}
	}
}

// TestCommandArtifactMatchesOracle pins the TCQ-V0-023/025 wire bytes and the
// `test-command:sha256:` identity for both clean-target statements.
func TestCommandArtifactMatchesOracle(t *testing.T) {
	vectors := loadVectors(t)
	attested, err := MakeTestCommand(
		[]string{"python3", "-m", "unittest"}, ".", "python-unittest", "3.13.7",
		"0123456789abcdef0123456789abcdef01234567", CleanTargetAttested)
	if err != nil {
		t.Fatalf("MakeTestCommand: %v", err)
	}
	if string(attested) != vectors.Command {
		t.Errorf("command = %s, oracle %s", attested, vectors.Command)
	}
	notAttested, err := MakeTestCommand(
		[]string{"go", "test", "./..."}, "sub/dir", "go-test", "1.24.0",
		"0123456789abcdef0123456789abcdef01234567", CleanTargetNotAttested)
	if err != nil {
		t.Fatalf("MakeTestCommand: %v", err)
	}
	if string(notAttested) != vectors.CommandNotAttested {
		t.Errorf("command = %s, oracle %s", notAttested, vectors.CommandNotAttested)
	}
}
