const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    cache: "no-store",
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `Request failed: ${res.status}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

export interface ProjectSettings {
  maxFileSizeKB?: number;
  rootPath?: string;
}

export interface Project {
  id: string;
  name: string;
  description: string;
  settings: ProjectSettings;
  createdAt: string;
  updatedAt: string;
}

export interface ProjectSource {
  id: string;
  projectId: string;
  path: string;
  sourceType: string;
  isCode: boolean;
  alias: string;
  lastIndexedCommit: string | null;
  lastIndexedBranch: string | null;
  lastIndexedAt: string | null;
  addedAt: string;
}

export interface ScanResult {
  path: string;
  name: string;
  sourceType: string;
  hasPackageJson: boolean;
}

export interface IndexStatus {
  status: string;
  lastIndexedAt: string | null;
  nodeCount: number;
  edgeCount: number;
}

export interface ChatResponse {
  message: string;
  sources: Array<{ nodeId: string; filePath: string; qualifiedName: string }>;
}

export interface CrawlFile {
  absPath: string;
  relPath: string;
  extension: string;
  sizeBytes: number;
}

export interface CrawlResponse {
  files: CrawlFile[];
  stats: {
    total: number;
    skipped: number;
    byExtension: Record<string, number>;
  };
}

export interface ParseNode {
  name: string;
  qualifiedName: string;
  kind: string;
  signature: string;
  startLine: number;
  endLine: number;
  sourceCode: string;
  docstring: string;
  bodyHash: string;
}

export interface ParseEdge {
  source: string;
  target: string;
  kind: string;
}

export interface ParseResponse {
  nodes: ParseNode[];
  edges: ParseEdge[];
  stats: {
    nodeCount: number;
    edgeCount: number;
    byKind: Record<string, number>;
  };
}

export interface EmbedResponse {
  vector: number[];
  dimensions: number;
  tokenCount: number;
  model: string;
  truncated: boolean;
}

export interface CompareResponse {
  similarity: number;
  tokenCount1: number;
  tokenCount2: number;
  dimensions: number;
}

export interface WorkspacePackage {
  name: string;
  path: string;
  version: string;
}

export interface WorkspaceResponse {
  workspaceType: string;
  packageManager: string;
  packages: WorkspacePackage[];
  aliasMap: Record<string, string>;
  tsconfigPaths: Record<string, string>;
}

export interface ReadFileResponse {
  content: string;
  language: string;
  lineCount: number;
}

export interface ChangesResponse {
  isGitRepo: boolean;
  currentCommit: string;
  lastIndexedCommit: string;
  isFullIndex: boolean;
  addedFiles: string[];
  modifiedFiles: string[];
  deletedFiles: string[];
  thresholdExceeded: boolean;
}

export const api = {
  projects: {
    list: () => request<Project[]>("/projects"),
    get: (id: string) => request<Project>(`/projects/${id}`),
    create: (name: string, description: string) =>
      request<Project>("/projects", {
        method: "POST",
        body: JSON.stringify({ name, description }),
      }),
    update: (id: string, name: string, description: string) =>
      request<Project>(`/projects/${id}`, {
        method: "PUT",
        body: JSON.stringify({ name, description }),
      }),
    delete: (id: string) =>
      request<void>(`/projects/${id}`, { method: "DELETE" }),
    updateSettings: (id: string, settings: ProjectSettings) =>
      request<Project>(`/projects/${id}/settings`, {
        method: "PATCH",
        body: JSON.stringify({ settings }),
      }),
  },

  sources: {
    list: (projectId: string) =>
      request<ProjectSource[]>(`/projects/${projectId}/sources`),
    add: (
      projectId: string,
      path: string,
      sourceType: string,
      isCode: boolean,
      alias: string,
    ) =>
      request<ProjectSource>(`/projects/${projectId}/sources`, {
        method: "POST",
        body: JSON.stringify({ path, sourceType, isCode, alias }),
      }),
    remove: (projectId: string, sourceId: string) =>
      request<void>(`/projects/${projectId}/sources/${sourceId}`, {
        method: "DELETE",
      }),
  },

  scan: (path: string) =>
    request<ScanResult[]>("/scan", {
      method: "POST",
      body: JSON.stringify({ path }),
    }),

  indexing: {
    trigger: (projectId: string) =>
      request<{ status: string; jobId: string }>(
        `/projects/${projectId}/index`,
        { method: "POST" },
      ),
    status: (projectId: string) =>
      request<IndexStatus>(`/projects/${projectId}/index/status`),
  },

  chat: {
    send: (projectId: string, message: string) =>
      request<ChatResponse>(`/projects/${projectId}/chat`, {
        method: "POST",
        body: JSON.stringify({ message }),
      }),
    history: (projectId: string) =>
      request<Array<{ role: string; content: string }>>(
        `/projects/${projectId}/chat/history`,
      ),
  },

  debug: {
    crawl: (path: string, maxFileSizeKB?: number) =>
      request<CrawlResponse>("/debug/crawl", {
        method: "POST",
        body: JSON.stringify({ path, maxFileSizeKB }),
      }),
    parse: (filePath: string) =>
      request<ParseResponse>("/debug/parse", {
        method: "POST",
        body: JSON.stringify({ filePath }),
      }),
    embedText: (text: string) =>
      request<EmbedResponse>("/debug/embed-text", {
        method: "POST",
        body: JSON.stringify({ text }),
      }),
    compare: (text1: string, text2: string) =>
      request<CompareResponse>("/debug/compare", {
        method: "POST",
        body: JSON.stringify({ text1, text2 }),
      }),
    workspace: (path: string) =>
      request<WorkspaceResponse>("/debug/workspace", {
        method: "POST",
        body: JSON.stringify({ path }),
      }),
    changes: (path: string) =>
      request<ChangesResponse>("/debug/changes", {
        method: "POST",
        body: JSON.stringify({ path }),
      }),
    readFile: (filePath: string) =>
      request<ReadFileResponse>("/debug/read-file", {
        method: "POST",
        body: JSON.stringify({ filePath }),
      }),
  },
};
