import { Suspense } from "react";
import { api } from "@/lib/api";
import { ProjectDetail } from "@/components/project-detail";

export default async function ProjectPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return (
    <Suspense
      fallback={
        <div className="max-w-5xl mx-auto px-6 py-10">
          <p className="text-sm text-muted-foreground">loading...</p>
        </div>
      }
    >
      <ProjectLoader id={id} />
    </Suspense>
  );
}

async function ProjectLoader({ id }: { id: string }) {
  const [project, sources, indexStatus] = await Promise.all([
    api.projects.get(id),
    api.sources.list(id),
    api.indexing.status(id),
  ]);
  return (
    <ProjectDetail
      id={id}
      initialProject={project}
      initialSources={sources}
      initialIndexStatus={indexStatus}
    />
  );
}
