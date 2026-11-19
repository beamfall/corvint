# SPDX-License-Identifier: Apache-2.0
# CIN-V1-942: jq -s -f runtime-edges.jq narrowing.json plan.json.
# Preserve the existing decision and retain the trusted planner's edge-local widening.
.[0] as $record | .[1] as $plan |
[$plan.plan.unknown[]? |
  select(.reason == "LANGUAGE_FRONTIER") | .detail |
  select(startswith("runtime:unresolved-edge: tests="))] as $edges |
$record | if ($edges | length) > 0 then
  . + {degradedRuntimeEdges: $edges} else . end
