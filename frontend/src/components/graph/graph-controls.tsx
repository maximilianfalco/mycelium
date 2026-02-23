"use client";

import { Input } from "@/components/ui/input";

const KIND_LABELS: Record<string, string> = {
  function: "fn",
  method: "method",
  class: "class",
  struct: "struct",
  interface: "iface",
  type: "type",
  variable: "var",
  constant: "const",
  enum: "enum",
  module: "module",
  package: "pkg",
  field: "field",
};

export function GraphControls({
  kinds,
  hiddenKinds,
  onToggleKind,
  searchQuery,
  onSearchChange,
  nodeCount,
  edgeCount,
}: {
  kinds: Record<string, number>;
  hiddenKinds: Set<string>;
  onToggleKind: (kind: string) => void;
  searchQuery: string;
  onSearchChange: (query: string) => void;
  nodeCount: number;
  edgeCount: number;
}) {
  const sortedKinds = Object.entries(kinds).sort((a, b) => b[1] - a[1]);

  return (
    <div className="flex items-center gap-3 pb-3 border-b border-border text-xs overflow-x-auto">
      <Input
        placeholder="search nodes..."
        value={searchQuery}
        onChange={(e) => onSearchChange(e.target.value)}
        className="w-48 h-7 text-xs shrink-0"
      />
      <div className="flex items-center gap-1.5 shrink-0">
        {sortedKinds.map(([kind, count]) => (
          <button
            key={kind}
            onClick={() => onToggleKind(kind)}
            className={`px-1.5 py-0.5 border transition-opacity ${
              hiddenKinds.has(kind)
                ? "border-border text-muted-foreground opacity-40"
                : "border-foreground/20 text-foreground"
            }`}
            title={`${kind} (${count})`}
          >
            {KIND_LABELS[kind] || kind}
          </button>
        ))}
      </div>
      <div className="ml-auto text-muted-foreground shrink-0">
        {nodeCount.toLocaleString()} nodes &middot;{" "}
        {edgeCount.toLocaleString()} edges
      </div>
    </div>
  );
}
