// SPDX-License-Identifier: Apache-2.0

// Package testinventory lists tests and refreshes the committed runner inventory.
// Its read API uses the same codec and judgments as consumers in the shared layer.
package testinventory

import (
	"context"

	reader "github.com/Beamfall/corvint/internal/testinventory/reader"
)

const (
	KindStatic          = reader.KindStatic
	StateAbsent         = reader.StateAbsent
	StateRead           = reader.StateRead
	KindExecuting       = reader.KindExecuting
	MaxBytes            = reader.MaxBytes
	MaxErrorBytes       = reader.MaxErrorBytes
	Path                = reader.Path
	Profile             = reader.Profile
	ReasonAbsent        = reader.ReasonAbsent
	ReasonDrift         = reader.ReasonDrift
	ReasonInvalid       = reader.ReasonInvalid
	ReasonSelectionFull = reader.ReasonSelectionFull
	ReasonStale         = reader.ReasonStale
	ReasonTooLarge      = reader.ReasonTooLarge
	ReasonUnassigned    = reader.ReasonUnassigned
	StateInvalid        = reader.StateInvalid
	VerdictProfile      = reader.VerdictProfile
)

type (
	Inventory       = reader.Inventory
	Instance        = reader.Instance
	Lister          = reader.Lister
	Receipt         = reader.Receipt
	File            = reader.File
	Test            = reader.Test
	Failure         = reader.Failure
	Tree            = reader.Tree
	MapTree         = reader.MapTree
	Git             = reader.Git
	CommitTree      = reader.CommitTree
	WorkTree        = reader.WorkTree
	Verdict         = reader.Verdict
	InstanceVerdict = reader.InstanceVerdict
	ListerIdentity  = reader.ListerIdentity
	Declared        = reader.Declared
)

var (
	ErrInvalid  = reader.ErrInvalid
	ErrTooLarge = reader.ErrTooLarge
)

func Key(runner, root string) string {
	return reader.Key(runner, root)
}

func Decode(data []byte) (Inventory, error) {
	return reader.Decode(data)
}

func Encode(inventory Inventory) ([]byte, error) {
	return reader.Encode(inventory)
}

func BlobID(objectFormat string, content []byte) string {
	return reader.BlobID(objectFormat, content)
}

func NewCommitTree(ctx context.Context, git Git, revision string) (*CommitTree, error) {
	return reader.NewCommitTree(ctx, git, revision)
}

func NewWorkTree(ctx context.Context, git Git) (*WorkTree, error) {
	return reader.NewWorkTree(ctx, git)
}

func Judge(tree Tree) Verdict {
	return reader.Judge(tree)
}

func JudgeListed(tree Tree) (Verdict, map[string]map[string][]string) {
	return reader.JudgeListed(tree)
}

func Listed(data []byte, verdict Verdict) map[string]map[string][]string {
	return reader.Listed(data, verdict)
}

func JudgeData(data []byte, present bool, readErr error, files map[string]string, declared []Declared) Verdict {
	return reader.JudgeData(data, present, readErr, files, declared)
}

func CurrentLister(runner string) (ListerIdentity, bool) {
	return reader.CurrentLister(runner)
}

func Detect(files map[string]string, read func(name string, limit int) ([]byte, error)) []Declared {
	return reader.Detect(files, read)
}

func NearestLockfile(exists func(name string) bool, runner, root string) string {
	return reader.NearestLockfile(exists, runner, root)
}

func Candidates(files map[string]string, instance Declared, all []Declared) []string {
	return reader.Candidates(files, instance, all)
}
