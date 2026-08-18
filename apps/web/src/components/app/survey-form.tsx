"use client";

import { useState } from "react";

export interface Question {
  id: string;
  type: "single" | "multiple" | "boolean" | "likert" | "numeric" | "text";
  prompt: string;
  options?: { id: string; label: string }[];
  required: boolean;
}

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

  const toggle = (question: Question, optionId: string) => {
    const current = answers[question.id]?.values ?? [];
    set(
      question,
      current.includes(optionId)
        ? current.filter((value) => value !== optionId)
        : [...current, optionId],
    );
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
            {(question.type === "single" || question.type === "boolean") &&
              question.options?.map((option) => (
                <Choice
                  key={option.id}
                  name={question.id}
                  label={option.label}
                  type="radio"
                  checked={answers[question.id]?.values.includes(option.id) ?? false}
                  onChange={() => set(question, [option.id])}
                />
              ))}

            {question.type === "multiple" &&
              question.options?.map((option) => (
                <Choice
                  key={option.id}
                  name={question.id}
                  label={option.label}
                  type="checkbox"
                  checked={answers[question.id]?.values.includes(option.id) ?? false}
                  onChange={() => toggle(question, option.id)}
                />
              ))}

            {question.type === "likert" && (
              <div className="flex gap-1.5">
                {[1, 2, 3, 4, 5].map((value) => {
                  const selected = answers[question.id]?.values[0] === String(value);
                  return (
                    <button
                      key={value}
                      type="button"
                      onClick={() => set(question, [String(value)])}
                      aria-pressed={selected}
                      className={`h-10 flex-1 rounded-lg border text-sm transition-colors duration-[120ms] ${
                        selected
                          ? "border-accent bg-accent/15 text-ink"
                          : "border-line text-ink-2 hover:border-line-strong"
                      }`}
                      data-numeric
                    >
                      {value}
                    </button>
                  );
                })}
              </div>
            )}

            {question.type === "numeric" && (
              <input
                type="number"
                inputMode="decimal"
                className="h-10 w-40 rounded-lg border border-line bg-surface-0 px-3 text-sm outline-none focus:border-accent"
                onChange={(event) => set(question, [event.target.value])}
                data-numeric
              />
            )}

            {question.type === "text" && (
              <textarea
                rows={3}
                className="w-full rounded-lg border border-line bg-surface-0 px-3 py-2 text-sm outline-none focus:border-accent"
                placeholder="Votre réponse"
                onChange={(event) => set(question, [event.target.value])}
              />
            )}
          </div>

          {question.type === "likert" && (
            <p className="mt-2 flex justify-between text-2xs text-ink-3">
              <span>Pas du tout</span>
              <span>Tout à fait</span>
            </p>
          )}
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

function Choice({
  name,
  label,
  type,
  checked,
  onChange,
}: {
  name: string;
  label: string;
  type: "radio" | "checkbox";
  checked: boolean;
  onChange: () => void;
}) {
  return (
    <label className="flex cursor-pointer items-center gap-2.5 rounded-lg px-1 py-2 text-sm text-ink-2 hover:text-ink">
      <input
        type={type}
        name={name}
        checked={checked}
        onChange={onChange}
        className="size-4 accent-[var(--color-accent)]"
      />
      {label}
    </label>
  );
}
