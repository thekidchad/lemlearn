"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

/**
 * Les deux gestes qu'on pose depuis la fiche d'une personne.
 *
 * Quand le geste n'est pas possible, on dit pourquoi au lieu de désactiver un
 * bouton sans explication : « sans compte » ne dit pas s'il faut inviter la
 * personne ou attendre qu'elle choisisse son mot de passe.
 */
export function ContactActions({
  orgId,
  contactId,
  hasAccount,
  accountHint,
  fileId,
  fileReference,
}: {
  orgId: string;
  contactId: string;
  hasAccount: boolean;
  accountHint?: string;
  fileId?: string;
  fileReference?: string;
}) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const entrer = async () => {
    setError(null);
    setBusy(true);
    try {
      const response = await fetch(
        `/api/admin/orgs/${orgId}/contacts/${contactId}/impersonate`,
        { method: "POST" },
      );
      const body = (await response.json()) as { landing?: string; error?: string };
      if (!response.ok) throw new Error(body.error ?? "ouverture impossible");
      router.push(body.landing ?? "/pipeline");
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "ouverture impossible");
      setBusy(false);
    }
  };

  return (
    <div className="flex flex-col items-end gap-1.5">
      <div className="flex flex-wrap items-center gap-2">
        {fileId ? (
          // Un lien, pas un appel : le navigateur enregistre l'archive
          // lui-même, sans faire transiter plusieurs mégaoctets par la mémoire
          // de la page.
          <a
            href={`/api/admin/orgs/${orgId}/dossiers/${fileId}/export`}
            className="btn-secondary"
            title={fileReference ? `Dossier ${fileReference}` : undefined}
          >
            Exporter le dossier
          </a>
        ) : (
          <span className="text-2xs text-ink-3">aucun dossier à exporter</span>
        )}

        {hasAccount ? (
          <button
            type="button"
            className="btn-primary"
            disabled={busy}
            onClick={entrer}
            title="Ouvre une session sur ce compte. L'accès est tracé et visible du client."
          >
            {busy ? "Ouverture…" : "Ouvrir sa session"}
          </button>
        ) : (
          <span className="text-2xs text-ink-3">{accountHint ?? "sans compte"}</span>
        )}
      </div>
      {error && <p className="text-2xs text-danger">{error}</p>}
    </div>
  );
}
