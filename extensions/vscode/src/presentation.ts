import { realpath, stat } from "node:fs/promises";
import * as path from "node:path";
import * as vscode from "vscode";
import {
  display,
  diagnosticClass,
  orderedEvidence,
  orderedResults,
  projectionClass,
  stableEvidenceId,
  stableProjectionId,
  stableResultId,
  type CorvintSnapshot,
  type EvidenceItem,
  type ProjectionClass,
  type ResultItem,
} from "./model.js";

export type CorvintStatus = "Restricted" | "No CLI" | "Incompatible" | "Ready" | "Busy" | "Degraded" | "Unsupported";

interface TreeNode {
  readonly id: string;
  readonly label: string;
  readonly description?: string;
  readonly tooltip: string;
  readonly icon?: string;
  readonly children: readonly TreeNode[];
}

interface DecorationRanges {
  readonly evidence: vscode.Range[];
  readonly candidate: vscode.Range[];
  readonly unknown: vscode.Range[];
}

interface DiagnosticProjection {
  readonly uri: vscode.Uri;
  readonly diagnostics: vscode.Diagnostic[];
  readonly ranges: DecorationRanges;
}

export class CorvintTreeProvider implements vscode.TreeDataProvider<TreeNode>, vscode.Disposable {
  private readonly changed = new vscode.EventEmitter<TreeNode | undefined>();
  private roots: readonly TreeNode[] = [];
  readonly onDidChangeTreeData = this.changed.event;

  replace(roots: readonly TreeNode[]): void {
    this.roots = roots;
    this.changed.fire(undefined);
  }

  getTreeItem(node: TreeNode): vscode.TreeItem {
    const item = new vscode.TreeItem(node.label, node.children.length === 0
      ? vscode.TreeItemCollapsibleState.None
      : vscode.TreeItemCollapsibleState.Expanded);
    item.id = node.id;
    if (node.description !== undefined) {
      item.description = node.description;
    }
    item.tooltip = node.tooltip;
    if (node.icon !== undefined) {
      item.iconPath = new vscode.ThemeIcon(node.icon);
    }
    return item;
  }

  getChildren(node?: TreeNode): TreeNode[] {
    return [...(node === undefined ? this.roots : node.children)];
  }

  dispose(): void {
    this.changed.dispose();
  }
}

export class CorvintPresentation implements vscode.Disposable {
  readonly evidence = new CorvintTreeProvider();
  readonly impact = new CorvintTreeProvider();
  readonly why = new CorvintTreeProvider();
  private readonly diagnostics = vscode.languages.createDiagnosticCollection("corvint");
  private readonly evidenceDecoration = vscode.window.createTextEditorDecorationType({
    isWholeLine: true,
    overviewRulerColor: new vscode.ThemeColor("editorInfo.foreground"),
    overviewRulerLane: vscode.OverviewRulerLane.Right,
    before: { contentText: "Corvint evidence · ", color: new vscode.ThemeColor("descriptionForeground") },
  });
  private readonly candidateDecoration = vscode.window.createTextEditorDecorationType({
    isWholeLine: true,
    overviewRulerColor: new vscode.ThemeColor("editorInfo.foreground"),
    overviewRulerLane: vscode.OverviewRulerLane.Right,
    before: { contentText: "Corvint candidate · ", color: new vscode.ThemeColor("descriptionForeground") },
  });
  private readonly unknownDecoration = vscode.window.createTextEditorDecorationType({
    isWholeLine: true,
    overviewRulerColor: new vscode.ThemeColor("editorWarning.foreground"),
    overviewRulerLane: vscode.OverviewRulerLane.Right,
    before: { contentText: "Corvint unknown · ", color: new vscode.ThemeColor("editorWarning.foreground") },
  });
  private currentRanges = new Map<string, DecorationRanges>();
  private generation = 0;

  constructor(readonly status: vscode.StatusBarItem) {}

  setStatus(state: CorvintStatus, detail?: string): void {
    this.status.text = `$(symbol-namespace) Corvint: ${state}`;
    this.status.name = `Corvint ${state}`;
    this.status.accessibilityInformation = { label: `Corvint ${state}${detail === undefined ? "" : `: ${display(detail, 300)}`}` };
    this.status.tooltip = detail === undefined ? `Corvint ${state}` : `Corvint ${state}: ${display(detail, 2_000)}`;
    this.status.show();
  }

