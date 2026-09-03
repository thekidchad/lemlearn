"use client";

import Link from "next/link";
import { useCallback, useState } from "react";
import type { Contact } from "@/lib/api";

/**
 * Table des contacts, allongeable par curseur.
 *
 * La première tranche est rendue par le serveur — l'écran s'affiche complet
 * sans attendre le JavaScript — et les suivantes s'ajoutent ici. Sans numéros
 * de page : DynamoDB ne sait pas sauter au millième élément sans lire les neuf
 * cent quatre-vingt-dix-neuf premiers, et le total n'est pas connu sans tout
 * compter.
 */
export function ContactsTable({
  kind,
  initial,
  initialCursor,
}: {
  kind: string;
  initial: Contact[];
  initialCursor?: string;
}) {
  const [rows, setRows] = useState(initial);
  const [cursor, setCursor] = useState(initialCursor ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [terme, setTerme] = useState("");
  const [source, setSource] = useState("");
  const [joignables, setJoignables] = useState(false);
  const [filtre, setFiltre] = useState(false);

  // Le filtre est appliqué par le serveur et remplace la liste : filtrer les
  // seules lignes chargées ferait conclure qu'une fiche n'existe pas parce
  // qu'on n'a pas encore déroulé jusqu'à elle.
  const filtrer = useCallback(async () => {
    setError(null);
    setBusy(true);
    try {
      const url = new URL("/api/contacts", window.location.origin);
      url.searchParams.set("kind", kind);
      if (terme) url.searchParams.set("q", terme);
      if (source) url.searchParams.set("source", source);
      if (joignables) url.searchParams.set("joignables", "1");
      const response = await fetch(url);
      const body = (await response.json()) as {
        contacts?: Contact[] | null;
        cursor?: string;
        error?: string;
      };
      if (!response.ok) throw new Error(body.error ?? "filtre impossible");
      setRows(body.contacts ?? []);
      setCursor(body.cursor ?? "");
      setFiltre(Boolean(terme || source || joignables));
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "filtre impossible");
    } finally {
      setBusy(false);
    }
  }, [kind, terme, source, joignables]);

  const load = useCallback(async () => {
    setError(null);
    setBusy(true);
    try {
      const url = new URL("/api/contacts", window.location.origin);
      url.searchParams.set("kind", kind);
      url.searchParams.set("curseur", cursor);
      // Le filtre suit la pagination : sans lui, « charger la suite » ramènerait
      // des lignes que le filtre venait d'écarter.
      if (terme) url.searchParams.set("q", terme);
      if (source) url.searchParams.set("source", source);
      if (joignables) url.searchParams.set("joignables", "1");
      const response = await fetch(url);
      const body = (await response.json()) as {
        contacts?: Contact[] | null;
        cursor?: string;
        error?: string;
      };
      if (!response.ok) throw new Error(body.error ?? "chargement impossible");
      setRows((precedents) => [...precedents, ...(body.contacts ?? [])]);
      // Le curseur est le seul signal de fin fiable : une tranche plus courte
      // que la limite n'en est pas un, DynamoDB bornant aussi par la taille lue.
      setCursor(body.cursor ?? "");
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "chargement impossible");
    } finally {
      setBusy(false);
    }
  }, [cursor, kind, terme, source, joignables]);

  const barre = (
    <form
      className="flex flex-wrap items-center gap-2 border-b border-line px-6 py-2.5"
      onSubmit={(event) => {
        event.preventDefault();
        void filtrer();
      }}
    >
      <input
        type="search"
        value={terme}
        onChange={(event) => setTerme(event.target.value)}
        placeholder="Nom, courriel, ville, SIRET…"
        className="field h-8 w-64 text-xs"
      />
      <input
        type="search"
        value={source}
        onChange={(event) => setSource(event.target.value)}
        placeholder="Source"
        className="field h-8 w-36 text-xs"
      />
      <label className="flex items-center gap-1.5 text-2xs text-ink-2">
        <input
          type="checkbox"
          checked={joignables}
          onChange={(event) => setJoignables(event.target.checked)}
          className="size-3.5 accent-[var(--color-accent)]"
        />
        Avec courriel
      </label>
      <button type="submit" className="btn-secondary" disabled={busy}>
        Filtrer
      </button>
      {filtre && (
        <button
          type="button"
          className="btn-ghost"
          disabled={busy}
          onClick={() => {
            setTerme("");
            setSource("");
            setJoignables(false);
            setFiltre(false);
            window.location.reload();
          }}
        >
          Tout voir
        </button>
      )}
    </form>
  );

  if (rows.length === 0) {
    return (
      <>
        {barre}
        <p className="px-6 py-16 text-center text-xs text-ink-3">
          {filtre
            ? "Aucune fiche ne correspond à ce filtre."
            : "Aucun contact de ce type pour l'instant."}
        </p>
      </>
    );
  }

  return (
    <>
      {barre}
      <table className="w-full text-left">
        <thead>
          <tr className="border-b border-line text-2xs tracking-wide text-ink-3 uppercase">
            <th className="px-6 py-2.5 font-medium">Nom</th>
            <th className="px-6 py-2.5 font-medium">Contact</th>
            {kind === "learner" && <th className="px-6 py-2.5 font-medium">Naissance</th>}
            {kind !== "learner" && <th className="px-6 py-2.5 font-medium">SIRET</th>}
            <th className="px-6 py-2.5 font-medium">Ville</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((contact) => (
            <tr
              key={contact.id}
              className="border-b border-line/60 text-sm transition-colors duration-[120ms] hover:bg-surface-1"
            >
              <td className="px-6 py-2.5">
                <Link
                  href={`/stagiaires/${contact.id}`}
                  className="hover:text-accent-ink hover:underline"
                >
                  {displayName(contact)}
                </Link>
              </td>
              <td className="px-6 py-2.5 text-xs text-ink-2">
                {contact.email ?? "—"}
                {contact.phone ? ` · ${contact.phone}` : ""}
              </td>
              {kind === "learner" && (
                <td className="px-6 py-2.5 font-mono text-xs text-ink-2">
                  {contact.birthDate ?? "—"}
                </td>
              )}
              {kind !== "learner" && (
                <td className="px-6 py-2.5 font-mono text-xs text-ink-2">
                  {contact.siret ?? "—"}
                </td>
              )}
              <td className="px-6 py-2.5 text-xs text-ink-2">
                {contact.address?.city ?? "—"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      {error && <p className="px-6 py-3 text-xs text-danger">{error}</p>}

      {cursor && (
        <div className="flex justify-center border-t border-line py-4">
          <button type="button" className="btn-secondary" disabled={busy} onClick={load}>
            {busy ? "Chargement…" : "Charger la suite"}
          </button>
        </div>
      )}

      <p className="px-6 pb-4 text-2xs text-ink-3" data-numeric>
        {rows.length} contact{rows.length > 1 ? "s" : ""} affiché
        {rows.length > 1 ? "s" : ""}
        {cursor ? "" : " — c'est tout"}
      </p>
    </>
  );
}

function displayName(contact: Contact): string {
  if (contact.companyName) return contact.companyName;
  return `${contact.firstName ?? ""} ${contact.lastName ?? ""}`.trim() || "—";
}
