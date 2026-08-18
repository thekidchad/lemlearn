"use client";

import { useRouter } from "next/navigation";
import { useState, useTransition } from "react";
import { moveFile } from "@/app/actions/crm";
import { STAGES, type Stage } from "@/lib/stages";

/**
 * Déplacement d'un dossier dans le pipeline.
 *
 * Le changement d'étape est un événement de la chaîne de preuve, pas un
 * champ : il s'écrit au journal avec son auteur et son horodatage, et c'est
 * pour cela qu'il passe par l'API plutôt que par un état local.
 */
export function StagePicker({ fileId, stage }: { fileId: string; stage: Stage }) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [error, setError] = useState<string | null>(null);

  return (
    <div className="flex items-center gap-2">
      <select
        defaultValue={stage}
        disabled={pending}
        onChange={(event) => {
          const next = event.target.value;
          startTransition(async () => {
            const result = await moveFile(fileId, next);
            setError(result.error ?? null);
            if (!result.error) router.refresh();
          });
        }}
        className="h-8 rounded-md border border-line bg-surface-0 px-2 text-xs outline-none focus:border-accent"
      >
        {STAGES.map((entry) => (
          <option key={entry.key} value={entry.key}>
            {entry.label}
          </option>
        ))}
      </select>
      {error && <span className="text-2xs text-danger">{error}</span>}
    </div>
  );
}
