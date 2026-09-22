#!/bin/sh
# Fake agent for proving the cw-trial pipeline without a model: ignores the
# prompt (its only argument) and replies with one fixed claims block.
cat <<'EOF'
The fake agent commits to one test file regardless of the task.

```json
{"claims":[{"kind":"test-file","value":"tests/help_test.rs","confidence":"certain","evidence":"none"}]}
```
EOF
