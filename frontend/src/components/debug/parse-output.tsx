"use client";

import { useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { ExpandIcon } from "lucide-react";
import type { ParseResponse } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { TruncatedText } from "@/components/ui/truncated-text";

const EDGE_ROW_HEIGHT = 32;

export function ParseOutput({ data }: { data: ParseResponse }) {
  const edgeParentRef = useRef<HTMLDivElement>(null);
  const edgeVirtualizer = useVirtualizer({
    count: data.edges.length,
    getScrollElement: () => edgeParentRef.current,
    estimateSize: () => EDGE_ROW_HEIGHT,
    overscan: 20,
  });

  return (
    <div className="space-y-4">
      <div className="flex gap-4 text-xs text-muted-foreground">
        <span>{data.stats.nodeCount} nodes</span>
        <span>{data.stats.edgeCount} edges</span>
        {Object.entries(data.stats.byKind).map(([kind, count]) => (
          <span key={kind}>
            {kind}: {count}
          </span>
        ))}
      </div>

      <div className="space-y-2">
        <div className="flex items-center gap-2">
          <h3 className="text-sm font-medium">nodes</h3>
          <Badge variant="outline" className="text-xs">
            {data.stats.nodeCount}
          </Badge>
        </div>
        <div className="max-h-[500px] overflow-y-auto space-y-2">
          {data.nodes.map((node) => (
            <div
              key={node.qualifiedName}
              className="border border-border px-3 py-2"
            >
              <div className="flex items-center gap-2 mb-1 min-w-0">
                <Badge variant="outline" className="text-xs shrink-0">
                  {node.kind}
                </Badge>
                <span className="text-sm font-mono font-medium truncate">
                  {node.name}
                </span>
                <span className="text-xs text-muted-foreground shrink-0">
                  L{node.startLine}–{node.endLine}
                </span>
              </div>
              {node.signature && (
                <div className="overflow-hidden">
                  <TruncatedText className="text-xs font-mono text-muted-foreground mb-1">
                    {node.signature}
                  </TruncatedText>
                </div>
              )}
              {node.docstring && (
                <p className="text-xs text-muted-foreground">
                  {node.docstring}
                </p>
              )}
              {(() => {
                const lines = node.sourceCode.split("\n");
                const snippet = lines.slice(0, 10).join("\n");
                const hasMore = lines.length > 10;
                return (
                  <div className="mt-1">
                    <pre className="text-xs font-mono bg-accent/30 px-2 py-1 overflow-hidden">
                      {snippet}
                    </pre>
                    {hasMore && (
                      <button
                        onClick={() => console.log(node)}
                        className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground mt-2 cursor-pointer"
                      >
                        <ExpandIcon className="size-3" />
                        <span>View more ({lines.length} lines)</span>
                      </button>
                    )}
                  </div>
                );
              })()}
            </div>
          ))}
        </div>
      </div>

      <div className="space-y-2">
        <div className="flex items-center gap-2">
          <h3 className="text-sm font-medium">edges</h3>
          <Badge variant="outline" className="text-xs">
            {data.stats.edgeCount}
          </Badge>
        </div>
        <div className="border border-border">
          <div className="grid grid-cols-[1fr_100px_1fr] text-xs text-muted-foreground border-b border-border">
            <span className="px-3 py-1.5">source</span>
            <span className="px-3 py-1.5 text-center">type</span>
            <span className="px-3 py-1.5 text-right">target</span>
          </div>
          <div ref={edgeParentRef} className="max-h-[500px] overflow-y-auto">
            <div
              className="relative w-full"
              style={{ height: edgeVirtualizer.getTotalSize() }}
            >
              {edgeVirtualizer.getVirtualItems().map((virtualRow) => {
                const edge = data.edges[virtualRow.index];
                return (
                  <div
                    key={virtualRow.index}
                    className="absolute left-0 w-full grid grid-cols-[1fr_100px_1fr] items-center text-xs font-mono border-b border-border"
                    style={{
                      height: virtualRow.size,
                      transform: `translateY(${virtualRow.start}px)`,
                    }}
                  >
                    <span className="px-3 py-1.5 overflow-hidden">
                      <TruncatedText>{edge.source}</TruncatedText>
                    </span>
                    <span className="px-3 py-1.5 flex justify-center">
                      <Badge variant="secondary" className="text-xs">
                        {edge.kind}
                      </Badge>
                    </span>
                    <span className="px-3 py-1.5 overflow-hidden">
                      <TruncatedText>{edge.target}</TruncatedText>
                    </span>
                  </div>
                );
              })}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
