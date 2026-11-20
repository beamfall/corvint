// SPDX-License-Identifier: Apache-2.0

package appflows

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/gitrun"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testinventory/reader"
)

const proposalProfile = "corvint-flow-link-proposals/0"
const proposalFile = "flow-link-proposals.json"
const confidenceFormula = "flow-link-confidence/1"

type LinkProposals struct {
	Profile   string         `json:"profile"`
	Commit    string         `json:"pinned_commit"`
	Proposals []LinkProposal `json:"proposals"`
}
type LinkProposal struct {
	Flow          string      `json:"flow_id"`
	Variation     string      `json:"variation_id"`
	Test          string      `json:"test_id"`
	Evidence      []string    `json:"evidence"`
	Confidence    json.Number `json:"confidence"`
	Formula       string      `json:"confidenceFormula"`
	TestHash      string      `json:"test_content_hash"`
	VariationHash string      `json:"variation_digest"`
}

// Hash the JSON member as stored, preserving the distinction between missing, null
// and empty arrays. Canonicalization removes only JSON representation differences.
func (s *IntentSet) bindVariationDigests(raw []byte, intent FlowIntent) error {
	if len(intent.Variations) == 0 {
		return nil
	}
	var records struct {
		Variations []jsontext.Value `json:"variations"`
	}
	if err := json.Unmarshal(raw, &records); err != nil {
		return err
	}
	if s.variationDigests == nil {
		s.variationDigests = map[string]string{}
	}
	for i, record := range records.Variations {
		if err := record.Canonicalize(); err != nil {
			return err
		}
		s.variationDigests[intent.Variations[i].VariationID] = fmt.Sprintf("%x", sha256.Sum256(record))
	}
	return nil
}

type linkVariation struct {
	flow, id, digest string
	sources          []string
}

func linkVariations(set IntentSet) map[string]linkVariation {
	variations := map[string]linkVariation{}
	for _, f := range set.Flows {
		var sources []string
		for _, l := range f.Links {
			if l.Basis == "declared" && l.Target.Type == "source" {
				sources = appendUnique(sources, l.Target.Path)
			}
		}
		slices.Sort(sources)
		for _, v := range f.Variations {
			variations[v.VariationID] = linkVariation{flow: f.FlowID, id: v.VariationID, digest: set.variationDigests[v.VariationID], sources: sources}
		}
	}
	return variations
}

func browserTest(runner string, t testinventory.Test) bool {
	return runner == testid.Playwright || (runner == testid.Pytest || runner == testid.RSpec) && t.Kind == affected.TestKindE2E ||
		runner == testid.RSpec && strings.Contains("/"+t.File, "/spec/system/")
}

func linkScore(id testid.ID, v linkVariation, source []byte, graph *affected.Graph, commit string) (json.Number, []string) {
	evidence := []string{"title:absent", "source-tag:absent", "static-reach:absent"}
	score := 0
	title := id.Segments
	if id.Runner == testid.Playwright && len(title) > 0 {
		title = title[1:]
	} // The first segment names the project.
	for _, part := range title {
		if strings.EqualFold(part, v.flow) || strings.EqualFold(part, v.id) {
			evidence[0] = "title:" + commit + ":" + id.String()
			score += 35
			break
		}
	}
	// Tags are delimited tokens. An @flow:cart tag cannot name cart-v2.
	tags := strings.FieldsFunc(string(source), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("@:._-", r))
	})
	if slices.Contains(tags, "@flow:"+v.flow) {
		evidence[1] = "source-tag:" + commit + ":" + id.File + " @flow:" + v.flow
		score += 25
	}
	if graph != nil {
		for _, p := range v.sources {
			if via := reach(graph, changedUnits(graph, []string{p}), []string{p}, id.File); len(via) > 0 {
				evidence[2] = "static-reach:" + commit + ":" + strings.Join(via, " > ")
				score += 40
				break
			}
		}
	}
	return json.Number(strconv.FormatFloat(float64(score)/100, 'f', 2, 64)), evidence
}

func proposalDirectory(root string) (string, error) {
	gitDir, _, err := gitauth.WorktreeDirectories(root)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(gitDir, "corvint")
	info, err := os.Lstat(dir)
	if err == nil && !info.IsDir() {
		return "", errors.New("private proposal directory is not a directory")
	}
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return dir, nil
}

