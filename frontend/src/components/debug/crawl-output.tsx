"use client";

import { useMemo, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import fuzzysort from "fuzzysort";
import { ArrowUpRightIcon, EyeIcon, XIcon } from "lucide-react";
import { toast } from "sonner";
import type { CrawlFile, CrawlResponse } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { TruncatedText } from "@/components/ui/truncated-text";

function formatKB(bytes: number): string {
  return (bytes / 1024).toFixed(1);
}

const ROW_HEIGHT = 33;
const WHITESPACE_RE = /\s+/;

type SearchMode = "exact" | "fuzzy";

const byScoreThenAlpha = (
  left: Fuzzysort.KeysResult<CrawlFile>,
  right: Fuzzysort.KeysResult<CrawlFile>,
) => {
  if (!right[0] || !left[0]) return 0;
  const byScore = right[0].score - left[0].score;
  return byScore !== 0
    ? byScore
    : left[0].target.localeCompare(right[0].target);
};

export function CrawlOutput({
  data,
  maxFileSizeKB = 100,
  onSelectFile,
  onOpenFile,
}: {
  data: CrawlResponse;
  maxFileSizeKB?: number;
  onSelectFile?: (relPath: string) => void;
  onOpenFile?: (absPath: string) => void;
}) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const [search, setSearch] = useState("");
  const [mode, setMode] = useState<SearchMode>("exact");

  const filtered = useMemo(() => {
    const trimmed = search.trim();
    if (!trimmed) return data.files;
    if (data.files.length === 0) return data.files;

    if (mode === "exact") {
      const terms = trimmed.toLowerCase().split(WHITESPACE_RE);
      return data.files.filter((f) => {
        const path = f.relPath.toLowerCase();
        return terms.every((t) => path.includes(t));
      });
    }

    const results = fuzzysort.go(trimmed, data.files, {
      keys: ["relPath"],
      threshold: -1000,
    });
    return [...results].sort(byScoreThenAlpha).map((r) => r.obj);
  }, [data.files, search, mode]);

  const virtualizer = useVirtualizer({
    count: filtered.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 20,
  });

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <div className="flex gap-4 text-xs text-muted-foreground">
          <span>{data.stats.total} files</span>
          <span>{data.stats.skipped} skipped</span>
          {Object.entries(data.stats.byExtension).map(([ext, count]) => (
            <span key={ext}>
              {ext}: {count}
            </span>
          ))}
        </div>
        <span className="text-xs text-muted-foreground">
          max file size: {maxFileSizeKB} KB
        </span>
      </div>
      <div className="flex gap-2 items-center">
        <div className="relative flex-1">
          <Input
            placeholder="search files..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="font-mono text-xs h-8 pr-8"
          />
          {search && (
            <button
              onClick={() => setSearch("")}
              className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              aria-label="Clear search"
              title="Clear search"
            >
              <XIcon className="size-3.5" />
            </button>
          )}
        </div>
        <div className="flex border border-border text-xs">
          <Button
            variant={mode === "exact" ? "secondary" : "ghost"}
            size="sm"
            className="h-8 px-2.5 text-xs rounded-none"
            onClick={() => setMode("exact")}
            title="Exact match"
          >
            exact
          </Button>
          <Button
            variant={mode === "fuzzy" ? "secondary" : "ghost"}
            size="sm"
            className="h-8 px-2.5 text-xs rounded-none"
            onClick={() => setMode("fuzzy")}
            title="Fuzzy match"
          >
            fuzzy
          </Button>
        </div>
      </div>
      <div className="border border-border">
        <div
          className={`grid text-xs text-muted-foreground border-b border-border ${onOpenFile ? "grid-cols-[1fr_80px_80px_36px]" : "grid-cols-[1fr_80px_80px]"}`}
        >
          <span className="px-3 py-1.5">
            path{search.trim() && ` (${filtered.length} matched)`}
          </span>
          <span className="px-3 py-1.5 text-center">ext</span>
          <span className="px-3 py-1.5 text-right">size</span>
          {onOpenFile && <span />}
        </div>
        <div ref={scrollRef} className="max-h-64 overflow-y-auto">
          <div
            style={{ height: virtualizer.getTotalSize(), position: "relative" }}
          >
            {virtualizer.getVirtualItems().map((virtualRow) => {
              const f = filtered[virtualRow.index];
              return (
                <div
                  key={f.relPath}
                  className={`grid border-b border-border last:border-0 items-center ${onOpenFile ? "grid-cols-[1fr_80px_80px_36px]" : "grid-cols-[1fr_80px_80px]"}`}
                  style={{
                    position: "absolute",
                    top: 0,
                    left: 0,
                    width: "100%",
                    height: ROW_HEIGHT,
                    transform: `translateY(${virtualRow.start}px)`,
                  }}
                >
                  <span className="px-3 py-1.5 font-mono text-xs min-w-0 flex items-center gap-1 group">
                    <button
                      className="truncate cursor-pointer hover:text-foreground text-left"
                      onClick={() => {
                        navigator.clipboard.writeText(f.relPath);
                        toast("copied to clipboard", {
                          description: f.relPath,
                        });
                      }}
                      aria-label={`Copy path ${f.relPath}`}
                      title="Copy path"
                    >
                      <TruncatedText>{f.relPath}</TruncatedText>
                    </button>
                    {onSelectFile && (
                      <button
                        onClick={() => {
                          onSelectFile(f.relPath);
                          toast("file path set!", {
                            description: f.relPath,
                          });
                        }}
                        className="shrink-0 opacity-0 group-hover:opacity-100 focus:opacity-100 text-muted-foreground hover:text-foreground"
                        aria-label={`Use ${f.relPath} as parse target`}
                        title="Set as parse target"
                      >
                        <ArrowUpRightIcon className="size-3" />
                      </button>
                    )}
                  </span>
                  <span className="px-3 py-1.5 flex justify-center">
                    <Badge variant="outline" className="text-xs">
                      {f.extension}
                    </Badge>
                  </span>
                  <span className="px-3 py-1.5 text-right text-xs text-muted-foreground">
                    {formatKB(f.sizeBytes)} KB
                  </span>
                  {onOpenFile && (
                    <span className="flex justify-center">
                      <button
                        onClick={() => onOpenFile(f.absPath)}
                        className="text-muted-foreground hover:text-foreground"
                        aria-label={`View ${f.relPath}`}
                        title="View file"
                      >
                        <EyeIcon className="size-3.5" />
                      </button>
                    </span>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      </div>
    </div>
  );
}
