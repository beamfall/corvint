package contextindex

import (
	"context"
	"slices"
	"testing"
)

func TestTaskContextReportsDeclaredRequiredSupport(t *testing.T) {
	t.Setenv("CORVINT_CONTEXT_TERMS", "")
	t.Setenv("CORVINT_CONTEXT_FRAME_RELATION", "")
	for _, test := range []struct {
		name, task, left, right, reason, verdict string
		known, required, support, supporting     int
	}{
		{"zero known", "`absentOne` and `absentTwo`", "", "",
			"no source names any specific term: `absentOne`, `absentTwo`", "unsupported-conjunction", 0, 0, 0, 0},
		{"one unknown answers without support", "`absentOne` alone", "", "",
			"one unknown name is a feature as often as an absence; no conjunction to test", "not-withheld", 0, 0, 0, 0},
		{"one known", "`alphaOne` and `absentTwo`", "alphaOne", "",
			"1 sources name at least 1 of the 1 known specific terms together; no source names `absentTwo`", "supported", 1, 1, 1, 1},
		{"two disjoint", "`alphaOne` and `betaTwo`", "alphaOne", "betaTwo",
			"0 sources name at least 2 of the 2 known specific terms together", "not-withheld", 2, 2, 1, 0},
		{"two together", "`alphaOne` and `betaTwo`", "alphaOne betaTwo", "",
			"1 sources name at least 2 of the 2 known specific terms together", "supported", 2, 2, 2, 1},
		{"three known", "`alphaOne` and `betaTwo` and `gammaThree`", "alphaOne betaTwo", "gammaThree",
			"1 sources name at least 2 of the 3 known specific terms together", "supported", 3, 2, 2, 1},
		{"four known", "`alphaOne` and `betaTwo` and `gammaThree` and `deltaFour`", "alphaOne betaTwo", "gammaThree deltaFour",
			"2 sources name at least 2 of the 4 known specific terms together", "supported", 4, 2, 2, 2},
	} {
		t.Run("TCP-V0-016 "+test.name, func(t *testing.T) {
			root := impactRepositoryWithFiles(t, map[string]string{
				"go.mod":   "module example.test/support\n\ngo 1.27.0\n",
				"left.go":  "package support\n// " + test.left + "\n",
				"right.go": "package support\n// " + test.right + "\n",
			})
			index, err := Build(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			packet, err := TaskContext(context.Background(), index, test.task, "", 20)
			if err != nil {
				t.Fatal(err)
			}
			answer := packet["coverage"].(map[string]any)["answerability"].(map[string]any)
			for field, want := range map[string]int{"known": test.known, "required": test.required, "support": test.support, "supporting_sources": test.supporting} {
				if answer[field] != want {
					t.Errorf("TCP-V0-016(b): %s = %v, want %d", field, answer[field], want)
				}
			}
			if answer["reason"] != test.reason {
				t.Errorf("support disclosure = %v, want %q", answer["reason"], test.reason)
			}
			if answer["verdict"] != test.verdict {
				t.Errorf("selection verdict = %v, want %q", answer["verdict"], test.verdict)
			}
		})
	}
}

func TestTaskContextSupportedVerdictNamesItsSupport(t *testing.T) {
	t.Setenv("CORVINT_CONTEXT_TERMS", "")
	t.Setenv("CORVINT_CONTEXT_FRAME_RELATION", "")
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":  "module example.test/support\n\ngo 1.27.0\n",
		"left.go": "package support\n// alphaOne betaTwo\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := TaskContext(context.Background(), index, "`alphaOne` and `betaTwo` and `absentThree`", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	answer := packet["coverage"].(map[string]any)["answerability"].(map[string]any)
	claims := mapsFromAny(answer["nearest_claims"])
	if answer["verdict"] != "supported" || len(claims) != 1 {
		t.Fatalf("answerability = %v, want one named supporting claim", answer)
	}
	if claims[0]["path"] != "left.go" || claims[0]["blob_hash"] == "" {
		t.Fatalf("supporting claim = %v, want pinned left.go", claims[0])
	}
	wantSupports := []any{"alphaOne", "betaTwo"}
	wantLacks := []any{"absentThree"}
	if got := claims[0]["supports"]; !slices.Equal(got.([]any), wantSupports) {
		t.Fatalf("supports = %v, want %v", got, wantSupports)
	}
	if got := claims[0]["lacks"]; !slices.Equal(got.([]any), wantLacks) {
		t.Fatalf("lacks = %v, want %v", got, wantLacks)
	}
}

func TestTaskContextNotWithheldVerdictHasNoClaims(t *testing.T) {
	t.Setenv("CORVINT_CONTEXT_TERMS", "")
	t.Setenv("CORVINT_CONTEXT_FRAME_RELATION", "")
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":  "module example.test/support\n\ngo 1.27.0\n",
		"left.go": "package support\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := TaskContext(context.Background(), index, "`absentOne` alone", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	answer := packet["coverage"].(map[string]any)["answerability"].(map[string]any)
	if answer["verdict"] != "not-withheld" || len(mapsFromAny(answer["nearest_claims"])) != 0 {
		t.Fatalf("answerability = %v, want not-withheld with an empty claim list", answer)
	}
}
