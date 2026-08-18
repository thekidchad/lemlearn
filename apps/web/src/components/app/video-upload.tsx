"use client";

import { useRouter } from "next/navigation";
import { useRef, useState } from "react";

/**
 * Dépôt de la vidéo d'un module.
 *
 * Le fichier part directement du navigateur vers S3, par URL présignée : le
 * faire transiter par l'API imposerait de dimensionner une fonction pour une
 * heure de vidéo, et de payer pour la traverser deux fois.
 *
 * La durée est lue dans le navigateur pour que le module soit exploitable tout
 * de suite ; celle mesurée au transcodage la remplacera, et c'est elle qui
 * sert de dénominateur à l'assiduité — un client peut mentir sur une durée.
 */
export function VideoUpload({
  courseId,
  moduleId,
  hasAsset,
}: {
  courseId: string;
  moduleId: string;
  hasAsset: boolean;
}) {
  const router = useRouter();
  const input = useRef<HTMLInputElement | null>(null);
  const [step, setStep] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const upload = async (file: File) => {
    setError(null);
    try {
      setStep("réservation");
      const reserve = await fetch("/api/videos", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ contentType: file.type || "video/mp4" }),
      });
      const reserved = (await reserve.json()) as {
        asset?: { id: string };
        uploadUrl?: string;
        error?: string;
      };
      if (!reserve.ok || !reserved.uploadUrl || !reserved.asset) {
        throw new Error(reserved.error ?? "réservation impossible");
      }

      setStep("téléversement");
      const put = await fetch(reserved.uploadUrl, {
        method: "PUT",
        headers: { "Content-Type": file.type || "video/mp4" },
        body: file,
      });
      if (!put.ok) throw new Error(`le dépôt a échoué (${put.status})`);

      setStep("transcodage");
      const durationMs = await durationOf(file);
      const done = await fetch(`/api/videos?id=${reserved.asset.id}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ durationMs }),
      });
      const finished = (await done.json()) as { error?: string };
      if (!done.ok) throw new Error(finished.error ?? "transcodage refusé");

      const attach = await fetch(`/api/courses/${courseId}/modules/${moduleId}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ assetId: reserved.asset.id, durationMs }),
      });
      const attached = (await attach.json()) as { error?: string };
      if (!attach.ok) throw new Error(attached.error ?? "rattachement au module refusé");

      setStep(null);
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "dépôt impossible");
      setStep(null);
    }
  };

  return (
    <div className="flex items-center gap-2">
      <input
        ref={input}
        type="file"
        accept="video/*"
        hidden
        onChange={(event) => {
          const file = event.target.files?.[0];
          if (file) void upload(file);
        }}
      />
      <button
        type="button"
        onClick={() => input.current?.click()}
        disabled={step !== null}
        className="h-8 rounded-md border border-line px-2.5 text-xs text-ink-2 hover:border-accent hover:text-ink disabled:opacity-50"
      >
        {step ? `${step}…` : hasAsset ? "Remplacer la vidéo" : "Téléverser une vidéo"}
      </button>
      {error && <span className="max-w-56 text-2xs text-danger">{error}</span>}
    </div>
  );
}

/** durationOf lit la durée du fichier sans l'envoyer nulle part. */
function durationOf(file: File): Promise<number> {
  return new Promise((resolve) => {
    const element = document.createElement("video");
    element.preload = "metadata";
    element.onloadedmetadata = () => {
      URL.revokeObjectURL(element.src);
      resolve(Number.isFinite(element.duration) ? Math.round(element.duration * 1000) : 0);
    };
    element.onerror = () => resolve(0);
    element.src = URL.createObjectURL(file);
  });
}
