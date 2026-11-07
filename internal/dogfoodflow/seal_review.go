// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var externalReviewHeader = regexp.MustCompile(`(?m)^dogfood-review: reviewed-head=([0-9a-f]{40}) verdict=PASS$`)

// externalReviewDeclaration is a line that declares an external header; a
// prose mention elsewhere in a line is neither a header nor a format switch.
var externalReviewDeclaration = regexp.MustCompile(`(?m)^dogfood-review:`)
var legacyReviewHeader = regexp.MustCompile(`(?m)^codex-review: reviewed-head=([0-9a-f]{40})(?: .*)?$`)

// sealReview binds caller-provided review bytes to the exact head. Legacy
// reports retain the existing metadata-only descendant rule (DWC-V0-046).
func (c *check) sealReview() string {
	path := os.Getenv("DOGFOOD_REVIEW")
	if path == "" {
		c.say("dogfood-seal: NOTE review-before-seal NOT_OBSERVED (set DOGFOOD_REVIEW to a saved head-bound report)\n")
		return ""
	}
	raw, err := readRegularInput(path)
	if err != nil || len(raw) > 1<<20 || !utf8.Valid(raw) {
		c.refuseSealReview(path)
	}
	head := c.gitValue("rev-parse", "HEAD^{commit}")
	c.sealReviewTarget = head
	matches := externalReviewHeader.FindAllSubmatch(raw, -1)
	reviewed, verdict := "", "PASS"
	if declarations := len(externalReviewDeclaration.FindAllIndex(raw, -1)); declarations > 0 {
		if len(matches) != 1 || declarations != 1 {
			c.refuseSealReview(path)
		}
		reviewed = string(matches[0][1])
		if reviewed != head {
			c.refuseSealReview(path)
		}
	} else {
		pins := legacyReviewHeader.FindAllSubmatch(raw, -1)
		if len(pins) != 1 {
			c.refuseSealReview(path)
		}
		reviewed = string(pins[0][1])
		if !c.gitSucceeds("merge-base", "--is-ancestor", reviewed, head) {
			c.refuseSealReview(path)
		}
		changed, status := c.git(false, "diff", "--no-renames", "--name-only", reviewed, head, "--", ".", ":(exclude).corvint")
		if status != 0 || strings.TrimSpace(changed) != "" {
			c.refuseSealReview(path)
		}
		// Legacy reports have no machine verdict. Do not manufacture a pass.
		verdict = "NOT_OBSERVED"
	}
	evidence := fmt.Sprintf("review-before-seal OBSERVED path=%s reviewed-head=%s sha256=sha256:%s verdict=%s", strconv.Quote(path), reviewed, sha256Hex(raw), verdict)
	c.say("dogfood-seal: %s\n", evidence)
	return evidence
}

func (c *check) refuseSealReview(path string) {
	c.say("dogfood-seal: REFUSE review-not-current DOGFOOD_REVIEW=%s\n", strconv.Quote(path))
	c.say("  fix: save a regular report of at most 1 MiB with one header: dogfood-review: reviewed-head=<exact HEAD SHA> verdict=PASS; rerun with DOGFOOD_REVIEW set to its path\n")
	c.exit(2)
}
