"use client";

import { useEffect, useRef, useState } from "react";
import { XIcon, CopyIcon, Loader2Icon } from "lucide-react";
import { toast } from "sonner";
import { api, type ReadFileResponse } from "@/lib/api";
import { Button } from "@/components/ui/button";

const EXT_TO_LANG: Record<string, string> = {
  ts: "typescript",
  tsx: "tsx",
  js: "javascript",
  jsx: "jsx",
  go: "go",
  py: "python",
  rs: "rust",
  json: "json",
  yaml: "yaml",
  yml: "yaml",
  md: "markdown",
  css: "css",
  scss: "scss",
  html: "html",
  sql: "sql",
  sh: "bash",
  bash: "bash",
  zsh: "bash",
  toml: "toml",
  xml: "xml",
  svg: "xml",
  graphql: "graphql",
  gql: "graphql",
  mod: "go",
  txt: "plaintext",
};

function getLang(ext: string): string {
  return EXT_TO_LANG[ext.toLowerCase()] || "plaintext";
}

export interface CodeViewerFile {
  filePath: string;
  scrollToLine?: number;
  highlightEndLine?: number;
}

export function CodeViewer({
  file,
  onClose,
}: {
  file: CodeViewerFile;
  onClose: () => void;
}) {
  const [data, setData] = useState<ReadFileResponse | null>(null);
  const [html, setHtml] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  const fileName = file.filePath.split("/").pop() || file.filePath;

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    setHtml("");
    setData(null);

    api.debug
      .readFile(file.filePath)
      .then(async (res) => {
        if (cancelled) return;
        setData(res);

        const { codeToHtml } = await import("shiki");
        if (cancelled) return;

        const highlighted = await codeToHtml(res.content, {
          lang: getLang(res.language),
          theme: "github-dark-default",
        });
        if (cancelled) return;
        setHtml(highlighted);
      })
      .catch((err) => {
        if (cancelled) return;
        setError(err.message || "Failed to read file");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [file.filePath]);

  useEffect(() => {
    if (!html || !file.scrollToLine || !scrollRef.current) return;

    requestAnimationFrame(() => {
      const container = scrollRef.current;
      if (!container) return;
      const lines = container.querySelectorAll(".line");
      const start = file.scrollToLine! - 1;
      const end = file.highlightEndLine ? file.highlightEndLine - 1 : start;

      for (let i = start; i <= end && i < lines.length; i++) {
        lines[i].classList.add("highlighted");
      }

      const targetLine = lines[start];
      if (targetLine) {
        targetLine.scrollIntoView({ block: "center" });
      }
    });
  }, [html, file.scrollToLine, file.highlightEndLine]);

  return (
    <div className="flex flex-col h-full min-h-0">
      <div className="flex items-center justify-between px-3 py-2 border-b border-border shrink-0">
        <div className="flex items-center gap-2 min-w-0">
          <span className="text-sm font-mono font-medium truncate">
            {fileName}
          </span>
          {data && (
            <span className="text-xs text-muted-foreground shrink-0">
              {data.lineCount} lines
            </span>
          )}
        </div>
        <div className="flex items-center gap-1 shrink-0">
          {data && (
            <Button
              variant="ghost"
              size="icon"
              className="size-7"
              aria-label="Copy file contents"
              title="Copy file contents"
              onClick={() => {
                navigator.clipboard.writeText(data.content);
                toast("copied file contents");
              }}
            >
              <CopyIcon className="size-3.5" />
            </Button>
          )}
          <Button
            variant="ghost"
            size="icon"
            className="size-7"
            aria-label="Close code viewer"
            title="Close code viewer"
            onClick={onClose}
          >
            <XIcon className="size-3.5" />
          </Button>
        </div>
      </div>

      <div className="text-[10px] text-muted-foreground px-3 py-1 border-b border-border truncate shrink-0 font-mono">
        {file.filePath}
      </div>

      <div
        ref={scrollRef}
        className="code-viewer flex-1 overflow-auto min-h-0 text-xs"
      >
        {loading && (
          <div className="flex items-center justify-center py-12 text-muted-foreground">
            <Loader2Icon className="size-4 motion-safe:animate-spin mr-2" />
            <span className="text-xs">loading...</span>
          </div>
        )}
        {error && (
          <div className="px-3 py-4 text-xs text-destructive">{error}</div>
        )}
        {html && <div dangerouslySetInnerHTML={{ __html: html }} />}
      </div>
    </div>
  );
}
