import type { ChangesResponse } from "@/lib/api";
import { Badge } from "@/components/ui/badge";

function FileList({ label, files, variant }: { label: string; files: string[]; variant: "default" | "secondary" | "destructive" }) {
  if (files.length === 0) return null;
  return (
    <div className="space-y-1">
      <div className="flex items-center gap-2">
        <p className="text-xs text-muted-foreground font-medium">{label}</p>
        <Badge variant={variant} className="text-xs">
          {files.length}
        </Badge>
      </div>
      {files.map((f) => (
        <div key={f} className="px-3 py-1 border border-border text-xs font-mono">
          {f}
        </div>
      ))}
    </div>
  );
}

export function ChangesOutput({ data }: { data: ChangesResponse }) {
  return (
    <div className="space-y-3">
      <div className="flex gap-4 text-xs text-muted-foreground">
        <span>git: {data.isGitRepo ? "yes" : "no"}</span>
        <span>current: {data.currentCommit}</span>
        <span>last indexed: {data.lastIndexedCommit || "never"}</span>
        {data.isFullIndex && <Badge variant="secondary" className="text-xs">full re-index</Badge>}
        {data.thresholdExceeded && <Badge variant="destructive" className="text-xs">threshold exceeded</Badge>}
      </div>

      <FileList label="added" files={data.addedFiles} variant="default" />
      <FileList label="modified" files={data.modifiedFiles} variant="secondary" />
      <FileList label="deleted" files={data.deletedFiles} variant="destructive" />
    </div>
  );
}
