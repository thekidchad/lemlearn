import type { Metadata } from "next";
import Link from "next/link";
import { apiFetch } from "@/lib/api";

export const metadata: Metadata = { title: "Bilan pédagogique et financier" };

interface Ligne {
  source: string;
  label: string;
  dossiers: number;
  montantHT: number;
}

interface Bilan {
  annee: number;
  produits: Ligne[] | null;
  totalHT: number;
  dossiers: number;
  stagiaires: number;
  heuresStagiaire: number;
  sessions: number;
  sansOrigine?: string[] | null;
}

/**
 * Bilan pédagogique et financier.
 *
 * L'écran ne reproduit pas le Cerfa : il donne les nombres à y reporter.
 * L'organisme déclare sur le portail Mon Activité Formation, et ce qui lui
 * manque à ce moment-là, ce sont les totaux — pas un formulaire de plus à
 * maintenir au fil des révisions annuelles.
 */
export default async function BilanPage({ searchParams }: PageProps<"/qualiopi/bilan">) {
  const params = await searchParams;
  const demande = typeof params.annee === "string" ? params.annee : "";
  const { bilan, echeance } = await apiFetch<{ bilan: Bilan; echeance: string }>(
    `/v1/organisme/bilan${demande ? `?annee=${encodeURIComponent(demande)}` : ""}`,
  );

  const annees = [bilan.annee + 1, bilan.annee, bilan.annee - 1, bilan.annee - 2];
  const produits = bilan.produits ?? [];
  const manquants = bilan.sansOrigine ?? [];

  return (
    <>
      <header className="flex h-14 items-center gap-3 border-b border-line px-6">
        <Link href="/qualiopi" className="text-xs text-ink-3 hover:text-ink">
          Conformité
        </Link>
        <span className="text-ink-3">/</span>
        <h1 className="text-sm font-medium">Bilan pédagogique et financier</h1>
      </header>

      <div className="mx-auto max-w-3xl px-6 py-6">
        <p className="text-sm text-ink-2">
          Déclaration annuelle obligatoire. À déposer avant le{" "}
          <strong>{new Date(echeance).toLocaleDateString("fr-FR")}</strong> pour
          l&apos;exercice {bilan.annee} — un report au 31 mai est accordé chaque
          année par le ministère du travail, mais il n&apos;est pas acquis. Son
          défaut suspend l&apos;exonération de TVA.
        </p>

        <nav className="mt-5 flex gap-2">
          {annees.map((annee) => (
            <Link
              key={annee}
              href={`/qualiopi/bilan?annee=${annee}`}
              aria-current={annee === bilan.annee ? "page" : undefined}
              className={`rounded-lg border px-3 py-1.5 text-xs transition-colors duration-[120ms] ${
                annee === bilan.annee
                  ? "border-accent bg-accent-dim text-ink"
                  : "border-line text-ink-2 hover:border-line-strong hover:text-ink"
              }`}
              data-numeric
            >
              {annee}
            </Link>
          ))}
        </nav>

        <div className="mt-6 grid grid-cols-3 gap-px overflow-hidden rounded-xl border border-line bg-line">
          <Chiffre label="stagiaires" value={String(bilan.stagiaires)} />
          <Chiffre
            label="heures-stagiaires"
            value={bilan.heuresStagiaire.toLocaleString("fr-FR")}
          />
          <Chiffre label="sessions" value={String(bilan.sessions)} />
        </div>
        <p className="mt-2 text-2xs text-ink-3">
          Une heure-stagiaire est une heure suivie par une personne : c&apos;est
          l&apos;effectif multiplié par la durée, et c&apos;est cette grandeur que
          demande le formulaire.
        </p>

        <h2 className="mt-8 text-sm font-medium">Origine des produits</h2>
        <p className="mt-1.5 text-xs text-ink-2">
          Ventilation du cadre C, en comptabilité d&apos;engagement : ce qui a été
          conventionné sur l&apos;exercice, non ce qui a été encaissé.
        </p>

        {produits.length === 0 ? (
          <p className="mt-5 text-sm text-ink-3">
            Aucun dossier engagé sur cet exercice.
          </p>
        ) : (
          <div className="mt-4 space-y-px overflow-hidden rounded-xl border border-line bg-line">
            {produits.map((ligne) => (
              <div
                key={ligne.source}
                className="flex items-center justify-between gap-4 bg-surface-1 px-4 py-3"
              >
                <span className="min-w-0 flex-1 truncate text-sm">{ligne.label}</span>
                <span className="shrink-0 text-2xs text-ink-3" data-numeric>
                  {ligne.dossiers} dossier{ligne.dossiers > 1 ? "s" : ""}
                </span>
                <span className="w-32 shrink-0 text-right text-sm" data-numeric>
                  {euros(ligne.montantHT)}
                </span>
              </div>
            ))}
            <div className="flex items-center justify-between gap-4 bg-surface-2 px-4 py-3">
              <span className="flex-1 text-sm font-medium">Total</span>
              <span className="w-32 text-right text-sm font-semibold" data-numeric>
                {euros(bilan.totalHT)}
              </span>
            </div>
          </div>
        )}

        {manquants.length > 0 && (
          <div className="mt-5 rounded-xl border border-warn/40 bg-warn/10 p-4">
            <p className="text-xs font-medium text-warn">
              {manquants.length} dossier{manquants.length > 1 ? "s" : ""} sans
              origine de fonds renseignée
            </p>
            <p className="mt-1.5 text-2xs text-ink-2">
              Ils sont comptés en « autres produits », ce qui fausse la
              ventilation. Renseignez-la sur chaque dossier avant de déclarer :{" "}
              {manquants.slice(0, 8).join(", ")}
              {manquants.length > 8 ? "…" : ""}
            </p>
          </div>
        )}

        <p className="mt-8 text-2xs text-ink-3">
          Ces nombres se reportent sur le formulaire Cerfa 10443, ou se saisissent
          directement sur le portail Mon Activité Formation. Les charges et le
          personnel formateur — cadres D et E — ne sont pas connus du produit et
          restent à votre comptabilité.
        </p>
      </div>
    </>
  );
}

function Chiffre({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-surface-1 px-4 py-3.5">
      <p className="text-2xs text-ink-3">{label}</p>
      <p className="mt-0.5 text-xl font-semibold tracking-[-0.03em]" data-numeric>
        {value}
      </p>
    </div>
  );
}

function euros(montant: number): string {
  return montant.toLocaleString("fr-FR", { style: "currency", currency: "EUR", maximumFractionDigits: 0 });
}
