"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

/**
 * Correction du programme d'une formation.
 *
 * Une formation était figée dès sa création : corriger une faute dans un
 * objectif obligeait à en créer une seconde et à abandonner la première. C'est
 * aussi ce qui rendait la publication inatteignable, puisqu'elle exige des
 * mentions qu'on ne pouvait plus renseigner.
 *
 * Le panneau reste replié : cet écran sert d'abord à consulter un programme et
 * à y ajouter des modules, pas à le réécrire.
 */
interface Course {
  title: string;
  goal?: string;
  objectives?: string[] | null;
  prerequisites?: string;
  audience?: string;
  means?: string;
  assessment?: string;
  sanction?: string;
  accessibility?: string;
  durationHours: number;
  priceHT: number;
  tags?: string[] | null;
}

export function CourseForm({ courseId, course }: { courseId: string; course: Course }) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setError(null);
    setBusy(true);
    try {
      const response = await fetch(`/api/courses/${courseId}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          title: String(form.get("title") ?? ""),
          goal: String(form.get("goal") ?? ""),
          // Un objectif par ligne : c'est ainsi qu'on les écrit, et une liste
          // séparée par des virgules coupe au premier objectif qui en contient.
          objectives: String(form.get("objectives") ?? "")
            .split("\n")
            .map((ligne) => ligne.trim())
            .filter(Boolean),
          prerequisites: String(form.get("prerequisites") ?? ""),
          audience: String(form.get("audience") ?? ""),
          means: String(form.get("means") ?? ""),
          assessment: String(form.get("assessment") ?? ""),
          sanction: String(form.get("sanction") ?? ""),
          accessibility: String(form.get("accessibility") ?? ""),
          durationHours: Number(form.get("durationHours") ?? 0),
          priceHT: Number(form.get("priceHT") ?? 0),
          tags: String(form.get("tags") ?? "")
            .split(",")
            .map((tag) => tag.trim())
            .filter(Boolean),
        }),
      });
      const body = (await response.json()) as { error?: string };
      if (!response.ok) throw new Error(body.error ?? "correction refusée");
      setOpen(false);
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "correction refusée");
    } finally {
      setBusy(false);
    }
  };

  if (!open) {
    return (
      <button type="button" className="btn-secondary" onClick={() => setOpen(true)}>
        Modifier le programme
      </button>
    );
  }

  return (
    <form onSubmit={submit} className="surface-card mt-2 max-w-2xl p-5">
      <h2 className="text-sm font-medium">Programme de la formation</h2>
      <p className="mt-1 text-2xs text-ink-3">
        L&apos;objectif, le public, les modalités d&apos;évaluation, la sanction et
        la durée sont les mentions que l&apos;article L.6353-1 impose : sans elles,
        la formation ne peut pas être publiée.
      </p>

      <div className="mt-4 space-y-3">
        <Champ label="Intitulé" name="title" defaultValue={course.title} />
        <Champ label="Objectif" name="goal" defaultValue={course.goal} />
        <Zone
          label="Objectifs pédagogiques"
          name="objectives"
          defaultValue={(course.objectives ?? []).join("\n")}
          hint="Un par ligne."
        />
        <Champ label="Public visé" name="audience" defaultValue={course.audience} />
        <Champ label="Prérequis" name="prerequisites" defaultValue={course.prerequisites} />
        <Champ label="Moyens pédagogiques" name="means" defaultValue={course.means} />
        <Champ
          label="Modalités d'évaluation"
          name="assessment"
          defaultValue={course.assessment}
        />
        <Champ label="Sanction de la formation" name="sanction" defaultValue={course.sanction} />
        <Champ
          label="Accessibilité"
          name="accessibility"
          defaultValue={course.accessibility}
          hint="Accueil des personnes en situation de handicap."
        />
        <div className="grid grid-cols-2 gap-3">
          <Champ
            label="Durée (heures)"
            name="durationHours"
            type="number"
            defaultValue={String(course.durationHours)}
          />
          <Champ
            label="Prix HT (€)"
            name="priceHT"
            type="number"
            defaultValue={String(course.priceHT)}
          />
        </div>
        <Champ
          label="Étiquettes"
          name="tags"
          defaultValue={(course.tags ?? []).join(", ")}
          hint="Séparées par des virgules."
        />
      </div>

      {error && <p className="mt-3 text-xs text-danger">{error}</p>}

      <div className="mt-5 flex gap-2">
        <button type="submit" className="btn-primary" disabled={busy}>
          {busy ? "Enregistrement…" : "Enregistrer"}
        </button>
        <button type="button" className="btn-ghost" onClick={() => setOpen(false)}>
          Annuler
        </button>
      </div>
    </form>
  );
}

function Champ({
  label,
  name,
  defaultValue,
  type,
  hint,
}: {
  label: string;
  name: string;
  defaultValue?: string;
  type?: string;
  hint?: string;
}) {
  return (
    <label className="block">
      <span className="eyebrow">{label}</span>
      <input
        name={name}
        type={type}
        step={type === "number" ? "any" : undefined}
        defaultValue={defaultValue ?? ""}
        className="field mt-1.5"
      />
      {hint && <span className="mt-1 block text-2xs text-ink-3">{hint}</span>}
    </label>
  );
}

function Zone({
  label,
  name,
  defaultValue,
  hint,
}: {
  label: string;
  name: string;
  defaultValue?: string;
  hint?: string;
}) {
  return (
    <label className="block">
      <span className="eyebrow">{label}</span>
      <textarea
        name={name}
        rows={4}
        defaultValue={defaultValue ?? ""}
        className="mt-1.5 block w-full rounded-lg border border-line bg-surface-0 px-3 py-2 text-sm outline-none focus:border-accent"
      />
      {hint && <span className="mt-1 block text-2xs text-ink-3">{hint}</span>}
    </label>
  );
}
