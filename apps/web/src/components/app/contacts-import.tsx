"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

/**
 * Reprise d'un portefeuille depuis un tableur.
 *
 * C'est ce qui décide d'un changement de logiciel : un organisme arrive avec
 * trois cents lignes, et sans import il n'arrive pas du tout.
 *
 * Le résultat dit trois choses, dans cet ordre : ce qui est entré, quelles
 * colonnes ont été comprises, et quelles lignes ont été refusées avec leur
 * numéro dans le fichier. Un import qui annonce « 287 importés » sans nommer
 * les treize autres oblige à comparer deux listes à la main.
 */
interface Refusee {
  numero: number;
  nom?: string;
  erreur?: string;
}

export function ContactsImport({ kind }: { kind: "learner" | "company" | "funder" }) {
  const router = useRouter();
  const [ouvert, setOuvert] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [bilan, setBilan] = useState<{
    importes: number;
    colonnes: string[];
    refusees: Refusee[];
  } | null>(null);

  const importer = async (file: File) => {
    setError(null);
    setBilan(null);
    setBusy(true);
    try {
      const response = await fetch(`/api/contacts/import?kind=${kind}`, {
        method: "POST",
        headers: { "Content-Type": "text/csv" },
        body: await file.text(),
      });
      const body = (await response.json()) as {
        importes?: number;
        colonnes?: string[] | null;
        refusees?: Refusee[] | null;
        error?: string;
      };
      if (!response.ok) throw new Error(body.error ?? "import refusé");
      setBilan({
        importes: body.importes ?? 0,
        colonnes: body.colonnes ?? [],
        refusees: body.refusees ?? [],
      });
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "import impossible");
    } finally {
      setBusy(false);
    }
  };

  if (!ouvert) {
    return (
      <button type="button" className="btn-secondary" onClick={() => setOuvert(true)}>
        Importer
      </button>
    );
  }

  return (
    <div className="absolute top-14 right-6 z-20 w-96 rounded-xl border border-line bg-surface-1 p-5 shadow-lg">
      <div className="flex items-baseline gap-3">
        <h2 className="text-sm font-medium">Importer un fichier</h2>
        <button
          type="button"
          className="btn-ghost ml-auto"
          onClick={() => {
            setOuvert(false);
            setBilan(null);
            setError(null);
          }}
        >
          Fermer
        </button>
      </div>

      <p className="mt-2 text-2xs text-ink-2">
        Un CSV, séparé par des virgules ou des points-virgules. Les colonnes sont
        reconnues par leur intitulé : Prénom, Nom, Email, Téléphone, Raison
        sociale, SIRET, Date de naissance, Adresse, Code postal, Ville, Source.
        Celles qu&apos;on ne connaît pas sont ignorées, pas refusées.
      </p>

      {!bilan && (
        <label className="btn-primary mt-4 inline-flex cursor-pointer">
          {busy ? "Import en cours…" : "Choisir le fichier"}
          <input
            type="file"
            accept=".csv,text/csv"
            className="hidden"
            disabled={busy}
            onChange={(event) => {
              const file = event.target.files?.[0];
              event.target.value = "";
              if (file) void importer(file);
            }}
          />
        </label>
      )}

      {error && <p className="mt-3 text-xs text-danger">{error}</p>}

      {bilan && (
        <div className="mt-4 space-y-3">
          <p className="text-sm">
            <span className="font-medium" data-numeric>
              {bilan.importes}
            </span>{" "}
            fiche{bilan.importes > 1 ? "s" : ""} importée
            {bilan.importes > 1 ? "s" : ""}.
          </p>

          {bilan.colonnes.length > 0 && (
            <p className="text-2xs text-ink-3">
              Colonnes comprises : {bilan.colonnes.join(", ")}.
            </p>
          )}

          {bilan.refusees.length > 0 && (
            <div className="rounded-lg border border-warn/40 bg-warn/10 p-3">
              <p className="text-2xs font-medium text-warn">
                {bilan.refusees.length} ligne
                {bilan.refusees.length > 1 ? "s" : ""} refusée
                {bilan.refusees.length > 1 ? "s" : ""}
              </p>
              <ul className="mt-1.5 space-y-1">
                {bilan.refusees.slice(0, 10).map((refusee) => (
                  <li key={refusee.numero} className="text-2xs text-ink-2">
                    <span className="font-mono">ligne {refusee.numero}</span>
                    {refusee.nom ? ` — ${refusee.nom}` : ""} : {refusee.erreur}
                  </li>
                ))}
              </ul>
              {bilan.refusees.length > 10 && (
                <p className="mt-1 text-2xs text-ink-3">
                  et {bilan.refusees.length - 10} autre
                  {bilan.refusees.length - 10 > 1 ? "s" : ""}.
                </p>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
