"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { useRouter } from "next/navigation";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import {
  api,
  type Project,
  type ProjectSource,
  type ScanResult,
  type IndexStatus,
} from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { DebugTab } from "@/components/debug/debug-tab";
import {
  CodeViewer,
  type CodeViewerFile,
} from "@/components/debug/code-viewer";
import { SettingsPanel } from "@/components/settings-panel";
import { ConfirmationDialog } from "@/components/ui/confirm-dialog";
import { CopyIcon } from "lucide-react";
import { toast } from "sonner";

function IndexedAt({ date }: { date: string }) {
  const formatted = new Date(date).toLocaleString();
  return <span suppressHydrationWarning>indexed {formatted}</span>;
}

export function ProjectDetail({
  id,
  initialProject,
  initialSources,
  initialIndexStatus,
}: {
  id: string;
  initialProject: Project;
  initialSources: ProjectSource[];
  initialIndexStatus: IndexStatus | null;
}) {
  const router = useRouter();
  const [project, setProject] = useState<Project>(initialProject);
  const [sources, setSources] = useState<ProjectSource[]>(initialSources);
  const [indexStatus, setIndexStatus] = useState<IndexStatus | null>(
    initialIndexStatus,
  );
  const [tab, setTab] = useState<"sources" | "chat" | "debug" | "graph">(
    "sources",
  );

  const [scanOpen, setScanOpen] = useState(false);
  const [scanPath, setScanPath] = useState(
    initialProject.settings?.rootPath || "~/Desktop/Code",
  );
  const [scanResults, setScanResults] = useState<ScanResult[]>([]);
  const [scanning, setScanning] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());

  const [messages, setMessages] = useState<
    Array<{
      role: "user" | "assistant";
      content: string;
      sources?: Array<{ nodeId: string; filePath: string; qualifiedName: string }>;
    }>
  >([]);
  const [chatInput, setChatInput] = useState("");
  const [sending, setSending] = useState(false);
  const chatScrollRef = useRef<HTMLDivElement>(null);
  const abortRef = useRef<AbortController | null>(null);
  const messagesRef = useRef(messages);
  messagesRef.current = messages;
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [linking, setLinking] = useState(false);
  const [removingSource, setRemovingSource] = useState<string | null>(null);
  const [viewerFile, setViewerFile] = useState<CodeViewerFile | null>(null);

  const load = async () => {
    try {
      const [p, s, idx] = await Promise.all([
        api.projects.get(id),
        api.sources.list(id),
        api.indexing.status(id),
      ]);
      setProject(p);
      setSources(s);
      setIndexStatus(idx);
      if (p.settings?.rootPath) {
        setScanPath(p.settings.rootPath);
      }
    } catch {
      // handle error
    }
  };

  const handleScan = async () => {
    setScanning(true);
    try {
      const home =
        process.env.NEXT_PUBLIC_HOME_DIR || "/Users/maximilianwidjaya";
      const expandedPath = scanPath.replace(/^~/, home);
      const results = await api.scan(expandedPath);
      setScanResults(results);
      setSelected(new Set());
    } catch (e) {
      alert(e instanceof Error ? e.message : "Scan failed");
    } finally {
      setScanning(false);
    }
  };

  const toggleSelect = (path: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });
  };

  const handleLink = async () => {
    setLinking(true);
    try {
      await Promise.all(
        scanResults
          .filter((r) => selected.has(r.path))
          .map((r) =>
            api.sources
              .add(id, r.path, r.sourceType, true, r.name)
              .catch(() => {}),
          ),
      );
      setScanOpen(false);
      setScanResults([]);
      setSelected(new Set());
      load();
    } finally {
      setLinking(false);
    }
  };

  const handleRemoveSource = async (sourceId: string) => {
    setRemovingSource(sourceId);
    try {
      await api.sources.remove(id, sourceId);
      load();
    } finally {
      setRemovingSource(null);
    }
  };

  const [indexing, setIndexing] = useState(
    initialIndexStatus?.status === "running",
  );
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const loadRef = useRef(load);
  loadRef.current = load;

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, []);

  const startPolling = useCallback(() => {
    stopPolling();
    pollRef.current = setInterval(async () => {
      try {
        const idx = await api.indexing.status(id);
        setIndexStatus(idx);
        if (idx.status !== "running") {
          stopPolling();
          setIndexing(false);
          loadRef.current();
        }
      } catch {
        // keep polling on transient errors
      }
    }, 1500);
  }, [id, stopPolling]);

  useEffect(() => {
    if (initialIndexStatus?.status === "running") {
      startPolling();
    }
    return stopPolling;
  }, [initialIndexStatus?.status, startPolling, stopPolling]);

  const handleIndex = async (force?: boolean) => {
    try {
      await api.indexing.trigger(id, force);
      setIndexing(true);
      const idx = await api.indexing.status(id);
      setIndexStatus(idx);
      startPolling();
    } catch (e) {
      if (e instanceof Error && e.message.includes("already in progress")) {
        setIndexing(true);
        startPolling();
      }
    }
  };

  const scrollToBottom = () => {
    chatScrollRef.current?.scrollTo({
      top: chatScrollRef.current.scrollHeight,
    });
  };

  const handleChat = () => {
    if (!chatInput.trim() || sending) return;
    const msg = chatInput;
    setChatInput("");
    setSending(true);

    setMessages((prev) => [
      ...prev,
      { role: "user", content: msg },
      { role: "assistant", content: "" },
    ]);
    setTimeout(scrollToBottom, 0);

    const assistantIdx = { current: -1 };

    const history = messagesRef.current
      .filter((m) => m.content)
      .map((m) => ({
        role: m.role,
        content: m.content,
      }));

    const controller = api.chat.stream(
      id,
      msg,
      history,
      (delta) => {
        setMessages((prev) => {
          const next = [...prev];
          if (assistantIdx.current === -1) {
            assistantIdx.current = next.length - 1;
          }
          const idx = assistantIdx.current;
          next[idx] = { ...next[idx], content: next[idx].content + delta };
          return next;
        });
        scrollToBottom();
      },
      (sources) => {
        setMessages((prev) => {
          const next = [...prev];
          const idx = assistantIdx.current >= 0 ? assistantIdx.current : next.length - 1;
          next[idx] = { ...next[idx], sources: sources.length ? sources : undefined };
          return next;
        });
        setSending(false);
        abortRef.current = null;
      },
      (error) => {
        setMessages((prev) => {
          const next = [...prev];
          const idx = assistantIdx.current >= 0 ? assistantIdx.current : next.length - 1;
          const existing = next[idx].content;
          next[idx] = {
            ...next[idx],
            content: existing || `error: ${error}`,
          };
          return next;
        });
        setSending(false);
        abortRef.current = null;
      },
    );
    abortRef.current = controller;
  };

  const handleDelete = async () => {
    setDeleting(true);
    try {
      await api.projects.delete(id);
      router.push("/");
    } finally {
      setDeleting(false);
    }
  };

  return (
    <div
      className={
        viewerFile
          ? "flex h-screen overflow-hidden"
          : "max-w-5xl mx-auto px-6 py-10"
      }
    >
      <div
        className={
          viewerFile ? "flex-1 min-w-0 overflow-y-auto px-6 py-10" : "contents"
        }
      >
        <div className="flex items-center justify-between mb-6">
          <div>
            <button
              onClick={() => router.push("/")}
              className="text-xs text-muted-foreground hover:text-foreground mb-1 block"
              title="Back to colonies"
            >
              &larr; colonies
            </button>
            <h1 className="text-lg font-medium">{project.name}</h1>
            {project.description && (
              <p className="text-sm text-muted-foreground">
                {project.description}
              </p>
            )}
          </div>
          <div className="flex gap-2">
            <SettingsPanel
              projectId={id}
              settings={project.settings ?? {}}
              onSave={load}
              onReindex={() => handleIndex(true)}
              open={settingsOpen}
              onOpenChange={setSettingsOpen}
            />
            <Button
              variant="secondary"
              size="sm"
              onClick={() => setDeleteOpen(true)}
              title="Delete colony"
            >
              delete
            </Button>
          </div>
        </div>

        <ConfirmationDialog
          open={deleteOpen}
          onOpenChange={setDeleteOpen}
          title="delete colony"
          body="this will permanently delete this colony and all its linked substrates. this cannot be undone."
          cancel="cancel"
          yes="delete"
          loading={deleting}
          onCancel={() => setDeleteOpen(false)}
          onAccept={handleDelete}
        />

        {indexStatus && (
          <div className="mb-6 space-y-1">
            <div className="flex gap-4 text-xs text-muted-foreground">
              <span>{indexStatus.nodeCount} nodes</span>
              <span>{indexStatus.edgeCount} edges</span>
              {indexStatus.lastIndexedAt && (
                <IndexedAt date={indexStatus.lastIndexedAt} />
              )}
            </div>
            {indexStatus.status === "running" && (
              <div className="text-xs text-muted-foreground">
                <span className="text-foreground">
                  {indexStatus.stage || "starting"}
                </span>
                {indexStatus.progress && (
                  <>
                    {" \u2014 "}
                    {indexStatus.progress}
                  </>
                )}
              </div>
            )}
            {indexStatus.status === "failed" && indexStatus.error && (
              <div className="text-xs text-destructive">
                failed: {indexStatus.error}
              </div>
            )}
            {indexStatus.status === "completed" && indexStatus.result && (
              <div className="text-xs text-muted-foreground">
                indexed {indexStatus.result.sourcesProcessed} source
                {indexStatus.result.sourcesProcessed !== 1 ? "s" : ""},{" "}
                {indexStatus.result.totalEmbedded} embedded in{" "}
                {(indexStatus.result.duration / 1e9).toFixed(1)}s
              </div>
            )}
          </div>
        )}

        <div className="flex gap-4 border-b border-border mb-6">
          <button
            className={`pb-2 text-sm ${tab === "sources" ? "border-b border-foreground text-foreground" : "text-muted-foreground"}`}
            onClick={() => setTab("sources")}
            title="Substrates"
          >
            substrates
          </button>
          <button
            className={`pb-2 text-sm ${tab === "chat" ? "border-b border-foreground text-foreground" : "text-muted-foreground"}`}
            onClick={() => setTab("chat")}
            title="Forage"
          >
            forage
          </button>
          <button
            className={`pb-2 text-sm ${tab === "debug" ? "border-b border-foreground text-foreground" : "text-muted-foreground"}`}
            onClick={() => setTab("debug")}
            title="Spore lab"
          >
            spore lab
          </button>
          <button
            className={`pb-2 text-sm ${tab === "graph" ? "border-b border-foreground text-foreground" : "text-muted-foreground"}`}
            onClick={() => setTab("graph")}
            title="Mycelial map"
          >
            mycelial map
          </button>
        </div>

        {tab === "sources" && (
          <div>
            <div className="flex items-center justify-between mb-4">
              <span className="text-sm text-muted-foreground">
                {sources.length} linked substrate
                {sources.length !== 1 ? "s" : ""}
              </span>
              <div className="flex gap-2">
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => handleIndex()}
                  disabled={indexing}
                  title="Decompose"
                >
                  {indexing ? "decomposing..." : "decompose"}
                </Button>
                <Dialog open={scanOpen} onOpenChange={setScanOpen}>
                  <DialogTrigger asChild>
                    <Button
                      variant="secondary"
                      size="sm"
                      title="Feed substrate"
                    >
                      + feed
                    </Button>
                  </DialogTrigger>
                  <DialogContent className="max-w-lg">
                    <DialogHeader>
                      <DialogTitle>feed substrate</DialogTitle>
                    </DialogHeader>
                    <div className="space-y-4 pt-2">
                      <div className="flex gap-2">
                        <Input
                          placeholder="directory path"
                          value={scanPath}
                          onChange={(e) => setScanPath(e.target.value)}
                          onKeyDown={(e) => e.key === "Enter" && handleScan()}
                        />
                        <Button
                          variant="secondary"
                          onClick={handleScan}
                          disabled={scanning}
                          title="Scan directory"
                        >
                          {scanning ? "scanning..." : "scan"}
                        </Button>
                      </div>
                      {scanResults.length > 0 && (
                        <div className="border border-border max-h-64 overflow-y-auto">
                          {scanResults.filter((r) => !sources.some((s) => s.path === r.path)).map((r) => (
                            <label
                              key={r.path}
                              className="flex items-center gap-3 px-3 py-2 hover:bg-accent/50 cursor-pointer text-sm"
                            >
                              <input
                                type="checkbox"
                                checked={selected.has(r.path)}
                                onChange={() => toggleSelect(r.path)}
                                className="accent-primary"
                              />
                              <span className="flex-1 truncate">{r.name}</span>
                              <Badge variant="outline" className="text-xs">
                                {r.sourceType}
                              </Badge>
                            </label>
                          ))}
                        </div>
                      )}
                      {selected.size > 0 && (
                        <Button
                          onClick={handleLink}
                          disabled={linking}
                          className="w-full"
                          title="Link substrates"
                        >
                          {linking
                            ? "linking..."
                            : `link ${selected.size} substrate${selected.size !== 1 ? "s" : ""}`}
                        </Button>
                      )}
                      <button
                        onClick={() => {
                          setScanOpen(false);
                          setSettingsOpen(true);
                        }}
                        className="text-xs text-muted-foreground hover:text-foreground"
                        title="Change source directory"
                      >
                        change source directory
                      </button>
                    </div>
                  </DialogContent>
                </Dialog>
              </div>
            </div>

            {sources.length === 0 ? (
              <div className="border border-dashed border-border py-12 text-center">
                <p className="text-sm text-muted-foreground">
                  no substrates linked
                </p>
                <p className="text-xs text-muted-foreground mt-1">
                  feed this colony with local repos or directories
                </p>
              </div>
            ) : (
              <div className="space-y-1">
                {sources.map((s) => (
                  <div
                    key={s.id}
                    className="flex items-center justify-between px-3 py-2 border border-border"
                  >
                    <div className="flex items-center gap-3 min-w-0">
                      {s.lastIndexedAt ? (
                        <span className="text-green-500 text-sm shrink-0" title={`indexed ${new Date(s.lastIndexedAt).toLocaleString()}`}>&#10003;</span>
                      ) : (
                        <span className="text-muted-foreground text-sm shrink-0" title="not indexed">&#x2013;</span>
                      )}
                      <span className="text-sm truncate">
                        {s.alias || s.path}
                      </span>
                      <Badge variant="outline" className="text-xs shrink-0">
                        {s.sourceType}
                      </Badge>
                      <Badge
                        variant={s.isCode ? "default" : "secondary"}
                        className="text-xs shrink-0"
                      >
                        {s.isCode ? "code" : "non-code"}
                      </Badge>
                    </div>
                    <button
                      onClick={() => handleRemoveSource(s.id)}
                      disabled={removingSource === s.id}
                      className="text-xs text-muted-foreground hover:text-destructive ml-2 disabled:opacity-50"
                      title="Remove substrate"
                    >
                      {removingSource === s.id ? "removing..." : "remove"}
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        {tab === "chat" && (
          <div className="flex flex-col h-[60vh]">
            <div
              ref={chatScrollRef}
              className="flex-1 overflow-y-auto space-y-4 mb-4"
            >
              {messages.length === 0 && (
                <p className="text-sm text-muted-foreground text-center py-16">
                  ask questions about your indexed code
                </p>
              )}
              {messages.map((m, i) => (
                <div
                  key={i}
                  className={`chat-message text-sm px-3 py-2 ${
                    m.role === "user"
                      ? "bg-accent/50 ml-12"
                      : "bg-card border border-border mr-12"
                  }`}
                >
                  <span className="text-xs text-muted-foreground block mb-1">
                    {m.role === "user" ? "you" : "mycelium"}
                  </span>
                  {m.role === "user" ? (
                    <div className="whitespace-pre-wrap">{m.content}</div>
                  ) : m.content ? (
                    <div className="prose prose-sm prose-invert max-w-none prose-pre:bg-accent/50 prose-pre:border prose-pre:border-border prose-code:text-foreground prose-code:before:content-none prose-code:after:content-none prose-headings:text-foreground prose-headings:font-medium prose-p:text-foreground prose-li:text-foreground prose-strong:text-foreground">
                      <ReactMarkdown remarkPlugins={[remarkGfm]}>
                        {m.content}
                      </ReactMarkdown>
                    </div>
                  ) : sending ? (
                    <span className="text-muted-foreground">thinking...</span>
                  ) : null}
                  {m.sources && m.sources.length > 0 && (
                    <div className="mt-2 pt-2 border-t border-border">
                      <span className="text-xs text-muted-foreground block mb-1">
                        sources
                      </span>
                      <div className="flex flex-wrap gap-1">
                        {m.sources.map((s, j) => (
                          <span
                            key={j}
                            className="text-xs bg-accent/50 px-2 py-0.5 font-mono truncate max-w-[300px]"
                            title={`${s.filePath} — ${s.qualifiedName}`}
                          >
                            {s.qualifiedName}
                          </span>
                        ))}
                      </div>
                    </div>
                  )}
                  {m.role === "assistant" && m.content && (
                    <button
                      onClick={() => {
                        navigator.clipboard.writeText(m.content);
                        toast("copied to clipboard");
                      }}
                      className="mt-2 text-muted-foreground hover:text-foreground"
                      title="Copy response"
                    >
                      <CopyIcon className="size-3.5" />
                    </button>
                  )}
                </div>
              ))}
            </div>
            <div className="flex gap-2">
              <Textarea
                placeholder="ask about your codebase..."
                value={chatInput}
                onChange={(e) => setChatInput(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && !e.shiftKey) {
                    e.preventDefault();
                    handleChat();
                  }
                }}
                rows={2}
                className="flex-1 resize-none"
              />
              <Button
                onClick={handleChat}
                disabled={!chatInput.trim() || sending}
                className="h-full"
                title="Send message"
              >
                send
              </Button>
            </div>
          </div>
        )}

        {tab === "debug" && (
          <DebugTab
            key={project.settings?.rootPath ?? ""}
            rootPath={project.settings?.rootPath}
            maxFileSizeKB={project.settings?.maxFileSizeKB}
            onOpenFile={(absPath, scrollToLine, highlightEndLine) =>
              setViewerFile({
                filePath: absPath,
                scrollToLine,
                highlightEndLine,
              })
            }
          />
        )}

        {tab === "graph" && (
          <div className="flex flex-col items-center justify-center py-24 text-center">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src="/icon.svg"
              alt=""
              width={64}
              height={64}
              className="mb-6"
            />
            <p className="text-sm text-muted-foreground">coming soon</p>
          </div>
        )}
      </div>

      {viewerFile && (
        <div className="w-[45vw] shrink-0 border-l border-border bg-background">
          <CodeViewer file={viewerFile} onClose={() => setViewerFile(null)} />
        </div>
      )}
    </div>
  );
}
