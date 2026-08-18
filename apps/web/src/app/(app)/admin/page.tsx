import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { OrgRow } from "@/components/app/org-row";
import { apiFetch, ApiError } from "@/lib/api";

export const metadata: Metadata = { title: "Organisations" };

export interface Plan {
  code: string;
  label: string;
  priceCents: number;
  maxLearners: number;
  maxStorageGb: number;
  maxSignatures: number;
  maxVideoHours: number;
  description: string;
}

export interface Usage {
  learners: number;
  files: number;
  sessions: number;
  signatures: number;
  videoMs: number;
  storageMb: number;
}

export interface Org {
  orgId: string;
  name: string;
  plan: string;
  owner?: string;
  priceCents: number;
  usage?: Usage;
  overage?: string[];
  createdAt: string;
}

export default async function AdminPage() {
  let data: { orgs: Org[]; mrrCents: number; plans: Plan[] };
  try {
    data = await apiFetch("/v1/admin/orgs");
  } catch (error) {
    // Un compte qui n'est pas de l'équipe ne doit pas apprendre que cet écran
    // existe : 403 se rend en 404.
    if (error instanceof ApiError && (error.status === 403 || error.status === 404)) notFound();
    throw error;
  }

  const overdue = data.orgs.filter((org) => (org.overage?.length ?? 0) > 0);

  return (
    <>
      <header className="flex h-14 items-center justify-between border-b border-line px-6">
        <h1 className="text-sm font-medium">Organisations</h1>
        <p className="font-mono text-2xs text-ink-3" data-numeric>
          {data.orgs.length} client{data.orgs.length > 1 ? "s" : ""} ·{" "}
          {euros(data.mrrCents)} / mois
        </p>
      </header>

      <div className="px-6 py-6">
        <div className="grid grid-cols-3 gap-px overflow-hidden rounded-xl border border-line bg-line">
          <Stat label="revenu mensuel" value={euros(data.mrrCents)} />
          <Stat
            label="en essai"
            value={String(data.orgs.filter((org) => org.plan === "trial").length)}
          />
          <Stat label="en dépassement" value={String(overdue.length)} tone={overdue.length > 0} />
        </div>

        {overdue.length > 0 && (
          <p className="mt-4 rounded-lg border border-warn/40 bg-warn/10 px-3 py-2 text-2xs text-warn">
            {/* Dire quoi faire, pas seulement qu'il y a un problème. */}
            {overdue.length === 1 ? "Une organisation dépasse" : `${overdue.length} organisations dépassent`}{" "}
            leur formule. Un dépassement ne coupe rien : il se règle en changeant
            de palier, pas en bloquant une session de formation.
          </p>
        )}

        <div className="mt-6 space-y-px overflow-hidden rounded-xl border border-line bg-line">
          {data.orgs.map((org) => (
            <OrgRow key={org.orgId} org={org} plans={data.plans} />
          ))}
        </div>
      </div>
    </>
  );
}

function Stat({ label, value, tone }: { label: string; value: string; tone?: boolean }) {
  return (
    <div className="bg-surface-1 px-4 py-3.5">
      <p className="font-mono text-2xs tracking-wide text-ink-3 uppercase">{label}</p>
      <p
        className={`mt-1 text-lg font-semibold tracking-[-0.02em] ${tone ? "text-warn" : ""}`}
        data-numeric
      >
        {value}
      </p>
    </div>
  );
}

function euros(cents: number): string {
  return new Intl.NumberFormat("fr-FR", {
    style: "currency",
    currency: "EUR",
    maximumFractionDigits: 0,
  }).format(cents / 100);
}
