"use client";

import { useRouter } from "next/navigation";
import { useRef, useState } from "react";

/**
 * Pièce d'identité de l'apprenant.
 *
 * Le fichier monte directement vers le compartiment chiffré, sans passer par
 * l'application : une carte d'identité qui traverse une fonction finit dans
 * ses journaux au premier incident. Elle ne se consulte que par un lien d'une
 * minute, et chaque consultation part au journal d'audit.
 */
export function IdentityDoc({
  contactId,
  present,
}: {
  contactId: string;
  present: boolean;
}) {
  const router = useRouter();
  const input = useRef<HTMLInputElement | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const upload = async (file: File) => {
    setError(null);
    setBusy("dépôt");
    try {
      const reserve = await fetch(`/api/contacts/${contactId}/piece`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ filename: file.name, contentType: file.type }),
      });
      const reserved = (await reserve.json()) as {
        uploadUrl?: string;
        key?: string;
        error?: string;
      };
      if (!reserve.ok || !reserved.uploadUrl || !reserved.key) {
        throw new Error(reserved.error ?? "dépôt refusé");
      }

      const put = await fetch(reserved.uploadUrl, {
        method: "PUT",
        headers: { "Content-Type": file.type },
        body: file,
      });
      if (!put.ok) throw new Error(`le dépôt a échoué (${put.status})`);

      const attach = await fetch(`/api/contacts/${contactId}/piece`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ key: reserved.key }),
      });
      const attached = (await attach.json()) as { error?: string };
      if (!attach.ok) throw new Error(attached.error ?? "enregistrement refusé");

      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "dépôt impossible");
    } finally {
      setBusy(null);
    }
  };

  const open = async () => {
    setError(null);
    setBusy("ouverture");
    try {
      const response = await fetch(`/api/contacts/${contactId}/piece`);
      const body = (await response.json()) as { url?: string; error?: string };
      if (!response.ok || !body.url) throw new Error(body.error ?? "lien indisponible");
      window.open(body.url, "_blank", "noopener");
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "lien indisponible");
    } finally {
      setBusy(null);
    }
  };

  const remove = async () => {
    setError(null);
    setBusy("suppression");
    try {
      const response = await fetch(`/api/contacts/${contactId}/piece`, { method: "DELETE" });
      const body = (await response.json()) as { error?: string };
      if (!response.ok) throw new Error(body.error ?? "suppression impossible");
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "suppression impossible");
    } finally {
      setBusy(null);
    }
  };

  return (
    <section className="surface-card p-5">
      <h2 className="text-sm font-medium">Pièce d&apos;identité</h2>
      <p className="mt-1.5 text-xs text-ink-2">
        {present
          ? "Déposée. Elle ne s'affiche que par un lien d'une minute, et chaque consultation est journalisée."
          : "Aucune pièce déposée. Elle fait partie des treize pièces attendues d'un dossier complet."}
      </p>

      <input
        ref={input}
        type="file"
        accept="image/jpeg,image/png,image/heic,application/pdf"
        hidden
        onChange={(event) => {
          const file = event.target.files?.[0];
          if (file) void upload(file);
        }}
      />

      <div className="mt-4 flex flex-wrap items-center gap-2">
        <button
          type="button"
          onClick={() => input.current?.click()}
          disabled={busy !== null}
          className="h-8 rounded-md border border-line px-2.5 text-xs text-ink-2 hover:border-accent hover:text-ink disabled:opacity-50"
        >
          {busy === "dépôt" ? "Dépôt…" : present ? "Remplacer" : "Déposer une pièce"}
        </button>

        {present && (
          <>
            <button
              type="button"
              onClick={open}
              disabled={busy !== null}
              className="h-8 rounded-md border border-line px-2.5 text-xs text-ink-2 hover:border-accent hover:text-ink disabled:opacity-50"
            >
              {busy === "ouverture" ? "Ouverture…" : "Consulter"}
            </button>
            <button
              type="button"
              onClick={remove}
              disabled={busy !== null}
              className="h-8 rounded-md px-2.5 text-xs text-ink-3 hover:text-danger disabled:opacity-50"
            >
              {busy === "suppression" ? "Suppression…" : "Supprimer"}
            </button>
          </>
        )}

        {error && <span className="text-2xs text-danger">{error}</span>}
      </div>

      <p className="mt-3 text-2xs text-ink-3">
        {/* La règle CNIL n'est pas une option de configuration : elle est
            appliquée par le compartiment lui-même. */}
        Chiffrée par une clé dédiée et effacée automatiquement au bout de
        quatre-vingt-dix jours — recommandation CNIL : une pièce d&apos;identité ne
        se conserve que le temps de vérifier le dossier, pas la durée de
        l&apos;archivage légal.
      </p>
    </section>
  );
}
