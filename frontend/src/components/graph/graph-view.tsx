"use client";

import { useState, useEffect, useMemo, useCallback } from "react";
import { api, type GraphVizData } from "@/lib/api";
import { GraphCanvas } from "./graph-canvas";
import { GraphControls } from "./graph-controls";
import { GraphNodePanel } from "./graph-node-panel";

export function GraphView({ projectId }: { projectId: string }) {
  const [data, setData] = useState<GraphVizData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [hiddenKinds, setHiddenKinds] = useState<Set<string>>(new Set());

  useEffect(() => {
    api.graph
      .data(projectId)
      .then(setData)
      .catch((e) =>
        setError(e instanceof Error ? e.message : "Failed to load graph"),
      )
      .finally(() => setLoading(false));
  }, [projectId]);

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedSearch(searchQuery), 300);
    return () => clearTimeout(timer);
  }, [searchQuery]);

  const toggleKind = useCallback((kind: string) => {
    setHiddenKinds((prev) => {
      const next = new Set(prev);
      if (next.has(kind)) next.delete(kind);
      else next.add(kind);
      return next;
    });
  }, []);

  const handleNodeClick = useCallback((nodeId: string) => {
    setSelectedNodeId(nodeId || null);
  }, []);

  const filtered = useMemo(() => {
    if (!data) return { nodes: [], edges: [] };
    if (hiddenKinds.size === 0)
      return { nodes: data.nodes, edges: data.edges };

    const nodes = data.nodes.filter((n) => !hiddenKinds.has(n.kind));
    const visibleIds = new Set(nodes.map((n) => n.id));
    const edges = data.edges.filter(
      (e) => visibleIds.has(e.source) && visibleIds.has(e.target),
    );
    return { nodes, edges };
  }, [data, hiddenKinds]);

  if (loading) {
    return (
      <div className="flex flex-col items-center justify-center py-24 text-center">
        <p className="text-sm text-muted-foreground">loading graph data...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center py-24 text-center">
        <p className="text-sm text-destructive">{error}</p>
      </div>
    );
  }

  if (!data || data.nodes.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-24 text-center">
        <p className="text-sm text-muted-foreground">
          no nodes indexed yet — run decompose first
        </p>
      </div>
    );
  }

  return (
    <div className="flex h-[calc(100vh-200px)]">
      <div className="flex-1 flex flex-col min-w-0">
        <GraphControls
          kinds={data.stats.kinds}
          hiddenKinds={hiddenKinds}
          onToggleKind={toggleKind}
          searchQuery={searchQuery}
          onSearchChange={setSearchQuery}
          nodeCount={filtered.nodes.length}
          edgeCount={filtered.edges.length}
        />
        <GraphCanvas
          nodes={filtered.nodes}
          edges={filtered.edges}
          searchQuery={debouncedSearch}
          onNodeClick={handleNodeClick}
          selectedNodeId={selectedNodeId}
        />
      </div>
      {selectedNodeId && (
        <div className="w-80 shrink-0 border-l border-border bg-background overflow-hidden">
          <GraphNodePanel
            projectId={projectId}
            nodeId={selectedNodeId}
            onClose={() => setSelectedNodeId(null)}
          />
        </div>
      )}
    </div>
  );
}
