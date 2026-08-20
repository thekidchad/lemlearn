import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";
import { Logo } from "@/components/brand/logo";
import { SignInForm } from "@/components/app/sign-in-form";
import { hasSession } from "@/lib/api";

export const metadata: Metadata = { title: "Connexion" };

export default async function SignInPage() {
  // Une session déjà ouverte n'a pas à repasser par la connexion.
  if (await hasSession()) {
    redirect("/pipeline");
  }

  return (
    <div className="relative flex min-h-full items-center justify-center px-6 py-16">
      <div
        aria-hidden
        className="glow-accent pointer-events-none absolute inset-x-0 top-0 h-80 opacity-50"
      />

      <div className="relative w-full max-w-sm">
        <Link href="/" className="inline-block">
          <Logo />
        </Link>

        <h1 className="mt-8 text-2xl font-semibold tracking-[-0.035em]">Connexion</h1>
        <p className="mt-1.5 text-xs text-ink-2">
          Accédez à vos dossiers, vos sessions et vos preuves.
        </p>

        <SignInForm />

        <p className="mt-6 text-2xs text-ink-3">
          Pas encore de compte ?{" "}
          <Link href="/inscription" className="text-accent-ink underline-offset-4 hover:underline">
            Créer votre organisme
          </Link>
        </p>
      </div>
    </div>
  );
}
