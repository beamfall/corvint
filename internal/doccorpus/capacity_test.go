package doccorpus

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCorpusLargeProfile(t *testing.T) {
	t.Run("DCP-V1-033 beyond legacy record and byte bounds", func(t *testing.T) {
		root, m := providerFixture(t, func(p *ProviderRecord) {
			p.Schema = AdoptionProviderSchema
			sample := p.Claims[0]
			p.Claims = nil
			for i := 0; i < 6000; i++ {
				c := sample
				c.ID = fmt.Sprintf("adapter:claim:%05d", i)
				c.Text = strings.Repeat("source-declared bounded corpus evidence ", 16)
				p.Claims = append(p.Claims, c)
			}
		})
		m.Schema = ManifestSchemaV2
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := Encode(a)
		if err != nil {
			t.Fatal(err)
		}
		if len(raw) <= MaxBytes || len(a.Claims) < 6000 {
			t.Fatal("did not exercise old limits")
		}
		if err = os.WriteFile(filepath.Join(root, "corpus.json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
		captured, err := ReadCorpusFile(root, "corpus.json")
		if err != nil {
			t.Fatal(err)
		}
		opened, err := Open(context.Background(), root, captured)
		if err != nil {
			t.Fatal(err)
		}
		result, err := Query(opened, Request{Operation: "get", ID: "adapter:claim:05999"}, "fresh", nil)
		if err != nil || len(result.Results) != 1 {
			t.Fatalf("last record: %+v %v", result, err)
		}
		if _, err = ReadFile(root, "corpus.json"); err == nil {
			t.Fatal("legacy receipt bound widened")
		}
		m.Schema = ManifestSchema
		if _, err = Build(context.Background(), root, m); err == nil {
			t.Fatal("large profile admitted under /1")
		}
	})
}

func TestCorpusBoundedImpact(t *testing.T) {
	t.Run("DCP-V1-036 impact omissions retain fallback", func(t *testing.T) {
		a := &Artifact{Schema: SchemaV2, Subjects: []Subject{}, Capabilities: []Capability{}}
		for i := 0; i < 1000; i++ {
			a.Subjects = append(a.Subjects, Subject{ID: fmt.Sprint(i), Name: strings.Repeat("x", 8192), Evidence: Evidence{Anchors: []Anchor{{Path: "source.go"}}}})
		}
		r := Impact(a, []string{"source.go"}, "fresh")
		raw, err := Encode(r)
		if err != nil || len(raw) > 1<<20 {
			t.Fatal("oversized impact projection", err)
		}
		omitted := r["omitted"].(map[string]int)["subjects"]
		if omitted+len(r["subjects"].([]Subject)) != 1000 || omitted == 0 || r["selection"].(map[string]any)["narrowing_allowed"] != false {
			t.Fatal("lost omissions or fallback")
		}
	})
}
