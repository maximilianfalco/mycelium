"use client";

import { useState } from "react";
import {
  api,
  type CrawlResponse,
  type ParseResponse,
  type WorkspaceResponse,
  type ChangesResponse,
} from "@/lib/api";
import { Input } from "@/components/ui/input";
import { TooltipProvider } from "@/components/ui/tooltip";
import { StageCard } from "./stage-card";
import { CrawlOutput } from "./crawl-output";
import { ParseOutput } from "./parse-output";
import { WorkspaceOutput } from "./workspace-output";
import { ChangesOutput } from "./changes-output";
import { EmbeddingPlayground } from "./embedding-playground";

export function DebugTab({
  rootPath,
  maxFileSizeKB = 100,
  onOpenFile,
}: {
  rootPath?: string;
  maxFileSizeKB?: number;
  onOpenFile?: (
    absPath: string,
    scrollToLine?: number,
    highlightEndLine?: number,
  ) => void;
}) {
  const [path, setPath] = useState(rootPath || "~/Desktop/Code");
  const [filePath, setFilePath] = useState("");

  const [crawlData, setCrawlData] = useState<CrawlResponse | null>(null);
  const [parseData, setParseData] = useState<ParseResponse | null>(null);
  const [workspaceData, setWorkspaceData] = useState<WorkspaceResponse | null>(
    null,
  );
  const [changesData, setChangesData] = useState<ChangesResponse | null>(null);

  const expandPath = (p: string) => {
    const home = process.env.NEXT_PUBLIC_HOME_DIR || "/Users/maximilianwidjaya";
    return p.replace(/^~/, home);
  };

  return (
    <TooltipProvider>
      <div className="space-y-4">
        <div className="space-y-2">
          <div>
            <label
              htmlFor="debug-target-path"
              className="text-xs text-muted-foreground block mb-1"
            >
              target path
            </label>
            <Input
              id="debug-target-path"
              placeholder="~/Desktop/Code/my-project"
              value={path}
              onChange={(e) => setPath(e.target.value)}
              className="font-mono text-sm"
            />
          </div>
          <div>
            <label
              htmlFor="debug-file-path"
              className="text-xs text-muted-foreground block mb-1"
            >
              file path (for parse)
            </label>
            <Input
              id="debug-file-path"
              placeholder="~/Desktop/Code/my-project/src/index.ts"
              value={filePath}
              onChange={(e) => setFilePath(e.target.value)}
              className="font-mono text-sm"
            />
          </div>
        </div>

        <div className="space-y-2">
          <StageCard
            title="stage 0: changes"
            description="detect git changes since last index"
            onRun={async () => {
              const res = await api.debug.changes(expandPath(path));
              setChangesData(res);
            }}
            disabled={!path.trim()}
          >
            {changesData && <ChangesOutput data={changesData} />}
          </StageCard>

          <StageCard
            title="stage 1: workspace"
            description="detect workspace type and packages"
            onRun={async () => {
              const res = await api.debug.workspace(expandPath(path));
              setWorkspaceData(res);
            }}
            disabled={!path.trim()}
          >
            {workspaceData && <WorkspaceOutput data={workspaceData} />}
          </StageCard>

          <StageCard
            title="stage 2: crawl"
            description="crawl directory for indexable files"
            onRun={async () => {
              const res = await api.debug.crawl(
                expandPath(path),
                maxFileSizeKB,
              );
              setCrawlData(res);
            }}
            disabled={!path.trim()}
          >
            {crawlData && (
              <CrawlOutput
                data={crawlData}
                maxFileSizeKB={maxFileSizeKB}
                onSelectFile={(relPath) => {
                  const abs = expandPath(path) + "/" + relPath;
                  setFilePath(abs);
                }}
                onOpenFile={
                  onOpenFile ? (absPath) => onOpenFile(absPath) : undefined
                }
              />
            )}
          </StageCard>

          <StageCard
            title="stage 3: parse"
            description="parse a single file into nodes and edges"
            onRun={async () => {
              const target =
                filePath ||
                crawlData?.files?.[0]?.absPath ||
                path + "/src/index.ts";
              const res = await api.debug.parse(expandPath(target));
              setParseData(res);
            }}
            disabled={!path.trim()}
          >
            {parseData && (
              <ParseOutput
                data={parseData}
                onViewSource={
                  onOpenFile
                    ? (_name, startLine, endLine) => {
                        const target =
                          filePath ||
                          crawlData?.files?.[0]?.absPath ||
                          path + "/src/index.ts";
                        onOpenFile(expandPath(target), startLine, endLine);
                      }
                    : undefined
                }
              />
            )}
          </StageCard>
        </div>

        <EmbeddingPlayground />
      </div>
    </TooltipProvider>
  );
}
