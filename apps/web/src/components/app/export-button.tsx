"use client";

import { useState } from "react";

/**
 * Export du dossier probatoire.
 *
 * Le nombre de pièces et de manques est annoncé après coup : un dossier
 * exporté avec des trous reste utile — c'est même la façon la plus rapide de
 * voir ce qui manque — mais il ne doit pas passer pour complet.
 */
export function ExportButton({ fileId }: { fileId: string }) {
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const run = async () => {
    setBusy(true);
    setError(null);
    setNote(null);
    try {
      const response = await fetch(`/api/dossiers/${fileId}/export`, { method: "POST" });
      if (!response.ok) {
        const body = (await response.json()) as { error?: string };
        throw new Error(body.error ?? "export impossible");
      }

      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download =
        response.headers.get("Content-Disposition")?.match(/filename="([^"]+)"/)?.[1] ??
        "dossier.zip";
      link.click();
      URL.revokeObjectURL(url);

      const pieces = response.headers.get("X-Lemlearn-Pieces");
      const missing = Number(response.headers.get("X-Lemlearn-Missing") ?? 0);
      setNote(
        missing > 0
          ? `${pieces} pièces · ${missing} manquante${missing > 1 ? "s" : ""}`
          : `${pieces} pièces, dossier complet`,
      );
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "export impossible");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex items-center gap-2">
      <button
        type="button"
        onClick={run}
        disabled={busy}
        className="h-8 rounded-md border border-line px-2.5 text-xs text-ink-2 hover:border-accent hover:text-ink disabled:opacity-50"
      >
        {busy ? "Assemblage…" : "Exporter le dossier"}
      </button>
      {note && <span className="text-2xs text-ink-3">{note}</span>}
      {error && <span className="text-2xs text-danger">{error}</span>}
    </div>
  );
}
