// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/jobinputs"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// parseAffectedJobsOptions reads `affected jobs --base FULL_COMMIT_ID
// [--declaration PATH]` (AFP-V0-2341), each option at most once.
func parseAffectedJobsOptions(rest []string) (affectedInvocation, error) {
	invocation := affectedInvocation{}
	declaration := ""
	baseSet := false
	for index := 0; index < len(rest); index++ {
		name, value, inline := strings.Cut(rest[index], "=")
		if name != "--base" && name != "--declaration" {
			return affectedInvocation{}, argumentError("unrecognized arguments: " + rest[index] + "; usage: corvint affected jobs --base FULL_COMMIT_ID [--declaration PATH]")
		}
		if !inline {
			if index+1 >= len(rest) {
				return affectedInvocation{}, argumentError(name + " requires exactly one value")
			}
			index++
			value = rest[index]
		}
		switch name {
		case "--base":
			if err := setAffectedBase(&invocation, &baseSet, value); err != nil {
				return affectedInvocation{}, err
			}
		case "--declaration":
			if declaration != "" || value == "" {
				return affectedInvocation{}, argumentError("--declaration requires exactly one nonempty value")
			}
			declaration = value
		}
	}
	if !baseSet {
		return affectedInvocation{}, argumentError("affected jobs requires --base FULL_COMMIT_ID: a job plan evaluates a committed range")
	}
	if declaration == "" {
		declaration = jobinputs.DeclarationPath
	}
	invocation.JobsDeclaration = declaration
	return invocation, nil
}

// runAffectedJobs writes the corvint-ci-job-plan/0 of the range: exit 0 with
// a plan, 3 when the declaration cannot be read (every job then runs), 2 on
// an input or planning error.
func runAffectedJobs(ctx context.Context, invocation affectedInvocation, stdout, stderr io.Writer) int {
	plan := jobinputs.Plan{Profile: jobinputs.PlanProfile, Tool: "affected jobs", Base: invocation.Base, Declaration: invocation.JobsDeclaration, Jobs: []jobinputs.JobResult{}}
	declarationPath := invocation.JobsDeclaration
	full := declarationPath
	if !filepath.IsAbs(full) {
		full = filepath.Join(invocation.Root, filepath.FromSlash(declarationPath))
	}
	if relative, err := filepath.Rel(invocation.Root, full); err == nil && !strings.HasPrefix(relative, "..") {
		declarationPath = filepath.ToSlash(relative)
	}
	body, err := readBoundedFile(full, 1<<20)
	var declaration jobinputs.Declaration
	if err == nil {
		declaration, err = jobinputs.Parse(body)
	}
	if err != nil {
		plan.DeclarationError = "DECLARATION_INVALID: " + err.Error()
		_ = gokernel.WriteCanonicalJSON(stdout, plan)
		return 3
	}
	plan.DeclarationDigest = jobinputs.DigestBytes(body)
	receipt, err := compileAffected(ctx, invocation)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	plan.Revision, plan.Changed, plan.PlanScope = receipt.Revision, len(receipt.Plan.Dirty), receipt.Plan.Scope
	evidence := jobinputs.Evidence{Changed: receipt.Plan.Dirty, PlanScope: receipt.Plan.Scope, Selected: map[string]string{}}
	for _, selection := range receipt.Plan.Selected {
		evidence.Selected[selection.UnitID] = selection.Witness.Kind
	}
	if module, err := golang.ReadModulePath(invocation.Root); err == nil {
		evidence.Module = module
	} else {
		evidence.PlanError = "root module: " + err.Error()
	}
	evidence.Tree, evidence.ListingBytes, err = affectedJobsTree(ctx, invocation.Root, receipt.Revision)
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "unsupported-affected-jobs", Message: "the tree of " + receipt.Revision + " cannot be listed: " + err.Error()})
		return 2
	}
	evidence.GoDirectories = func(packages jobinputs.GoPackages) (map[string]string, error) {
		return jobinputs.GoDirectories(ctx, invocation.Root, "", nil, packages)
	}
	plan.Jobs = jobinputs.Evaluate(declaration, declarationPath, evidence)
	plan.OK = true
	if err := gokernel.WriteCanonicalJSON(stdout, plan); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write the job plan"})
		return 2
	}
	return 0
}

// affectedJobsTree lists revision's tree with blob sizes, and the byte size
// of its plain recursive listing, which an exportedTree job's export reads.
func affectedJobsTree(ctx context.Context, root, revision string) (map[string]jobinputs.TreeEntry, int, error) {
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		return nil, 0, err
	}
	run := func(arguments ...string) ([]byte, error) {
		command := hermeticGitCommand(ctx, gitExecutable, root, arguments...)
		var stdout bytes.Buffer
		command.Stdout = &stdout
		if err := command.Run(); err != nil {
			return nil, err
		}
		return stdout.Bytes(), nil
	}
	plain, err := run("ls-tree", "-r", "-z", "--full-tree", revision)
	if err != nil {
		return nil, 0, err
	}
	long, err := run("ls-tree", "-r", "-z", "-l", "--full-tree", revision)
	if err != nil {
		return nil, 0, err
	}
	tree := map[string]jobinputs.TreeEntry{}
	for _, record := range bytes.Split(long, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		meta, path, ok := bytes.Cut(record, []byte{'\t'})
		fields := strings.Fields(string(meta))
		if !ok || len(fields) != 4 {
			return nil, 0, errors.New("malformed ls-tree record")
		}
		size := int64(-1)
		if fields[3] != "-" {
			if size, err = strconv.ParseInt(fields[3], 10, 64); err != nil {
				return nil, 0, errors.New("malformed ls-tree size")
			}
		}
		tree[string(path)] = jobinputs.TreeEntry{Mode: fields[0], Size: size}
	}
	return tree, len(plain), nil
}
