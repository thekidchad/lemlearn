"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import type { Template } from "@/app/(app)/admin/gabarits/page";

/**
 * Éditeur d'un gabarit de courriel.
 *
 * L'aperçu se rafraîchit à mesure qu'on tape, rendu par le serveur avec les
 * mêmes valeurs d'exemple que celles qui valideront l'enregistrement : ce
 * qu'on voit est exactement ce qui partira. Un gabarit mal formé se refuse
 * ici, pas au moment où un signataire attend son code.
 */
export function TemplateEditor({ template }: { template: Template }) {
  const router = useRouter();
  const [subject, setSubject] = useState(template.subject);
  const [body, setBody] = useState(template.body);
  const [preview, setPreview] = useState<{ subject: string; html: string } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState<string | null>(null);

  // Aperçu différé : rendre à chaque frappe ferait une requête par lettre.
  useEffect(() => {
    const timer = setTimeout(async () => {
      try {
        const response = await fetch(`/api/admin/gabarits/${template.key}/apercu`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ subject, body }),
        });
        const parsed = (await response.json()) as {
          subject?: string;
          html?: string;
          error?: string;
        };
        if (!response.ok) throw new Error(parsed.error ?? "aperçu impossible");
        setPreview({ subject: parsed.subject ?? "", html: parsed.html ?? "" });
        setError(null);
      } catch (failure) {
        setError(failure instanceof Error ? failure.message : "aperçu impossible");
      }
    }, 400);
    return () => clearTimeout(timer);
  }, [template.key, subject, body]);

  const save = async () => {
    setBusy(true);
    setError(null);
    setSaved(null);
    try {
      const response = await fetch(`/api/admin/gabarits/${template.key}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ subject, body }),
      });
      const parsed = (await response.json()) as { error?: string };
      if (!response.ok) throw new Error(parsed.error ?? "enregistrement refusé");
      setSaved("Enregistré. Les prochains envois utilisent ce texte.");
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "enregistrement refusé");
    } finally {
      setBusy(false);
    }
  };

  const reset = async () => {
    setBusy(true);
    setError(null);
    try {
      const response = await fetch(`/api/admin/gabarits/${template.key}`, { method: "DELETE" });
      const parsed = (await response.json()) as { subject?: string; body?: string; error?: string };
      if (!response.ok) throw new Error(parsed.error ?? "retour impossible");
      setSubject(template.defaultSubject);
      setBody(template.defaultBody);
      setSaved("Gabarit d'origine rétabli.");
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "retour impossible");
    } finally {
      setBusy(false);
    }
  };

  const modified = subject !== template.subject || body !== template.body;

  return (
    <div className="grid gap-6 lg:grid-cols-2">
      <div className="min-w-0">
        <h2 className="text-sm font-medium">{template.label}</h2>
        <p className="mt-1.5 text-xs text-ink-2">{template.purpose}</p>

        <label className="mt-5 block">
          <span className="mb-1 block text-2xs text-ink-3">Objet</span>
          <input
            value={subject}
            onChange={(event) => setSubject(event.target.value)}
            className="h-9 w-full rounded-lg border border-line bg-surface-0 px-3 text-sm outline-none focus:border-accent"
          />
        </label>

        <label className="mt-3 block">
          <span className="mb-1 block text-2xs text-ink-3">Corps du message (HTML)</span>
          <textarea
            value={body}
            onChange={(event) => setBody(event.target.value)}
            rows={18}
            spellCheck={false}
            className="w-full rounded-lg border border-line bg-surface-0 px-3 py-2 font-mono text-2xs leading-relaxed outline-none focus:border-accent"
          />
        </label>

        <div className="mt-3 rounded-lg border border-line p-3">
          <p className="text-2xs text-ink-3">
            Variables disponibles — cliquez pour insérer :
          </p>
          <div className="mt-2 flex flex-wrap gap-1.5">
            {template.variables.map((variable) => (
              <button
                key={variable.name}
                type="button"
                title={`${variable.purpose} — exemple : ${variable.sample}`}
                onClick={() => setBody((current) => `${current}{{.${variable.name}}}`)}
                className="rounded border border-line px-1.5 py-0.5 font-mono text-2xs text-ink-2 hover:border-accent hover:text-ink"
              >
                {`{{.${variable.name}}}`}
              </button>
            ))}
          </div>
        </div>

        {error && (
          <p className="mt-3 rounded-lg border border-danger/40 bg-danger/10 px-3 py-2 text-2xs text-danger">
            {error}
          </p>
        )}
        {saved && <p className="mt-3 text-2xs text-ok">{saved}</p>}

        <div className="mt-4 flex flex-wrap items-center gap-2">
          <button
            type="button"
            onClick={save}
            disabled={busy || !modified || error !== null}
            className="h-9 rounded-lg bg-accent px-4 text-xs font-medium text-white hover:bg-accent-hover disabled:opacity-50"
          >
            {busy ? "Enregistrement…" : "Enregistrer"}
          </button>
          {template.overridden && (
            <button
              type="button"
              onClick={reset}
              disabled={busy}
              className="h-9 rounded-lg border border-line px-4 text-xs text-ink-2 hover:border-accent hover:text-ink disabled:opacity-50"
            >
              Revenir au gabarit d&apos;origine
            </button>
          )}
          {template.overridden && template.updatedBy && (
            <span className="text-2xs text-ink-3">
              modifié par {template.updatedBy}
              {template.updatedAt
                ? ` le ${new Date(template.updatedAt).toLocaleDateString("fr-FR")}`
                : ""}
            </span>
          )}
        </div>
      </div>

      <div className="min-w-0">
        <p className="text-2xs text-ink-3">Aperçu, avec des valeurs d&apos;exemple</p>
        <div className="mt-2 overflow-hidden rounded-xl border border-line">
          <p className="border-b border-line bg-surface-2 px-3 py-2 text-xs">
            <span className="text-ink-3">Objet : </span>
            {preview?.subject ?? "…"}
          </p>
          <iframe
            title="Aperçu du courriel"
            srcDoc={preview?.html ?? ""}
            sandbox=""
            className="h-[36rem] w-full bg-white"
          />
        </div>
      </div>
    </div>
  );
}
