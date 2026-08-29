"use client";

import { useRouter } from "next/navigation";
import { useRef, useState } from "react";
import { CourseCover } from "@/components/app/course-cover";

/** Ce que l'API accepte : une photographie, jamais du SVG servi en public. */
const ACCEPTED = "image/jpeg,image/png,image/webp";

/**
 * Dépôt du visuel d'une formation.
 *
 * C'est la première chose que voit un stagiaire dans son espace : une liste de
 * titres se parcourt mal, et quelqu'un qui suit deux formations les distingue
 * par l'image avant de lire l'intitulé.
 */
export function CourseCoverUpload({
  courseId,
  title,
  coverUrl,
}: {
  courseId: string;
  title: string;
  coverUrl?: string;
}) {
  const router = useRouter();
  const input = useRef<HTMLInputElement | null>(null);
  const [url, setUrl] = useState(coverUrl ?? "");
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const upload = async (file: File) => {
    setError(null);
    setBusy("dépôt");
    try {
      const reserve = await fetch(`/api/courses/${courseId}/visuel`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ contentType: file.type }),
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

      const attach = await fetch(`/api/courses/${courseId}/visuel`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ key: reserved.key }),
      });
      const attached = (await attach.json()) as { coverUrl?: string; error?: string };
      if (!attach.ok) throw new Error(attached.error ?? "enregistrement refusé");

      // L'aperçu lit le fichier local : l'objet vient d'être écrit sous une
      // adresse que le navigateur a peut-être déjà en cache.
      setUrl(URL.createObjectURL(file));
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "dépôt impossible");
    } finally {
      setBusy(null);
    }
  };

  const remove = async () => {
    setError(null);
    setBusy("retrait");
    try {
      const response = await fetch(`/api/courses/${courseId}/visuel`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ key: "" }),
      });
      if (!response.ok) throw new Error("retrait refusé");
      setUrl("");
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "retrait impossible");
    } finally {
      setBusy(null);
    }
  };

  return (
    <section className="surface-card p-5">
      <h2 className="text-sm font-medium">Visuel de la formation</h2>
      <p className="mt-1.5 text-xs text-ink-2">
        Affiché dans l&apos;espace de vos apprenants. Sans visuel, une bande dans
        votre couleur porte les initiales de la formation.
      </p>

      <div className="mt-4 max-w-sm">
        <CourseCover title={title} url={url || undefined} />
      </div>

      <input
        ref={input}
        type="file"
        accept={ACCEPTED}
        hidden
        onChange={(event) => {
          const file = event.target.files?.[0];
          if (file) void upload(file);
          event.target.value = "";
        }}
      />

      <div className="mt-4 flex flex-wrap items-center gap-3">
        <button
          type="button"
          className="btn-secondary"
          disabled={busy !== null}
          onClick={() => input.current?.click()}
        >
          {busy === "dépôt" ? "Dépôt…" : url ? "Remplacer" : "Déposer un visuel"}
        </button>
        {url && (
          <button type="button" className="btn-ghost" disabled={busy !== null} onClick={remove}>
            Retirer
          </button>
        )}
        <span className="text-2xs text-ink-3">JPEG, PNG ou WebP · format large</span>
      </div>

      {error && <p className="mt-3 text-xs text-danger">{error}</p>}
    </section>
  );
}
