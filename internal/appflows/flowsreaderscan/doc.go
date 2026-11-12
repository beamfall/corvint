// SPDX-License-Identifier: Apache-2.0

// Package flowsreaderscan holds the FSV-V1-003 source scan: no package of
// cmd/ or internal/ other than internal/appflows names `.corvint/flows.json`,
// so the testing and documentation products read the file only through
// appflows.ReadFlowsDeclaration. It is its own package so that only this scan,
// not every appflows test, is selected for each edit under cmd/ and internal/.
package flowsreaderscan
