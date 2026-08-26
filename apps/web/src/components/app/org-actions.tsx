"use client";

import { useRouter } from "next/navigation";
import { useState, useTransition } from "react";
import type { Plan } from "@/app/(app)/admin/page";

/**
 * Actions du support sur une organisation cliente.
 *
 * Ouvrir une session chez un client est un besoin réel — personne ne dépanne
 * un dossier qu'il ne peut pas voir — et un danger tout aussi réel. Le
 * garde-fou n'est pas de l'interdire, mais de le rendre impossible à
 * dissimuler : le bouton le dit, et le client le voit dans sa propre barre
 * latérale.
 */
export function OrgActions({
  orgId,
  plan,
  plans,
}: {
  orgId: string;
  plan: string;
  plans: Plan[];
}) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const call = async (action: "plan" | "impersonate", body?: unknown) => {
    setBusy(true);
    setError(null);
    try {
      const response = await fetch(`/api/admin/${orgId}/${action}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body ?? {}),
      });
      const parsed = (await response.json()) as { error?: string };
      if (!response.ok) throw new Error(parsed.error ?? "action impossible");
      if (action === "impersonate") {
        router.push("/pipeline");
        return;
      }
      startTransition(() => router.refresh());
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "action impossible");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex flex-wrap items-center gap-2">
      <select
        defaultValue={plan}
        disabled={busy || pending}
        onChange={(event) => call("plan", { plan: event.target.value })}
        className="h-8 rounded-md border border-line bg-surface-0 px-2 text-xs outline-none focus:border-accent"
      >
        {plans.map((entry) => (
          <option key={entry.code} value={entry.code}>
            {entry.label} · {entry.priceCents / 100} €
          </option>
        ))}
      </select>

      <button
        type="button"
        onClick={() => call("impersonate")}
        disabled={busy || pending}
        title="Ouvre une session sur cette organisation. L'accès est tracé et visible du client."
        className="h-8 rounded-md border border-line px-2.5 text-xs text-ink-2 hover:border-accent hover:text-ink disabled:opacity-50"
      >
        Ouvrir la session
      </button>

      {error && <span className="text-2xs text-danger">{error}</span>}
    </div>
  );
}