  clear(message = "No Corvint snapshot"): void {
    this.generation += 1;
    const node = leaf(`empty:${this.generation}`, message, "No receipt or observation is current.", "circle-slash");
    this.evidence.replace([node]);
    this.impact.replace([node]);
    this.why.replace([node]);
    this.diagnostics.clear();
    this.currentRanges.clear();
    this.applyDecorations();
  }

  async apply(snapshot: CorvintSnapshot, root: string): Promise<void> {
    const generation = ++this.generation;
    const prefix = snapshot.snapshotKey;
    const sortedResults = orderedResults(snapshot.results);
    const evidenceRoots = sortedResults.map((result) => resultNode(prefix, result));
    const impactRoots = snapshot.operation === "impact"
      ? impactSections(prefix, sortedResults, snapshot)
      : [leaf(`${prefix}:impact-empty`, "No impact receipt", "Run Corvint: Inspect Active File Impact.", "circle-slash")];
    const whyRoots = orderedEvidence(snapshot.evidence).map((evidence) => whyNode(prefix, evidence));
    for (const uncertainty of orderedStrings(snapshot.uncertainty)) {
      whyRoots.push(leaf(`${prefix}:uncertainty:${stableProjectionId([uncertainty])}`, `Unknown: ${display(uncertainty, 180)}`, uncertainty, "warning"));
    }
    if (snapshot.transportAuthority !== undefined) {
      whyRoots.push(transportAuthorityNode(prefix, snapshot));
    }
    whyRoots.push(leaf(`${prefix}:profile`, snapshot.adapterProfile, `Revision: ${snapshot.revision}\nFreshness: ${snapshot.freshness}\nWorktree mixed paths: ${snapshot.worktreeMixedPathCount}`, "info"));
    for (const suggestion of orderedStrings(snapshot.verification)) {
      whyRoots.push(leaf(`${prefix}:verification:${stableProjectionId([suggestion])}`, `Suggested verification: ${display(suggestion, 170)}`, "Inert Corvint verification suggestion; this extension does not execute it.", "beaker"));
    }
    const diagnostics = await this.collectDiagnostics(snapshot, root, generation);
    if (diagnostics === undefined || generation !== this.generation) {
      return;
    }
    this.evidence.replace(cap(evidenceRoots));
    this.impact.replace(cap(impactRoots));
    this.why.replace(cap(whyRoots));
    this.diagnostics.clear();
    this.currentRanges.clear();
    for (const [key, bucket] of diagnostics) {
      this.diagnostics.set(bucket.uri, bucket.diagnostics);
      this.currentRanges.set(key, bucket.ranges);
    }
    this.applyDecorations();
  }

  private async collectDiagnostics(
    snapshot: CorvintSnapshot,
    root: string,
    generation: number,
  ): Promise<Map<string, DiagnosticProjection> | undefined> {
    const byUri = new Map<string, DiagnosticProjection>();
    let diagnosticCount = 0;
    let rangeCount = 0;
    let truncated = false;
    for (const evidence of orderedEvidence(snapshot.evidence)) {
      const uri = await containedFile(root, evidence.path, () => generation === this.generation);
      if (uri === undefined || generation !== this.generation) {
        if (generation !== this.generation) {
          return undefined;
        }
        continue;
      }
      let document: vscode.TextDocument;
      try {
        document = await vscode.workspace.openTextDocument(uri);
      } catch {
        if (generation !== this.generation) {
          return undefined;
        }
        continue;
      }
      if (generation !== this.generation) {
        return undefined;
      }
      const line = evidence.line - 1;
      if (line < 0 || line >= document.lineCount) {
        continue;
      }
      const key = uri.toString();
      const bucket = byUri.get(key) ?? { uri, diagnostics: [], ranges: { evidence: [], candidate: [], unknown: [] } };
      const range = document.lineAt(line).range;
      if (rangeCount < 2_000) {
        bucket.ranges[decorationClass(evidence.resultState)].push(range);
        rangeCount += 1;
      }
      const severity = diagnosticSeverity(evidence.resultState);
      if (severity !== undefined) {
        if (diagnosticCount >= 499 || bucket.diagnostics.length >= 99) {
          truncated = true;
        } else {
          const diagnostic = new vscode.Diagnostic(range, display(`${evidence.reason} [${evidence.authority}; ${evidence.confidence}]`, 1_000), severity);
          diagnostic.source = "Corvint";
          diagnostic.code = evidence.resultId;
          bucket.diagnostics.push(diagnostic);
          diagnosticCount += 1;
        }
      }
      byUri.set(key, bucket);
    }
    if (generation !== this.generation) {
      return undefined;
    }
    if (truncated) {
      const first = [...byUri.values()].find((bucket) => bucket.diagnostics.length > 0);
      const range = first?.diagnostics[0]?.range;
      if (first !== undefined && range !== undefined) {
        const diagnostic = new vscode.Diagnostic(range, "Corvint diagnostics were truncated at the frozen snapshot bound.", vscode.DiagnosticSeverity.Warning);
        diagnostic.source = "Corvint";
        diagnostic.code = `${snapshot.snapshotKey}:diagnostics-truncated`;
        first.diagnostics.push(diagnostic);
      }
    }
    return byUri;
  }

