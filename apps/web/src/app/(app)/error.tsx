"use client";

import Link from "next/link";
import { useEffect } from "react";

/**
 * Écran d'erreur de l'application.
 *
 * Il existe surtout pour un cas précis : un apprenant qui tape l'adresse d'un
 * écran de l'organisme. L'API lui répond 403, à juste titre — mais un « une
 * erreur est survenue » le laisse croire à une panne, alors qu'il n'y a rien
 * de cassé. Le dire proprement vaut mieux que de masquer les liens et
 * d'espérer que personne n'essaie.
 */
export default function AppError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    // Le détail part dans la console du navigateur, pas à l'écran : un
    // message d'API brut n'aide pas celui qui le lit.
    console.error(error);
  }, [error]);

  const forbidden = /403|réservé|interdit|inscription/i.test(error.message);

  return (
    <main className="mx-auto flex min-h-dvh max-w-md flex-col justify-center px-6">
      <h1 className="text-lg font-semibold tracking-[-0.03em]">
        {forbidden ? "Cet écran n'est pas pour vous" : "Cet écran n'a pas pu s'afficher"}
      </h1>
      <p className="mt-2 text-sm text-ink-2">
        {forbidden
          ? "Il est réservé à l'équipe de l'organisme de formation. Votre espace, lui, regroupe vos modules, vos questionnaires et votre progression."
          : "Le service n'a pas répondu comme prévu. Réessayer suffit le plus souvent ; si cela persiste, prévenez votre organisme."}
      </p>

      <div className="mt-6 flex flex-wrap gap-2">
        <Link
          href="/apprenant"
          className="flex h-9 items-center rounded-lg bg-accent px-4 text-xs font-medium text-white hover:bg-accent-hover"
        >
          Aller à mon espace
        </Link>
        {!forbidden && (
          <button
            type="button"
            onClick={reset}
            className="h-9 rounded-lg border border-line px-4 text-xs text-ink-2 hover:border-accent hover:text-ink"
          >
            Réessayer
          </button>
        )}
      </div>
    </main>
  );
}
