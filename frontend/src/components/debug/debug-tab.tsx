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
}: {
  rootPath?: string;
  maxFileSizeKB?: number;
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
            <label className="text-xs text-muted-foreground block mb-1">
              target path
            </label>
            <Input
              placeholder="~/Desktop/Code/my-project"
              value={path}
              onChange={(e) => setPath(e.target.value)}
              className="font-mono text-sm"
            />
          </div>
          <div>
            <label className="text-xs text-muted-foreground block mb-1">
              file path (for parse)
            </label>
            <Input
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
              />
            )}
          </StageCard>

          <StageCard
            title="stage 3: parse"
            description="parse a single file into nodes and edges"
            onRun={async () => {
              const res = await api.debug.parse(
                expandPath(filePath || path + "/src/index.ts"),
              );
              setParseData(res);
            }}
            disabled={!path.trim()}
          >
            {parseData && <ParseOutput data={parseData} />}
          </StageCard>
        </div>

        <EmbeddingPlayground />
      </div>
    </TooltipProvider>
  );
}