  applyDecorations(): void {
    for (const editor of vscode.window.visibleTextEditors) {
      const ranges = this.currentRanges.get(editor.document.uri.toString());
      editor.setDecorations(this.evidenceDecoration, ranges?.evidence ?? []);
      editor.setDecorations(this.candidateDecoration, ranges?.candidate ?? []);
      editor.setDecorations(this.unknownDecoration, ranges?.unknown ?? []);
    }
  }

  dispose(): void {
    this.clear("Corvint extension deactivated");
    this.evidence.dispose();
    this.impact.dispose();
    this.why.dispose();
    this.diagnostics.dispose();
    this.evidenceDecoration.dispose();
    this.candidateDecoration.dispose();
    this.unknownDecoration.dispose();
    this.status.dispose();
  }
}

function diagnosticSeverity(resultState: string | undefined): vscode.DiagnosticSeverity | undefined {
  switch (diagnosticClass(resultState)) {
    case "error":
      return vscode.DiagnosticSeverity.Error;
    case "warning":
      return vscode.DiagnosticSeverity.Warning;
    case "information":
      return vscode.DiagnosticSeverity.Information;
    case "none":
      return undefined;
  }
}

function decorationClass(resultState: string | undefined): "evidence" | "candidate" | "unknown" {
  const kind = projectionClass(resultState);
  return kind === "unknown" ? "unknown" : kind === "candidate" || kind === "informational" ? "candidate" : "evidence";
}

function resultNode(prefix: string, result: ResultItem): TreeNode {
  return {
    id: `${prefix}:result:${stableResultId(result)}`,
    label: display(result.id, 200),
    description: display(`${result.kind}${result.score === undefined ? "" : ` · ${result.score}`}`, 300),
    tooltip: display(result.summary || result.id, 2_000),
    icon: result.kind === "test" ? "beaker" : "references",
    children: orderedEvidence(result.evidence).map((evidence) => evidenceNode(prefix, evidence)),
  };
}

function impactSections(prefix: string, results: readonly ResultItem[], snapshot: CorvintSnapshot): TreeNode[] {
  if (results.length === 0 && snapshot.transportAuthority?.state === "ABSTAINED") {
    const reason = snapshot.transportAuthority.abstentionReason;
    return [{
      id: `${prefix}:impact:section:unknown`,
      label: "Unknowns",
      description: "1",
      tooltip: "The MCP bridge abstained without a native receipt or evidence.",
      icon: "question",
      children: [leaf(
        `${prefix}:impact:abstention:${stableProjectionId([reason])}`,
        `Unknown: ${reason}`,
        "Corvint preserved the bounded MCP abstention; no native receipt or evidence was supplied.",
        "warning",
      )],
    }];
  }
  const groups = new Map<ProjectionClass, ResultItem[]>();
  for (const result of results) {
    const kind = projectionClass(result.state);
    const group = groups.get(kind) ?? [];
    group.push(result);
    groups.set(kind, group);
  }
  const order: readonly ProjectionClass[] = ["evidence", "candidate", "unknown", "informational"];
  return order.flatMap((kind) => {
    const group = groups.get(kind);
    if (group === undefined) {
      return [];
    }
    const label = kind === "evidence" ? "Evidence" : kind === "candidate" ? "Candidates" : kind === "unknown" ? "Unknowns" : "Informational";
    return [{
      id: `${prefix}:impact:section:${kind}`,
      label,
      description: `${group.length}`,
      tooltip: `${label} as classified by the exact receipt result state.`,
      icon: kind === "unknown" ? "question" : kind === "candidate" || kind === "informational" ? "search" : "verified",
      children: group.map((result) => impactNode(prefix, result, snapshot)),
    }];
  });
}

