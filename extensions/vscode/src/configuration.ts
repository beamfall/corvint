import * as vscode from "vscode";

type Inspection<T> = ReturnType<vscode.WorkspaceConfiguration["inspect"]> & {
  readonly globalLanguageValue?: T;
  readonly workspaceLanguageValue?: T;
  readonly workspaceFolderLanguageValue?: T;
};

function isExplicit<T>(inspection: Inspection<T> | undefined): boolean {
  return inspection !== undefined && [
    inspection.globalValue,
    inspection.workspaceValue,
    inspection.workspaceFolderValue,
    inspection.globalLanguageValue,
    inspection.workspaceLanguageValue,
    inspection.workspaceFolderLanguageValue,
  ].some((value) => value !== undefined);
}

export function configurationValue<T>(key: string, fallback: T, resource: vscode.Uri): T {
  const current = vscode.workspace.getConfiguration("corvint", resource);
  if (isExplicit(current.inspect<T>(key))) return current.get<T>(key, fallback);
  return fallback;
}
