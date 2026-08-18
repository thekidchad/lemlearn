import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ModulePlayer } from "@/components/app/player";
import { QuizRunner, type Questionnaire } from "@/components/app/quiz-runner";
import { apiFetch, ApiError } from "@/lib/api";

export const metadata: Metadata = { title: "Module" };

interface Module {
  id: string;
  title: string;
  summary?: string;
  position: number;
  durationMs: number;
  quizId?: string;
  minCoveragePercent: number;
}

interface Coverage {
  percent: number;
  watchedMs: number;
  coveredMs: number;
  lastPosMs: number;
  sessions: number;
  gaps?: [number, number][];
}

export default async function ModulePage({
  params,
  searchParams,
}: PageProps<"/apprenant/[sessionId]/[courseId]/[moduleId]">) {
  const { sessionId, courseId, moduleId } = await params;
  const query = await searchParams;
  const contactId = typeof query.contactId === "string" ? query.contactId : undefined;
  const suffix = contactId ? `?contactId=${encodeURIComponent(contactId)}` : "";

  let course: { course: { id: string; title: string }; modules: Module[] };
  try {
    course = await apiFetch(`/v1/courses/${courseId}`);
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) notFound();
    throw error;
  }

  const lesson = course.modules.find((entry) => entry.id === moduleId);
  if (!lesson) notFound();

  // Le questionnaire est chargé ici, avec sa tentative : l'apprenant ne doit
  // pas attendre un aller-retour de plus après la vidéo. Une erreur — plus de
  // tentatives, questionnaire dépublié — n'empêche pas d'afficher le module.
  const quiz = lesson.quizId
    ? await apiFetch<{
        questionnaire: Questionnaire;
        attempt: number;
        maxAttempts: number;
      }>(
        `/v1/learn/${sessionId}/courses/${courseId}/quizzes/${lesson.quizId}${suffix}`,
      ).catch(() => null)
    : null;

  const coverage = await apiFetch<Coverage>(
    `/v1/learn/${sessionId}/courses/${courseId}/modules/${moduleId}/progress${suffix}`,
  ).catch<Coverage>(() => ({
    percent: 0,
    watchedMs: 0,
    coveredMs: 0,
    lastPosMs: 0,
    sessions: 0,
    gaps: [],
  }));

  return (
    <>
      <header className="flex h-14 items-center gap-3 border-b border-line px-6">
        <Link href={`/apprenant${suffix}`} className="text-xs text-ink-3 hover:text-ink">
          Mon parcours
        </Link>
        <span className="text-ink-3">/</span>
        <span className="truncate text-xs text-ink-2">{course.course.title}</span>
      </header>

      <div className="mx-auto max-w-3xl px-6 py-6">
        <p className="font-mono text-2xs text-ink-3">Module {lesson.position}</p>
        <h1 className="mt-1 text-xl font-semibold tracking-[-0.03em]">{lesson.title}</h1>
        {lesson.summary && <p className="mt-2 text-xs text-ink-2">{lesson.summary}</p>}

        <div className="mt-6">
          <ModulePlayer
            beatUrl={`/api/learn/${sessionId}/${courseId}/${moduleId}/beat${suffix}`}
            playbackUrl={`/api/learn/${sessionId}/${courseId}/${moduleId}/playback${suffix}`}
            manifestUrl={`/api/learn/${sessionId}/${courseId}/${moduleId}/manifest.m3u8${suffix}`}
            durationMs={lesson.durationMs}
            initial={coverage}
          />
        </div>

        <div className="mt-4 grid grid-cols-3 gap-px overflow-hidden rounded-xl border border-line bg-line">
          <Stat label="couverture" value={`${coverage.percent} %`} />
          <Stat label="requis" value={`${lesson.minCoveragePercent} %`} />
          <Stat label="séances" value={String(coverage.sessions)} />
        </div>

        {coverage.gaps && coverage.gaps.length > 0 && (
          <p className="mt-3 text-2xs text-ink-3">
            {/* Dire *où* il manque quelque chose plutôt que de laisser
                l'apprenant chercher : c'est la différence entre une jauge et
                une aide. */}
            Passages non visionnés :{" "}
            {coverage.gaps
              .slice(0, 5)
              .map(([from, to]) => `${clock(from)}–${clock(to)}`)
              .join(", ")}
          </p>
        )}

        {lesson.quizId && (
          <section className="surface-card mt-6 p-5">
            <h2 className="text-sm font-medium">Questionnaire du module</h2>
            <p className="mt-1.5 text-xs text-ink-2">
              À la fin de la vidéo, répondez au questionnaire. Le corrigé et son
              explication s&apos;affichent après votre réponse — ce contrôle sert à
              apprendre, pas à sanctionner.
            </p>
            <p className="mt-3 text-2xs text-ink-3">
              Le module est validé lorsque la couverture atteint{" "}
              {lesson.minCoveragePercent} % <em>et</em> que le questionnaire est réussi.
            </p>

            {quiz ? (
              <QuizRunner
                endpoint={`/api/learn/${sessionId}/${courseId}/${moduleId}/quiz/${lesson.quizId}${suffix}`}
                quiz={quiz.questionnaire}
                attempt={{ number: quiz.attempt, max: quiz.maxAttempts }}
              />
            ) : (
              <p className="mt-3 text-2xs text-ink-3">
                Ce questionnaire n&apos;est pas disponible pour l&apos;instant —
                tentatives épuisées, ou version non publiée.
              </p>
            )}
          </section>
        )}
      </div>
    </>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-surface-1 px-4 py-3">
      <p className="text-2xs text-ink-3">{label}</p>
      <p className="mt-0.5 text-lg font-semibold tracking-[-0.03em]" data-numeric>
        {value}
      </p>
    </div>
  );
}

function clock(ms: number): string {
  const total = Math.floor(ms / 1000);
  return `${String(Math.floor(total / 60)).padStart(2, "0")}:${String(total % 60).padStart(2, "0")}`;
}
