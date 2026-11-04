// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

// reboundCommitNeeded stops the standalone rebind before the predictable
// excluded-artifact/status/outcome cascade (DWC-V0-044). Nested finish-lane
// passes already own the commit transition and its compact output.
func (c *change) reboundCommitNeeded() bool {
	if c.nested || !c.cemPrepared || !c.manifestValid || !c.complete() ||
		!c.gitSucceeds("cat-file", "-e", c.target+":.corvint/change.cem.json") {
		return false
	}
	// Include staged changes and untracked paths: another dirty input is not
	// the commit-only recovery this shortcut describes.
	switch c.gitValue("status", "--porcelain", "--untracked-files=all") {
	case " M .corvint/change.cem.json", "M  .corvint/change.cem.json", "MM .corvint/change.cem.json":
	default:
		return false
	}
	for _, name := range []string{"ocm-aggregate", "cem-status", "local-outcome"} {
		c.addStep(name, "NOT_PRODUCED", "commit-rebind-required")
	}
	c.renderReport()
	c.say("dogfood-change: NOT_PRODUCED commit-rebind-required; commit the rebound .corvint/change.cem.json and rerun corvint dogfood change %s\n", c.base)
	return true
}
