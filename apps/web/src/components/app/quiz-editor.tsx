"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import type { Quiz } from "@/app/(app)/questionnaires/page";

type QuestionType = "single" | "multiple" | "boolean" | "likert" | "numeric" | "text";

interface Draft {
  id: string;
  type: QuestionType;
  prompt: string;
  options: { id: string; label: string }[];
  correct: string[];
  points: number;
  required: boolean;
  explanation: string;
}

const TYPES: { value: QuestionType; label: string }[] = [
  { value: "single", label: "Choix unique" },
  { value: "multiple", label: "Choix multiple" },
  { value: "boolean", label: "Vrai / faux" },
  { value: "likert", label: "Échelle 1 à 5" },
  { value: "numeric", label: "Valeur numérique" },
  { value: "text", label: "Texte libre" },
];

const KIND_OPTIONS = [
  { value: "positioning", label: "Positionnement (entrée)" },
  { value: "post_module", label: "Après module" },
  { value: "intermediate", label: "Intermédiaire" },
  { value: "final", label: "Évaluation finale" },
  { value: "satisfaction_hot", label: "Satisfaction à chaud" },
  { value: "satisfaction_cold", label: "Satisfaction à froid" },
];

/**
 * Éditeur de questionnaire.
 *
 * Une version publiée n'est pas modifiable : l'éditeur propose d'en créer une
 * nouvelle plutôt que de laisser croire à une modification qui sera refusée
 * par l'API. C'est ce qui permet de prouver ce qui a été demandé le jour J.
 */
