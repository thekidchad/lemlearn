"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

/**
 * Faire sortir une formation du brouillon, ou l'y remettre.
 *
 * L'état existait depuis toujours au modèle et s'affichait sur cet écran, mais
 * rien ne permettait d'en changer : une formation créée en brouillon y restait.
 *
 * Le refus est détaillé plutôt que résumé. L'API vérifie ce qu'un programme de
 * formation doit porter — objectif, public, évaluation, sanction, durée — et
 * répond avec ce qui manque ; afficher « publication refusée » obligerait à
 * deviner.
 */
export function CoursePublish({
  courseId,
  published,
}: {
  courseId: string;
  published: boolean;
}) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Le geste le plus fréquent d'un catalogue : une même formation revient en
  // version courte, en intensif, en intra-entreprise. La réécrire à chaque fois
  // fait diverger les mentions obligatoires.
  const dupliquer = async () => {
    setError(null);
    setBusy(true);
    try {
      const response = await fetch(`/api/courses/${courseId}/copie`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: "{}",
      });
      const body = (await response.json()) as { course?: { id: string }; error?: string };
      if (!response.ok || !body.course) throw new Error(body.error ?? "duplication refusée");
      router.push(`/catalogue/${body.course.id}`);
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "duplication refusée");
      setBusy(false);
    }
  };

  const basculer = async () => {
    setError(null);
    setBusy(true);
    try {
      const response = await fetch(`/api/courses/${courseId}/publication`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ published: !published }),
      });
      const body = (await response.json()) as { error?: string };
      if (!response.ok) throw new Error(body.error ?? "publication refusée");
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "publication refusée");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex flex-col items-start gap-1.5">
      <div className="flex items-center gap-2.5">
        <span
          className={`inline-flex h-6 items-center rounded-md px-2 text-2xs ${
            published ? "bg-ok/15 text-ok" : "bg-surface-2 text-ink-3"
          }`}
        >
          {published ? "Publiée" : "Brouillon"}
        </span>
        <button
          type="button"
          className="btn-secondary"
          disabled={busy}
          onClick={dupliquer}
          title="Recopie la formation et ses modules, en brouillon."
        >
          Dupliquer
        </button>
        <button type="button" className="btn-secondary" disabled={busy} onClick={basculer}>
          {busy
            ? "…"
            : published
              ? "Repasser en brouillon"
              : "Publier la formation"}
        </button>
      </div>
      {error && <p className="max-w-sm text-2xs text-danger">{error}</p>}
      {!published && !error && (
        <p className="text-2xs text-ink-3">
          Tant qu&apos;elle est en brouillon, elle ne peut pas recevoir de session.
        </p>
      )}
    </div>
  );
}
