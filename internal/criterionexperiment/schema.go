// Package criterionexperiment implements the experimental, caller-reported
// criterion experiment companion. Its receipts are not execution attestations.
package criterionexperiment

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"strings"

	cw "github.com/Beamfall/corvint/internal/cem/wire"
)

const Profile = "cem-criterion-experiments/0"
const MaxDocument = 4 << 20

type Selector struct {
	Commit string `json:"commit"`
	Path   string `json:"path"`
}
type Authority struct {
	Reviewer     string `json:"reviewer"`
	AnchorCommit string `json:"anchorCommit"`
	AnchorPath   string `json:"anchorPath"`
	AnchorSha256 string `json:"anchorSha256"`
	Independent  bool   `json:"independent"`
}
type Criterion struct {
	Index            int       `json:"index"`
	AcceptanceSha256 string    `json:"acceptanceSha256"`
	Relation         string    `json:"relation"`
	Test             string    `json:"test"`
	Package          string    `json:"package"`
	Assertion        string    `json:"assertion"`
	Hunks            []string  `json:"hunks"`
	Oracle           Selector  `json:"oracle"`
	Authority        Authority `json:"authority"`
	Controls         []string  `json:"controls"`
}
type Request struct {
	Schema         string      `json:"schema"`
	Base           string      `json:"base"`
	Target         string      `json:"target"`
	CEM            string      `json:"cem"`
	Ticket         string      `json:"ticket"`
	Attempt        string      `json:"attempt"`
	Module         string      `json:"module"`
	GoBinary       string      `json:"goBinary"`
	GoSha256       string      `json:"goSha256"`
	TimeoutSeconds int         `json:"timeoutSeconds"`
	Criteria       []Criterion `json:"criteria"`
}
type Binding struct {
	Ticket             string `json:"ticket"`
	AcceptanceRevision string `json:"acceptanceRevision"`
	AcceptanceSha256   string `json:"acceptanceSha256"`
	Attempt            string `json:"attempt"`
	Generation         string `json:"generation"`
	PolicySha256       string `json:"policySha256"`
	ConfigSha256       string `json:"configSha256"`
	CandidateTree      string `json:"candidateTree"`
}
type Plan struct {
	Schema         string  `json:"schema"`
	Request        Request `json:"request"`
	Binding        Binding `json:"binding"`
	CEMSha256      string  `json:"cemSha256"`
	CapturesSha256 string  `json:"capturesSha256"`
}
type Scenario struct {
	Criterion      int    `json:"criterion"`
	Role           string `json:"role"`
	Commit         string `json:"commit"`
	SnapshotSha256 string `json:"snapshotSha256"`
	StdoutSha256   string `json:"stdoutSha256"`
	StderrSha256   string `json:"stderrSha256"`
	ExitCode       int    `json:"exitCode"`
	Complete       bool   `json:"complete"`
	Classification string `json:"classification"`
}
type Receipt struct {
	Schema          string     `json:"schema"`
	PlanSha256      string     `json:"planSha256"`
	Assurance       string     `json:"assurance"`
	OracleAssurance string     `json:"oracleAssurance"`
	Scenarios       []Scenario `json:"scenarios"`
}
type Summary struct {
	Schema                 string   `json:"schema"`
	State                  string   `json:"state"`
	PlanSha256             string   `json:"planSha256"`
	ReceiptSha256          string   `json:"receiptSha256"`
	CEMSha256              string   `json:"cemSha256"`
	AcceptanceSha256       string   `json:"acceptanceSha256"`
	ArtifactManifestSha256 string   `json:"artifactManifestSha256"`
	Binding                Binding  `json:"binding"`
	Criteria               int      `json:"criteria"`
	RegisteredControls     int      `json:"registeredControls"`
	ApplicableControls     int      `json:"applicableControls"`
	ExecutedControls       int      `json:"executedControls"`
	KilledControls         int      `json:"killedControls"`
	Unknowns               []string `json:"unknowns"`
}

func Digest(b []byte) string { v := sha256.Sum256(b); return hex.EncodeToString(v[:]) }
func Encode(v any) []byte {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return b
}
func CanonicalDigest(v any) string {
	b := Encode(v)
	parsed, e := cw.Parse(b)
	if e != nil {
		panic(e)
	}
	return Digest(cw.CanonicalValue(parsed))
}

