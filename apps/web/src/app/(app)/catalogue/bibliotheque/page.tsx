import type { Metadata } from "next";
import Link from "next/link";
import { ImportCourse } from "@/components/app/import-course";
import { apiFetch } from "@/lib/api";
import type { LibraryCourse } from "@/app/(app)/admin/bibliotheque/page";

export const metadata: Metadata = { title: "Bibliothèque lemlearn" };

/**
 * Formations prêtes à l'emploi, proposées par lemlearn.
 *
 * Importer en prend une copie : elle devient la vôtre, avec vos moyens et
 * votre public. C'est vous qui la signez sur une convention, donc c'est vous
 * qui l'assumez — d'où la copie plutôt que la référence.
 */
export default async function OrgLibraryPage() {
  const { courses } = await apiFetch<{ courses: LibraryCourse[] }>("/v1/bibliotheque");

  return (
    <>
      <header className="flex h-14 items-center gap-3 border-b border-line px-6">
        <Link href="/catalogue" className="text-xs text-ink-3 hover:text-ink">
          Catalogue
        </Link>
        <span className="text-ink-3">/</span>
        <h1 className="text-sm font-medium">Bibliothèque lemlearn</h1>
      </header>

      <div className="mx-auto max-w-4xl px-6 py-6">
        <p className="text-xs text-ink-2">
          Des programmes complets, avec leurs mentions Qualiopi déjà rédigées.
          Importer en place une copie dans votre catalogue, en brouillon :
          relisez-la, adaptez-la à vos moyens et à votre public, puis publiez-la.
          C&apos;est votre nom qui figurera sur la convention.
        </p>

        {courses.length === 0 ? (
          <p className="mt-8 text-xs text-ink-3">
            Aucune formation disponible pour l&apos;instant.
          </p>
        ) : (
          <div className="mt-6 grid gap-4 md:grid-cols-2">
            {courses.map((course) => (
              <article key={course.id} className="surface-card flex flex-col p-5">
                <h2 className="text-sm font-medium">{course.title}</h2>
                <p className="mt-1 font-mono text-2xs text-ink-3">
                  {course.durationHours} h
                  {course.audience ? ` · ${course.audience}` : ""}
                </p>
                {course.summary && <p className="mt-2.5 text-xs text-ink-2">{course.summary}</p>}

                {course.objectives && course.objectives.length > 0 && (
                  <ul className="mt-3 flex-1 space-y-1.5">
                    {course.objectives.slice(0, 4).map((objective) => (
                      <li key={objective} className="flex gap-2 text-2xs text-ink-2">
                        <span className="mt-1.5 size-1 shrink-0 rounded-full bg-accent" />
                        {objective}
                      </li>
                    ))}
                  </ul>
                )}

                <div className="mt-4">
                  <ImportCourse courseId={course.id} title={course.title} />
                </div>
              </article>
            ))}
          </div>
        )}
      </div>
    </>
  );
}
