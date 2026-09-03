"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

/**
 * Les modules d'une formation de la bibliothèque.
 *
 * Ils manquaient entièrement à l'écran : on pouvait écrire une formation et la
 * publier, jamais lui donner de contenu. Un organisme qui l'importait recevait
 * donc un programme sans séquence — une coquille, dont il ne pouvait rien
 * faire et qu'il ne pouvait pas non plus corriger, l'import étant une copie.
 *
 * La durée est saisie en minutes : personne ne compte en millisecondes, et
 * c'est pourtant l'unité dont a besoin la mesure d'assiduité.
 */
export interface LibraryModule {
  id: string;
  courseId: string;
  position: number;
  title: string;
  summary?: string;
  durationMs: number;
  minCoveragePercent: number;
  assetId?: string;
}

export function LibraryModules({
  courseId,
  modules,
}: {
  courseId: string;
  modules: LibraryModule[];
}) {
  const router = useRouter();
  const [editing, setEditing] = useState<LibraryModule | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const vierge: LibraryModule = {
    id: "",
    courseId,
    position: modules.length + 1,
    title: "",
    durationMs: 0,
    minCoveragePercent: 80,
  };

  const enregistrer = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setError(null);
    setBusy(true);
    try {
      const response = await fetch(`/api/admin/bibliotheque/${courseId}/modules`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          id: String(form.get("id") ?? ""),
          courseId,
          position: Number(form.get("position") ?? 1),
          title: String(form.get("title") ?? ""),
          summary: String(form.get("summary") ?? ""),
          durationMs: Math.round(Number(form.get("durationMinutes") ?? 0) * 60_000),
          minCoveragePercent: Number(form.get("minCoveragePercent") ?? 80),
          assetId: String(form.get("assetId") ?? ""),
        }),
      });
      const body = (await response.json()) as { error?: string };
      if (!response.ok) throw new Error(body.error ?? "enregistrement refusé");
      setEditing(null);
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "enregistrement refusé");
    } finally {
      setBusy(false);
    }
  };

  const supprimer = async (module: LibraryModule) => {
    setError(null);
    setBusy(true);
    try {
      const response = await fetch(
        `/api/admin/bibliotheque/${courseId}/modules?id=${module.id}`,
        { method: "DELETE" },
      );
      if (!response.ok) {
        const body = (await response.json().catch(() => ({}))) as { error?: string };
        throw new Error(body.error ?? "suppression refusée");
      }
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "suppression refusée");
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="surface-card overflow-hidden">
      <header className="flex flex-wrap items-center gap-3 border-b border-line px-5 py-3">
        <h2 className="text-sm font-medium">Modules</h2>
        <span className="font-mono text-2xs text-ink-3" data-numeric>
          {modules.length}
        </span>
        <button
          type="button"
          className="btn-secondary ml-auto"
          onClick={() => setEditing(vierge)}
        >
          Ajouter un module
        </button>
      </header>

      {error && <p className="px-5 py-2 text-xs text-danger">{error}</p>}

      {modules.length === 0 && !editing && (
        <p className="px-5 py-10 text-center text-xs text-ink-3">
          Aucun module. Une formation sans séquence s&apos;importe comme une
          coquille vide : l&apos;organisme reçoit un programme dont il ne peut
          rien faire.
        </p>
      )}

      {modules.length > 0 && (
        <ul className="divide-y divide-line/60">
          {modules.map((module) => (
            <li
              key={module.id}
              className="group flex flex-wrap items-center gap-x-4 gap-y-1 px-5 py-3"
            >
              <span className="w-6 shrink-0 font-mono text-2xs text-ink-3" data-numeric>
                {module.position}
              </span>
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm">{module.title}</p>
                <p className="truncate font-mono text-2xs text-ink-3">
                  {Math.round(module.durationMs / 60000)} min · seuil{" "}
                  {module.minCoveragePercent} % ·{" "}
                  {module.assetId ? "vidéo attachée" : "sans vidéo"}
                </p>
              </div>
              <div className="flex shrink-0 items-center gap-2">
                <button
                  type="button"
                  className="btn-ghost"
                  disabled={busy}
                  onClick={() => setEditing(module)}
                >
                  Modifier
                </button>
                <button
                  type="button"
                  className="btn-ghost"
                  disabled={busy}
                  onClick={() => supprimer(module)}
                >
                  Retirer
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}

      {editing && (
        // La clé remonte le formulaire quand on passe d'un module à un autre :
        // sans elle, les valeurs par défaut du précédent resteraient à l'écran.
        <form
          key={editing.id || "nouveau"}
          onSubmit={enregistrer}
          className="border-t border-line bg-surface-2 px-5 py-4"
        >
          <input type="hidden" name="id" value={editing.id} />
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="block sm:col-span-2">
              <span className="eyebrow">Intitulé</span>
              <input name="title" required defaultValue={editing.title} className="field mt-1.5" />
            </label>
            <label className="block sm:col-span-2">
              <span className="eyebrow">Résumé</span>
              <input name="summary" defaultValue={editing.summary ?? ""} className="field mt-1.5" />
            </label>
            <label className="block">
              <span className="eyebrow">Position</span>
              <input
                name="position"
                type="number"
                min={1}
                defaultValue={editing.position}
                className="field mt-1.5"
              />
            </label>
            <label className="block">
              <span className="eyebrow">Durée (minutes)</span>
              <input
                name="durationMinutes"
                type="number"
                min={0}
                defaultValue={Math.round(editing.durationMs / 60000)}
                className="field mt-1.5"
              />
            </label>
            <label className="block">
              <span className="eyebrow">Couverture minimale (%)</span>
              <input
                name="minCoveragePercent"
                type="number"
                min={0}
                max={100}
                defaultValue={editing.minCoveragePercent}
                className="field mt-1.5"
              />
            </label>
            <label className="block">
              <span className="eyebrow">Identifiant vidéo</span>
              <input name="assetId" defaultValue={editing.assetId ?? ""} className="field mt-1.5" />
              <span className="mt-1 block text-2xs text-ink-3">
                Une vidéo déjà transcodée. La fiche est recopiée à l&apos;import,
                pas le fichier.
              </span>
            </label>
          </div>

          <div className="mt-4 flex gap-2">
            <button type="submit" className="btn-primary" disabled={busy}>
              {busy ? "Enregistrement…" : "Enregistrer le module"}
            </button>
            <button type="button" className="btn-ghost" onClick={() => setEditing(null)}>
              Annuler
            </button>
          </div>
        </form>
      )}
    </section>
  );
}
