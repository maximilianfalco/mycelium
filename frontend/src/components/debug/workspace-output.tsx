import type { WorkspaceResponse } from "@/lib/api";
import { Badge } from "@/components/ui/badge";

export function WorkspaceOutput({ data }: { data: WorkspaceResponse }) {
  return (
    <div className="space-y-3">
      <div className="flex gap-4 text-xs text-muted-foreground">
        <span>type: {data.workspaceType}</span>
        <span>manager: {data.packageManager}</span>
        <span>{data.packages.length} packages</span>
      </div>

      <div className="space-y-1">
        <p className="text-xs text-muted-foreground font-medium">packages</p>
        {data.packages.map((pkg) => (
          <div key={pkg.name} className="flex items-center gap-3 px-3 py-1.5 border border-border text-sm">
            <span className="font-mono">{pkg.name}</span>
            <Badge variant="outline" className="text-xs">
              {pkg.version}
            </Badge>
            <span className="text-xs text-muted-foreground ml-auto">{pkg.path}</span>
          </div>
        ))}
      </div>

      {Object.keys(data.aliasMap).length > 0 && (
        <div className="space-y-1">
          <p className="text-xs text-muted-foreground font-medium">alias map</p>
          {Object.entries(data.aliasMap).map(([alias, target]) => (
            <div key={alias} className="flex items-center gap-2 px-3 py-1 border border-border text-xs font-mono">
              <span>{alias}</span>
              <span className="text-muted-foreground">&rarr;</span>
              <span className="text-muted-foreground">{target}</span>
            </div>
          ))}
        </div>
      )}

      {Object.keys(data.tsconfigPaths).length > 0 && (
        <div className="space-y-1">
          <p className="text-xs text-muted-foreground font-medium">tsconfig paths</p>
          {Object.entries(data.tsconfigPaths).map(([alias, target]) => (
            <div key={alias} className="flex items-center gap-2 px-3 py-1 border border-border text-xs font-mono">
              <span>{alias}</span>
              <span className="text-muted-foreground">&rarr;</span>
              <span className="text-muted-foreground">{target}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
