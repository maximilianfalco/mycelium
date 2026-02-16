import type { CrawlResponse } from "@/lib/api";
import { Badge } from "@/components/ui/badge";

export function CrawlOutput({ data }: { data: CrawlResponse }) {
  return (
    <div className="space-y-3">
      <div className="flex gap-4 text-xs text-muted-foreground">
        <span>{data.stats.total} files</span>
        <span>{data.stats.skipped} skipped</span>
        {Object.entries(data.stats.byExtension).map(([ext, count]) => (
          <span key={ext}>
            {ext}: {count}
          </span>
        ))}
      </div>
      <div className="border border-border max-h-64 overflow-y-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-xs text-muted-foreground">
              <th className="text-left px-3 py-1.5 font-normal">path</th>
              <th className="text-left px-3 py-1.5 font-normal">ext</th>
              <th className="text-right px-3 py-1.5 font-normal">size</th>
            </tr>
          </thead>
          <tbody>
            {data.files.map((f) => (
              <tr key={f.relPath} className="border-b border-border last:border-0">
                <td className="px-3 py-1.5 font-mono text-xs">{f.relPath}</td>
                <td className="px-3 py-1.5">
                  <Badge variant="outline" className="text-xs">
                    {f.extension}
                  </Badge>
                </td>
                <td className="px-3 py-1.5 text-right text-xs text-muted-foreground">
                  {f.sizeBytes.toLocaleString()} B
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
