"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { api, type Project } from "@/lib/api";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";

export function ColonyList({
  initialProjects,
}: {
  initialProjects: Project[];
}) {
  const router = useRouter();
  const [projects, setProjects] = useState<Project[]>(initialProjects);
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [creating, setCreating] = useState(false);

  const handleCreate = async () => {
    if (!name.trim()) return;
    setCreating(true);
    try {
      const project = await api.projects.create(name, description);
      setOpen(false);
      setName("");
      setDescription("");
      router.push(`/projects/${project.id}`);
    } catch (e) {
      alert(e instanceof Error ? e.message : "Failed to create project");
    } finally {
      setCreating(false);
    }
  };

  return (
    <div className="max-w-3xl mx-auto px-6 py-10">
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-lg font-medium">colonies</h1>
          <p className="text-sm text-muted-foreground">
            your code intelligence projects
          </p>
        </div>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button variant="secondary" size="sm">
              + new colony
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>create colony</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 pt-2">
              <Input
                placeholder="colony name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && handleCreate()}
              />
              <Textarea
                placeholder="description (optional)"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                rows={3}
              />
              <Button
                onClick={handleCreate}
                disabled={!name.trim() || creating}
                className="w-full"
              >
                {creating ? "creating..." : "create"}
              </Button>
            </div>
          </DialogContent>
        </Dialog>
      </div>

      {projects.length === 0 ? (
        <div className="border border-dashed border-border py-16 text-center">
          <p className="text-sm text-muted-foreground">no colonies yet</p>
          <p className="text-xs text-muted-foreground mt-1">
            create one to start indexing your code
          </p>
        </div>
      ) : (
        <div className="space-y-2">
          {projects.map((p) => (
            <Card
              key={p.id}
              className="cursor-pointer hover:bg-accent/50 transition-colors"
              onClick={() => router.push(`/projects/${p.id}`)}
            >
              <CardHeader className="py-4">
                <CardTitle className="text-sm font-medium">{p.name}</CardTitle>
                {p.description && (
                  <CardDescription className="text-xs">
                    {p.description}
                  </CardDescription>
                )}
              </CardHeader>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
