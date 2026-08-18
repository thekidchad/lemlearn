"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { QuestionField, type Question } from "@/components/app/question-field";

export interface Questionnaire {
  id: string;
  title: string;
  kind: string;
  questions: Question[];
  passPercent: number;
}

interface Answer {
  values: string[];
  changeCount: number;
  openedAt: number;
}

interface Graded {
  attempt: { answers: { questionId: string; isCorrect: boolean; points: number }[] };
  percent: number;
  passed: boolean;
  questionnaire: Questionnaire;
}

/**
 * Contrôle après module.
 *
 * Le corrigé et son explication s'affichent après la soumission, jamais avant :
 * ce contrôle sert à apprendre. Le temps passé et le nombre de changements
 * d'avis partent avec les réponses — ils ne notent rien, mais ils distinguent
 * un apprenant qui réfléchit d'un questionnaire cliqué en huit secondes, et
 * cette distinction figure au dossier.
 */
export function QuizRunner({
  endpoint,
  quiz,
  attempt,
}: {
  endpoint: string;
  // Le questionnaire est chargé par le serveur : l'apprenant ne doit pas voir
  // un écran vide le temps d'un aller-retour, et le corrigé n'a de toute façon
  // pas à transiter avant la soumission.
  quiz: Questionnaire;
  attempt: { number: number; max: number };
}) {
  const router = useRouter();
  const [answers, setAnswers] = useState<Record<string, Answer>>({});
  const [result, setResult] = useState<Graded | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const set = (question: Question, values: string[]) =>
    setAnswers((current) => {
      const previous = current[question.id];
      return {
        ...current,
        [question.id]: {
          values,
          changeCount: previous ? previous.changeCount + 1 : 0,
          openedAt: previous?.openedAt ?? Date.now(),
        },
      };
    });

  const submit = async () => {
    const missing = quiz.questions.find(
      (question) => question.required && !answers[question.id]?.values.length,
    );
    if (missing) {
      setError("Une question obligatoire est restée sans réponse.");
      return;
    }

    setBusy(true);
    setError(null);
    try {
      const response = await fetch(endpoint, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          answers: quiz.questions.map((question) => {
            const answer = answers[question.id];
            return {
              questionId: question.id,
              values: answer?.values ?? [],
              timeSpentMs: answer ? Date.now() - answer.openedAt : 0,
              changeCount: answer?.changeCount ?? 0,
              answeredAt: new Date().toISOString(),
            };
          }),
        }),
      });
      const body = (await response.json()) as Graded & { error?: string };
      if (!response.ok) throw new Error(body.error ?? "soumission refusée");
      setResult(body);
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "soumission refusée");
    } finally {
      setBusy(false);
    }
  };

  const corrected = new Map(
    (result?.attempt.answers ?? []).map((answer) => [answer.questionId, answer]),
  );

  return (
    <div className="mt-4">
      {result && (
        <div
          className={`mb-4 rounded-lg border px-3 py-2.5 text-xs ${
            result.passed
              ? "border-ok/40 bg-ok/10 text-ok"
              : "border-warn/40 bg-warn/10 text-warn"
          }`}
        >
          <p data-numeric>
            {Math.round(result.percent)} % — {result.passed ? "réussi" : "non atteint"} (seuil{" "}
            {quiz.passPercent} %)
          </p>
          {!result.passed && attempt.max > 0 && attempt.number < attempt.max && (
            <button
              type="button"
              onClick={() => {
                setAnswers({});
                setResult(null);
                router.refresh();
              }}
              className="mt-1.5 underline"
            >
              Recommencer ({attempt.max - attempt.number} tentative
              {attempt.max - attempt.number > 1 ? "s" : ""} restante
              {attempt.max - attempt.number > 1 ? "s" : ""})
            </button>
          )}
        </div>
      )}

      <div className="space-y-3">
        {quiz.questions.map((question, index) => {
          const correction = corrected.get(question.id);
          return (
            <fieldset
              key={question.id}
              className={`rounded-lg border p-4 ${
                correction === undefined
                  ? "border-line"
                  : correction.isCorrect
                    ? "border-ok/40"
                    : "border-danger/40"
              }`}
            >
              <legend className="sr-only">{question.prompt}</legend>
              <p className="text-sm">
                <span className="mr-2 font-mono text-2xs text-ink-3">{index + 1}</span>
                {question.prompt}
                {question.required && <span className="ml-1 text-danger">*</span>}
              </p>

              <div className="mt-3">
                <QuestionField
                  question={question}
                  values={answers[question.id]?.values ?? []}
                  onChange={(values) => set(question, values)}
                  disabled={result !== null}
                />
              </div>

              {correction && (
                <p className="mt-3 border-t border-line pt-2.5 text-2xs text-ink-2">
                  <span className={correction.isCorrect ? "text-ok" : "text-danger"}>
                    {correction.isCorrect ? "Bonne réponse" : "Réponse incorrecte"}
                  </span>
                  {result?.questionnaire.questions
                    .find((entry) => entry.id === question.id)
                    ?.explanation && (
                    <>
                      {" — "}
                      {
                        result.questionnaire.questions.find(
                          (entry) => entry.id === question.id,
                        )?.explanation
                      }
                    </>
                  )}
                </p>
              )}
            </fieldset>
          );
        })}
      </div>

      {error && (
        <p className="mt-3 rounded-lg border border-danger/40 bg-danger/10 px-3 py-2 text-xs text-danger">
          {error}
        </p>
      )}

      {!result && (
        <button
          type="button"
          onClick={submit}
          disabled={busy}
          className="mt-4 h-10 w-full rounded-lg bg-accent text-sm font-medium text-white transition-colors duration-[120ms] hover:bg-accent-hover disabled:opacity-60"
        >
          {busy ? "Correction…" : "Valider mes réponses"}
        </button>
      )}
    </div>
  );
}