function impactNode(prefix: string, result: ResultItem, snapshot: CorvintSnapshot): TreeNode {
  return {
    ...resultNode(prefix, result),
    id: `${prefix}:impact:${stableResultId(result)}`,
    description: display(`State: ${result.state ?? "not supplied"} · Kind: ${result.kind} · Receipt: ${snapshot.state}`, 300),
  };
}

function evidenceNode(prefix: string, evidence: EvidenceItem): TreeNode {
  return leaf(
    `${prefix}:evidence:${stableEvidenceId(evidence)}`,
    `${display(evidence.path, 170)}:${evidence.line}`,
    `${evidence.reason}\nAuthority: ${evidence.authority}\nConfidence: ${evidence.confidence}\nBlob: ${evidence.blobHash}`,
    "file-code",
    `${evidence.authority} · ${evidence.confidence}`,
  );
}

function whyNode(prefix: string, evidence: EvidenceItem): TreeNode {
  return leaf(
    `${prefix}:why:${stableEvidenceId(evidence)}`,
    display(evidence.reason, 200),
    `Source: ${evidence.path}:${evidence.line}\nAuthority: ${evidence.authority}\nConfidence: ${evidence.confidence}`,
    "question",
    `${evidence.authority} · ${evidence.confidence}`,
  );
}

function transportAuthorityNode(prefix: string, snapshot: CorvintSnapshot): TreeNode {
  const authority = snapshot.transportAuthority;
  if (authority === undefined) {
    throw new Error("transport authority is unavailable");
  }
  const repository = authority.repository;
  const repositoryText = repository === undefined
    ? "Repository identity: not observed"
    : `Commit: ${repository.commitRevision}\nTree: ${repository.treeRevision}\nObject format: ${repository.objectFormat}\nProfile: ${repository.profileId}\nWorktree: ${repository.worktreeState}\nDirty paths: ${repository.dirtyPathCount}\nDirty paths digest: ${repository.dirtyPathsSha256}`;
  return leaf(
    `${prefix}:transport-authority`,
    `MCP authority: ${authority.state}`,
    `Bridge: ${authority.profile}\nTool: ${authority.tool}\nProtocol: ${authority.protocol}\nServer version: ${authority.serverVersion}\nExecutable SHA-256: ${authority.executableSha256}\nEpistemic class: ${authority.epistemicClass}\nAuthority class: ${authority.authorityClass}\nAbstention reason: ${authority.abstentionReason}\n${repositoryText}`,
    authority.state === "ABSTAINED" ? "warning" : "verified",
    `${authority.epistemicClass} · ${authority.authorityClass}`,
  );
}

function leaf(id: string, label: string, tooltip: string, icon?: string, description?: string): TreeNode {
  return { id, label: display(label, 200), tooltip: display(tooltip, 2_000), ...(icon === undefined ? {} : { icon }),
    ...(description === undefined ? {} : { description: display(description, 300) }), children: [] };
}

function cap(nodes: readonly TreeNode[]): readonly TreeNode[] {
  if (nodes.length <= 2_000) {
    return nodes;
  }
  return [...nodes.slice(0, 1_999), leaf("corvint:truncated", "Truncated", "Corvint view exceeded the 2,000-node cap.", "warning")];
}

function orderedStrings(values: readonly string[]): string[] {
  return [...new Set(values)].sort((left, right) => Buffer.compare(Buffer.from(left, "utf8"), Buffer.from(right, "utf8")));
}

async function containedFile(root: string, relativePath: string, isCurrent: () => boolean): Promise<vscode.Uri | undefined> {
  const candidate = path.resolve(root, ...relativePath.split("/"));
  const relation = path.relative(root, candidate);
  if (relation.startsWith("..") || path.isAbsolute(relation)) {
    return undefined;
  }
  try {
    const resolved = await realpath(candidate);
    if (!isCurrent()) {
      return undefined;
    }
    const resolvedRelation = path.relative(root, resolved);
    if (resolvedRelation.startsWith("..") || path.isAbsolute(resolvedRelation)) {
      return undefined;
    }
    const identity = await stat(resolved);
    if (!isCurrent() || !identity.isFile()) {
      return undefined;
    }
    return vscode.Uri.file(resolved);
  } catch {
    return undefined;
  }
}
