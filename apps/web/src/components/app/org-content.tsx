"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

/**
 * Le contenu d'un organisme client, vu par l'équipe.
 *
 * Six vues sur la même mécanique : on demande une tranche, on l'affiche, on
 * charge la suite au curseur. Elles ne se chargent qu'à l'ouverture d'un
 * onglet — la fiche d'un organisme s'ouvre pour vingt raisons, et précharger
 * six listes à chaque fois coûterait six lectures pour rien.
 */
const VUES = [
  { clef: "repertoire-learner", vue: "repertoire", label: "Stagiaires", champ: "contacts", params: { kind: "learner" } },
  { clef: "repertoire-company", vue: "repertoire", label: "Entreprises", champ: "contacts", params: { kind: "company" } },
  { clef: "repertoire-funder", vue: "repertoire", label: "Financeurs", champ: "contacts", params: { kind: "funder" } },
  { clef: "formations", vue: "formations", label: "Formations", champ: "courses", params: {} },
  { clef: "sessions", vue: "sessions", label: "Sessions", champ: "sessions", params: {} },
  { clef: "dossiers", vue: "dossiers", label: "Dossiers", champ: "files", params: { etape: "agreement" } },
] as const;

type Vue = (typeof VUES)[number];
type Ligne = Record<string, unknown>;

export function OrgContent({ orgId }: { orgId: string }) {
  const [vue, setVue] = useState<Vue>(VUES[0]);

  return (
    <div className="surface-card overflow-hidden">
      <div className="flex flex-wrap gap-1 border-b border-line px-4 py-2.5">
        {VUES.map((candidate) => (
          <button
            key={candidate.clef}
            type="button"
            aria-current={candidate.clef === vue.clef ? "page" : undefined}
            onClick={() => setVue(candidate)}
            className={`rounded-md px-2.5 py-1 text-xs transition-colors duration-[120ms] ${
              candidate.clef === vue.clef
                ? "bg-surface-2 text-ink"
                : "text-ink-3 hover:bg-surface-2 hover:text-ink"
            }`}
          >
            {candidate.label}
          </button>
        ))}
      </div>

      {/* La clé remonte la liste à chaque changement d'onglet. Remettre son
          état à zéro en réaction au changement déclencherait un rendu en
          cascade ; un remontage dit la même chose, et le dit à React. */}
      <OrgList key={vue.clef} orgId={orgId} vue={vue} />
    </div>
  );
}

function OrgList({ orgId, vue }: { orgId: string; vue: Vue }) {
  const [rows, setRows] = useState<Ligne[]>([]);
  const [cursor, setCursor] = useState("");
  const [pret, setPret] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const adresse = useCallback(
    (curseur: string) => {
      const url = new URL(`/api/admin/orgs/${orgId}/contenu`, window.location.origin);
      url.searchParams.set("vue", vue.vue);
      for (const [clef, valeur] of Object.entries(vue.params)) {
        url.searchParams.set(clef, valeur);
      }
      if (curseur) url.searchParams.set("curseur", curseur);
      return url;
    },
    [orgId, vue],
  );

  // Chargement initial. Aucun état n'est posé dans le corps de l'effet : tout
  // se fait après la réponse, ce qui évite un rendu en cascade au montage.
  useEffect(() => {
    const controller = new AbortController();
    (async () => {
      try {
        const response = await fetch(adresse(""), { signal: controller.signal });
        const body = (await response.json()) as Record<string, unknown>;
        if (!response.ok) throw new Error(String(body.error ?? "lecture impossible"));
        setRows((body[vue.champ] as Ligne[] | null) ?? []);
        setCursor(String(body.cursor ?? ""));
      } catch (failure) {
        if (controller.signal.aborted) return;
        setError(failure instanceof Error ? failure.message : "lecture impossible");
      } finally {
        if (!controller.signal.aborted) setPret(true);
      }
    })();
    return () => controller.abort();
  }, [adresse, vue.champ]);

  const suite = async () => {
    setError(null);
    setBusy(true);
    try {
      const response = await fetch(adresse(cursor));
      const body = (await response.json()) as Record<string, unknown>;
      if (!response.ok) throw new Error(String(body.error ?? "lecture impossible"));
      setRows((precedents) => [...precedents, ...((body[vue.champ] as Ligne[] | null) ?? [])]);
      setCursor(String(body.cursor ?? ""));
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "lecture impossible");
    } finally {
      setBusy(false);
    }
  };

  if (!pret) {
    return <p className="px-4 py-10 text-center text-xs text-ink-3">Lecture…</p>;
  }

  return (
    <>
      {error && <p className="px-4 py-3 text-xs text-danger">{error}</p>}

      {!error && rows.length === 0 && (
        <p className="px-4 py-10 text-center text-xs text-ink-3">
          Rien de ce type chez cet organisme.
        </p>
      )}

      {rows.length > 0 && (
        <>
          <ul className="divide-y divide-line/60">
            {rows.map((ligne, index) => (
              <li
                key={String(ligne.id ?? index)}
                className="flex items-baseline gap-3 px-4 py-2.5"
              >
                {/* Une fiche s'ouvre d'un clic depuis ici : c'est le chemin
                    qu'on prend en vrai — on part de l'organisme au téléphone,
                    et on descend jusqu'à la personne. Les formations, sessions
                    et dossiers n'ont pas d'écran à eux dans la console : les
                    rendre cliquables mènerait à une page vide. */}
                {vue.champ === "contacts" && ligne.id ? (
                  <Link
                    href={`/admin/${orgId}/contacts/${String(ligne.id)}`}
                    className="min-w-0 flex-1 truncate text-sm hover:text-accent-ink hover:underline"
                  >
                    {titre(ligne)}
                  </Link>
                ) : (
                  <span className="min-w-0 flex-1 truncate text-sm">{titre(ligne)}</span>
                )}
                <span className="shrink-0 truncate font-mono text-2xs text-ink-3">
                  {detail(ligne)}
                </span>
              </li>
            ))}
          </ul>
          <p className="px-4 py-2 text-2xs text-ink-3" data-numeric>
            {rows.length} ligne{rows.length > 1 ? "s" : ""}
            {cursor ? "" : " — c'est tout"}
          </p>
        </>
      )}

      {cursor && (
        <div className="flex justify-center border-t border-line py-3">
          <button type="button" className="btn-secondary" disabled={busy} onClick={suite}>
            {busy ? "Chargement…" : "Charger la suite"}
          </button>
        </div>
      )}
    </>
  );
}

/** Le libellé d'une ligne, quelle que soit sa nature. */
function titre(ligne: Ligne): string {
  const nom = [ligne.firstName, ligne.lastName].filter(Boolean).join(" ");
  return (
    (ligne.companyName as string) ||
    nom ||
    (ligne.title as string) ||
    (ligne.reference as string) ||
    (ligne.id as string) ||
    "—"
  );
}

/** Ce qui identifie la ligne en second : adresse, référence, date, durée. */
function detail(ligne: Ligne): string {
  if (ligne.email) return String(ligne.email);
  if (ligne.reference) return String(ligne.reference);
  if (ligne.startsAt) return new Date(String(ligne.startsAt)).toLocaleDateString("fr-FR");
  if (ligne.durationHours) return `${ligne.durationHours} h`;
  return "";
}
