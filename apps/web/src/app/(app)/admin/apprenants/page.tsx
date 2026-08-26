import type { Metadata } from "next";
import Link from "next/link";
import { apiFetch, ApiError } from "@/lib/api";

export const metadata: Metadata = { title: "Retrouver un apprenant" };

interface Match {
  orgId: string;
  orgName: string;
  contactId: string;
  name: string;
  email: string;
  hasAccount: boolean;
}

/**
 * Recherche d'un apprenant à travers les organisations.
 *
 * Sur l'adresse exacte, pas sur un fragment de nom : c'est la seule clé
 * résoluble sans parcourir les données de tous les clients, et un support qui
 * peut lister les apprenants de tout le monde n'a pas sa place dans un produit
 * qui vend la protection des données. Chaque recherche est journalisée sur
 * l'organisation concernée.
 */
export default async function FindLearnerPage({ searchParams }: PageProps<"/admin/apprenants">) {
  const params = await searchParams;
  const email = typeof params.email === "string" ? params.email.trim() : "";

  let matches: Match[] = [];
  let error: string | null = null;

  if (email) {
    try {
      ({ matches } = await apiFetch<{ matches: Match[] }>(
        `/v1/admin/apprenants?email=${encodeURIComponent(email)}`,
      ));
    } catch (failure) {
      error = failure instanceof ApiError ? failure.message : "recherche impossible";
    }
  }

  return (
    <>
      <header className="flex h-14 items-center gap-3 border-b border-line px-6">
        <Link href="/admin" className="text-xs text-ink-3 hover:text-ink">
          Organisations
        </Link>
        <span className="text-ink-3">/</span>
        <h1 className="text-sm font-medium">Retrouver un apprenant</h1>
      </header>

      <div className="mx-auto max-w-2xl px-6 py-6">
        <p className="text-xs text-ink-2">
          Quelqu&apos;un vous écrit sans dire de quel organisme il dépend. Son
          adresse suffit à le retrouver.
        </p>

        <form className="mt-5 flex gap-2">
          <input
            name="email"
            type="email"
            defaultValue={email}
            placeholder="adresse@exemple.fr"
            className="h-10 flex-1 rounded-lg border border-line bg-surface-0 px-3 text-sm outline-none focus:border-accent"
          />
          <button
            type="submit"
            className="h-10 rounded-lg bg-accent px-4 text-xs font-medium text-white hover:bg-accent-hover"
          >
            Chercher
          </button>
        </form>

        {error && (
          <p className="mt-4 rounded-lg border border-danger/40 bg-danger/10 px-3 py-2 text-xs text-danger">
            {error}
          </p>
        )}

        {email && !error && matches.length === 0 && (
          <p className="mt-6 text-xs text-ink-3">
            Aucune fiche ne porte cette adresse.
          </p>
        )}

        {matches.length > 0 && (
          <div className="mt-6 space-y-px overflow-hidden rounded-xl border border-line bg-line">
            {matches.map((match) => (
              <div key={`${match.orgId}-${match.contactId}`} className="bg-surface-1 px-4 py-3.5">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <div className="min-w-0">
                    <p className="truncate text-sm">{match.name}</p>
                    <p className="mt-0.5 font-mono text-2xs text-ink-3">
                      {match.email} · {match.orgName}
                      {match.hasAccount ? " · compte actif" : " · sans compte"}
                    </p>
                  </div>
                  <Link
                    href={`/admin/${match.orgId}`}
                    className="h-8 shrink-0 rounded-md border border-line px-2.5 text-xs leading-8 text-ink-2 hover:border-accent hover:text-ink"
                  >
                    Ouvrir l&apos;organisation
                  </Link>
                </div>
              </div>
            ))}
          </div>
        )}

        <p className="mt-8 text-2xs text-ink-3">
          {/* Dire pourquoi on ne va pas plus loin vaut mieux que de laisser
              chercher un bouton qui n'existe pas. */}
          Pour consulter sa fiche complète ou exporter son dossier, ouvrez une
          session sur l&apos;organisation depuis sa page. L&apos;accès est alors
          journalisé chez le client et visible dans sa barre latérale — c&apos;est
          délibéré : personne ne doit pouvoir extraire le dossier d&apos;un
          apprenant sans que son organisme le sache.
        </p>
      </div>
    </>
  );
}
