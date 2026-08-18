import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { QuizEditor } from "@/components/app/quiz-editor";
import { QuizResults } from "@/components/app/quiz-results";
import { apiFetch, ApiError } from "@/lib/api";
import { KINDS, type Quiz } from "@/app/(app)/questionnaires/page";

export const metadata: Metadata = { title: "Questionnaire" };

export interface Results {
  submitted: number;
  passed: number;
  averageDurationMs: number;
  questions: {
    questionId: string;
    prompt: string;
    answered: number;
    correct: number;
    choices?: Record<string, number>;
  }[];
  attempts: {
    id: string;
    enrollmentId: string;
    number: number;
    version: number;
    score: number;
    maxScore: number;
    passed: boolean;
    durationMs: number;
    submittedAt: string;
  }[];
}

export default async function QuizPage({ params }: PageProps<"/questionnaires/[quizId]">) {
  const { quizId } = await params;

  if (quizId === "nouveau") {
    return (
      <>
        <Header title="Nouveau questionnaire" />
        <div className="px-6 py-6">
          <QuizEditor />
        </div>
      </>
    );
  }

  let versions: Quiz[];
  try {
    ({ versions } = await apiFetch<{ versions: Quiz[] }>(`/v1/quizzes/${quizId}`));
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) notFound();
    throw error;
  }

  const latest = versions[0];
  const results = await apiFetch<Results>(`/v1/quizzes/${quizId}/resultats`).catch(() => null);

  return (
    <>
      <Header title={latest.title} subtitle={`${KINDS[latest.kind] ?? latest.kind} · v${latest.version}`} />

      <div className="space-y-6 px-6 py-6">
        <QuizEditor quiz={latest} />

        {versions.length > 1 && (
          <section className="surface-card p-5">
            <h2 className="text-sm font-medium">Versions</h2>
            <p className="mt-1.5 text-xs text-ink-2">
              Une passation reste rattachée à la version qu&apos;elle a passée :
              c&apos;est ce qui permet de réimprimer une copie telle qu&apos;elle a
              été présentée.
            </p>
            <ul className="mt-3 space-y-1.5 font-mono text-2xs text-ink-3">
              {versions.map((version) => (
                <li key={version.version}>
                  v{version.version} · {version.questions?.length ?? 0} questions ·{" "}
                  {version.published ? "publiée" : "brouillon"}
                </li>
              ))}
            </ul>
          </section>
        )}

        {results && results.submitted > 0 && <QuizResults results={results} />}
      </div>
    </>
  );
}

function Header({ title, subtitle }: { title: string; subtitle?: string }) {
  return (
    <header className="flex h-14 items-center gap-3 border-b border-line px-6">
      <Link href="/questionnaires" className="text-xs text-ink-3 hover:text-ink">
        Questionnaires
      </Link>
      <span className="text-ink-3">/</span>
      <span className="truncate text-xs text-ink-2">{title}</span>
      {subtitle && <span className="ml-auto font-mono text-2xs text-ink-3">{subtitle}</span>}
    </header>
  );
}
