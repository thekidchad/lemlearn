"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

/**
 * Toute la plateforme, tous organismes confondus.
 *
 * Chaque ligne dit à quel organisme elle appartient — sans quoi la liste serait
 * un tas. Le nom de l'organisme est cliquable : c'est de là qu'on ouvre une
 * session chez lui pour intervenir.
 */
const VUES = [
  { clef: "stagiaires", label: "Stagiaires" },
  { clef: "entreprises", label: "Entreprises" },
  { clef: "financeurs", label: "Financeurs" },
  { clef: "formations", label: "Formations" },
  { clef: "sessions", label: "Sessions" },
  { clef: "dossiers", label: "Dossiers" },
] as const;

interface Ligne {
  orgId: string;
  orgName: string;
  id: string;
  label: string;
  detail?: string;
}

export function PlatformData() {
  const [vue, setVue] = useState<string>(VUES[0].clef);

  return (
    <>
      <div className="mb-4 flex flex-wrap gap-1">
        {VUES.map((candidate) => (
          <button
            key={candidate.clef}
            type="button"
            aria-current={candidate.clef === vue ? "page" : undefined}
            onClick={() => setVue(candidate.clef)}
            className={`rounded-md px-2.5 py-1 text-xs transition-colors duration-[120ms] ${
              candidate.clef === vue
                ? "bg-surface-2 text-ink"
                : "text-ink-3 hover:bg-surface-2 hover:text-ink"
            }`}
          >
            {candidate.label}
          </button>
        ))}
      </div>

      {/* La clé remonte la liste à chaque changement de vue : remettre son état
          à zéro en réaction ferait un rendu en cascade. */}
      <Liste key={vue} vue={vue} />
    </>
  );
}

function Liste({ vue }: { vue: string }) {
  const [rows, setRows] = useState<Ligne[]>([]);
  const [cursor, setCursor] = useState("");
  const [orgs, setOrgs] = useState(0);
  const [pret, setPret] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const adresse = useCallback(
    (curseur: string) => {
      const url = new URL("/api/admin/tout", window.location.origin);
      url.searchParams.set("vue", vue);
      if (curseur) url.searchParams.set("curseur", curseur);
      return url;
    },
    [vue],
  );

  useEffect(() => {
    const controller = new AbortController();
    (async () => {
      try {
        const response = await fetch(adresse(""), { signal: controller.signal });
        const body = (await response.json()) as {
          lignes?: Ligne[] | null;
          cursor?: string;
          organismes?: number;
          error?: string;
        };
        if (!response.ok) throw new Error(body.error ?? "lecture impossible");
        setRows(body.lignes ?? []);
        setCursor(body.cursor ?? "");
        setOrgs(body.organismes ?? 0);
      } catch (failure) {
        if (controller.signal.aborted) return;
        setError(failure instanceof Error ? failure.message : "lecture impossible");
      } finally {
        if (!controller.signal.aborted) setPret(true);
      }
    })();
    return () => controller.abort();
  }, [adresse]);

  const suite = async () => {
    setError(null);
    setBusy(true);
    try {
      const response = await fetch(adresse(cursor));
      const body = (await response.json()) as {
        lignes?: Ligne[] | null;
        cursor?: string;
        error?: string;
      };
      if (!response.ok) throw new Error(body.error ?? "lecture impossible");
      setRows((precedents) => [...precedents, ...(body.lignes ?? [])]);
      setCursor(body.cursor ?? "");
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "lecture impossible");
    } finally {
      setBusy(false);
    }
  };

  if (!pret) {
    return <p className="py-16 text-center text-xs text-ink-3">Lecture…</p>;
  }

  if (error) {
    return <p className="py-16 text-center text-xs text-danger">{error}</p>;
  }

  if (rows.length === 0) {
    return (
      <p className="py-16 text-center text-xs text-ink-3">
        Rien de ce type sur la plateforme — {orgs} organisme{orgs > 1 ? "s" : ""}{" "}
        parcouru{orgs > 1 ? "s" : ""}.
      </p>
    );
  }

  return (
    <div className="overflow-hidden rounded-xl border border-line">
      <table className="w-full text-left">
        <thead>
          <tr className="border-b border-line text-2xs tracking-wide text-ink-3 uppercase">
            <th className="px-4 py-2.5 font-medium">Organisme</th>
            <th className="px-4 py-2.5 font-medium">Libellé</th>
            <th className="px-4 py-2.5 font-medium">Détail</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr
              key={`${row.orgId}-${row.id}`}
              className="border-b border-line/60 text-sm transition-colors duration-[120ms] hover:bg-surface-1"
            >
              <td className="px-4 py-2.5">
                <Link
                  href={`/admin/${row.orgId}`}
                  className="text-xs text-ink-2 hover:text-accent-ink hover:underline"
                >
                  {row.orgName}
                </Link>
              </td>
              <td className="px-4 py-2.5">{row.label}</td>
              <td className="px-4 py-2.5 font-mono text-2xs text-ink-3">
                {row.detail ?? ""}
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      <div className="flex items-center gap-3 border-t border-line px-4 py-3">
        <p className="text-2xs text-ink-3" data-numeric>
          {rows.length} ligne{rows.length > 1 ? "s" : ""} sur {orgs} organisme
          {orgs > 1 ? "s" : ""}
          {cursor ? "" : " — c'est tout"}
        </p>
        {cursor && (
          <button
            type="button"
            className="btn-secondary ml-auto"
            disabled={busy}
            onClick={suite}
          >
            {busy ? "Chargement…" : "Charger la suite"}
          </button>
        )}
      </div>
    </div>
  );
}
