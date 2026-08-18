"use client";

import { useActionState } from "react";
import { countersign, type FormState } from "@/app/actions/crm";

/**
 * Contresignature de la feuille d'émargement par le formateur.
 *
 * C'est elle qui ferme la feuille : une feuille signée par les seuls
 * apprenants n'engage personne, et c'est la première chose que regarde un
 * contrôleur.
 */
export function CountersignButton({ sessionId }: { sessionId: string }) {
  const [state, action, pending] = useActionState<FormState, FormData>(countersign, {});

  return (
    <form action={action} className="flex items-center gap-2">
      <input type="hidden" name="sessionId" value={sessionId} />
      <button
        type="submit"
        disabled={pending}
        className="flex h-7 items-center gap-1.5 rounded-md border border-warn/40 px-2.5 text-2xs text-warn hover:border-warn disabled:opacity-50"
      >
        <span className="size-1.5 rounded-full bg-warn" />
        {pending ? "Signature…" : "Contresigner la feuille"}
      </button>
      {state.error && <span className="text-2xs text-danger">{state.error}</span>}
    </form>
  );
}
