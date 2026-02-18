"use client";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";

export function ConfirmationDialog({
  open,
  onOpenChange,
  title,
  body,
  cancel = "cancel",
  yes = "confirm",
  loading = false,
  onCancel,
  onAccept,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  body: string;
  cancel?: string;
  yes?: string;
  loading?: boolean;
  onCancel: () => void;
  onAccept: () => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent showCloseButton={false}>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{body}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="secondary" onClick={onCancel} disabled={loading}>
            {cancel}
          </Button>
          <Button variant="destructive" onClick={onAccept} disabled={loading}>
            {loading ? "deleting..." : yes}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
