"use client";

import Link from "next/link";
import { useEffect } from "react";

/**
 * Écran d'erreur de l'espace apprenant.
 *
 * Sans lui, Next affichait son propre message — « This page couldn't load », en
 * anglais, au milieu d'un produit qui parle français, et sans rien proposer.
 * Un apprenant qui tombe dessus n'a aucune idée de ce qu'il doit faire.
 *
 * Le message n'essaie pas de deviner la cause : React efface les messages
 * d'erreur dans un build de production, et un écran qui s'appuie dessus dirait
 * en production le contraire de ce qu'il disait en développement.
 */
export default function LearnerError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <main className="mx-auto flex min-h-dvh max-w-lg flex-col justify-center px-5 py-12 sm:px-8">
      <h1 className="learner-title">Cet écran n&apos;a pas pu s&apos;afficher</h1>
      <p className="learner-body mt-3">
        Le service n&apos;a pas répondu comme prévu. Réessayer suffit le plus
        souvent ; si cela recommence, votre organisme de formation peut nous le
        signaler.
      </p>
      {error.digest && (
        <p className="mt-3 font-mono text-2xs text-ink-3">
          Référence de l&apos;incident : {error.digest}
        </p>
      )}

      <div className="mt-8 flex flex-wrap gap-3">
        <button
          type="button"
          onClick={reset}
          className="inline-flex h-11 items-center rounded-xl bg-accent px-5 text-sm font-medium text-white hover:bg-accent-hover"
        >
          Réessayer
        </button>
        <Link
          href="/apprenant"
          className="inline-flex h-11 items-center rounded-xl border border-line px-5 text-sm hover:border-accent"
        >
          Revenir à mon parcours
        </Link>
      </div>
    </main>
  );
}
