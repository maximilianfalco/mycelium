"use client";

import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

export function TruncatedText({
  children,
  className = "",
}: {
  children: string;
  className?: string;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className={`truncate max-w-full ${className}`}>{children}</span>
      </TooltipTrigger>
      <TooltipContent
        side="top"
        align="start"
        className="max-w-md break-all font-mono text-xs"
      >
        {children}
      </TooltipContent>
    </Tooltip>
  );
}
