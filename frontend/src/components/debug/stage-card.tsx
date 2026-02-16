"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";

export function StageCard({
  title,
  description,
  onRun,
  disabled,
  children,
}: {
  title: string;
  description: string;
  onRun: () => Promise<void>;
  disabled?: boolean;
  children?: React.ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [hasRun, setHasRun] = useState(false);

  const handleRun = async () => {
    setLoading(true);
    try {
      await onRun();
      setHasRun(true);
      setOpen(true);
    } catch {
      // error handled by parent
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="border border-border">
      <div className="flex items-center justify-between px-4 py-3">
        <div className="flex items-center gap-3">
          <button
            onClick={() => hasRun && setOpen(!open)}
            className="text-sm font-medium text-foreground hover:underline"
          >
            {title}
          </button>
          <span className="text-xs text-muted-foreground">{description}</span>
        </div>
        <Button
          variant="secondary"
          size="sm"
          onClick={handleRun}
          disabled={disabled || loading}
        >
          {loading ? "running..." : "run"}
        </Button>
      </div>
      {open && hasRun && children && (
        <div className="border-t border-border px-4 py-3">{children}</div>
      )}
    </div>
  );
}
