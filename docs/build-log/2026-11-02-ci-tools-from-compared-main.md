# CI trusted tools build from the compared main commit (V1-4354, CIN-V0-011)

Finding: after #66 cleared the AFP-V0-030 kill switch (queue run 41568676566 printed
`narrowing verdict: MET`), every shard still ran FULL. The CIN-V0-010 gate failed because
`CORVINT_PR_TOOL_SOURCE` and `CORVINT_TEST_SHADOW_SOURCE` (8daec98ce) lagged main d9e8294dd by 90
selection-path commits. Each selection change on main needed a manual repin PR, and none had
landed, so narrowing was off in practice.

Decision: the pins become floors. A new `pin-main` step resolves the commit the gate already compares
against: the tested merge's first parent, or the queue base or its newest main ancestor. The tools
build from that commit whenever it descends from the pin, so `pin-lag.sh` passes by construction.
Trust is unchanged, because that commit is reviewed main code, the same commit the gate already reads
`pin-lag.sh` and `mq-narrow.sh` from. Off-main or pre-pin cases keep the pin.

Evidence: `TestCINV0011ToolsBuildFromComparedMain` runs the step's own script on a fixture
repository. `TestCINV0010PinLagRefusesNarrowing` and `TestAFPV0047WorkflowBindings` still pass. The
live acceptance is a probe PR reporting NARROWED shards after this merges.
