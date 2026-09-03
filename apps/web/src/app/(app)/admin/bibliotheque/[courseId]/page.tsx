import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { LibraryModules, type LibraryModule } from "@/components/app/library-modules";
import { apiFetch, ApiError } from "@/lib/api";
import type { LibraryCourse } from "@/app/(app)/admin/bibliotheque/page";

export const metadata: Metadata = { title: "Formation de la bibliothèque" };

/**
 * Une formation de la bibliothèque, et ses modules.
 *
 * L'écran manquait : la palette y menait déjà, et tombait sur une page
 * inexistante. C'est pourtant le seul endroit d'où l'on peut donner un contenu
 * à une formation que des organismes vont importer.
 */
export default async function LibraryCoursePage({
  params,
}: PageProps<"/admin/bibliotheque/[courseId]">) {
  const { courseId } = await params;

  let data: { course: LibraryCourse; modules: LibraryModule[] | null };
  try {
    data = await apiFetch(`/v1/admin/bibliotheque/${courseId}`);
  } catch (error) {
    if (error instanceof ApiError && (error.status === 404 || error.status === 403)) notFound();
    throw error;
  }

  const { course } = data;
  const modules = [...(data.modules ?? [])].sort((a, b) => a.position - b.position);
  const minutes = modules.reduce((somme, module) => somme + module.durationMs, 0) / 60000;

  return (
    <>
      <header className="border-b border-line px-8 pt-6 pb-5">
        <nav className="flex items-center gap-2 text-2xs text-ink-3">
          <Link href="/admin/bibliotheque" className="hover:text-ink">
            Bibliothèque
          </Link>
          <span>/</span>
          <span>{course.published ? "Publiée" : "Brouillon"}</span>
        </nav>
        <h1 className="mt-2 text-xl font-medium tracking-tight">{course.title}</h1>
        {course.summary && <p className="mt-1.5 max-w-xl text-xs text-ink-2">{course.summary}</p>}
        <p className="mt-2 font-mono text-2xs text-ink-3" data-numeric>
          {course.durationHours} h annoncées · {Math.round(minutes)} min de contenu ·{" "}
          {modules.length} module{modules.length > 1 ? "s" : ""}
        </p>
      </header>

      <div className="mx-auto max-w-3xl space-y-6 px-8 py-6">
        {course.published && modules.length === 0 && (
          <p className="rounded-lg border border-warn/40 bg-warn/10 px-3 py-2 text-xs text-warn">
            Cette formation est publiée sans aucun module. Un organisme qui
            l&apos;importe reçoit un programme vide, et comme l&apos;import est
            une copie, il ne pourra pas le compléter depuis la bibliothèque.
          </p>
        )}

        <LibraryModules courseId={courseId} modules={modules} />

        <section className="surface-card overflow-hidden">
          <h2 className="border-b border-line px-5 py-3 text-sm font-medium">Programme</h2>
          <dl className="divide-y divide-line/60">
            <Ligne label="Objectif" valeur={course.goal} />
            <Ligne label="Public visé" valeur={course.audience} />
            <Ligne label="Prérequis" valeur={course.prerequisites} />
            <Ligne label="Moyens" valeur={course.means} />
            <Ligne label="Évaluation" valeur={course.assessment} />
            <Ligne label="Sanction" valeur={course.sanction} />
            <Ligne label="Accessibilité" valeur={course.accessibility} />
          </dl>
          <p className="border-t border-line px-5 py-3 text-2xs text-ink-3">
            Le programme se modifie depuis{" "}
            <Link href="/admin/bibliotheque" className="underline hover:text-ink">
              la liste
            </Link>
            .
          </p>
        </section>
      </div>
    </>
  );
}

function Ligne({ label, valeur }: { label: string; valeur?: string }) {
  return (
    <div className="flex items-baseline gap-4 px-5 py-2.5">
      <dt className="w-32 shrink-0 text-2xs text-ink-3">{label}</dt>
      <dd className="min-w-0 flex-1 text-xs">
        {valeur?.trim() ? valeur : <span className="text-ink-3">non renseigné</span>}
      </dd>
    </div>
  );
}
