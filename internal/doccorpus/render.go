package doccorpus

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

type Rendered struct {
	Schema         string            `json:"schema"`
	ArtifactSHA256 string            `json:"artifact_sha256"`
	ProfileSHA256  string            `json:"profile_sha256"`
	Derivation     string            `json:"derivation"`
	Files          map[string]string `json:"files"`
}

func mdQuote(s string) string {
	return "`" + strings.ReplaceAll(strconv.QuoteToASCII(s), "`", `\x60`) + "`"
}
func Render(a *Artifact) (Rendered, error) {
	r := Rendered{Schema: "corvint-corpus-render/1", ArtifactSHA256: a.SHA256, ProfileSHA256: a.ProfileSHA256, Derivation: "generated", Files: map[string]string{}}
	if a.Manifest.Profile.Format == "json" {
		data, err := Encode(a)
		if err != nil {
			return r, err
		}
		r.Files["corpus.json"] = string(data)
		return r, nil
	}
	groups := a.Manifest.Profile.Groups
	if len(groups) == 0 {
		groups = []string{"all"}
	}
	for _, group := range groups {
		if group != "all" && !subjectKinds[group] {
			return r, fail("unknown render group")
		}
		var b strings.Builder
		fmt.Fprintf(&b, "# %s\n\nGenerated evidence at %s. Artifact %s.\n\nIdentity checks do not establish accepted intent or behavioral adequacy.\n", mdQuote(a.Manifest.Profile.Title), mdQuote(a.Manifest.Repository.Revision), mdQuote(a.SHA256))
		for _, s := range a.Subjects {
			if group != "all" && s.Kind != group {
				continue
			}
			fmt.Fprintf(&b, "\n## %s\n\n%s / %s / %s\n", mdQuote(s.Name), mdQuote(s.Kind), mdQuote(s.Evidence.State), mdQuote(s.Evidence.Trust))
			for _, claim := range a.Claims {
				if claim.Subject == s.ID {
					fmt.Fprintf(&b, "\n%s\n", mdQuote(claim.Text))
					renderEvidence(&b, claim.Evidence)
				}
			}
			for _, rel := range a.Relations {
				if rel.From == s.ID {
					fmt.Fprintf(&b, "\nRelation %s → %s (%s).\n", mdQuote(rel.Type), mdQuote(rel.To), mdQuote(rel.Evidence.State))
					renderEvidence(&b, rel.Evidence)
				}
			}
			for _, journey := range a.Journeys {
				if journey.Subject == s.ID {
					data, err := Encode(journey)
					if err != nil {
						return r, err
					}
					fmt.Fprintf(&b, "\nJourney: %s\n", mdQuote(string(data)))
				}
			}
			for _, observation := range a.Observations {
				if observation.Link.Subject == s.ID {
					data, err := Encode(observation)
					if err != nil {
						return r, err
					}
					fmt.Fprintf(&b, "\nRetained observation: %s\n", mdQuote(string(data)))
				}
			}
			renderEvidence(&b, s.Evidence)
			if b.Len() > MaxBytes {
				return r, fail("render bound exceeded")
			}
		}
		for _, gap := range a.Gaps {
			fmt.Fprintf(&b, "\nGap %s: %s (%s).\n", mdQuote(gap.Subject), mdQuote(gap.Reason), mdQuote(gap.Kind))
		}
		body := b.String()
		r.Files[group+".md"] = fmt.Sprintf("<!-- corvint-corpus begin sha256=%s -->\n%s<!-- corvint-corpus end -->\n", Digest([]byte(body)), body)
	}
	if _, err := Encode(r); err != nil {
		return r, err
	}
	return r, nil
}
func renderEvidence(b *strings.Builder, e Evidence) {
	for _, a := range e.Anchors {
		fmt.Fprintf(b, "\nEvidence %s lines %d–%d; blob %s; span %s; %s.\n", mdQuote(a.Path), a.Start, a.End, mdQuote(a.Blob), mdQuote(a.SpanSHA256), mdQuote(a.Reason))
	}
	if e.Unknown != "" {
		fmt.Fprintf(b, "\nUnknown: %s.\n", mdQuote(e.Unknown))
	}
	for _, l := range e.Limitations {
		fmt.Fprintf(b, "\nLimit: %s.\n", mdQuote(l))
	}
}

type Maintenance struct {
	Schema         string `json:"schema"`
	ArtifactSHA256 string `json:"artifact_sha256"`
	Page           string `json:"page"`
	Before         string `json:"before_sha256"`
	After          string `json:"after_sha256"`
	Recovery       string `json:"recovery_path"`
	Applied        bool   `json:"applied"`
	Proposed       string `json:"proposed"`
}

// Maintain derives the proposal in-process. It never accepts caller-authored
// replacement bytes or a serialized preview as write authority.
func Maintain(root, page string, a *Artifact, apply bool) (Maintenance, error) {
	result := Maintenance{Schema: "corvint-corpus-maintenance/1", ArtifactSHA256: a.SHA256, Page: page}
	if !validCorpusPage(page) {
		return result, fail("maintenance requires a Markdown page outside Git metadata")
	}
	before, err := ReadFile(root, page)
	if err != nil {
		return result, err
	}
	if bytes.Contains(before, []byte("Intent status: accepted")) || bytes.Contains(before, []byte("intent: accepted")) {
		return result, fail("accepted prose cannot be overwritten")
	}
	render, err := Render(a)
	if err != nil {
		return result, err
	}
	if len(render.Files) != 1 || a.Manifest.Profile.Format != "markdown" {
		return result, fail("maintenance requires one Markdown render group")
	}
	var block string
	for _, value := range render.Files {
		block = value
	}
	next, err := replaceBlock(before, []byte(block))
	if err != nil {
		return result, err
	}
	if len(next) > MaxBytes {
		return result, fail("maintenance output bound exceeded")
	}
	result.Before = Digest(before)
	result.After = Digest(next)
	result.Proposed = string(next)
	if !apply {
		return result, nil
	}
	result.Recovery, err = applyCorpusPage(root, page, before, next, nil)
	if err != nil {
		return result, err
	}
	result.Applied = true
	return result, nil
}
func replaceBlock(page, block []byte) ([]byte, error) {
	prefix := []byte("<!-- corvint-corpus begin sha256=")
	suffix := []byte("<!-- corvint-corpus end -->\n")
	begins, ends := bytes.Count(page, []byte("<!-- corvint-corpus begin")), bytes.Count(page, []byte("<!-- corvint-corpus end"))
	if begins == 0 && ends == 0 {
		return append(append(append([]byte{}, page...), '\n'), block...), nil
	}
	if begins != 1 || ends != 1 {
		return nil, fail("malformed or duplicate generated markers")
	}
	start := bytes.Index(page, prefix)
	end := bytes.Index(page, suffix)
	if start < 0 || end < start {
		return nil, fail("malformed generated markers")
	}
	headerEnd := bytes.Index(page[start:], []byte(" -->\n"))
	if headerEnd < 0 {
		return nil, fail("malformed generated header")
	}
	headerEnd += start
	recorded := string(page[start+len(prefix) : headerEnd])
	bodyStart := headerEnd + 5
	if end < bodyStart || Digest(page[bodyStart:end]) != recorded {
		return nil, fail("generated block was tampered")
	}
	next := append([]byte{}, page[:start]...)
	next = append(next, block...)
	next = append(next, page[end+len(suffix):]...)
	return next, nil
}
