"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

export interface SlotView {
  id: string;
  label: string;
  start: string;
  end: string;
  hours: number;
  signed: boolean;
  signedAt?: string;
  method?: string;
  signable: boolean;
  reason?: string;
}

/**
 * Émargement d'un apprenant, créneau par créneau.
 *
 * L'acte est délibérément explicite : une case cochée d'un geste distrait ne
 * vaut rien devant un contrôleur, alors qu'un bouton nommé « Je confirme ma
 * présence » atteste d'une intention. L'horodatage, l'adresse et le navigateur
 * partent au journal d'audit avec la signature.
 *
 * Un créneau déjà émargé ne se reprend pas : corriger une erreur passe par le
 * formateur, ce qui laisse trace des deux états au lieu d'effacer le premier.
 */
export function AttendanceSign({
  sessionId,
  contactId,
  slots,
}: {
  sessionId: string;
  contactId?: string;
  slots: SlotView[];
}) {
  const router = useRouter();
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const sign = async (slotId: string) => {
    setError(null);
    setBusy(slotId);
    try {
      const response = await fetch(
        `/api/apprenant/${sessionId}/emargement/${slotId}${contactId ? `?contactId=${contactId}` : ""}`,
        { method: "POST" },
      );
      const body = (await response.json()) as { error?: string };
      if (!response.ok) throw new Error(body.error ?? "émargement refusé");
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "émargement impossible");
    } finally {
      setBusy(null);
    }
  };

  if (slots.length === 0) {
    return (
      <p className="text-xs text-ink-3">
        Aucun créneau à émarger pour cette session.
      </p>
    );
  }

  return (
    <>
      <ol className="divide-y divide-line border-t border-line">
        {slots.map((slot) => (
          <li key={slot.id} className="flex flex-wrap items-center gap-3 py-3">
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm">{slot.label}</span>
              <span className="block text-2xs text-ink-3">
                {slot.hours} h
                {slot.signed && slot.signedAt
                  ? ` · émargé le ${new Date(slot.signedAt).toLocaleString("fr-FR", {
                      dateStyle: "short",
                      timeStyle: "short",
                    })}`
                  : slot.reason
                    ? ` · ${slot.reason}`
                    : ""}
              </span>
            </span>

            {slot.signed ? (
              <span className="rounded-md border border-ok/40 bg-ok/10 px-2 py-1 text-2xs text-ok">
                {slot.method === "connection" ? "Présence établie" : "Émargé"}
              </span>
            ) : slot.signable ? (
              <button
                type="button"
                className="btn-primary"
                disabled={busy !== null}
                onClick={() => void sign(slot.id)}
              >
                {busy === slot.id ? "Enregistrement…" : "Je confirme ma présence"}
              </button>
            ) : (
              <span className="text-2xs text-ink-3">Indisponible</span>
            )}
          </li>
        ))}
      </ol>

      {error && <p className="mt-3 text-xs text-danger">{error}</p>}
    </>
  );
}
