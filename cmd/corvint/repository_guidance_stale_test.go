package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// RGV-V0-013 (V1-5012), AFP-V0-9730: `corvint review` embeds a plan computed
// by this binary's selector, so it applies the same freshness judgement as
// `corvint affected --base`. A self-binary-stale binary refuses with exit 2,
// the self-binary-stale code and a fix: line without the opt-in review does
// not accept, and writes no receipt. A self-binary-unverified binary still
// reviews: review.affected carries the degradation members, byte-equal to the
// default affected document and to the affectedFull digest, and a
// receipt-level unknown names it. A fresh binary's review carries neither.
func TestGuidanceReviewJudgesSelfBinaryFreshness_V15012(t *testing.T) {
	root, built := selfBinaryRepository(t)
	useAffectedUserCacheDir(t, "", errors.New("no cache in this run"))
	commitSelfBinaryFiles(t, root, map[string]string{"self.go": "package self\n\nfunc Self() int { return 2 }\n"})
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	module := "example.test/self"
	review := func(commit string) (int, string, string) {
		stubSelfBuildInfo(t, selfBinaryInfo(module, commit))
		var stdout, stderr bytes.Buffer
		code := runContext(context.Background(), []string{"--root", root, "review", "--base", built}, strings.NewReader(""), &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}

	code, stdout, stderr := review(built)
	if code != 2 || stdout != "" {
		t.Fatalf("a stale binary's review exited %d with stdout %q, want exit 2 and no receipt; stderr %s", code, stdout, stderr)
	}
	for _, want := range []string{`"code": "self-binary-stale"`, "this binary was built from commit " + shortRevision(built), "1 Go source paths differ at HEAD: self.go", "fix: rebuild the binary from this checkout (go install ./cmd/corvint)", "go run ./cmd/corvint review --base"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("the stale refusal lacks %q:\n%s", want, stderr)
		}
	}
	if strings.Contains(stderr, "--accept-stale-binary") {
		t.Fatalf("the review refusal offers an opt-in review does not accept:\n%s", stderr)
	}

	for _, state := range []struct{ name, commit, code string }{
		{"fresh", head, ""},
		{"unverified", strings.Repeat("ab", 20), selfBinaryUnverifiedCode},
	} {
		code, stdout, stderr := review(state.commit)
		if code != 0 {
			t.Fatalf("%s: review exited %d: %s", state.name, code, stderr)
		}
		var out guidanceReceipt
		if err := json.Unmarshal([]byte(stdout), &out); err != nil || out.Review == nil {
			t.Fatalf("%s: review receipt does not decode: %v\n%s", state.name, err, stdout)
		}
		affectedDoc := out.Review.Affected
		notice := ""
		if state.code != "" {
			notice = affectedDoc.DegradationDetails[state.code]
			if !reflect.DeepEqual(affectedDoc.Degradations, []string{state.code}) || !strings.Contains(notice, "built from commit "+shortRevision(state.commit)) {
				t.Fatalf("%s: review.affected carries %v %v, want the %s degradation", state.name, affectedDoc.Degradations, affectedDoc.DegradationDetails, state.code)
			}
		} else if len(affectedDoc.Degradations) != 0 || len(affectedDoc.DegradationDetails) != 0 {
			t.Fatalf("%s: a fresh binary's review.affected carries %v %v", state.name, affectedDoc.Degradations, affectedDoc.DegradationDetails)
		}
		named := false
		for _, line := range out.Unknown {
			if strings.Contains(line, "self-binary") {
				named = named || state.code != "" && line == "review.affected was planned by a "+state.code+" binary: "+notice
				if !named {
					t.Fatalf("%s: unexpected self-binary unknown %q", state.name, line)
				}
			}
		}
		if named != (state.code != "") {
			t.Fatalf("%s: the receipt-level unknown naming the degradation is %v, want %v: %v", state.name, named, state.code != "", out.Unknown)
		}
		// RGV-V0-015 still holds: the embedded projection is the default affected
		// document and the digest is the --full stdout, both run by this binary.
		var plan, full, ignored bytes.Buffer
		if code := runContext(context.Background(), []string{"--root", root, "affected", "--base", built}, strings.NewReader(""), &plan, &ignored); code != 0 || !bytes.Equal(bytes.TrimSuffix(plan.Bytes(), []byte("\n")), mustCanonical(t, affectedDoc)) {
			t.Fatalf("%s: review.affected differs from the default affected document (exit %d):\n%s\n%s", state.name, code, plan.String(), mustCanonical(t, affectedDoc))
		}
		if code := runContext(context.Background(), []string{"--root", root, "affected", "--base", built, "--full"}, strings.NewReader(""), &full, &ignored); code != 0 {
			t.Fatalf("%s: affected --full exited %d: %s", state.name, code, ignored.String())
		}
		sum := sha256.Sum256(bytes.TrimSuffix(full.Bytes(), []byte("\n")))
		if out.Review.AffectedFull.Digest != guidanceAffectedFullDigestPrefix+hex.EncodeToString(sum[:]) {
			t.Fatalf("%s: affectedFull digest %s does not match the --full stdout", state.name, out.Review.AffectedFull.Digest)
		}
	}
}
