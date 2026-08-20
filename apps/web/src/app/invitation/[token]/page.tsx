import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { AcceptInvitation } from "@/components/app/accept-invitation";
import { Logo } from "@/components/brand/logo";
import { apiFetch, ApiError } from "@/lib/api";

export const metadata: Metadata = {
  title: "Votre espace de formation",
  // Un lien d'invitation vaut jeton d'accès : il n'a rien à faire dans un
  // index.
  robots: { index: false, follow: false },
};

interface Invitation {
  email: string;
  org: string;
  expiresAt: string;
}

export default async function InvitationPage({ params }: PageProps<"/invitation/[token]">) {
  const { token } = await params;

  let invitation: Invitation;
  try {
    invitation = await apiFetch<Invitation>(`/v1/invitation/${encodeURIComponent(token)}`);
  } catch (error) {
    if (!(error instanceof ApiError)) throw error;
    if (error.status === 404 && !error.message) notFound();

    return (
      <main className="mx-auto flex min-h-dvh max-w-md flex-col justify-center px-5">
        <h1 className="text-lg font-semibold tracking-[-0.03em]">
          Ce lien n&apos;est plus utilisable
        </h1>
        <p className="mt-2 text-sm text-ink-2">{error.message}</p>
        <p className="mt-4 text-2xs text-ink-3">
          Un lien d&apos;invitation est personnel et à usage unique. Demandez-en un
          nouveau à votre organisme de formation.
        </p>
      </main>
    );
  }

  return (
    <main className="mx-auto flex min-h-dvh max-w-md flex-col justify-center px-5 py-10">
      <Logo />

      <h1 className="mt-8 text-2xl font-semibold tracking-[-0.03em]">
        Votre espace de formation
      </h1>
      <p className="mt-2 text-sm text-ink-2">
        <strong>{invitation.org}</strong> vous ouvre l&apos;accès à vos modules,
        vos questionnaires et votre progression. Choisissez un mot de passe pour
        entrer — le compte est au nom de {invitation.email}.
      </p>

      <AcceptInvitation token={token} />

      <p className="mt-6 text-2xs text-ink-3">
        Votre temps de visionnage est enregistré : il constitue la preuve
        d&apos;assiduité que votre organisme doit pouvoir présenter en cas de
        contrôle. Vous pouvez à tout moment demander l&apos;export ou
        l&apos;effacement de vos données.
      </p>
    </main>
  );
}
