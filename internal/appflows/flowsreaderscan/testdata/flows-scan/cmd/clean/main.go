// SPDX-License-Identifier: Apache-2.0

// Command clean is the FSV-V1-003 control: it names the path only inside
// help text and through the shared reader's constant.
package main

import "fmt"

const help = `A repository that commits .corvint/flows.json gets one plan.`

func main() { fmt.Println(help) }
