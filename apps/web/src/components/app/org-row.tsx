"use client";

import { useRouter } from "next/navigation";
import { useState, useTransition } from "react";
import type { Org, Plan } from "@/app/(app)/admin/page";

/**
 * Une organisation cliente, avec sa formule et sa consommation.
 *
 * Le dépassement s'affiche, il ne coupe rien : suspendre un organisme en
 * pleine session de formation ferait plus de dégâts qu'un mois facturé au
 * palier au-dessus, et c'est le genre de décision qui se prend au téléphone.
 */
export function OrgRow({ org, plans }: { org: Org; plans: Plan[] }) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const call = async (action: "plan" | "impersonate", body?: unknown) => {
    setBusy(true);
    setError(null);
    try {
      const response = await fetch(`/api/admin/${org.orgId}/${action}`, {
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

  const plan = plans.find((entry) => entry.code === org.plan);

  return (
    <div className="bg-surface-1 px-4 py-3.5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="min-w-0">
          <p className="truncate text-sm font-medium">{org.name}</p>
          <p className="truncate text-2xs text-ink-3">
            {org.owner ?? "aucun contact"} · depuis le{" "}
            {new Date(org.createdAt).toLocaleDateString("fr-FR")}
          </p>
        </div>

        <div className="flex items-center gap-2">
          <select
            defaultValue={org.plan}
            disabled={busy || pending}
            onChange={(event) => call("plan", { plan: event.target.value })}
            className="h-8 rounded-md border border-line bg-surface-0 px-2 text-xs outline-none focus:border-accent"
          >
            {plans.map((entry) => (
              <option key={entry.code} value={entry.code}>
                {entry.label} · {entry.priceCents / 100} €
              </option>
            ))}
            {!plan && <option value={org.plan}>{org.plan} (hors catalogue)</option>}
          </select>

          <button
            type="button"
            onClick={() => call("impersonate")}
            disabled={busy || pending}
            className="h-8 rounded-md border border-line px-2.5 text-xs text-ink-2 hover:border-accent hover:text-ink disabled:opacity-50"
            title="Ouvre une session sur cette organisation. L'accès est tracé et visible du client."
          >
            Ouvrir la session
          </button>
        </div>
      </div>

      {org.usage && (
        <dl className="mt-3 flex flex-wrap gap-x-5 gap-y-1 font-mono text-2xs text-ink-3">
          <Metric label="apprenants" value={org.usage.learners} max={plan?.maxLearners} />
          <Metric label="dossiers" value={org.usage.files} />
          <Metric label="sessions" value={org.usage.sessions} />
          <Metric label="signatures" value={org.usage.signatures} max={plan?.maxSignatures} />
          <Metric
            label="vidéo (h)"
            value={Math.round(org.usage.videoMs / 3_600_000)}
            max={plan?.maxVideoHours}
          />
          <Metric
            label="stockage (Go)"
            value={Math.round(org.usage.storageMb / 1024)}
            max={plan?.maxStorageGb}
          />
        </dl>
      )}

      {org.overage && org.overage.length > 0 && (
        <p className="mt-2 text-2xs text-warn">Dépasse : {org.overage.join(", ")}</p>
      )}

      {error && <p className="mt-2 text-2xs text-danger">{error}</p>}
    </div>
  );
}

function Metric({ label, value, max }: { label: string; value: number; max?: number }) {
  const over = max !== undefined && max > 0 && value > max;
  return (
    <div className="flex gap-1.5">
      <dt>{label}</dt>
      <dd className={over ? "text-warn" : "text-ink-2"} data-numeric>
        {value}
        {max ? ` / ${max}` : ""}
      </dd>
    </div>
  );
}
