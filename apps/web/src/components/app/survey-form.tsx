"use client";

import { useState } from "react";
import { QuestionField, type Question } from "@/components/app/question-field";

export type { Question };

export interface Questionnaire {
  id: string;
  title: string;
  questions: Question[];
}

interface Answer {
  values: string[];
  changeCount: number;
  openedAt: number;
}

/**
 * Passation d'un questionnaire.
 *
 * Le formulaire mesure aussi le temps passé et le nombre de changements d'avis
 * par question. Ce n'est pas de la statistique décorative : c'est ce qui
 * distingue un apprenant qui réfléchit d'un questionnaire cliqué en huit
 * secondes, et cette distinction figure au dossier.
 */
export function SurveyForm({
  questionnaire,
  action,
}: {
  questionnaire: Questionnaire;
  action: string;
}) {
  const [answers, setAnswers] = useState<Record<string, Answer>>({});
  const [sending, setSending] = useState(false);
  const [done, setDone] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const set = (question: Question, values: string[]) => {
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
  };

  const submit = async () => {
    const missing = questionnaire.questions.find(
      (question) => question.required && !(answers[question.id]?.values.length),
    );
    if (missing) {
      setError("Une question obligatoire est restée sans réponse.");
      return;
    }

    setSending(true);
    setError(null);
    try {
      const response = await fetch(action, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          answers: questionnaire.questions.map((question) => {
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
      const body = (await response.json()) as { error?: string };
      if (!response.ok) throw new Error(body.error ?? "envoi impossible");
      setDone(true);
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "envoi impossible");
    } finally {
      setSending(false);
    }
  };

  if (done) {
    return (
      <p className="surface-card mt-8 p-5 text-sm text-ink-2">
        Merci. Vos réponses sont enregistrées et rejoignent votre dossier de
        formation.
      </p>
    );
  }

  return (
    <div className="mt-8 space-y-4">
      {questionnaire.questions.map((question, index) => (
        <fieldset key={question.id} className="surface-card p-5">
          <legend className="sr-only">{question.prompt}</legend>
          <p className="text-sm font-medium">
            <span className="mr-2 font-mono text-2xs text-ink-3">{index + 1}</span>
            {question.prompt}
            {question.required && <span className="ml-1 text-danger">*</span>}
          </p>

          <div className="mt-3.5">
            <QuestionField
              question={question}
              values={answers[question.id]?.values ?? []}
              onChange={(values) => set(question, values)}
            />
          </div>
        </fieldset>
      ))}

      {error && (
        <p className="rounded-lg border border-danger/40 bg-danger/10 px-3 py-2 text-xs text-danger">
          {error}
        </p>
      )}

      <button
        type="button"
        onClick={submit}
        disabled={sending}
        className="h-11 w-full rounded-lg bg-accent text-sm font-medium text-white transition-colors duration-[120ms] hover:bg-accent-hover disabled:opacity-60"
      >
        {sending ? "Envoi…" : "Envoyer mes réponses"}
      </button>
    </div>
  );
}