export function QuizEditor({ quiz }: { quiz?: Quiz }) {
  const router = useRouter();
  const [kind, setKind] = useState(quiz?.kind ?? "post_module");
  const [title, setTitle] = useState(quiz?.title ?? "");
  const [moduleId, setModuleId] = useState(quiz?.moduleId ?? "");
  const [courseId, setCourseId] = useState("");
  const [passPercent, setPassPercent] = useState(quiz?.passPercent ?? 70);
  const [maxAttempts, setMaxAttempts] = useState(quiz?.maxAttempts ?? 3);
  const [questions, setQuestions] = useState<Draft[]>(
    (quiz?.questions as unknown as Draft[])?.map((question) => ({
      ...question,
      options: question.options ?? [],
      correct: question.correct ?? [],
      explanation: question.explanation ?? "",
    })) ?? [],
  );
  const [version, setVersion] = useState(quiz?.version ?? 1);
  const [locked, setLocked] = useState(quiz?.published ?? false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState<string | null>(null);

  const addQuestion = () =>
    setQuestions((current) => [
      ...current,
      {
        id: `q${current.length + 1}`,
        type: "single",
        prompt: "",
        options: [
          { id: "a", label: "" },
          { id: "b", label: "" },
        ],
        correct: [],
        points: 1,
        required: true,
        explanation: "",
      },
    ]);

  const patch = (index: number, change: Partial<Draft>) =>
    setQuestions((current) =>
      current.map((question, i) => (i === index ? { ...question, ...change } : question)),
    );

  const move = (index: number, by: number) =>
    setQuestions((current) => {
      const next = [...current];
      const target = index + by;
      if (target < 0 || target >= next.length) return current;
      [next[index], next[target]] = [next[target], next[index]];
      return next;
    });

  const save = async (asNewVersion: boolean) => {
    setBusy(true);
    setError(null);
    setSaved(null);
    try {
      const response = await fetch("/api/quizzes", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          id: quiz?.id,
          version: asNewVersion ? version + 1 : version,
          kind,
          title,
          moduleId: kind === "post_module" ? moduleId : "",
          courseId: kind === "post_module" ? courseId : "",
          passPercent,
          maxAttempts,
          questions: questions.map((question) => ({
            ...question,
            // Les formes non corrigées n'ont ni corrigé ni barème : les
            // envoyer quand même ferait apparaître un score sur une
            // satisfaction, ce qui n'a aucun sens.
            correct: scorable(question.type) ? question.correct : [],
            points: scorable(question.type) ? question.points : 0,
          })),
        }),
      });
      const body = (await response.json()) as Quiz & { error?: string };
      if (!response.ok) throw new Error(body.error ?? "enregistrement refusé");

      setVersion(body.version);
      setLocked(false);
      setSaved(`Version ${body.version} enregistrée.`);
      if (!quiz) router.replace(`/questionnaires/${body.id}`);
      else router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "enregistrement refusé");
    } finally {
      setBusy(false);
    }
  };

  const publish = async () => {
    if (!quiz) return;
    setBusy(true);
    setError(null);
    try {
      const response = await fetch(`/api/quizzes?id=${quiz.id}&version=${version}`, {
        method: "POST",
      });
      const body = (await response.json()) as { error?: string };
      if (!response.ok) throw new Error(body.error ?? "publication refusée");
      setLocked(true);
      setSaved(`Version ${version} publiée : elle ne se modifie plus.`);
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "publication refusée");
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="surface-card p-5">
      {locked && (
        <p className="mb-4 rounded-lg border border-ok/40 bg-ok/10 px-3 py-2 text-2xs text-ok">
          Cette version est publiée : elle ne se modifie plus. « Enregistrer
          comme nouvelle version » repart de son contenu sans toucher aux
          passations déjà faites.
        </p>
      )}

      <div className="grid gap-3 sm:grid-cols-2">
        <Field label="Intitulé">
          <input
            value={title}
            onChange={(event) => setTitle(event.target.value)}
            placeholder="Contrôle du module 1"
            className="h-9 w-full rounded-lg border border-line bg-surface-0 px-3 text-sm outline-none focus:border-accent"
          />
        </Field>

        <Field label="Usage">
          <select
            value={kind}
            onChange={(event) => setKind(event.target.value)}
            className="h-9 w-full rounded-lg border border-line bg-surface-0 px-2 text-sm outline-none focus:border-accent"
          >
            {KIND_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </Field>

        {kind === "post_module" && (
          <>
            <Field label="Formation (identifiant)">
              <input
                value={courseId}
                onChange={(event) => setCourseId(event.target.value)}
                className="h-9 w-full rounded-lg border border-line bg-surface-0 px-3 font-mono text-xs outline-none focus:border-accent"
              />
            </Field>
            <Field label="Module (identifiant)">
              <input
                value={moduleId}
                onChange={(event) => setModuleId(event.target.value)}
                className="h-9 w-full rounded-lg border border-line bg-surface-0 px-3 font-mono text-xs outline-none focus:border-accent"
              />
            </Field>
          </>
        )}

        <Field label="Seuil de réussite (%)">
          <input
            type="number"
            min={0}
            max={100}
            value={passPercent}
            onChange={(event) => setPassPercent(Number(event.target.value))}
            className="h-9 w-full rounded-lg border border-line bg-surface-0 px-3 text-sm outline-none focus:border-accent"
            data-numeric
          />
        </Field>

        <Field label="Tentatives autorisées (0 = illimité)">
          <input
            type="number"
            min={0}
            value={maxAttempts}
            onChange={(event) => setMaxAttempts(Number(event.target.value))}
            className="h-9 w-full rounded-lg border border-line bg-surface-0 px-3 text-sm outline-none focus:border-accent"
            data-numeric
          />
        </Field>
      </div>

      <div className="mt-6 space-y-3">
        {questions.map((question, index) => (
          <article key={index} className="rounded-lg border border-line p-4">
            <div className="flex items-start gap-3">
              <span className="mt-2 font-mono text-2xs text-ink-3">{index + 1}</span>
              <div className="min-w-0 flex-1 space-y-2.5">
                <input
                  value={question.prompt}
                  onChange={(event) => patch(index, { prompt: event.target.value })}
                  placeholder="Énoncé de la question"
                  className="h-9 w-full rounded-lg border border-line bg-surface-0 px-3 text-sm outline-none focus:border-accent"
                />

                <div className="flex flex-wrap items-center gap-2">
                  <select
                    value={question.type}
                    onChange={(event) =>
                      patch(index, { type: event.target.value as QuestionType })
                    }
                    className="h-8 rounded-md border border-line bg-surface-0 px-2 text-xs outline-none focus:border-accent"
                  >
                    {TYPES.map((type) => (
                      <option key={type.value} value={type.value}>
                        {type.label}
                      </option>
                    ))}
                  </select>

                  {scorable(question.type) && (
                    <label className="flex items-center gap-1.5 text-2xs text-ink-3">
                      points
                      <input
                        type="number"
                        min={0}
                        step={0.5}
                        value={question.points}
                        onChange={(event) => patch(index, { points: Number(event.target.value) })}
                        className="h-8 w-16 rounded-md border border-line bg-surface-0 px-2 text-xs outline-none focus:border-accent"
                        data-numeric
                      />
                    </label>
                  )}

                  <label className="flex items-center gap-1.5 text-2xs text-ink-3">
                    <input
                      type="checkbox"
                      checked={question.required}
                      onChange={(event) => patch(index, { required: event.target.checked })}
                      className="size-3.5 accent-[var(--color-accent)]"
                    />
                    obligatoire
                  </label>

                  <div className="ml-auto flex gap-1">
                    <IconButton label="Monter" onClick={() => move(index, -1)}>
                      ↑
                    </IconButton>
                    <IconButton label="Descendre" onClick={() => move(index, 1)}>
                      ↓
                    </IconButton>
                    <IconButton
                      label="Supprimer"
                      onClick={() =>
                        setQuestions((current) => current.filter((_, i) => i !== index))
                      }
                    >
                      ×
                    </IconButton>
                  </div>
                </div>

                {hasOptions(question.type) && (
                  <div className="space-y-1.5">
                    {question.options.map((option, optionIndex) => (
                      <div key={option.id} className="flex items-center gap-2">
                        <input
                          type={question.type === "multiple" ? "checkbox" : "radio"}
                          name={`correct-${index}`}
                          checked={question.correct.includes(option.id)}
                          onChange={() =>
                            patch(index, {
                              correct:
                                question.type === "multiple"
                                  ? question.correct.includes(option.id)
                                    ? question.correct.filter((id) => id !== option.id)
                                    : [...question.correct, option.id]
                                  : [option.id],
                            })
                          }
                          title="Bonne réponse"
                          className="size-3.5 accent-[var(--color-accent)]"
                        />
                        <input
                          value={option.label}
                          onChange={(event) =>
                            patch(index, {
                              options: question.options.map((entry, i) =>
                                i === optionIndex ? { ...entry, label: event.target.value } : entry,
                              ),
                            })
                          }
                          placeholder={`Option ${optionIndex + 1}`}
                          className="h-8 flex-1 rounded-md border border-line bg-surface-0 px-2.5 text-xs outline-none focus:border-accent"
                        />
                      </div>
                    ))}
                    <button
                      type="button"
                      onClick={() =>
                        patch(index, {
                          options: [
                            ...question.options,
                            {
                              id: String.fromCharCode(97 + question.options.length),
                              label: "",
                            },
                          ],
                        })
                      }
                      className="text-2xs text-ink-3 hover:text-ink"
                    >
                      + option
                    </button>
                  </div>
                )}

                {scorable(question.type) && (
                  <input
                    value={question.explanation}
                    onChange={(event) => patch(index, { explanation: event.target.value })}
                    placeholder="Explication affichée après la réponse"
                    className="h-8 w-full rounded-md border border-line bg-surface-0 px-2.5 text-xs outline-none focus:border-accent"
                  />
                )}
              </div>
            </div>
          </article>
        ))}

        <button
          type="button"
          onClick={addQuestion}
          className="h-9 w-full rounded-lg border border-dashed border-line-strong text-xs text-ink-2 hover:border-accent hover:text-ink"
        >
          + Ajouter une question
        </button>
      </div>

      {error && (
        <p className="mt-4 rounded-lg border border-danger/40 bg-danger/10 px-3 py-2 text-xs text-danger">
          {error}
        </p>
      )}
      {saved && <p className="mt-4 text-2xs text-ok">{saved}</p>}

      <div className="mt-5 flex flex-wrap gap-2">
        <button
          type="button"
          onClick={() => save(locked)}
          disabled={busy || !title || questions.length === 0}
          className="h-9 rounded-lg bg-accent px-4 text-xs font-medium text-white hover:bg-accent-hover disabled:opacity-50"
        >
          {locked ? "Enregistrer comme nouvelle version" : "Enregistrer"}
        </button>
        {quiz && !locked && (
          <button
            type="button"
            onClick={publish}
            disabled={busy}
            className="h-9 rounded-lg border border-line-strong px-4 text-xs font-medium hover:border-accent disabled:opacity-50"
          >
            Publier la version {version}
          </button>
        )}
      </div>
    </section>
  );
}

function scorable(type: QuestionType): boolean {
  return type === "single" || type === "multiple" || type === "boolean" || type === "numeric";
}

function hasOptions(type: QuestionType): boolean {
  return type === "single" || type === "multiple" || type === "boolean";
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1 block text-2xs text-ink-3">{label}</span>
      {children}
    </label>
  );
}

function IconButton({
  label,
  onClick,
  children,
}: {
  label: string;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      title={label}
      className="flex size-7 items-center justify-center rounded-md border border-line text-xs text-ink-3 hover:border-accent hover:text-ink"
    >
      {children}
    </button>
  );
}
