"use client";

import { useEffect, useState, use } from "react";
import { useRouter } from "next/navigation";
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

export default function ProjectPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const router = useRouter();
  const [project, setProject] = useState<Project | null>(null);
  const [sources, setSources] = useState<ProjectSource[]>([]);
  const [indexStatus, setIndexStatus] = useState<IndexStatus | null>(null);
  const [tab, setTab] = useState<"sources" | "chat">("sources");

  // Scan state
  const [scanOpen, setScanOpen] = useState(false);
  const [scanPath, setScanPath] = useState("~/Desktop/Code");
  const [scanResults, setScanResults] = useState<ScanResult[]>([]);
  const [scanning, setScanning] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());

  // Chat state
  const [messages, setMessages] = useState<
    Array<{ role: "user" | "assistant"; content: string }>
  >([]);
  const [chatInput, setChatInput] = useState("");
  const [sending, setSending] = useState(false);

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
    } catch {
      // handle error
    }
  };

  useEffect(() => {
    load();
  }, [id]);

  const handleScan = async () => {
    setScanning(true);
    try {
      // Expand ~ to home directory
      const expandedPath = scanPath.replace(/^~/, "/Users/maximilianwidjaya");
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
  };

  const handleRemoveSource = async (sourceId: string) => {
    await api.sources.remove(id, sourceId);
    load();
  };

  const handleIndex = async () => {
    await api.indexing.trigger(id);
    load();
  };

  const handleChat = async () => {
    if (!chatInput.trim()) return;
    const msg = chatInput;
    setChatInput("");
    setMessages((prev) => [...prev, { role: "user", content: msg }]);
    setSending(true);
    try {
      const res = await api.chat.send(id, msg);
      setMessages((prev) => [
        ...prev,
        { role: "assistant", content: res.message },
      ]);
    } catch {
      setMessages((prev) => [
        ...prev,
        { role: "assistant", content: "error: failed to get response" },
      ]);
    } finally {
      setSending(false);
    }
  };

  const handleDelete = async () => {
    if (!confirm("delete this colony? this cannot be undone.")) return;
    await api.projects.delete(id);
    router.push("/");
  };

  if (!project) {
    return (
      <div className="max-w-3xl mx-auto px-6 py-10">
        <p className="text-sm text-muted-foreground">loading...</p>
      </div>
    );
  }

  return (
    <div className="max-w-3xl mx-auto px-6 py-10">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <button
            onClick={() => router.push("/")}
            className="text-xs text-muted-foreground hover:text-foreground mb-1 block"
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
        <Button variant="secondary" size="sm" onClick={handleDelete}>
          delete
        </Button>
      </div>

      {/* Stats */}
      {indexStatus && (
        <div className="flex gap-4 mb-6 text-xs text-muted-foreground">
          <span>{indexStatus.nodeCount} nodes</span>
          <span>{indexStatus.edgeCount} edges</span>
          <span>status: {indexStatus.status}</span>
        </div>
      )}

      {/* Tabs */}
      <div className="flex gap-4 border-b border-border mb-6">
        <button
          className={`pb-2 text-sm ${tab === "sources" ? "border-b border-foreground text-foreground" : "text-muted-foreground"}`}
          onClick={() => setTab("sources")}
        >
          substrates
        </button>
        <button
          className={`pb-2 text-sm ${tab === "chat" ? "border-b border-foreground text-foreground" : "text-muted-foreground"}`}
          onClick={() => setTab("chat")}
        >
          forage
        </button>
      </div>

      {/* Sources tab */}
      {tab === "sources" && (
        <div>
          <div className="flex items-center justify-between mb-4">
            <span className="text-sm text-muted-foreground">
              {sources.length} linked substrate{sources.length !== 1 ? "s" : ""}
            </span>
            <div className="flex gap-2">
              <Button variant="secondary" size="sm" onClick={handleIndex}>
                decompose
              </Button>
              <Dialog open={scanOpen} onOpenChange={setScanOpen}>
                <DialogTrigger asChild>
                  <Button variant="secondary" size="sm">
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
                      >
                        {scanning ? "scanning..." : "scan"}
                      </Button>
                    </div>
                    {scanResults.length > 0 && (
                      <div className="border border-border max-h-64 overflow-y-auto">
                        {scanResults.map((r) => (
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
                      <Button onClick={handleLink} className="w-full">
                        link {selected.size} substrate
                        {selected.size !== 1 ? "s" : ""}
                      </Button>
                    )}
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
                    className="text-xs text-muted-foreground hover:text-destructive ml-2"
                  >
                    remove
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Chat tab */}
      {tab === "chat" && (
        <div className="flex flex-col h-[60vh]">
          <div className="flex-1 overflow-y-auto space-y-4 mb-4">
            {messages.length === 0 && (
              <p className="text-sm text-muted-foreground text-center py-16">
                ask questions about your indexed code
              </p>
            )}
            {messages.map((m, i) => (
              <div
                key={i}
                className={`text-sm px-3 py-2 ${
                  m.role === "user"
                    ? "bg-accent/50 ml-12"
                    : "bg-card border border-border mr-12"
                }`}
              >
                <span className="text-xs text-muted-foreground block mb-1">
                  {m.role === "user" ? "you" : "mycelium"}
                </span>
                {m.content}
              </div>
            ))}
            {sending && (
              <div className="text-sm text-muted-foreground px-3 py-2">
                thinking...
              </div>
            )}
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
              className="self-end"
            >
              send
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
