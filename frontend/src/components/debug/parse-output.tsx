import type { ParseResponse } from "@/lib/api";
import { Badge } from "@/components/ui/badge";

export function ParseOutput({ data }: { data: ParseResponse }) {
  return (
    <div className="space-y-3">
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
        <p className="text-xs text-muted-foreground font-medium">nodes</p>
        {data.nodes.map((node) => (
          <div key={node.qualifiedName} className="border border-border px-3 py-2">
            <div className="flex items-center gap-2 mb-1">
              <Badge variant="outline" className="text-xs">
                {node.kind}
              </Badge>
              <span className="text-sm font-mono font-medium">{node.name}</span>
              <span className="text-xs text-muted-foreground">
                L{node.startLine}–{node.endLine}
              </span>
            </div>
            {node.signature && (
              <p className="text-xs font-mono text-muted-foreground mb-1">
                {node.signature}
              </p>
            )}
            {node.docstring && (
              <p className="text-xs text-muted-foreground">{node.docstring}</p>
            )}
            <pre className="text-xs font-mono bg-accent/30 px-2 py-1 mt-1 overflow-x-auto">
              {node.sourceCode}
            </pre>
          </div>
        ))}
      </div>

      <div className="space-y-1">
        <p className="text-xs text-muted-foreground font-medium">edges</p>
        {data.edges.map((edge, i) => (
          <div key={i} className="flex items-center gap-2 text-xs font-mono px-3 py-1 border border-border">
            <span>{edge.source}</span>
            <Badge variant="secondary" className="text-xs">
              {edge.kind}
            </Badge>
            <span>{edge.target}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
