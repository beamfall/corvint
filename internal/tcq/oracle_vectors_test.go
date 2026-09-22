package tcq

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// The vectors in testdata/oracle-vectors.json are captured from the Python
// reference implementation in src/context_corvint_test_claim.py — never from this
// package. Candidate code may not certify itself, so every byte compared here
// originates in the oracle.

type oracleBlob struct {
	OID         string  `json:"oid"`
	Source      string  `json:"source"`
	Error       *string `json:"error"`
	PythonSpans []struct {
		RuntimeName string `json:"runtimeName"`
		NameStart   int    `json:"nameStart"`
		NameEnd     int    `json:"nameEnd"`
		DocStart    int    `json:"docStart"`
		DocEnd      int    `json:"docEnd"`
	} `json:"pythonSpans"`
	GoSpans []struct {
		RuntimeName string `json:"runtimeName"`
		NameStart   int    `json:"nameStart"`
		NameEnd     int    `json:"nameEnd"`
		Cases       []struct {
			Start int    `json:"start"`
			End   int    `json:"end"`
			Value string `json:"value"`
		} `json:"cases"`
	} `json:"goSpans"`
	Units []struct {
		Path             string `json:"path"`
		BodyStart        int64  `json:"bodyStart"`
		BodyEnd          int64  `json:"bodyEnd"`
		BodySha256       string `json:"bodySha256"`
		ExecutionKey     string `json:"executionKey"`
		ExtractorProfile string `json:"extractorProfile"`
		RuntimeName      string `json:"runtimeName"`
		AssociationKind  string `json:"associationKind"`
		Empty            bool   `json:"empty"`
		Skipped          bool   `json:"skipped"`
		ID               string `json:"id"`
		Wire             string `json:"wire"`
	} `json:"units"`
	Candidates []struct {
		Producer string `json:"producer"`
		Selector string `json:"selector"`
		Start    int    `json:"start"`
		End      int    `json:"end"`
	} `json:"candidates"`
}

type oracleVectors struct {
	Blobs              map[string]oracleBlob `json:"blobs"`
	Canonical          map[string]string     `json:"canonical"`
	ExecutionKeys      map[string]string     `json:"executionKeys"`
	SelectorFragments  map[string]string     `json:"selectorFragments"`
	NormalPaths        map[string]string     `json:"normalPaths"`
	Command            string                `json:"command"`
	CommandNotAttested string                `json:"commandNotAttested"`
}

func loadVectors(t *testing.T) oracleVectors {
	t.Helper()
	raw, err := os.ReadFile("testdata/oracle-vectors.json")
	if err != nil {
		t.Fatalf("read oracle vectors: %v", err)
	}
	var vectors oracleVectors
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatalf("decode oracle vectors: %v", err)
	}
	return vectors
}

// UnitRows renders the oracle's unit rows in the same fingerprint the Go units
// are compared with.
func (blob oracleBlob) UnitRows() []string {
	rows := make([]string, 0, len(blob.Units))
	for _, unit := range blob.Units {
		rows = append(rows, fmt.Sprintf("%s|%d|%d|%s|%s|%s|%s|%v|%v|%s|%s",
			unit.Path, unit.BodyStart, unit.BodyEnd, unit.BodySha256, unit.ExecutionKey,
			unit.ExtractorProfile, unit.AssociationKind, unit.Empty, unit.Skipped,
			unit.ID, unit.Wire))
	}
	return rows
}

// CandidateKeys renders the oracle's claim-extractor proposals, keeping only the
// producers the TCQ-V0-005 dispatch table can reach.
func (blob oracleBlob) CandidateKeys() map[string]bool {
	keys := map[string]bool{}
	for _, candidate := range blob.Candidates {
		keys[fmt.Sprintf("%s|%s|%d|%d", candidate.Producer, candidate.Selector, candidate.Start, candidate.End)] = true
	}
	return keys
}
