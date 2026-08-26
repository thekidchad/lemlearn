"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import type { LibraryCourse } from "@/app/(app)/admin/bibliotheque/page";

/**
 * Édition de la bibliothèque.
 *
 * Publier exige les mêmes mentions qu'un organisme doit fournir — public visé,
 * objectifs, durée : une formation publiée ici produira des conventions chez
 * tous ceux qui l'importent, et une mention manquante s'y multiplierait.
 */
export function LibraryEditor({ courses }: { courses: LibraryCourse[] }) {
  const router = useRouter();
  const [editing, setEditing] = useState<LibraryCourse | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const blank: LibraryCourse = {
    id: "",
    title: "",
    durationHours: 7,
    published: false,
    updatedAt: "",
  };

  const save = async (course: LibraryCourse, publish: boolean) => {
    setBusy(true);
    setError(null);
    try {
      const response = await fetch(
        `/api/admin/bibliotheque${course.id ? `?id=${course.id}` : ""}`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            ...course,
            objectives: (course.objectives ?? []).filter(Boolean),
            tags: (course.tags ?? []).filter(Boolean),
            published: publish,
          }),
        },
      );
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

  const remove = async (course: LibraryCourse) => {
    setBusy(true);
    setError(null);
    try {
      const response = await fetch(`/api/admin/bibliotheque?id=${course.id}`, {
        method: "DELETE",
      });
      if (!response.ok) {
        const body = (await response.json()) as { error?: string };
        throw new Error(body.error ?? "suppression impossible");
      }
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "suppression impossible");
    } finally {
      setBusy(false);
    }
  };

  if (editing) {
    return (
      <CourseForm
        course={editing}
        busy={busy}
        error={error}
        onCancel={() => setEditing(null)}
        onSave={save}
      />
    );
  }

  return (
    <div className="mt-5">
      <button
        type="button"
        onClick={() => setEditing(blank)}
        className="h-9 rounded-lg bg-accent px-4 text-xs font-medium text-white hover:bg-accent-hover"
      >
        Nouvelle formation
      </button>

      {error && <p className="mt-3 text-2xs text-danger">{error}</p>}

      {courses.length === 0 ? (
        <p className="mt-6 text-xs text-ink-3">
          La bibliothèque est vide. Une formation publiée ici apparaît dans le
          catalogue de tous vos clients.
        </p>
      ) : (
        <div className="mt-4 space-y-px overflow-hidden rounded-xl border border-line bg-line">
          {courses.map((course) => (
            <div key={course.id} className="bg-surface-1 px-4 py-3.5">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">{course.title}</p>
                  <p className="mt-0.5 font-mono text-2xs text-ink-3">
                    {course.durationHours} h · {(course.objectives ?? []).length} objectif(s)
                    {course.audience ? ` · ${course.audience}` : ""}
                  </p>
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  <span
                    className={`rounded px-1.5 py-0.5 text-2xs ${
                      course.published
                        ? "bg-ok/15 text-ok"
                        : "border border-line-strong text-ink-3"
                    }`}
                  >
                    {course.published ? "publiée" : "brouillon"}
                  </span>
                  <button
                    type="button"
                    onClick={() => setEditing(course)}
                    className="h-8 rounded-md border border-line px-2.5 text-xs text-ink-2 hover:border-accent hover:text-ink"
                  >
                    Modifier
                  </button>
                  <button
                    type="button"
                    onClick={() => remove(course)}
                    disabled={busy}
                    className="h-8 px-2 text-2xs text-ink-3 hover:text-danger disabled:opacity-50"
                  >
                    Retirer
                  </button>
                </div>
              </div>
              {course.summary && <p className="mt-2 text-2xs text-ink-2">{course.summary}</p>}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function CourseForm({
  course,
  busy,
  error,
  onCancel,
  onSave,
}: {
  course: LibraryCourse;
  busy: boolean;
  error: string | null;
  onCancel: () => void;
  onSave: (course: LibraryCourse, publish: boolean) => void;
}) {
  const [draft, setDraft] = useState(course);
  const set = (change: Partial<LibraryCourse>) => setDraft((current) => ({ ...current, ...change }));

  return (
    <div className="surface-card mt-5 p-5">
      <div className="grid gap-3 sm:grid-cols-2">
        <Field label="Intitulé" value={draft.title} onChange={(title) => set({ title })} />
        <Field
          label="Durée (heures)"
          value={String(draft.durationHours)}
          onChange={(value) => set({ durationHours: Number(value) || 0 })}
        />
        <div className="sm:col-span-2">
          <Field
            label="Résumé pour les organismes"
            value={draft.summary ?? ""}
            onChange={(summary) => set({ summary })}
            hint="Pour qui, et ce que cette formation leur évite d'écrire."
          />
        </div>
        <div className="sm:col-span-2">
          <label className="block">
            <span className="mb-1 block text-2xs text-ink-3">
              Objectifs pédagogiques — un par ligne
            </span>
            <textarea
              rows={3}
              value={(draft.objectives ?? []).join("\n")}
              onChange={(event) => set({ objectives: event.target.value.split("\n") })}
              className="w-full rounded-lg border border-line bg-surface-0 px-3 py-2 text-sm outline-none focus:border-accent"
            />
          </label>
        </div>
        <Field label="Objectif général" value={draft.goal ?? ""} onChange={(goal) => set({ goal })} />
        <Field
          label="Public visé"
          value={draft.audience ?? ""}
          onChange={(audience) => set({ audience })}
        />
        <Field
          label="Prérequis"
          value={draft.prerequisites ?? ""}
          onChange={(prerequisites) => set({ prerequisites })}
        />
        <Field label="Moyens pédagogiques" value={draft.means ?? ""} onChange={(means) => set({ means })} />
        <Field
          label="Modalités d'évaluation"
          value={draft.assessment ?? ""}
          onChange={(assessment) => set({ assessment })}
        />
        <Field
          label="Sanction"
          value={draft.sanction ?? ""}
          onChange={(sanction) => set({ sanction })}
        />
        <div className="sm:col-span-2">
          <Field
            label="Accessibilité"
            value={draft.accessibility ?? ""}
            onChange={(accessibility) => set({ accessibility })}
            hint="Mention obligatoire : accueil des personnes en situation de handicap."
          />
        </div>
      </div>

      {error && (
        <p className="mt-3 rounded-lg border border-danger/40 bg-danger/10 px-3 py-2 text-2xs text-danger">
          {error}
        </p>
      )}

      <div className="mt-5 flex flex-wrap gap-2">
        <button
          type="button"
          onClick={() => onSave(draft, false)}
          disabled={busy || !draft.title}
          className="h-9 rounded-lg border border-line-strong px-4 text-xs hover:border-accent disabled:opacity-50"
        >
          Enregistrer en brouillon
        </button>
        <button
          type="button"
          onClick={() => onSave(draft, true)}
          disabled={busy || !draft.title}
          className="h-9 rounded-lg bg-accent px-4 text-xs font-medium text-white hover:bg-accent-hover disabled:opacity-50"
        >
          Publier aux organismes
        </button>
        <button type="button" onClick={onCancel} className="h-9 px-3 text-xs text-ink-3 hover:text-ink">
          Annuler
        </button>
      </div>
    </div>
  );
}

function Field({
  label,
  value,
  onChange,
  hint,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  hint?: string;
}) {
  return (
    <label className="block">
      <span className="mb-1 block text-2xs text-ink-3">{label}</span>
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="h-9 w-full rounded-lg border border-line bg-surface-0 px-3 text-sm outline-none focus:border-accent"
      />
      {hint && <span className="mt-1 block text-2xs text-ink-3">{hint}</span>}
    </label>
  );
}