func ProposeLinks(ctx context.Context, root, flow string, all bool) (LinkProposals, error) {
	result := LinkProposals{Profile: proposalProfile, Proposals: []LinkProposal{}}
	g := testinventory.Git{Root: root, Executable: gitstatus.Executable()}
	head, err := g.Head(ctx)
	if err != nil {
		return result, err
	}
	result.Commit = head
	tree, err := testinventory.NewCommitTree(ctx, g, head)
	if err != nil {
		return result, err
	}
	body, err := tree.Read(FlowsDeclarationPath, maxFlowsDeclarationBytes)
	if err == nil {
		d, err := decodeFlowsDeclaration(body)
		if err != nil {
			return result, err
		}
		if err = collectProposals(ctx, root, tree, d, flow, all, &result); err != nil {
			return result, err
		}
	} else if !errors.Is(err, fs.ErrNotExist) || flow != "" {
		return result, err
	}
	data, err := json.Marshal(result)
	if err != nil {
		return result, err
	}
	if len(data)+1 > MaxBytes {
		return result, errors.New("flow proposals exceed byte limit")
	}
	dir, err := proposalDirectory(root)
	if err != nil {
		return result, err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return result, err
	}
	tmp, err := os.CreateTemp(dir, ".proposals-*")
	if err != nil {
		return result, err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(append(data, '\n'))
	closeErr := tmp.Close()
	if err != nil {
		return result, err
	}
	if closeErr != nil {
		return result, closeErr
	}
	return result, os.Rename(tmp.Name(), filepath.Join(dir, proposalFile))
}

func proposalGraph(ctx context.Context, root, commit string) *affected.Graph {
	repo, err := gitauth.Open(root, gitrun.NewBudget(gitrun.DefaultOperations, gitrun.DefaultTotalBudget))
	if err != nil {
		return nil
	}
	defer repo.BeginObjectSession()()
	source, err := repo.RevisionFS(ctx, commit, affected.MaxSourceBytes)
	if err != nil {
		return nil
	}
	graph, _ := affected.BuildFS(source, languages.All()...)
	return graph
}

func collectProposals(ctx context.Context, root string, tree *testinventory.CommitTree, d *FlowsDeclaration, flow string, all bool, result *LinkProposals) error {
	set, err := LoadIntentsAt(ctx, root, d.Dir, result.Commit)
	if err != nil {
		return err
	}
	variants := linkVariations(set)
	if flow != "" && !slices.ContainsFunc(set.Flows, func(f FlowIntent) bool { return f.FlowID == flow }) {
		return errors.New("unknown flow")
	}
	data, err := tree.Read(testinventory.Path, testinventory.MaxBytes)
	if err != nil {
		return err
	}
	inv, err := testinventory.Decode(data)
	if err != nil {
		return err
	}
	verdict := testinventory.Judge(tree)
	listed := testinventory.Listed(data, verdict)
	files, _ := tree.Files()
	bindings := linkCandidate{files: files, variants: variants, invalidPins: invalidLinkPins(ctx, tree.Git, result.Commit, d)}
	byTest := linksByTest(d.Links)
	graph := proposalGraph(ctx, root, result.Commit)
	seen := map[string]bool{}
	comparisons := 0
	keys := slices.Sorted(maps.Keys(variants))
	for _, instance := range inv.Instances {
		for _, t := range instance.Tests {
			if !browserTest(instance.Runner, t) || seen[t.ID] || !slices.Contains(listed[instance.Key][t.File], t.ID) {
				continue
			}
			seen[t.ID] = true
			id, _ := testid.Parse(t.ID)
			if !all && bindings.assignmentReason(byTest[t.ID], t, id, graph) == "" {
				continue
			}
			source, _ := tree.Read(t.File, MaxBytes)
			start := len(result.Proposals)
			var best json.Number
			for _, key := range keys {
				v := variants[key]
				if flow != "" && flow != v.flow {
					continue
				}
				comparisons++
				if comparisons > maxFlowPlanInventory {
					return errors.New("flow proposal comparison limit exceeded; use --flow")
				}
				score, evidence := linkScore(id, v, source, graph, result.Commit)
				if score < best {
					continue
				}
				if score > best {
					result.Proposals = result.Proposals[:start]
					best = score
				}
				result.Proposals = append(result.Proposals, LinkProposal{v.flow, v.id, t.ID, evidence, score, confidenceFormula, files[t.File], v.digest})
			}
		}
	}
	slices.SortFunc(result.Proposals, func(a, b LinkProposal) int {
		if c := strings.Compare(string(b.Confidence), string(a.Confidence)); c != 0 {
			return c
		}
		if c := strings.Compare(a.Test, b.Test); c != 0 {
			return c
		}
		return strings.Compare(a.Variation, b.Variation)
	})
	return nil
}

func acceptRefusal(code string) error { return &gokernel.Error{Code: code, Message: code} }

func AcceptLinks(ctx context.Context, root, flow, by, variation string, only []string) (int, error) {
	dir, err := proposalDirectory(root)
	if err != nil {
		return 0, err
	}
	filename := filepath.Join(dir, proposalFile)
	if _, err = os.Lstat(filename); os.IsNotExist(err) {
		return 0, acceptRefusal("flow-proposals-absent")
	}
	data, err := ReadFile(filename)
	if err != nil {
		return 0, err
	}
	var proposals LinkProposals
	if err = json.Unmarshal(data, &proposals); err != nil || proposals.Profile != proposalProfile {
		return 0, errors.New("invalid flow proposals")
	}
	g := testinventory.Git{Root: root, Executable: gitstatus.Executable()}
	head, err := g.Head(ctx)
	if err != nil {
		return 0, err
	}
	if head != proposals.Commit {
		return 0, acceptRefusal("flow-proposals-stale")
	}
	if strings.TrimSpace(by) == "" || flow == "" {
		return 0, errors.New("accept requires --flow ID --by NAME")
	}
	// Serialize two reviewers updating different flows in this worktree.
	lock, err := os.OpenFile(filepath.Join(dir, "flow-link-accept.lock"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return 0, err
	}
	defer os.Remove(lock.Name())
	defer lock.Close()
	original, err := readSource(root, FlowsDeclarationPath)
	if err != nil {
		return 0, err
	}
	d, err := decodeFlowsDeclaration(original)
	if err != nil {
		return 0, err
	}
	tree, err := testinventory.NewCommitTree(ctx, g, head)
	if err != nil {
		return 0, err
	}
	pinned, err := tree.Read(FlowsDeclarationPath, maxFlowsDeclarationBytes)
	if err != nil {
		return 0, err
	}
	declared, err := decodeFlowsDeclaration(pinned)
	if err != nil {
		return 0, err
	}
	if declared.Dir != d.Dir {
		return 0, acceptRefusal("flow-proposals-stale")
	}
	// Rebuild the eligible bindings from immutable inputs. A private file edited after
	// proposal cannot introduce another test, score or citation into the reviewed record.
	verified := LinkProposals{Commit: head}
	if err := collectProposals(ctx, root, tree, declared, flow, true, &verified); err != nil {
		return 0, err
	}
	byVariation := map[string]LinkProposal{}
	for _, p := range verified.Proposals {
		byVariation[p.Test+"\x00"+p.Variation] = p
	}
	count := 0
	found := map[string]bool{}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, p := range proposals.Proposals {
		if p.Flow != flow || variation != "" && p.Variation != variation || len(only) > 0 && !slices.Contains(only, p.Test) {
			continue
		}
		expected, ok := byVariation[p.Test+"\x00"+p.Variation]
		if !ok || p.VariationHash != expected.VariationHash || p.TestHash != expected.TestHash || p.Formula != expected.Formula || p.Confidence != expected.Confidence || !slices.Equal(p.Evidence, expected.Evidence) {
			return 0, acceptRefusal("flow-proposals-stale")
		}
		score, err := p.Confidence.Float64()
		if err != nil {
			return 0, err
		}
		l := FlowTestLink{TestID: p.Test, VariationID: p.Variation, Evidence: p.Evidence, Confidence: score, ConfidenceFormula: p.Formula, AcceptedBy: by, AcceptedAt: now, PinnedCommit: head, TestContentHash: p.TestHash, VariationDigest: p.VariationHash}
		d.Links = slices.DeleteFunc(d.Links, func(old FlowTestLink) bool { return old.TestID == p.Test && old.VariationID == p.Variation })
		d.Links = append(d.Links, l)
		found[p.Test] = true
		count++
	}
	if count == 0 || slices.ContainsFunc(only, func(id string) bool { return !found[id] }) {
		return 0, errors.New("no matching flow proposals")
	}
	body, err := EncodeFlowsDeclaration(*d)
	if err != nil {
		return 0, err
	}
	if current, err := g.Head(ctx); err != nil || current != head {
		return 0, acceptRefusal("flow-proposals-stale")
	}
	current, err := readSource(root, FlowsDeclarationPath)
	if err != nil || !bytes.Equal(original, current) {
		return 0, acceptRefusal("flow-proposals-stale")
	}
	if err = ReplaceConfined(root, FlowsDeclarationPath, body); err != nil {
		return 0, err
	}
	return count, nil
}
