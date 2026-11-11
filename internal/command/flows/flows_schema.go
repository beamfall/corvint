// SPDX-License-Identifier: Apache-2.0

package flows

import (
	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// refuseUnreadableFlows is the check the flow reports that join flows with
// test evidence or results (`flows gaps`, `flows status`, `flows impact`)
// make before reading anything else (FSV-V1-005, PROD-DOCS-011): a flows
// declaration the shared reader refuses, of an unsupported schema version or
// otherwise invalid, ends the command with the reader's reason and a non-zero
// exit, and no partial report is written. No declaration, or one this build
// reads, changes nothing.
func refuseUnreadableFlows(root string) error {
	refusal := appflows.ReportRefusal(root)
	if refusal == nil {
		return nil
	}
	return &gokernel.Error{Code: refusal.Code(), Message: refusal.Reason + ": " + refusal.Error()}
}
