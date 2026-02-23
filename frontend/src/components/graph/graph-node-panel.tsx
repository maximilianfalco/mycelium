"use client";

import { useState, useEffect } from "react";
import { api, type GraphNodeDetail } from "@/lib/api";
import { Badge } from "@/components/ui/badge";

export function GraphNodePanel({
  projectId,
  nodeId,
  onClose,
}: {
  projectId: string;
  nodeId: string;
  onClose: () => void;
}) {
  const [detail, setDetail] = useState<GraphNodeDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    setError(null);
    api.graph
      .nodeDetail(projectId, nodeId)
      .then(setDetail)
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"))
      .finally(() => setLoading(false));
  }, [projectId, nodeId]);

  return (
    <div className="h-full flex flex-col">
      <div className="flex items-center justify-between px-4 py-3 border-b border-border shrink-0">
        <span className="text-sm font-medium truncate">node detail</span>
        <button
          onClick={onClose}
          className="text-xs text-muted-foreground hover:text-foreground"
        >
          close
        </button>
      </div>

      <div className="flex-1 overflow-y-auto px-4 py-3 space-y-4">
        {loading && (
          <p className="text-sm text-muted-foreground">loading...</p>
        )}
        {error && <p className="text-sm text-destructive">{error}</p>}
        {detail && (
          <>
            <div>
              <div className="flex items-center gap-2 mb-1">
                <span className="text-sm font-medium">{detail.name}</span>
                <Badge variant="outline" className="text-xs">
                  {detail.kind}
                </Badge>
              </div>
              <p className="text-xs text-muted-foreground font-mono truncate">
                {detail.qualifiedName}
              </p>
            </div>

            <div>
              <span className="text-xs text-muted-foreground block mb-1">
                file
              </span>
              <p className="text-xs font-mono truncate">{detail.filePath}</p>
              {detail.sourceAlias && (
                <p className="text-xs text-muted-foreground mt-0.5">
                  {detail.sourceAlias}
                </p>
              )}
            </div>

            {detail.signature && (
              <div>
                <span className="text-xs text-muted-foreground block mb-1">
                  signature
                </span>
                <pre className="text-xs font-mono bg-accent/50 border border-border p-2 overflow-x-auto whitespace-pre-wrap">
                  {detail.signature}
                </pre>
              </div>
            )}

            {detail.docstring && (
              <div>
                <span className="text-xs text-muted-foreground block mb-1">
                  docs
                </span>
                <p className="text-xs text-muted-foreground whitespace-pre-wrap">
                  {detail.docstring}
                </p>
              </div>
            )}

            <div>
              <span className="text-xs text-muted-foreground block mb-1">
                connections
              </span>
              <div className="flex gap-4 text-xs">
                <span>
                  <span className="text-foreground">{detail.callers}</span>{" "}
                  <span className="text-muted-foreground">callers</span>
                </span>
                <span>
                  <span className="text-foreground">{detail.callees}</span>{" "}
                  <span className="text-muted-foreground">callees</span>
                </span>
                <span>
                  <span className="text-foreground">{detail.importers}</span>{" "}
                  <span className="text-muted-foreground">importers</span>
                </span>
              </div>
            </div>

            {detail.sourceCode && (
              <div>
                <span className="text-xs text-muted-foreground block mb-1">
                  source
                </span>
                <pre className="text-xs font-mono bg-accent/50 border border-border p-2 overflow-x-auto max-h-64 whitespace-pre-wrap">
                  {detail.sourceCode}
                </pre>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
