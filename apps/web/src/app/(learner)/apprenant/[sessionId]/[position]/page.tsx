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

interface SessionView {
  course: { id: string; title: string };
  modules: Module[] | null;
}

/**
 * Un module, en plein écran.
 *
 * L'adresse ne porte que la session et le numéro du module : une session n'a
 * qu'une formation, et l'apprenant qui lit son URL doit y reconnaître quelque
 * chose. La formation se résout côté serveur.
 */
export default async function ModulePage({
  params,
  searchParams,
}: PageProps<"/apprenant/[sessionId]/[position]">) {
  const { sessionId, position } = await params;
  const query = await searchParams;
  const contactId = typeof query.contactId === "string" ? query.contactId : undefined;
  const suffix = contactId ? `?contactId=${encodeURIComponent(contactId)}` : "";

  let view: SessionView;
  try {
    view = await apiFetch<SessionView>(`/v1/learn/${sessionId}${suffix}`);
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) notFound();
    throw error;
  }

  const modules = view.modules ?? [];
  const lesson = modules.find((entry) => String(entry.position) === position);
  if (!lesson) notFound();

  const courseId = view.course.id;
  const moduleId = lesson.id;

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

  const suivant = modules.find((entry) => entry.position === lesson.position + 1);

  return (
    <div className="mx-auto max-w-3xl px-5 py-10 sm:px-8 sm:py-12">
      <Link
        href={`/apprenant${suffix}`}
        className="text-sm text-ink-3 transition-colors duration-[120ms] hover:text-ink"
      >
        ← Mon parcours
      </Link>

      <p className="eyebrow mt-8">
        {view.course.title} · module {lesson.position} sur {modules.length}
      </p>
      <h1 className="learner-title mt-2">{lesson.title}</h1>
      {lesson.summary && <p className="learner-body mt-3">{lesson.summary}</p>}

      <div className="mt-8">
        <ModulePlayer
          beatUrl={`/api/learn/${sessionId}/${courseId}/${moduleId}/beat${suffix}`}
          playbackUrl={`/api/learn/${sessionId}/${courseId}/${moduleId}/playback${suffix}`}
          manifestUrl={`/api/learn/${sessionId}/${courseId}/${moduleId}/manifest.m3u8${suffix}`}
          durationMs={lesson.durationMs}
          initial={coverage}
        />
      </div>

      {/* Ce que l'apprenant doit savoir de sa progression tient en une phrase.
          Le détail — trois compteurs côte à côte — est un réflexe de tableau
          de bord : ici, seul l'écart au seuil a une conséquence pour lui. */}
      <p className="mt-4 text-sm text-ink-2" data-numeric>
        Vous avez vu {coverage.percent} % de cette vidéo
        {coverage.percent >= lesson.minCoveragePercent
          ? " — le seuil d'assiduité est atteint."
          : `, il en faut ${lesson.minCoveragePercent} % pour valider le module.`}
      </p>

      {coverage.gaps && coverage.gaps.length > 0 && (
        <p className="mt-2 text-sm text-ink-3">
          {/* Dire *où* il manque quelque chose plutôt que de laisser chercher :
              c'est la différence entre une jauge et une aide. */}
          Passages non vus :{" "}
          {coverage.gaps
            .slice(0, 5)
            .map(([from, to]) => `${clock(from)}–${clock(to)}`)
            .join(", ")}
        </p>
      )}

      {lesson.quizId && (
        <section className="surface-card mt-8 p-5 sm:p-6">
          <h2 className="learner-heading">Questionnaire du module</h2>
          <p className="learner-body mt-2 text-[0.9375rem]">
            Répondez après la vidéo. Le corrigé s&apos;affiche aussitôt : ce
            contrôle sert à apprendre, pas à sanctionner.
          </p>

          {quiz ? (
            <QuizRunner
              endpoint={`/api/learn/${sessionId}/${courseId}/${moduleId}/quiz/${lesson.quizId}${suffix}`}
              quiz={quiz.questionnaire}
              attempt={{ number: quiz.attempt, max: quiz.maxAttempts }}
            />
          ) : (
            <p className="mt-3 text-sm text-ink-3">
              Ce questionnaire n&apos;est pas disponible pour l&apos;instant —
              tentatives épuisées, ou version non publiée.
            </p>
          )}
        </section>
      )}

      {suivant && (
        <Link
          href={`/apprenant/${sessionId}/${suivant.position}${suffix}`}
          className="mt-8 flex items-center gap-4 rounded-xl border border-line px-5 py-4 transition-colors duration-[120ms] hover:border-line-strong hover:bg-surface-2/60"
        >
          <span className="min-w-0 flex-1">
            <span className="block text-xs text-ink-3">Module suivant</span>
            <span className="mt-0.5 block truncate text-base font-medium">
              {suivant.title}
            </span>
          </span>
          <span aria-hidden className="text-lg text-ink-3">
            →
          </span>
        </Link>
      )}
    </div>
  );
}

function clock(ms: number): string {
  const total = Math.floor(ms / 1000);
  return `${String(Math.floor(total / 60)).padStart(2, "0")}:${String(total % 60).padStart(2, "0")}`;
}
