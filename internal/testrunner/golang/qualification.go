// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"fmt"
	"sort"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/liveverify/mutate"
	"github.com/Beamfall/corvint/internal/testrunner"
)

// Corpus pins a qualification population; checkout locations remain operator
// inputs and are deliberately absent from this declaration.
type Corpus struct{ Repository, Commit string }
type ClassStatus struct{ Class, Mode, Reason string }
type QualificationDeclaration struct {
	Key               testrunner.InventoryKey
	Model             string
	Corpora           []Corpus
	Classes           []ClassStatus
	MutationOperators []string
}

// Qualification declares inputs for T6. It accepts no success flag, trial
// count, miss count or claimed bound: only an admitted T6 record can enable a
// class. Until that integration exists, every class is FULL/unqualified.
func (p *Plugin) Qualification(corpora []Corpus, model string) (QualificationDeclaration, error) {
	if model != "traced" && model != "untraced" {
		return QualificationDeclaration{}, fmt.Errorf("go-qualification-model-unsupported")
	}
	if len(corpora) == 0 || len(corpora) > 4096 {
		return QualificationDeclaration{}, fmt.Errorf("go-qualification-corpus-missing-or-capped")
	}
	corpora = append([]Corpus(nil), corpora...)
	sort.Slice(corpora, func(i, j int) bool {
		if corpora[i].Repository != corpora[j].Repository {
			return corpora[i].Repository < corpora[j].Repository
		}
		return corpora[i].Commit < corpora[j].Commit
	})
	for i, c := range corpora {
		if c.Repository == "" || !wire.IsGitOid(c.Commit) || i > 0 && corpora[i-1] == c {
			return QualificationDeclaration{}, fmt.Errorf("go-qualification-corpus-unpinned")
		}
	}
	classes := []string{"body-edit", "declaration-edit"}
	if model == "traced" {
		classes = append(classes, "data-edit", "add", "delete-rename", "exec-target", "runner-config")
	}
	result := QualificationDeclaration{Key: p.key, Model: model, Corpora: corpora, MutationOperators: mutate.SelectionOperators()}
	for _, class := range classes {
		result.Classes = append(result.Classes, ClassStatus{Class: class, Mode: "FULL", Reason: "unqualified"})
	}
	return result, nil
}
