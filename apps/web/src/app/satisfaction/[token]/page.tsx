import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { SurveyForm, type Questionnaire } from "@/components/app/survey-form";
import { apiFetch, ApiError, type Brand } from "@/lib/api";
import { OrgBrand, brandStyle } from "@/components/brand/org-brand";

export const metadata: Metadata = {
  title: "Votre avis sur la formation",
  // Un lien de questionnaire n'a rien à faire dans un index : il vaut jeton
  // d'accès.
  robots: { index: false, follow: false },
};

interface Survey {
  brand: Brand;
  questionnaire: Questionnaire;
  learner: string;
  course: string;
  answered: boolean;
}

export default async function SatisfactionPage({
  params,
}: PageProps<"/satisfaction/[token]">) {
  const { token } = await params;

  let survey: Survey;
  try {
    survey = await apiFetch<Survey>(`/v1/satisfaction/${encodeURIComponent(token)}`);
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) notFound();
    throw error;
  }

  return (
    <main className="mx-auto min-h-dvh max-w-xl px-5 py-10" style={brandStyle(survey.brand)}>
      <div className="mb-8">
        <OrgBrand brand={survey.brand} />
      </div>
      <p className="font-mono text-2xs tracking-wide text-ink-3 uppercase">
        Satisfaction à trois mois
      </p>
      <h1 className="mt-2 text-2xl font-semibold tracking-[-0.03em]">{survey.course}</h1>
      <p className="mt-3 text-sm text-ink-2">
        {survey.learner}, vous avez suivi cette formation il y a trois mois. Nous
        vous sollicitons maintenant, et pas à chaud, parce que c&apos;est avec ce
        recul qu&apos;on sait ce qui a réellement servi.
      </p>

      {survey.answered ? (
        <p className="surface-card mt-8 p-5 text-sm text-ink-2">
          Vous avez déjà répondu à ce questionnaire. Merci — vos réponses sont
          conservées avec votre dossier de formation.
        </p>
      ) : (
        <SurveyForm
          questionnaire={survey.questionnaire}
          action={`/api/satisfaction/${encodeURIComponent(token)}`}
        />
      )}

      <p className="mt-8 text-2xs text-ink-3">
        Vos réponses sont conservées avec votre dossier de formation et peuvent
        être consultées lors d&apos;un contrôle de l&apos;organisme. Elles ne sont
        transmises à personne d&apos;autre.
      </p>
    </main>
  );
}
