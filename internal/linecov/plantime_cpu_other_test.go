// SPDX-License-Identifier: Apache-2.0
//go:build !unix && !windows

package linecov

import (
	"time"
)

func planCPUTime() (time.Duration, error) {
	return 0, errPlanCPUUnavailable
}
