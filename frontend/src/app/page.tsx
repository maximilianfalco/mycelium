import { Suspense } from "react";
import { api } from "@/lib/api";
import { ColonyList } from "@/components/colony-list";

export default function HomePage() {
  return (
    <Suspense
      fallback={
        <div className="max-w-3xl mx-auto px-6 py-10">
          <p className="text-sm text-muted-foreground">loading...</p>
        </div>
      }
    >
      <ColonyListLoader />
    </Suspense>
  );
}

async function ColonyListLoader() {
  const projects = await api.projects.list();
  return <ColonyList initialProjects={projects} />;
}