// Decode uses the existing strict CEM JSON foundation, then exact field-name
// matching before Go decoding (encoding/json alone accepts case variants).
func Decode(b []byte, out any) error {
	if len(b) > MaxDocument {
		return fmt.Errorf("document exceeds bound")
	}
	v, e := cw.Parse(b)
	if e != nil {
		return e
	}
	if e = shape(v, reflect.TypeOf(out).Elem()); e != nil {
		return e
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	return d.Decode(out)
}
func shape(v cw.Value, t reflect.Type) error {
	if t.Kind() == reflect.Struct {
		if v.Kind != cw.KindObject {
			return fmt.Errorf("expected object")
		}
		fields := map[string]reflect.Type{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			fields[f.Tag.Get("json")] = f.Type
		}
		if len(v.Obj.Keys) != len(fields) {
			return fmt.Errorf("missing or extra fields")
		}
		for _, k := range v.Obj.Keys {
			ft, ok := fields[k]
			if !ok {
				return fmt.Errorf("unknown field %q", k)
			}
			if e := shape(v.Obj.Values[k], ft); e != nil {
				return e
			}
		}
	} else if t.Kind() == reflect.Slice {
		if v.Kind != cw.KindArray {
			return fmt.Errorf("expected array")
		}
		for _, x := range v.Arr {
			if e := shape(x, t.Elem()); e != nil {
				return e
			}
		}
	}
	if t.Kind() == reflect.String && v.Kind != cw.KindString {
		return fmt.Errorf("expected string")
	}
	if t.Kind() == reflect.Bool && v.Kind != cw.KindBool {
		return fmt.Errorf("expected boolean")
	}
	if t.Kind() == reflect.Int && v.Kind != cw.KindInt {
		return fmt.Errorf("expected integer")
	}
	return nil
}
func Read(path string) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() {
		return nil, fmt.Errorf("not regular artifact")
	}
	b, e := io.ReadAll(io.LimitReader(f, MaxDocument+1))
	if len(b) > MaxDocument {
		return nil, fmt.Errorf("artifact exceeds bound")
	}
	return b, e
}

var testName = regexp.MustCompile(`^Test[A-Za-z0-9_]+$`)

func literal(p string) bool {
	return cw.ValidatePath(p) == nil && !strings.ContainsAny(p, "*?[]\\:") && !strings.Contains(p, "...")
}
func (r Request) validate() error {
	if r.Schema != Profile || !cw.IsGitOid(r.Base) || !cw.IsGitOid(r.Target) || r.Base == r.Target || !literal(r.Module) || !cw.IsSha256(r.GoSha256) || r.TimeoutSeconds < 1 || r.TimeoutSeconds > 120 || len(r.Criteria) < 1 || len(r.Criteria) > 16 || r.Ticket == "" || r.Attempt == "" {
		return fmt.Errorf("unsupported request identity or bounds")
	}
	for i, c := range r.Criteria {
		if c.Index != i || !cw.IsSha256(c.AcceptanceSha256) || !testName.MatchString(c.Test) || (c.Package != "." && !literal(c.Package)) || c.Assertion == "" || len(c.Assertion) > 256 || strings.ContainsAny(c.Assertion, "\r\n") || len(c.Hunks) == 0 || len(c.Controls) > 4 {
			return fmt.Errorf("invalid criterion %d", i)
		}
		if c.Relation != "repair" && c.Relation != "preservation" && c.Relation != "new-behavior" {
			return fmt.Errorf("invalid relation")
		}
		a := c.Authority
		if !a.Independent || strings.TrimSpace(a.Reviewer) == "" || len(a.Reviewer) > 256 || !cw.IsGitOid(a.AnchorCommit) || a.AnchorCommit == r.Target || !literal(a.AnchorPath) || !cw.IsSha256(a.AnchorSha256) || !cw.IsGitOid(c.Oracle.Commit) || !literal(c.Oracle.Path) || !strings.HasSuffix(c.Oracle.Path, "_test.go") || !strings.HasPrefix(c.Oracle.Path, r.Module+"/") {
			return fmt.Errorf("missing independent oracle authority")
		}
		seen := map[string]bool{}
		for _, h := range c.Hunks {
			if seen[h] || h == "" {
				return fmt.Errorf("duplicate/empty hunk")
			}
			seen[h] = true
		}
		seen = map[string]bool{}
		for _, commit := range c.Controls {
			if !cw.IsGitOid(commit) || commit == r.Target || seen[commit] {
				return fmt.Errorf("invalid control")
			}
			seen[commit] = true
		}
	}
	return nil
}
