"use client";

import { useState } from "react";
import { SettingsIcon } from "lucide-react";
import { api, type ProjectSettings } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";

export function SettingsPanel({
  projectId,
  settings,
  onSave,
  onReindex,
  open: externalOpen,
  onOpenChange: externalOnOpenChange,
}: {
  projectId: string;
  settings: ProjectSettings;
  onSave: () => void;
  onReindex?: () => void;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const [internalOpen, setInternalOpen] = useState(false);
  const open = externalOpen ?? internalOpen;
  const setOpen = externalOnOpenChange ?? setInternalOpen;
  const [maxFileSizeKB, setMaxFileSizeKB] = useState(
    settings.maxFileSizeKB ?? 100,
  );
  const [rootPath, setRootPath] = useState(settings.rootPath ?? "");
  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    setSaving(true);
    try {
      await api.projects.updateSettings(projectId, {
        maxFileSizeKB,
        rootPath: rootPath || undefined,
      });
      onSave();
      setOpen(false);
    } catch (e) {
      alert(e instanceof Error ? e.message : "Failed to save settings");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild>
        <Button variant="secondary" size="sm" aria-label="Colony settings">
          <SettingsIcon className="size-4" />
        </Button>
      </SheetTrigger>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>colony settings</SheetTitle>
        </SheetHeader>
        <div className="space-y-6 px-4">
          <div className="space-y-2">
            <label
              htmlFor="maxFileSize"
              className="text-sm text-muted-foreground"
            >
              max file size (KB)
            </label>
            <Input
              id="maxFileSize"
              type="number"
              min={1}
              value={maxFileSizeKB}
              onChange={(e) => setMaxFileSizeKB(Number(e.target.value))}
            />
            <p className="text-xs text-muted-foreground">
              files larger than this are skipped during crawling
            </p>
          </div>
          <div className="space-y-2">
            <label htmlFor="rootPath" className="text-sm text-muted-foreground">
              root path override
            </label>
            <Input
              id="rootPath"
              placeholder="leave empty to use default"
              value={rootPath}
              onChange={(e) => setRootPath(e.target.value)}
            />
            <p className="text-xs text-muted-foreground">
              override the root directory for file crawling
            </p>
          </div>
          <Button onClick={handleSave} disabled={saving} className="w-full">
            {saving ? "saving..." : "save settings"}
          </Button>
          <div className="border-t border-border pt-6 space-y-2">
            <label className="text-sm text-muted-foreground">
              reindex colony
            </label>
            <p className="text-xs text-muted-foreground">
              force a full re-index of all sources, bypassing the file change
              limit. use this after updating the indexing pipeline or if
              incremental indexing is blocked.
            </p>
            <Button
              variant="secondary"
              onClick={() => {
                setOpen(false);
                onReindex?.();
              }}
              className="w-full"
            >
              reindex colony
            </Button>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}
