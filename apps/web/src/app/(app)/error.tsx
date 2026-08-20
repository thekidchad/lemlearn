"use client";

import Link from "next/link";
import { useEffect } from "react";

/**
 * Écran d'erreur de l'espace de l'organisme.
 *
 * Il ne cherche pas à deviner la cause dans le message : React efface les
 * messages d'erreur dans un build de production, et un écran qui s'appuie
 * dessus dit juste en production le contraire de ce qu'il disait en
 * développement. Les cas prévisibles — un apprenant sur un écran de
 * l'organisme — sont écartés à la porte par la coque, pas rattrapés ici.
 */
export default function AppError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    // L'empreinte permet de retrouver la trace côté serveur ; le message,
    // lui, n'existe plus à ce stade en production.
    console.error(error);
  }, [error]);

  return (
    <main className="mx-auto flex min-h-dvh max-w-md flex-col justify-center px-6">
      <h1 className="text-lg font-semibold tracking-[-0.03em]">
        Cet écran n&apos;a pas pu s&apos;afficher
      </h1>
      <p className="mt-2 text-sm text-ink-2">
        Le service n&apos;a pas répondu comme prévu. Réessayer suffit le plus
        souvent.
      </p>
      {error.digest && (
        <p className="mt-3 font-mono text-2xs text-ink-3">
          Référence de l&apos;incident : {error.digest}
        </p>
      )}

      <div className="mt-6 flex flex-wrap gap-2">
        <button
          type="button"
          onClick={reset}
          className="h-9 rounded-lg bg-accent px-4 text-xs font-medium text-white hover:bg-accent-hover"
        >
          Réessayer
        </button>
        <Link
          href="/pipeline"
          className="flex h-9 items-center rounded-lg border border-line px-4 text-xs text-ink-2 hover:border-accent hover:text-ink"
        >
          Revenir au pipeline
        </Link>
      </div>
    </main>
  );
}
