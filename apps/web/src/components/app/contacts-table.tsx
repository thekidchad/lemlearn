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

  const load = useCallback(async () => {
    setError(null);
    setBusy(true);
    try {
      const url = new URL("/api/contacts", window.location.origin);
      url.searchParams.set("kind", kind);
      url.searchParams.set("curseur", cursor);
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
  }, [cursor, kind]);

  if (rows.length === 0) {
    return (
      <p className="px-6 py-16 text-center text-xs text-ink-3">
        Aucun contact de ce type pour l&apos;instant.
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
