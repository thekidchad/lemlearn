"use client";

import { useState } from "react";

/**
 * Ouverture de l'accès à l'espace apprenant.
 *
 * L'apprenant ne s'inscrit pas lui-même : un espace en libre inscription
 * laisserait n'importe qui se déclarer stagiaire de l'organisme. C'est donc
 * d'ici que part son accès.
 */
export function LearnerAccess({ contactId, email }: { contactId: string; email?: string }) {
  const [busy, setBusy] = useState(false);
  const [sent, setSent] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const invite = async () => {
    setBusy(true);
    setError(null);
    try {
      const response = await fetch(`/api/contacts/${contactId}/invitation`, { method: "POST" });
      const body = (await response.json()) as {
        sentTo?: string;
        warning?: string;
        error?: string;
      };
      if (!response.ok) throw new Error(body.error ?? "invitation impossible");
      setSent(body.warning ?? `Invitation envoyée à ${body.sentTo}.`);
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "invitation impossible");
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="surface-card p-5">
      <h2 className="text-sm font-medium">Accès à l&apos;espace apprenant</h2>
      <p className="mt-1.5 text-xs text-ink-2">
        {email
          ? `Un courriel part à ${email} avec un lien personnel pour choisir un mot de passe. Il vaut quatorze jours.`
          : "Cette fiche n'a pas d'adresse de courriel : ajoutez-la avant d'ouvrir un accès."}
      </p>

      <div className="mt-4 flex flex-wrap items-center gap-2">
        <button
          type="button"
          onClick={invite}
          disabled={busy || !email}
          className="h-8 rounded-md border border-line px-2.5 text-xs text-ink-2 hover:border-accent hover:text-ink disabled:opacity-50"
        >
          {busy ? "Envoi…" : sent ? "Renvoyer l'invitation" : "Ouvrir l'accès"}
        </button>
        {sent && <span className="text-2xs text-ok">{sent}</span>}
        {error && <span className="text-2xs text-danger">{error}</span>}
      </div>

      <p className="mt-3 text-2xs text-ink-3">
        {/* Réinviter ne recrée pas de compte : c'est le même, avec un lien
            neuf. */}
        Réinviter quelqu&apos;un ne crée pas un second compte : le lien précédent
        cesse simplement de valoir.
      </p>
    </section>
  );
}
