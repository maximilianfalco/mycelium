"use client";

import { useState, useTransition } from "react";
import { Loader2Icon } from "lucide-react";
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
  const [isPending, startTransition] = useTransition();
  const [hasRun, setHasRun] = useState(false);

  const handleRun = () => {
    startTransition(async () => {
      try {
        await onRun();
        setHasRun(true);
        setOpen(true);
      } catch {
        // error handled by parent
      }
    });
  };

  return (
    <div className="border border-border">
      <div className="flex items-center justify-between px-4 py-3">
        <div className="flex items-center gap-3">
          <button
            onClick={() => hasRun && setOpen(!open)}
            className="text-sm font-medium text-foreground hover:underline"
            aria-expanded={hasRun ? open : undefined}
            title={title}
          >
            {title}
          </button>
          <span className="text-xs text-muted-foreground">{description}</span>
        </div>
        <Button
          variant="secondary"
          size="sm"
          onClick={handleRun}
          disabled={disabled || isPending}
          title="Run stage"
        >
          {isPending ? "running..." : "run"}
        </Button>
      </div>
      {isPending && (
        <div className="border-t border-border flex items-center justify-center py-8">
          <Loader2Icon className="size-4 motion-safe:animate-spin text-muted-foreground" />
        </div>
      )}
      {!isPending && open && hasRun && children && (
        <div className="border-t border-border px-4 py-3">{children}</div>
      )}
    </div>
  );
}
