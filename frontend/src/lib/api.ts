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

export interface IndexResult {
  sourcesProcessed: number;
  sourcesSkipped: number;
  totalNodes: number;
  totalEdges: number;
  totalEmbedded: number;
  totalDeleted: number;
  duration: number;
  errors?: string[];
}

export interface IndexStatus {
  status: string;
  lastIndexedAt: string | null;
  nodeCount: number;
  edgeCount: number;
  jobId?: string;
  stage?: string;
  progress?: string;
  startedAt?: string;
  doneAt?: string | null;
  result?: IndexResult;
  error?: string;
}

export interface SemanticSearchResult {
  nodeId: string;
  qualifiedName: string;
  filePath: string;
  kind: string;
  similarity: number;
  signature: string;
  sourceCode?: string;
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

  search: {
    semantic: (
      query: string,
      projectId: string,
      limit?: number,
      kinds?: string[],
    ) =>
      request<SemanticSearchResult[]>("/search/semantic", {
        method: "POST",
        body: JSON.stringify({ query, projectId, limit, kinds }),
      }),
  },

  chat: {
    stream: (
      projectId: string,
      message: string,
      history: Array<{ role: string; content: string }>,
      onDelta: (delta: string) => void,
      onDone: (sources: ChatResponse["sources"]) => void,
      onError: (error: string) => void,
    ) => {
      const controller = new AbortController();
      (async () => {
        try {
          const res = await fetch(`${API_BASE}/projects/${projectId}/chat`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ message, history }),
            signal: controller.signal,
          });
          if (!res.ok) {
            const body = await res.json().catch(() => ({}));
            onError(body.error || `Request failed: ${res.status}`);
            return;
          }
          const reader = res.body?.getReader();
          if (!reader) {
            onError("No response body");
            return;
          }
          const decoder = new TextDecoder();
          let buffer = "";
          while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            buffer += decoder.decode(value, { stream: true });
            const lines = buffer.split("\n");
            buffer = lines.pop() || "";
            for (const line of lines) {
              if (!line.startsWith("data: ")) continue;
              const json_str = line.slice(6);
              try {
                const event = JSON.parse(json_str);
                if (event.error) {
                  onError(event.error);
                  return;
                }
                if (event.delta) {
                  onDelta(event.delta);
                } else if (event.done) {
                  onDone(event.sources || []);
                }
              } catch {
                // skip malformed JSON lines
              }
            }
          }
        } catch (err) {
          if ((err as Error).name !== "AbortError") {
            onError((err as Error).message || "Stream failed");
          }
        }
      })();
      return controller;
    },
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
