import type { Metadata } from "next";
import Link from "next/link";
import { apiFetch, type Me } from "@/lib/api";

export const metadata: Metadata = {
  title: "Page introuvable",
  robots: { index: false, follow: false },
};

/**
 * La page qui n'existe pas.
 *
 * Elle ne se contente pas d'annoncer le vide : une adresse fausse vient presque
 * toujours d'un lien vieilli, d'un signet ou d'une faute de frappe, et la seule
 * chose utile est de proposer l'endroit où la personne allait. On regarde donc
 * qui est connecté pour offrir ses écrans à elle — un stagiaire n'a que faire
 * d'un lien vers le pipeline, et un visiteur non connecté n'a que faire des
 * deux.
 *
 * La lecture est protégée : une page d'erreur qui échoue est une impasse.
 */
export default async function NotFound() {
  const me = await apiFetch<Me>("/v1/me").catch(() => null);

  const destinations = !me
    ? [
        { href: "/connexion", label: "Se connecter" },
        { href: "/", label: "Page d'accueil" },
      ]
    : me.user.role === "learner"
      ? [
          { href: "/apprenant", label: "Mon parcours" },
          { href: "/apprenant/documents", label: "Mes documents" },
          { href: "/apprenant/informations", label: "Mes informations" },
        ]
      : me.user.role === "superadmin" && !me.impersonatedBy
        ? [
            { href: "/admin/organismes", label: "Organismes" },
            { href: "/admin/journal", label: "Journal" },
          ]
        : [
            { href: "/pipeline", label: "Pipeline" },
            { href: "/stagiaires", label: "Stagiaires" },
            { href: "/catalogue", label: "Catalogue" },
          ];

  return (
    <main className="mx-auto flex min-h-dvh max-w-lg flex-col justify-center px-5 py-12 sm:px-8">
      <p className="eyebrow">Erreur 404</p>
      <h1 className="learner-title mt-2">Cette page n&apos;existe pas</h1>
      <p className="learner-body mt-3">
        L&apos;adresse demandée ne correspond à rien. Le plus souvent, c&apos;est
        un lien qui a vieilli ou un signet vers un écran qui a changé de nom.
      </p>

      <nav className="mt-8 flex flex-col gap-px overflow-hidden rounded-xl border border-line bg-line">
        {destinations.map((destination) => (
          <Link
            key={destination.href}
            href={destination.href}
            className="flex items-center justify-between bg-surface-1 px-5 py-4 text-sm transition-colors duration-[120ms] hover:bg-surface-2"
          >
            {destination.label}
            <span aria-hidden className="text-ink-3">
              →
            </span>
          </Link>
        ))}
      </nav>
    </main>
  );
}
