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

export interface Project {
  id: string;
  name: string;
  description: string;
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
};
