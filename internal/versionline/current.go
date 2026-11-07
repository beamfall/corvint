// SPDX-License-Identifier: Apache-2.0
package versionline

// Release is the common release line of Core and all companions. The release-pin
// test binds it to the repository VERSION file; development is a prerelease suffix.
const Release = "1.0.0-rc.3"
const Development = Release + ".dev"
