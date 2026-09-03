import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { CreatePanel, Field, TextArea } from "@/components/app/form";
import { CourseForm } from "@/components/app/course-form";
import { ModuleQuiz } from "@/components/app/module-quiz";
import { CoursePublish } from "@/components/app/course-publish";
import { CourseCoverUpload } from "@/components/app/course-cover-upload";
import { VideoUpload } from "@/components/app/video-upload";
import { addModule } from "@/app/actions/crm";
import { apiFetch, ApiError } from "@/lib/api";
import type { Quiz } from "@/app/(app)/questionnaires/page";

export const metadata: Metadata = { title: "Formation" };

interface Course {
  id: string;
  title: string;
  goal?: string;
  objectives?: string[];
  prerequisites?: string;
  audience?: string;
  means?: string;
  assessment?: string;
  sanction?: string;
  accessibility?: string;
  durationHours: number;
  priceHT: number;
  published: boolean;
  tags?: string[];
  /** Les deux évaluations qui bornent le parcours et conditionnent l'attestation. */
  positioningQuizId?: string;
  finalQuizId?: string;
  objectiveType?: string;
  certificationCode?: string;
}

interface Module {
  id: string;
  title: string;
  summary?: string;
  position: number;
  durationMs: number;
  assetId?: string;
  quizId?: string;
  minCoveragePercent: number;
}

