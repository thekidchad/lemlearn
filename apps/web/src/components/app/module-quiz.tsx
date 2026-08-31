"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

/**
 * Rattacher un questionnaire à un module déjà créé.
 *
 * Le choix existait au moment de créer le module, et nulle part après : un
 * module ajouté avant que le questionnaire n'existe restait sans contrôle pour
 * toujours. Or c'est l'ordre naturel — on découpe la formation, puis on écrit
 * les questionnaires.
 *
 * Ce n'est pas cosmétique : un module n'est validé que si la vidéo a été vue
 * *et* le contrôle réussi. Sans questionnaire rattaché, il ne reste que
 * l'assiduité, et c'est le couplage des deux qui fait la solidité d'un dossier.
 */
interface Quiz {
  id: string;
  title: string;
  kind: string;
  published: boolean;
}

export function ModuleQuiz({
  courseId,
  moduleId,
  quizId,
  quizzes,
}: {
  courseId: string;
  moduleId: string;
  quizId?: string;
  quizzes: Quiz[];
}) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const choisir = async (valeur: string) => {
    setError(null);
    setBusy(true);
    try {
      const response = await fetch(`/api/courses/${courseId}/modules/${moduleId}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ quizId: valeur }),
      });
      const body = (await response.json()) as { error?: string };
      if (!response.ok) throw new Error(body.error ?? "rattachement refusé");
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "rattachement refusé");
    } finally {
      setBusy(false);
    }
  };

  const disponibles = quizzes.filter((quiz) => quiz.kind === "post_module" && quiz.published);

  return (
    <div className="flex items-center gap-2">
      <select
        aria-label="Questionnaire de fin de module"
        defaultValue={quizId ?? ""}
        disabled={busy || disponibles.length === 0}
        onChange={(event) => choisir(event.target.value)}
        className="field h-8 w-52 text-xs"
      >
        <option value="">Sans questionnaire</option>
        {disponibles.map((quiz) => (
          <option key={quiz.id} value={quiz.id}>
            {quiz.title}
          </option>
        ))}
      </select>
      {error && <span className="text-2xs text-danger">{error}</span>}
    </div>
  );
}
