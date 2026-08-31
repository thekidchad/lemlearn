"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

/**
 * Une nature, sur toute la plateforme.
 *
 * L'équipe ne travaille pas sur son propre organisme — il est vide, et le
 * regarder n'a aucun sens. Ses écrans montrent donc ce que contient le produit,
 * tous organismes confondus, chaque ligne nommant celui auquel elle appartient.
 *
 * Deux gestes par ligne, et ce sont ceux qu'on fait quand quelqu'un appelle :
 * entrer dans son espace pour voir ce qu'il voit, et sortir son dossier pour le
 * lui envoyer.
 */
interface Ligne {
  orgId: string;
  orgName: string;
  id: string;
  label: string;
  detail?: string;
  canImpersonate?: boolean;
  fileId?: string;
  fileReference?: string;
}

export function PlatformRows({ vue }: { vue: "stagiaires" | "entreprises" | "financeurs" }) {
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
    return <p className="py-16 text-center text-xs text-ink-3">Lecture de la plateforme…</p>;
  }

  if (error) {
    return <p className="py-16 text-center text-xs text-danger">{error}</p>;
  }

  if (rows.length === 0) {
    return (
      <p className="py-16 text-center text-xs text-ink-3">
        Aucune ligne de ce type sur la plateforme — {orgs} organisme
        {orgs > 1 ? "s" : ""} parcouru{orgs > 1 ? "s" : ""}.
      </p>
    );
  }

  return (
    <>
      <table className="w-full text-left">
        <thead>
          <tr className="border-b border-line text-2xs tracking-wide text-ink-3 uppercase">
            <th className="px-6 py-2.5 font-medium">Nom</th>
            <th className="px-6 py-2.5 font-medium">Contact</th>
            <th className="px-6 py-2.5 font-medium">Organisme</th>
            <th className="px-6 py-2.5 font-medium">Actions</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <LigneRendue key={`${row.orgId}-${row.id}`} row={row} />
          ))}
        </tbody>
      </table>

      <div className="flex items-center gap-3 border-t border-line px-6 py-3">
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
    </>
  );
}

function LigneRendue({ row }: { row: Ligne }) {
  const router = useRouter();
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const entrer = async () => {
    setError(null);
    setBusy("session");
    try {
      const response = await fetch(
        `/api/admin/orgs/${row.orgId}/contacts/${row.id}/impersonate`,
        { method: "POST" },
      );
      const body = (await response.json()) as { landing?: string; error?: string };
      if (!response.ok) throw new Error(body.error ?? "ouverture impossible");
      router.push(body.landing ?? "/pipeline");
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "ouverture impossible");
      setBusy(null);
    }
  };

  return (
    <tr className="border-b border-line/60 text-sm transition-colors duration-[120ms] hover:bg-surface-1">
      <td className="px-6 py-2.5">{row.label}</td>
      <td className="px-6 py-2.5 text-xs text-ink-2">{row.detail || "—"}</td>
      <td className="px-6 py-2.5">
        <Link
          href={`/admin/${row.orgId}`}
          className="text-xs text-ink-2 hover:text-accent-ink hover:underline"
        >
          {row.orgName}
        </Link>
      </td>
      <td className="px-6 py-2.5">
        <div className="flex flex-wrap items-center gap-2">
          {row.canImpersonate ? (
            <button
              type="button"
              className="h-7 rounded-md border border-line px-2 text-2xs text-ink-2 hover:border-accent hover:text-ink disabled:opacity-50"
              disabled={busy !== null}
              onClick={entrer}
              title="Ouvre une session sur ce compte. L'accès est tracé et visible du client."
            >
              {busy === "session" ? "Ouverture…" : "Ouvrir la session"}
            </button>
          ) : (
            <span className="text-2xs text-ink-3" title="Aucun compte : la personne n'a pas encore été invitée, ou n'a pas choisi son mot de passe.">
              sans compte
            </span>
          )}

          {row.fileId ? (
            // Un lien et non un appel : le navigateur enregistre le fichier
            // lui-même, sans passer le ZIP par la mémoire de la page.
            <a
              href={`/api/admin/orgs/${row.orgId}/dossiers/${row.fileId}/export`}
              className="h-7 rounded-md border border-line px-2 text-2xs leading-7 text-ink-2 hover:border-accent hover:text-ink"
              title={`Dossier ${row.fileReference}`}
            >
              Exporter le dossier
            </a>
          ) : (
            <span className="text-2xs text-ink-3">sans dossier</span>
          )}

          {error && <span className="text-2xs text-danger">{error}</span>}
        </div>
      </td>
    </tr>
  );
}
