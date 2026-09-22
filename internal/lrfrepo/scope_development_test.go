//go:build corvint_development

package lrfrepo

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

const scopeDevelopmentLabelsSHA256 = "74bfe7c5f562a1bf7b724e20d199fe68204efb5593c187bf48f1783206363881"

type scopeDevelopmentSpan struct {
	Start     int    `json:"start"`
	End       int    `json:"end"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	SHA256    string `json:"sha256"`
}

type scopeDevelopmentNativeStatus struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type scopeDevelopmentLabel struct {
	Label  string                       `json:"label"`
	Path   string                       `json:"authority_path"`
	Blob   string                       `json:"authority_blob"`
	ID     string                       `json:"requirement_id"`
	Text   string                       `json:"requirement_text"`
	Span   *scopeDevelopmentSpan        `json:"requirement_span"`
	Native scopeDevelopmentNativeStatus `json:"native_ocm_admissibility"`
}
type scopeDevelopmentCase struct {
	ID                 string   `json:"case_id"`
	Base               string   `json:"base_commit"`
	Target             string   `json:"target_commit"`
	Paths              []string `json:"changed_paths"`
	Status             string   `json:"status"`
	Positive, Negative scopeDevelopmentLabel
}

type scopeDevelopmentPreparedCase struct {
	Case   scopeDevelopmentCase
	Bodies map[string][]byte
	Labels [2][]byte
}

func scopeDevelopmentGit(ctx context.Context, budget *gitrun.Budget, root string, args ...string) ([]byte, error) {
	options := gitrun.Options{
		Dir:          root,
		Env:          []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_SYSTEM=" + os.DevNull, "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_NO_REPLACE_OBJECTS=1", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1"},
		StdoutLimit:  16 << 20,
		PerOpTimeout: 5 * time.Second,
	}
	return gitrun.Run(ctx, budget, options, args...)
}

func scopeDevelopmentStatement(label scopeDevelopmentLabel, data []byte) ([]byte, error) {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(data))
	h.Write(data)
	if hex.EncodeToString(h.Sum(nil)) != label.Blob {
		return nil, fmt.Errorf("authority blob drift")
	}
	_, ids, scope, err := requirementsFromBlob(label.Path, label.Blob, data)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if id == label.ID {
			return requirementStatement(scope, id)
		}
	}
	return nil, fmt.Errorf("requirement absent from native grammar")
}

func scopeDevelopmentVerifyLabel(ctx context.Context, budget *gitrun.Budget, root, base string, label scopeDevelopmentLabel) ([]byte, error) {
	resolved, err := scopeDevelopmentGit(ctx, budget, root, "rev-parse", "--verify", base+":"+label.Path)
	if err != nil {
		return nil, fmt.Errorf("resolve base authority: %w", err)
	}
	if strings.TrimSpace(string(resolved)) != label.Blob {
		return nil, fmt.Errorf("base authority blob drift")
	}
	typeName, err := scopeDevelopmentGit(ctx, budget, root, "cat-file", "-t", label.Blob)
	if err != nil {
		return nil, fmt.Errorf("read authority type: %w", err)
	}
	if string(typeName) != "blob\n" {
		return nil, fmt.Errorf("authority object is not a blob")
	}
	data, err := scopeDevelopmentGit(ctx, budget, root, "cat-file", "blob", label.Blob)
	if err != nil {
		return nil, fmt.Errorf("read authority blob: %w", err)
	}
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(data))
	h.Write(data)
	if hex.EncodeToString(h.Sum(nil)) != label.Blob {
		return nil, fmt.Errorf("authority blob hash mismatch")
	}
	if label.Span == nil || label.Span.Start < 0 || label.Span.End <= label.Span.Start || label.Span.End > len(data) {
		return nil, fmt.Errorf("invalid requirement span")
	}
	anchored := data[label.Span.Start:label.Span.End]
	if !bytes.Equal(anchored, []byte(label.Text)) {
		return nil, fmt.Errorf("requirement text differs from anchored bytes")
	}
	if !bytes.Contains(anchored, []byte(label.ID)) {
		return nil, fmt.Errorf("requirement ID absent from anchored bytes")
	}
	digest := sha256.Sum256(anchored)
	if hex.EncodeToString(digest[:]) != label.Span.SHA256 {
		return nil, fmt.Errorf("requirement span digest mismatch")
	}
	startLine := 1 + bytes.Count(data[:label.Span.Start], []byte{'\n'})
	endLine := 1 + bytes.Count(data[:label.Span.End-1], []byte{'\n'})
	if startLine != label.Span.StartLine || endLine != label.Span.EndLine {
		return nil, fmt.Errorf("requirement line span mismatch")
	}
	return data, nil
}

func scopeDevelopmentSymbols(path string, data []byte) map[string]bool {
	names := map[string]bool{}
	if !strings.HasSuffix(path, ".go") {
		return names
	}
	f, err := parser.ParseFile(token.NewFileSet(), path, data, 0)
	if err != nil {
		return names
	}
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			names[d.Name.Name] = true
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					names[s.Name.Name] = true
				case *ast.ValueSpec:
					for _, name := range s.Names {
						names[name.Name] = true
					}
				}
			}
		}
	}
	return names
}

func scopeDevelopmentJoin(statement []byte, label scopeDevelopmentLabel, path string, body []byte) string {
	if containsExactRequirement(body, label.ID) {
		return "explicit requirement reference in changed source"
	}
	literals := regexp.MustCompile("`([^`]+)`").FindAllSubmatch(statement, -1)
	names := scopeDevelopmentSymbols(path, body)
	for _, literal := range literals {
		name := string(literal[1])
		if strings.Contains(name, "/") && (path == name || strings.HasPrefix(path, strings.TrimSuffix(name, "/")+"/")) {
			return "explicit path in base requirement"
		}
		if names[name] {
			return "explicit declared symbol in base requirement"
		}
	}
	return ""
}

func TestIndependentScopeDevelopmentScreen(t *testing.T) {
	t.Run("EAF-V0-009", func(t *testing.T) {
		manifest := os.Getenv("CORVINT_SCOPE_DEVELOPMENT_MANIFEST")
		if manifest == "" {
			t.Skip("requires explicitly frozen independent development labels")
		}
		root := os.Getenv("CORVINT_SCOPE_DEVELOPMENT_REPOSITORY")
		if root == "" {
			t.Fatal("requires pinned object repository")
		}
		ctx, stop := signal.NotifyContext(t.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		budget := gitrun.NewBudget(512, 60*time.Second)
		data, err := os.ReadFile(manifest)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != scopeDevelopmentLabelsSHA256 {
			t.Fatal("independent label manifest digest drift")
		}
		var document struct {
			Profile   string `json:"profile"`
			CaseCount int    `json:"case_count"`
			Cases     []scopeDevelopmentCase
		}
		if err := json.Unmarshal(data, &document); err != nil {
			t.Fatal(err)
		}
		if document.Profile != "audit-independent-scope-labels/0" || document.CaseCount != 20 || len(document.Cases) != 20 {
			t.Fatal("requires exact clean all20 label cohort")
		}
		prepared := make([]scopeDevelopmentPreparedCase, 0, len(document.Cases))
		semanticApplicable, semanticUnknown, nativeAdmissible, nativeUnsupported := 0, 0, 0, 0
		for _, c := range document.Cases {
			for _, revision := range []string{c.Base, c.Target} {
				typeName, err := scopeDevelopmentGit(ctx, budget, root, "cat-file", "-t", revision)
				if err != nil || string(typeName) != "commit\n" {
					t.Fatalf("%s unavailable immutable commit %s", c.ID, revision)
				}
			}
			changed, err := scopeDevelopmentGit(ctx, budget, root, "diff", "--name-only", "-z", "--no-ext-diff", c.Base, c.Target)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(changed, []byte(strings.Join(c.Paths, "\x00")+"\x00")) {
				t.Fatalf("%s changed universe drift", c.ID)
			}
			bodies := map[string][]byte{}
			for _, path := range c.Paths {
				if !strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, ".py") {
					continue
				}
				body, err := scopeDevelopmentGit(ctx, budget, root, "cat-file", "blob", c.Target+":"+path)
				if err != nil {
					body, err = scopeDevelopmentGit(ctx, budget, root, "cat-file", "blob", c.Base+":"+path)
				}
				if err != nil {
					t.Fatalf("%s unread source %s", c.ID, path)
				}
				bodies[path] = body
			}
			entry := scopeDevelopmentPreparedCase{Case: c, Bodies: bodies}
			switch c.Positive.Label {
			case "applicable":
				if c.Status != "LABELED" {
					t.Fatalf("%s applicable label has status %s", c.ID, c.Status)
				}
				semanticApplicable++
				switch c.Positive.Native.Status {
				case "ADMISSIBLE":
					nativeAdmissible++
				case "UNSUPPORTED":
					nativeUnsupported++
				default:
					t.Fatalf("%s invalid applicable native status %s", c.ID, c.Positive.Native.Status)
				}
			case "UNKNOWN":
				if c.Status != "UNKNOWN" || c.Positive.Path != "" || c.Positive.Blob != "" || c.Positive.ID != "" || c.Positive.Text != "" || c.Positive.Span != nil || c.Positive.Native.Status != "UNKNOWN" {
					t.Fatalf("%s malformed UNKNOWN label", c.ID)
				}
				semanticUnknown++
			default:
				t.Fatalf("%s invalid positive label %s", c.ID, c.Positive.Label)
			}
			if c.Positive.Label != "UNKNOWN" {
				entry.Labels[0], err = scopeDevelopmentVerifyLabel(ctx, budget, root, c.Base, c.Positive)
				if err != nil {
					t.Fatalf("%s positive provenance: %v", c.ID, err)
				}
			}
			if c.Negative.Label != "unrelated" || c.Negative.Native.Status != "ADMISSIBLE" {
				t.Fatalf("%s invalid unrelated control", c.ID)
			}
			entry.Labels[1], err = scopeDevelopmentVerifyLabel(ctx, budget, root, c.Base, c.Negative)
			if err != nil {
				t.Fatalf("%s negative provenance: %v", c.ID, err)
			}
			prepared = append(prepared, entry)
		}
		if semanticApplicable != 19 || semanticUnknown != 1 || nativeAdmissible != 15 || nativeUnsupported != 4 {
			t.Fatalf("label cohort counts drift: applicable=%d unknown=%d native_admissible=%d native_unsupported=%d", semanticApplicable, semanticUnknown, nativeAdmissible, nativeUnsupported)
		}

		hits, noise := 0, 0
		for _, entry := range prepared {
			c := entry.Case
			for i, label := range []scopeDevelopmentLabel{c.Positive, c.Negative} {
				if label.Label == "UNKNOWN" {
					t.Logf("case=%s expected=UNKNOWN candidate=NOT_SCORED reason=%s", c.ID, label.Native.Reason)
					continue
				}
				statement, err := scopeDevelopmentStatement(label, entry.Labels[i])
				if label.Native.Status == "UNSUPPORTED" {
					if err == nil {
						t.Fatalf("%s id=%s native grammar unexpectedly admitted frozen unsupported label", c.ID, label.ID)
					}
					t.Logf("case=%s id=%s expected_applicable=true candidate=false native_status=UNSUPPORTED reason=%s", c.ID, label.ID, label.Native.Reason)
					continue
				}
				if err != nil {
					t.Fatalf("%s id=%s native grammar drift: %v", c.ID, label.ID, err)
				}
				join := ""
				for _, path := range c.Paths {
					body, present := entry.Bodies[path]
					if !present {
						continue
					}
					if reason := scopeDevelopmentJoin(statement, label, path, body); reason != "" {
						join = reason
						break
					}
				}
				if join != "" {
					if i == 0 {
						hits++
					} else {
						noise++
					}
				}
				t.Logf("case=%s id=%s expected_applicable=%t candidate=%t native_status=ADMISSIBLE reason=%s", c.ID, label.ID, i == 0, join != "", join)
			}
		}
		missed := semanticApplicable - hits
		t.Logf("labels_sha256=%s cases=20 labelled_rows=39 semantic_applicable=%d semantic_unknown=%d native_admissible=%d native_unsupported=%d injected_baseline_omissions=%d caught=%d missed=%d unrelated_candidates=%d", hex.EncodeToString(digest[:]), semanticApplicable, semanticUnknown, nativeAdmissible, nativeUnsupported, semanticApplicable, hits, missed, noise)
		if missed != 0 {
			t.Errorf("REJECTED: explicit joins missed %d/%d applicable obligations; UNKNOWN excluded; do not promote scope completeness", missed, semanticApplicable)
		}
	})
}