export default async function CoursePage({ params }: PageProps<"/catalogue/[courseId]">) {
  const { courseId } = await params;

  let data: { course: Course; modules: Module[]; coverUrl?: string };
  try {
    data = await apiFetch(`/v1/courses/${courseId}`);
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) notFound();
    throw error;
  }

  const typologies = await apiFetch<{
    stagiaires: { code: string; label: string }[];
    objectifs: { code: string; label: string }[];
  }>("/v1/organisme/typologies").catch(() => ({ stagiaires: [], objectifs: [] }));

  const { quizzes } = await apiFetch<{ quizzes: Quiz[] }>("/v1/quizzes").catch(() => ({
    quizzes: [] as Quiz[],
  }));
  const modules = data.modules ?? [];

  return (
    <>
      <header className="flex h-14 items-center gap-3 border-b border-line px-6">
        <Link href="/catalogue" className="text-xs text-ink-3 hover:text-ink">
          Catalogue
        </Link>
        <span className="text-ink-3">/</span>
        <span className="truncate text-xs text-ink-2">{data.course.title}</span>

        <div className="ml-auto">
          <CreatePanel label="Ajouter un module" title="Nouveau module" action={addModule}>
            <input type="hidden" name="courseId" value={courseId} />
            <Field label="Intitulé" name="title" required />
            <TextArea label="Résumé" name="summary" rows={2} />
            <div className="grid grid-cols-2 gap-3">
              <Field
                label="Position"
                name="position"
                type="number"
                defaultValue={modules.length + 1}
              />
              <Field label="Durée (minutes)" name="durationMinutes" type="number" defaultValue={0} />
            </div>
            <label className="block">
              <span className="mb-1 block text-2xs text-ink-3">Questionnaire de fin de module</span>
              <select
                name="quizId"
                className="h-9 w-full rounded-lg border border-line bg-surface-0 px-2 text-sm outline-none focus:border-accent"
              >
                <option value="">Aucun</option>
                {quizzes
                  .filter((quiz) => quiz.kind === "post_module" && quiz.published)
                  .map((quiz) => (
                    <option key={quiz.id} value={quiz.id}>
                      {quiz.title}
                    </option>
                  ))}
              </select>
            </label>
            <Field
              label="Couverture minimale (%)"
              name="minCoveragePercent"
              type="number"
              defaultValue={80}
              hint="Part de la vidéo réellement vue pour valider le module."
            />
          </CreatePanel>
        </div>
      </header>

      <div className="mx-auto max-w-4xl px-6 py-6">
        <div className="mb-6">
          <CourseCoverUpload
            courseId={courseId}
            title={data.course.title}
            coverUrl={data.coverUrl}
          />
        </div>

        <h1 className="text-xl font-semibold tracking-[-0.03em]">{data.course.title}</h1>
        {data.course.goal && <p className="mt-2 text-sm text-ink-2">{data.course.goal}</p>}

        {/* L'état de publication est une action, pas une ligne de fiche : c'est
            le geste qu'on vient faire ici une fois le programme écrit. */}
        <div className="mt-4 flex flex-wrap items-start gap-3">
          <CoursePublish courseId={courseId} published={data.course.published} />
          <CourseForm
            courseId={courseId}
            course={data.course}
            quizzes={quizzes}
            objectifs={typologies.objectifs}
          />
        </div>

        <dl className="mt-5 grid grid-cols-2 gap-x-8 gap-y-3 text-xs sm:grid-cols-3">
          <Line label="Durée" value={`${data.course.durationHours} h`} />
          <Line label="Prix HT" value={`${data.course.priceHT} €`} />
          <Line label="Public visé" value={data.course.audience} />
          <Line label="Prérequis" value={data.course.prerequisites} />
          <Line label="Sanction" value={data.course.sanction} />
          <Line label="Accessibilité" value={data.course.accessibility} />
        </dl>

        {data.course.objectives && data.course.objectives.length > 0 && (
          <section className="mt-6">
            <h2 className="text-xs font-medium">Objectifs pédagogiques</h2>
            <ul className="mt-2 space-y-1.5">
              {data.course.objectives.map((objective) => (
                <li key={objective} className="flex gap-2.5 text-xs text-ink-2">
                  <span className="mt-1.5 size-1 shrink-0 rounded-full bg-accent" />
                  {objective}
                </li>
              ))}
            </ul>
          </section>
        )}

        <h2 className="mt-8 text-xs font-medium">
          Modules{" "}
          <span className="font-mono text-2xs text-ink-3" data-numeric>
            {modules.length}
          </span>
        </h2>

        <div className="mt-2 space-y-px overflow-hidden rounded-xl border border-line bg-line">
          {modules.length === 0 && (
            <p className="bg-surface-1 px-4 py-6 text-center text-xs text-ink-3">
              Aucun module. Une formation sans module ne produit ni assiduité ni
              relevé de connexion.
            </p>
          )}

          {modules.map((module) => (
            <div key={module.id} className="bg-surface-1 px-4 py-3.5">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="min-w-0">
                  <p className="truncate text-sm">
                    <span className="mr-2 font-mono text-2xs text-ink-3">
                      {module.position}
                    </span>
                    {module.title}
                  </p>
                  <p className="mt-0.5 font-mono text-2xs text-ink-3">
                    {Math.round(module.durationMs / 60000)} min ·{" "}
                    {module.assetId ? "vidéo attachée" : "sans vidéo"} ·{" "}
                    {module.quizId ? "questionnaire" : "sans questionnaire"} · seuil{" "}
                    {module.minCoveragePercent} %
                  </p>
                </div>

                <div className="flex flex-wrap items-center gap-2">
                  {/* Le questionnaire se rattache ici, et plus seulement à la
                      création : on découpe la formation d'abord, on écrit les
                      contrôles ensuite. */}
                  <ModuleQuiz
                    courseId={courseId}
                    moduleId={module.id}
                    quizId={module.quizId}
                    quizzes={quizzes}
                  />
                  <VideoUpload
                    courseId={courseId}
                    moduleId={module.id}
                    hasAsset={Boolean(module.assetId)}
                  />
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </>
  );
}

function Line({ label, value }: { label: string; value?: string }) {
  return (
    <div>
      <dt className="text-ink-3">{label}</dt>
      <dd className="mt-0.5 text-ink-2">{value || "—"}</dd>
    </div>
  );
}
