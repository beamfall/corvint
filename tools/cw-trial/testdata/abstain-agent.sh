#!/bin/sh
# Abstaining agent for a model-free retrieval reading (CWT-V0-016): ignores the
# prompt and commits to nothing, so the report carries only the arms' contexts
# and their retrieval-native metrics.
printf '```json\n{"claims":[]}\n```\n'
