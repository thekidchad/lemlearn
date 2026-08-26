"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

/**
 * Import d'une formation de la bibliothèque dans le catalogue de l'organisme.
 *
 * La copie arrive en brouillon, à dessein : elle porte des mentions rédigées
 * par lemlearn, et c'est l'organisme qui les signera sur une convention. Il
 * doit les relire avant qu'elles n'engagent son nom.
 */
export function ImportCourse({ courseId, title }: { courseId: string; title: string }) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const run = async () => {
    setBusy(true);
    setError(null);
    try {
      const response = await fetch(`/api/bibliotheque/${courseId}`, { method: "POST" });
      const body = (await response.json()) as {
        course?: { id: string };
        modules?: number;
        error?: string;
      };
      if (!response.ok || !body.course) throw new Error(body.error ?? "import impossible");
      setDone(body.course.id);
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "import impossible");
    } finally {
      setBusy(false);
    }
  };

  if (done) {
    return (
      <p className="text-2xs text-ok">
        « {title} » est dans votre catalogue, en brouillon.{" "}
        <a href={`/catalogue/${done}`} className="underline">
          L&apos;ouvrir
        </a>
      </p>
    );
  }

  return (
    <div className="flex flex-wrap items-center gap-2">
      <button
        type="button"
        onClick={run}
        disabled={busy}
        className="h-9 rounded-lg border border-line-strong px-4 text-xs font-medium hover:border-accent disabled:opacity-50"
      >
        {busy ? "Import…" : "Importer dans mon catalogue"}
      </button>
      {error && <span className="text-2xs text-danger">{error}</span>}
    </div>
  );
}
