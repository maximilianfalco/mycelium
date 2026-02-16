"use client";

import { useState } from "react";
import { api, type EmbedResponse, type CompareResponse } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";

export function EmbeddingPlayground() {
  const [text1, setText1] = useState("");
  const [text2, setText2] = useState("");
  const [embed1, setEmbed1] = useState<EmbedResponse | null>(null);
  const [embed2, setEmbed2] = useState<EmbedResponse | null>(null);
  const [comparison, setComparison] = useState<CompareResponse | null>(null);
  const [loading, setLoading] = useState<"embed1" | "embed2" | "compare" | null>(null);

  const handleEmbed = async (which: "embed1" | "embed2") => {
    const text = which === "embed1" ? text1 : text2;
    if (!text.trim()) return;
    setLoading(which);
    try {
      const res = await api.debug.embedText(text);
      if (which === "embed1") setEmbed1(res);
      else setEmbed2(res);
    } catch {
      // error silenced
    } finally {
      setLoading(null);
    }
  };

  const handleCompare = async () => {
    if (!text1.trim() || !text2.trim()) return;
    setLoading("compare");
    try {
      const res = await api.debug.compare(text1, text2);
      setComparison(res);
    } catch {
      // error silenced
    } finally {
      setLoading(null);
    }
  };

  return (
    <div className="border border-border">
      <div className="px-4 py-3 border-b border-border">
        <span className="text-sm font-medium">embedding playground</span>
        <span className="text-xs text-muted-foreground ml-3">
          embed text and compare similarity
        </span>
      </div>
      <div className="px-4 py-3 space-y-3">
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-2">
            <Textarea
              placeholder="enter text to embed..."
              value={text1}
              onChange={(e) => setText1(e.target.value)}
              rows={3}
              className="resize-none text-sm"
            />
            <Button
              variant="secondary"
              size="sm"
              onClick={() => handleEmbed("embed1")}
              disabled={!text1.trim() || loading === "embed1"}
            >
              {loading === "embed1" ? "embedding..." : "embed"}
            </Button>
            {embed1 && (
              <EmbedResult data={embed1} />
            )}
          </div>
          <div className="space-y-2">
            <Textarea
              placeholder="enter text to embed..."
              value={text2}
              onChange={(e) => setText2(e.target.value)}
              rows={3}
              className="resize-none text-sm"
            />
            <Button
              variant="secondary"
              size="sm"
              onClick={() => handleEmbed("embed2")}
              disabled={!text2.trim() || loading === "embed2"}
            >
              {loading === "embed2" ? "embedding..." : "embed"}
            </Button>
            {embed2 && (
              <EmbedResult data={embed2} />
            )}
          </div>
        </div>

        <div className="flex items-center gap-3">
          <Button
            variant="secondary"
            size="sm"
            onClick={handleCompare}
            disabled={!text1.trim() || !text2.trim() || loading === "compare"}
          >
            {loading === "compare" ? "comparing..." : "compare"}
          </Button>
          {comparison && (
            <div className="flex items-center gap-3 text-sm">
              <span className="font-mono font-medium">
                {(comparison.similarity * 100).toFixed(2)}% similar
              </span>
              <Badge variant="outline" className="text-xs">
                {comparison.tokenCount1} + {comparison.tokenCount2} tokens
              </Badge>
              <Badge variant="outline" className="text-xs">
                {comparison.dimensions}d
              </Badge>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function EmbedResult({ data }: { data: EmbedResponse }) {
  return (
    <div className="text-xs text-muted-foreground space-y-1">
      <div className="flex gap-3">
        <span>{data.dimensions}d</span>
        <span>{data.tokenCount} tokens</span>
        <span>{data.model}</span>
        {data.truncated && <Badge variant="destructive" className="text-xs">truncated</Badge>}
      </div>
      <p className="font-mono truncate">
        [{data.vector.map((v) => v.toFixed(4)).join(", ")}...]
      </p>
    </div>
  );
}
