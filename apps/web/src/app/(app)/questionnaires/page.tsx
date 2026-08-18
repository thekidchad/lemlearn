import type { Metadata } from "next";
import Link from "next/link";
import { apiFetch } from "@/lib/api";

export const metadata: Metadata = { title: "Questionnaires" };

export interface Quiz {
  id: string;
  version: number;
  kind: string;
  title: string;
  moduleId?: string;
  published: boolean;
  passPercent: number;
  maxAttempts: number;
  questions: { id: string; type: string; prompt: string; points: number }[];
}

export const KINDS: Record<string, string> = {
  positioning: "Positionnement",
  post_module: "Après module",
  intermediate: "Intermédiaire",
  final: "Évaluation finale",
  satisfaction_hot: "Satisfaction à chaud",
  satisfaction_cold: "Satisfaction à froid",
};

export default async function QuestionnairesPage() {
  const { quizzes } = await apiFetch<{ quizzes: Quiz[] }>("/v1/quizzes");

  return (
    <>
      <header className="flex h-14 items-center justify-between border-b border-line px-6">
        <h1 className="text-sm font-medium">Questionnaires</h1>
        <Link
          href="/questionnaires/nouveau"
          className="flex h-8 items-center rounded-md bg-accent px-3 text-xs font-medium text-white hover:bg-accent-hover"
        >
          Nouveau
        </Link>
      </header>

      <div className="px-6 py-6">
        {quizzes.length === 0 ? (
          <p className="text-xs text-ink-2">
            Aucun questionnaire. Qualiopi en attend au moins trois : un
            positionnement à l&apos;entrée, une évaluation de sortie, et la
            satisfaction — à chaud puis à froid.
          </p>
        ) : (
          <div className="space-y-px overflow-hidden rounded-xl border border-line bg-line">
            {quizzes.map((quiz) => (
              <Link
                key={quiz.id}
                href={`/questionnaires/${quiz.id}`}
                className="flex items-center justify-between bg-surface-1 px-4 py-3 hover:bg-surface-2"
              >
                <div className="min-w-0">
                  <p className="truncate text-sm">{quiz.title}</p>
                  <p className="mt-0.5 font-mono text-2xs text-ink-3">
                    {KINDS[quiz.kind] ?? quiz.kind} · v{quiz.version} ·{" "}
                    {quiz.questions?.length ?? 0} question
                    {(quiz.questions?.length ?? 0) > 1 ? "s" : ""} · seuil {quiz.passPercent} %
                  </p>
                </div>
                <span
                  className={`rounded-md px-2 py-0.5 text-2xs ${
                    quiz.published
                      ? "bg-ok/15 text-ok"
                      : "border border-line-strong text-ink-3"
                  }`}
                >
                  {quiz.published ? "publié" : "brouillon"}
                </span>
              </Link>
            ))}
          </div>
        )}

        <p className="mt-4 text-2xs text-ink-3">
          {/* Le versionnement n'est pas un détail technique : c'est ce qui
              permet de prouver ce qui a été demandé le jour J. */}
          Publier fige une version. Une version publiée ne se modifie plus : on
          en crée une nouvelle, et les passations en cours restent rattachées à
          celle qu&apos;elles ont passée.
        </p>
      </div>
    </>
  );
}
