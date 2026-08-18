import type { Metadata } from "next";
import Link from "next/link";
import { SignUpForm } from "@/components/app/sign-up-form";
import { Logo } from "@/components/brand/logo";

export const metadata: Metadata = {
  title: "Créer un compte",
  description:
    "Trente jours pour monter un dossier de formation complet, de la première prise de contact à l'attestation.",
};

export default async function InscriptionPage({ searchParams }: PageProps<"/inscription">) {
  const query = await searchParams;
  const plan = typeof query.plan === "string" ? query.plan : undefined;

  return (
    <main className="mx-auto flex min-h-dvh max-w-md flex-col justify-center px-5 py-10">
      <Link href="/" aria-label="lemlearn, accueil" className="mb-8">
        <Logo />
      </Link>

      <h1 className="text-2xl font-semibold tracking-[-0.03em]">
        Créer votre organisme
      </h1>
      <p className="mt-2 text-sm text-ink-2">
        Trente jours d&apos;essai, sans carte bancaire. De quoi monter un dossier
        complet — devis, convention signée, assiduité, attestation — et vérifier
        qu&apos;il tient devant un contrôle.
      </p>

      <SignUpForm plan={plan} />

      <p className="mt-6 text-2xs text-ink-3">
        Vous avez déjà un compte ?{" "}
        <Link href="/connexion" className="text-ink-2 underline hover:text-ink">
          Se connecter
        </Link>
      </p>
    </main>
  );
}
