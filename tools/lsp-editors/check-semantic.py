#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
"""Negative evidence regressions; never a substitute for actual editor observations."""
import copy
import importlib.util
from pathlib import Path

spec = importlib.util.spec_from_file_location('harness', Path(__file__).resolve().parents[2] / 'script/qualify-lsp-editors.py')
harness = importlib.util.module_from_spec(spec); spec.loader.exec_module(harness)
uri = 'file:///private/fixture/main.go'
def row(direction, method=None, **kwargs):
    message = kwargs
    if method: message['method'] = method
    return {'direction': direction, 'message': message}
for encoding, target in [('utf-8', 13), ('utf-16', 11)]:
    definition = [{'uri': uri, 'range': {'start': {'line': 1, 'character': target}, 'end': {'line': 1, 'character': target + 1}}}]
    observation = {'initializeResult': {'capabilities': {'positionEncoding': encoding}}, 'semantic': {'definition': definition, 'bufferModified': True, 'version': 2}}
    rows = [row('client-to-server', 'textDocument/didOpen', params={'textDocument': {'uri': uri, 'version': 1, 'text': 'package p\nvar disk int\n'}}),
            row('client-to-server', 'textDocument/didChange', params={'textDocument': {'uri': uri, 'version': 2}, 'contentChanges': [{'text': 'package p\n/*😀*/ var x int\nvar y = x\n'}]}),
            row('client-to-server', 'textDocument/definition', id=7, params={'textDocument': {'uri': uri}, 'position': {'line': 2, 'character': 8}}),
            row('server-to-client', id=7, result=definition)]
    assert harness.validate_semantic(rows, observation, uri, True)['valid']
    bad = [rows[2], rows[3], rows[0], rows[1]]
    assert not harness.validate_semantic(bad, observation, uri, True)['valid'], 'out-of-order accepted'
    bad = copy.deepcopy(rows); bad[1]['message']['params']['textDocument']['uri'] = 'file:///other.go'
    assert not harness.validate_semantic(bad, observation, uri, True)['valid'], 'foreign URI accepted'
    bad = copy.deepcopy(rows); bad[1]['message']['params']['textDocument']['version'] = 1
    assert not harness.validate_semantic(bad, observation, uri, True)['valid'], 'non-increasing version accepted'
    stale = copy.deepcopy(observation); stale['semantic']['version'] = 1
    assert not harness.validate_semantic(rows, stale, uri, True)['valid'], 'client snapshot mismatch accepted'
    late = copy.deepcopy(rows[1]); late['message']['params']['textDocument']['version'] = 3
    assert not harness.validate_semantic(rows[:3] + [late] + rows[3:], observation, uri, True)['valid'], 'changed-before-result accepted'
print('semantic ordered URI/version/snapshot evidence regressions passed; real client qualification unchanged')
