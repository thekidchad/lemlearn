import Link from "next/link";
import type { ReactNode } from "react";

/**
 * La coque du journal.
 *
 * Deux registres sous un seul nom, parce qu'on ne sait pas d'avance dans lequel
 * se trouve la réponse : « il dit n'avoir rien reçu » se règle dans les
 * courriels, « qui est entré sur ce compte » dans l'activité, et on passe
 * souvent de l'un à l'autre au cours du même appel.
 */
const ONGLETS = [
  { href: "/admin/journal", label: "Activité" },
  { href: "/admin/journal/courriels", label: "Courriels" },
] as const;

export function JournalShell({
  courant,
  chapeau,
  children,
}: {
  courant: string;
  chapeau: string;
  children: ReactNode;
}) {
  return (
    <>
      <header className="border-b border-line px-8 pt-6 pb-0">
        <p className="eyebrow">Équipe lemlearn</p>
        <h1 className="mt-1 text-xl font-medium tracking-tight">Journal</h1>
        <p className="mt-1.5 max-w-xl text-xs text-ink-2">{chapeau}</p>

        <nav className="-mb-px mt-5 flex gap-1">
          {ONGLETS.map((onglet) => (
            <Link
              key={onglet.href}
              href={onglet.href}
              aria-current={onglet.href === courant ? "page" : undefined}
              className={`border-b-2 px-3 py-2 text-xs transition-colors duration-[120ms] ${
                onglet.href === courant
                  ? "border-accent text-ink"
                  : "border-transparent text-ink-3 hover:text-ink"
              }`}
            >
              {onglet.label}
            </Link>
          ))}
        </nav>
      </header>

      {children}
    </>
  );
}
