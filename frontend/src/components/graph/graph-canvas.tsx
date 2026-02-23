"use client";

import { useRef, useCallback, useEffect, useState, useMemo } from "react";
import dynamic from "next/dynamic";
import type { GraphVizNode, GraphVizEdge } from "@/lib/api";

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const ForceGraph2D = dynamic(() => import("react-force-graph-2d"), {
  ssr: false,
  loading: () => (
    <div className="flex items-center justify-center h-full text-sm text-muted-foreground">
      loading graph...
    </div>
  ),
// eslint-disable-next-line @typescript-eslint/no-explicit-any
}) as any;

const NODE_COLORS: Record<string, string> = {
  function: "#8b5cf6",
  method: "#8b5cf6",
  class: "#f59e0b",
  struct: "#f59e0b",
  interface: "#06b6d4",
  type: "#06b6d4",
  variable: "#6b7280",
  constant: "#6b7280",
  enum: "#ec4899",
  module: "#10b981",
  package: "#10b981",
  field: "#9ca3af",
};

const EDGE_COLORS: Record<string, string> = {
  calls: "rgba(139, 92, 246, 0.15)",
  imports: "rgba(16, 185, 129, 0.15)",
  uses_type: "rgba(6, 182, 212, 0.12)",
  extends: "rgba(245, 158, 11, 0.15)",
  implements: "rgba(6, 182, 212, 0.15)",
  embeds: "rgba(107, 114, 128, 0.10)",
  depends_on: "rgba(107, 114, 128, 0.10)",
};

interface GraphNode extends GraphVizNode {
  x?: number;
  y?: number;
}

interface GraphLink extends GraphVizEdge {
  [key: string]: unknown;
}

export function GraphCanvas({
  nodes,
  edges,
  searchQuery,
  onNodeClick,
  selectedNodeId,
}: {
  nodes: GraphVizNode[];
  edges: GraphVizEdge[];
  searchQuery: string;
  onNodeClick: (nodeId: string) => void;
  selectedNodeId: string | null;
}) {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const fgRef = useRef<any>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const [dimensions, setDimensions] = useState({ width: 800, height: 600 });

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const observer = new ResizeObserver((entries) => {
      const { width, height } = entries[0].contentRect;
      setDimensions({ width, height });
    });
    observer.observe(container);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    const fg = fgRef.current;
    if (!fg) return;

    const charge = fg.d3Force("charge");
    if (charge?.strength) charge.strength(-30);
    const link = fg.d3Force("link");
    if (link?.distance) link.distance(40);
    const center = fg.d3Force("center");
    if (center?.strength) center.strength(0.05);
  }, [nodes]);

  const handleNodeClick = useCallback(
    (node: GraphNode) => {
      if (node.id) onNodeClick(String(node.id));
    },
    [onNodeClick],
  );

  const searchLower = useMemo(() => searchQuery.toLowerCase(), [searchQuery]);

  const nodeCanvasObject = useCallback(
    (node: GraphNode, ctx: CanvasRenderingContext2D, globalScale: number) => {
      const x = node.x ?? 0;
      const y = node.y ?? 0;
      const size = Math.sqrt((node.degree ?? 0) + 1) * 2;
      const isSelected = String(node.id) === selectedNodeId;
      const isSearchMatch =
        searchLower &&
        (node.qualifiedName ?? "").toLowerCase().includes(searchLower);
      const isDimmed = searchLower && !isSearchMatch;

      ctx.beginPath();
      ctx.arc(x, y, size, 0, 2 * Math.PI);

      if (isDimmed) {
        ctx.fillStyle = "rgba(100, 100, 100, 0.2)";
      } else if (isSelected) {
        ctx.fillStyle = "#ffffff";
      } else if (isSearchMatch) {
        ctx.fillStyle = "#ffffff";
      } else {
        ctx.fillStyle = NODE_COLORS[node.kind ?? ""] ?? "#6b7280";
      }
      ctx.fill();

      if (isSelected) {
        ctx.strokeStyle = "#ffffff";
        ctx.lineWidth = 1.5 / globalScale;
        ctx.stroke();
      }

      if (globalScale > 2.5 && !isDimmed) {
        const fontSize = 10 / globalScale;
        ctx.font = `${fontSize}px monospace`;
        ctx.fillStyle = "rgba(255, 255, 255, 0.7)";
        ctx.textAlign = "center";
        ctx.textBaseline = "top";
        ctx.fillText(node.name ?? "", x, y + size + 2 / globalScale);
      }
    },
    [selectedNodeId, searchLower],
  );

  const nodePointerAreaPaint = useCallback(
    (node: GraphNode, color: string, ctx: CanvasRenderingContext2D) => {
      const x = node.x ?? 0;
      const y = node.y ?? 0;
      const size = Math.sqrt((node.degree ?? 0) + 1) * 2 + 2;
      ctx.beginPath();
      ctx.arc(x, y, size, 0, 2 * Math.PI);
      ctx.fillStyle = color;
      ctx.fill();
    },
    [],
  );

  const linkColor = useCallback(
    (link: GraphLink) => {
      if (searchLower) return "rgba(60, 60, 60, 0.05)";
      return EDGE_COLORS[link.kind ?? ""] ?? "rgba(100, 100, 100, 0.08)";
    },
    [searchLower],
  );

  const graphData = useMemo(
    () => ({
      nodes: nodes as GraphNode[],
      links: edges as GraphLink[],
    }),
    [nodes, edges],
  );

  return (
    <div ref={containerRef} className="flex-1 min-h-0">
      <ForceGraph2D
        ref={fgRef}
        graphData={graphData}
        width={dimensions.width}
        height={dimensions.height}
        backgroundColor="transparent"
        nodeId="id"
        linkSource="source"
        linkTarget="target"
        nodeCanvasObjectMode={() => "replace"}
        nodeCanvasObject={nodeCanvasObject}
        nodePointerAreaPaint={nodePointerAreaPaint}
        nodeLabel=""
        linkColor={linkColor}
        linkWidth={0.3}
        warmupTicks={100}
        cooldownTicks={300}
        cooldownTime={5000}
        enableNodeDrag={true}
        enablePointerInteraction={true}
        onNodeClick={handleNodeClick}
        onBackgroundClick={() => onNodeClick("")}
      />
    </div>
  );
}
