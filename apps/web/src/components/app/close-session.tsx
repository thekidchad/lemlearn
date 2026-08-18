"use client";

import { useActionState } from "react";
import { closeSession, type FormState } from "@/app/actions/crm";

/**
 * Clôture d'une session.
 *
 * La clôture n'est pas qu'un drapeau : elle programme la satisfaction à froid
 * pour chaque inscrit, à trois mois. Le bouton le dit, parce que c'est la
 * conséquence qu'on oublie — et l'indicateur que les auditeurs réclament.
 */
export function CloseSessionButton({ sessionId }: { sessionId: string }) {
  const [state, action, pending] = useActionState<FormState, FormData>(closeSession, {});

  return (
    <form action={action} className="flex items-center gap-2">
      <input type="hidden" name="sessionId" value={sessionId} />
      <button
        type="submit"
        disabled={pending}
        title="Clôture la session et programme la satisfaction à froid à trois mois"
        className="h-8 rounded-md border border-line px-2.5 text-xs text-ink-2 hover:border-accent hover:text-ink disabled:opacity-50"
      >
        {pending ? "Clôture…" : "Clôturer"}
      </button>
      {state.error && <span className="text-2xs text-danger">{state.error}</span>}
      {state.ok && <span className="text-2xs text-ok">Relances programmées.</span>}
    </form>
  );
}
