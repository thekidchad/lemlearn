"use client";

import { useState } from "react";

/**
 * Le suivi d'une fiche, vu par l'équipe.
 *
 * En lecture seule, comme le reste de cette fiche : on regarde ce que le client
 * voit, on n'écrit pas à sa place. C'est ce qu'on cherche quand il appelle en
 * disant « je vous ai envoyé l'attestation » — la pièce est là ou elle n'y est
 * pas, et il n'y a pas à ouvrir une session pour le savoir.
 *
 * Ouvrir une pièce est un accès à une donnée personnelle chez un tiers : le
 * lien vit deux minutes et l'ouverture entre au journal du client.
 */
interface Note {
  id: string;
  body: string;
  author: string;
  createdAt: string;
}

interface Rappel {
  id: string;
  title: string;
  dueOn: string;
  assigneeName?: string;
  doneAt?: string;
}

interface Piece {
  id: string;
  name: string;
  sizeBytes?: number;
  author: string;
  createdAt: string;
}

export function AdminSuivi({
  orgId,
  contactId,
  notes,
  rappels,
  pieces,
}: {
  orgId: string;
  contactId: string;
  notes: Note[];
  rappels: Rappel[];
  pieces: Piece[];
}) {
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const ouvrir = async (piece: Piece) => {
    setError(null);
    setBusy(piece.id);
    try {
      const response = await fetch(
        `/api/admin/orgs/${orgId}/contacts/${contactId}/pieces/${piece.id}`,
      );
      const body = (await response.json()) as { url?: string; error?: string };
      if (!response.ok || !body.url) throw new Error(body.error ?? "lecture refusée");
      window.open(body.url, "_blank", "noopener");
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "lecture refusée");
    } finally {
      setBusy(null);
    }
  };

  if (notes.length === 0 && rappels.length === 0 && pieces.length === 0) {
    return (
      <section className="surface-card overflow-hidden">
        <h2 className="border-b border-line px-5 py-3 text-sm font-medium">Suivi</h2>
        <p className="px-5 py-8 text-center text-xs text-ink-3">
          Aucune note, aucun rappel, aucune pièce jointe sur cette fiche.
        </p>
      </section>
    );
  }

  return (
    <section className="surface-card overflow-hidden">
      <h2 className="border-b border-line px-5 py-3 text-sm font-medium">Suivi</h2>

      {error && <p className="px-5 py-2 text-xs text-danger">{error}</p>}

      {pieces.length > 0 && (
        <div className="border-b border-line/60">
          <p className="eyebrow px-5 pt-3">Pièces jointes</p>
          <ul className="divide-y divide-line/60">
            {pieces.map((piece) => (
              <li
                key={piece.id}
                className="flex flex-wrap items-center gap-x-3 gap-y-1 px-5 py-2.5"
              >
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm">{piece.name}</p>
                  <p className="truncate font-mono text-2xs text-ink-3">
                    {poids(piece.sizeBytes)} · déposée par {piece.author} le{" "}
                    {new Date(piece.createdAt).toLocaleDateString("fr-FR")}
                  </p>
                </div>
                <button
                  type="button"
                  className="btn-ghost"
                  disabled={busy !== null}
                  onClick={() => ouvrir(piece)}
                >
                  {busy === piece.id ? "Ouverture…" : "Ouvrir"}
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}

      {rappels.length > 0 && (
        <div className="border-b border-line/60">
          <p className="eyebrow px-5 pt-3">Rappels</p>
          <ul className="divide-y divide-line/60">
            {rappels.map((rappel) => (
              <li key={rappel.id} className="flex items-baseline gap-3 px-5 py-2.5">
                <span
                  className={`min-w-0 flex-1 truncate text-sm ${
                    rappel.doneAt ? "text-ink-3 line-through" : ""
                  }`}
                >
                  {rappel.title}
                </span>
                <span className="shrink-0 font-mono text-2xs text-ink-3">
                  {new Date(rappel.dueOn).toLocaleDateString("fr-FR")}
                  {rappel.assigneeName ? ` · ${rappel.assigneeName}` : ""}
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {notes.length > 0 && (
        <div>
          <p className="eyebrow px-5 pt-3">Notes</p>
          <ul className="divide-y divide-line/60">
            {notes.map((note) => (
              <li key={note.id} className="px-5 py-2.5">
                <p className="text-sm whitespace-pre-wrap">{note.body}</p>
                <p className="mt-1 font-mono text-2xs text-ink-3">
                  {note.author} · {new Date(note.createdAt).toLocaleString("fr-FR")}
                </p>
              </li>
            ))}
          </ul>
        </div>
      )}

      <p className="border-t border-line px-5 py-3 text-2xs text-ink-3">
        En lecture seule : écrire chez un client se fait dans son espace, sous
        son identité. Ouvrir une pièce est journalisé chez lui.
      </p>
    </section>
  );
}

function poids(octets?: number): string {
  if (!octets) return "—";
  if (octets < 1024) return `${octets} o`;
  if (octets < 1024 * 1024) return `${Math.round(octets / 1024)} ko`;
  return `${(octets / 1024 / 1024).toFixed(1)} Mo`;
}
