import Link from "next/link";
import type { ReactNode } from "react";

/**
 * La coque de la console de l'équipe.
 *
 * Elle ne ressemble pas aux écrans d'un organisme, et c'est délibéré : ici on
 * regarde la plateforme entière, pas un client. Un en-tête plus haut, un
 * sous-titre qui rappelle sur quoi on travaille, et quatre onglets qui sont les
 * quatre façons d'entrer dans la matière — par l'organisme, ou par la personne.
 *
 * Se tromper d'espace est la faute qui coûte le plus cher ici : on croit
 * regarder un client et on regarde tout le monde, ou l'inverse. Autant que les
 * deux ne se ressemblent pas.
 */
export const ONGLETS = [
  { href: "/admin/organismes", label: "Organismes" },
  { href: "/admin/stagiaires", label: "Stagiaires" },
  { href: "/admin/entreprises", label: "Entreprises" },
  { href: "/admin/financeurs", label: "Financeurs" },
] as const;

export function ConsoleShell({
  courant,
  chapeau,
  action,
  children,
}: {
  courant: string;
  chapeau: string;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <>
      <header className="border-b border-line px-8 pt-6 pb-0">
        <div className="flex flex-wrap items-start gap-4">
          <div className="min-w-0 flex-1">
            <p className="eyebrow">Équipe lemlearn</p>
            <h1 className="mt-1 text-xl font-medium tracking-tight">Plateforme</h1>
            <p className="mt-1.5 max-w-xl text-xs text-ink-2">{chapeau}</p>
          </div>
          {action}
        </div>

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
