// SPDX-License-Identifier: AGPL-3.0-or-later
const vscode = require('vscode');
const fs = require('fs');
const path = require('path');
const { LanguageClient } = require(path.join(process.env.CORVINT_EDITOR_CLIENT_MODULE, 'lib/node/main.js'));
let client;
exports.activate = async function () {
  const report = { client: 'vscode', version: vscode.version, status: 'FAILED' };
  const write = () => fs.writeFileSync(process.env.CORVINT_EDITOR_RESULT, JSON.stringify(report));
  const argv = JSON.parse(process.env.CORVINT_EDITOR_SERVER_ARGV);
  const timeout = setTimeout(async () => {
    report.status = 'TIMEOUT'; write();
    try { if (client) await client.stop(); } catch (_) {}
    await vscode.commands.executeCommand('workbench.action.quit');
  }, Number(process.env.CORVINT_EDITOR_TIMEOUT_MS));
  try {
    client = new LanguageClient('corvint-qualification', 'Corvint qualification',
      { command: argv[0], args: argv.slice(1) },
      { documentSelector: [{ scheme: 'file', language: 'go' }],
        workspaceFolder: vscode.workspace.workspaceFolders[0] });
    await client.start();
    report.initializeResult = client.initializeResult;
    report.root = vscode.workspace.workspaceFolders[0].uri.toString();
    const document = await vscode.workspace.openTextDocument(process.env.CORVINT_EDITOR_DOCUMENT);
    await vscode.window.showTextDocument(document);
    if (process.env.CORVINT_EDITOR_CONTEXT === '1') {
      const edit = new vscode.WorkspaceEdit();
      edit.replace(document.uri, new vscode.Range(document.positionAt(0), document.positionAt(document.getText().length)),
        'package p\n/*😀*/ var x int\nvar y = x\n');
      if (!await vscode.workspace.applyEdit(edit)) throw new Error('unsaved context fixture edit rejected');
      await new Promise(resolve => setTimeout(resolve, 200));
      const result = await client.sendRequest('corvint/context', {textDocument: {uri: document.uri.toString()}, task: process.env.CORVINT_EDITOR_CONTEXT_TASK, limit: 20});
      report.context = {result, bufferModified: document.isDirty, version: document.version};
      await vscode.commands.executeCommand('workbench.action.revertAndCloseActiveEditor');
    }
    if (process.env.CORVINT_EDITOR_SEMANTIC === '1') {
      const edit = new vscode.WorkspaceEdit();
      edit.replace(document.uri, new vscode.Range(document.positionAt(0), document.positionAt(document.getText().length)),
        'package p\n/*😀*/ var x int\nvar y = x\n');
      if (!await vscode.workspace.applyEdit(edit)) throw new Error('unsaved fixture edit rejected');
      await new Promise(resolve => setTimeout(resolve, 200));
      const position = client.code2ProtocolConverter.asPosition(new vscode.Position(2, 8));
      const definition = await client.sendRequest('textDocument/definition', {textDocument: {uri: document.uri.toString()}, position});
      report.semantic = {definition, requestPosition: position, bufferModified: document.isDirty, version: document.version};
      await vscode.commands.executeCommand('workbench.action.revertAndCloseActiveEditor');
    }
    await client.stop();
    report.status = 'LIFECYCLE_OBSERVED';
  } catch (err) { report.error = String(err); }
  finally {
    clearTimeout(timeout); write();
    await vscode.commands.executeCommand('workbench.action.quit');
  }
};
exports.deactivate = async function () { if (client && client.isRunning()) await client.stop(); };
