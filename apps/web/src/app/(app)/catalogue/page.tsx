import type { Metadata } from "next";
import Link from "next/link";
import { CreatePanel, Field, TextArea } from "@/components/app/form";
import { createCourse } from "@/app/actions/crm";
import { apiFetch } from "@/lib/api";

export const metadata: Metadata = { title: "Catalogue" };

interface Course {
  id: string;
  title: string;
  goal?: string;
  durationHours: number;
  priceHT: number;
  published: boolean;
  tags?: string[];
  objectives?: string[];
}

export default async function CataloguePage() {
  const { courses } = await apiFetch<{ courses: Course[] | null }>("/v1/courses");
  const rows = courses ?? [];

  return (
    <>
      <header className="flex h-14 items-center gap-2.5 border-b border-line px-6">
        <h1 className="text-sm font-medium">Catalogue</h1>
        <span className="rounded-md border border-line bg-surface-2 px-1.5 py-0.5 font-mono text-2xs text-ink-3">
          {rows.length}
        </span>

        <Link
          href="/catalogue/bibliotheque"
          className="ml-auto text-2xs text-ink-3 underline hover:text-ink"
        >
          Bibliothèque lemlearn
        </Link>

        <div>
          <CreatePanel
            label="Nouvelle formation"
            title="Nouvelle formation"
            action={createCourse}
          >
            <Field label="Intitulé" name="title" required placeholder="Sécurité incendie — SSIAP 1" />
            <TextArea label="Objectif général" name="goal" rows={2} />
            <TextArea
              label="Objectifs pédagogiques"
              name="objectives"
              rows={3}
              placeholder="Un objectif par ligne&#10;Exigé pour publier la formation"
            />
            <Field label="Public visé" name="audience" />
            <Field label="Prérequis" name="prerequisites" />
            <Field label="Moyens pédagogiques" name="means" />
            <Field label="Modalités d'évaluation" name="assessment" />
            <Field label="Sanction de la formation" name="sanction" />
            <Field
              label="Accessibilité"
              name="accessibility"
              hint="Mention obligatoire : accueil des personnes en situation de handicap."
            />
            <div className="grid grid-cols-2 gap-3">
              <Field label="Durée (heures)" name="durationHours" type="number" defaultValue={7} />
              <Field label="Prix HT (€)" name="priceHT" type="number" defaultValue={0} />
            </div>
            <Field label="Étiquettes" name="tags" placeholder="certifiante, présentiel" />
            <label className="flex items-center gap-2 text-2xs text-ink-2">
              <input
                type="checkbox"
                name="published"
                className="size-3.5 accent-[var(--color-accent)]"
              />
              Publier — une formation non publiée ne peut pas recevoir de session
            </label>
          </CreatePanel>
        </div>
      </header>

      {rows.length === 0 ? (
        <p className="px-6 py-16 text-center text-xs text-ink-3">
          Aucune formation au catalogue.
        </p>
      ) : (
        <div className="grid gap-4 p-6 md:grid-cols-2 xl:grid-cols-3">
          {rows.map((course) => (
            <Link
              key={course.id}
              href={`/catalogue/${course.id}`}
              className="surface-card flex flex-col p-4 transition-colors duration-[120ms] hover:border-line-strong"
            >
              <div className="flex items-start justify-between gap-3">
                <h2 className="text-sm font-medium">{course.title}</h2>
                {/* Une formation non publiée ne peut pas recevoir de session :
                    l'état doit se voir au premier coup d'œil. */}
                <span
                  className={`shrink-0 rounded px-1.5 py-0.5 text-2xs ${
                    course.published
                      ? "bg-ok/15 text-ok"
                      : "border border-line text-ink-3"
                  }`}
                >
                  {course.published ? "Publiée" : "Brouillon"}
                </span>
              </div>

              {course.goal && (
                <p className="mt-2 line-clamp-2 text-xs leading-relaxed text-ink-2">
                  {course.goal}
                </p>
              )}

              {course.objectives && course.objectives.length > 0 && (
                <ul className="mt-3 space-y-1">
                  {course.objectives.slice(0, 3).map((objective) => (
                    <li key={objective} className="flex gap-2 text-2xs text-ink-2">
                      <span className="mt-1.5 size-1 shrink-0 rounded-full bg-ok" />
                      {objective}
                    </li>
                  ))}
                </ul>
              )}

              <div className="mt-auto flex items-center justify-between gap-2 pt-4">
                <span className="font-mono text-2xs text-ink-3" data-numeric>
                  {course.durationHours} h
                </span>
                <span className="font-mono text-2xs text-ink-2" data-numeric>
                  {new Intl.NumberFormat("fr-FR", {
                    style: "currency",
                    currency: "EUR",
                    maximumFractionDigits: 0,
                  }).format(course.priceHT)}
                </span>
              </div>
            </Link>
          ))}
        </div>
      )}
    </>
  );
}
