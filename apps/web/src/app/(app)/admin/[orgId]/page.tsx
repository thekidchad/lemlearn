import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { OrgActions } from "@/components/app/org-actions";
import { apiFetch, ApiError } from "@/lib/api";
import type { Plan, Usage } from "@/app/(app)/admin/page";

export const metadata: Metadata = { title: "Organisation" };

interface Detail {
  org: { id: string; name: string; plan: string; siret?: string; createdAt: string };
  plan: Plan;
  plans: Plan[];
  usage?: Usage;
  overage?: string[];
  users?: { id: string; email: string; firstName: string; lastName: string; role: string }[];
  timeline?: {
    seq: number;
    at: string;
    action: string;
    actor: { label?: string; id: string };
    payload?: Record<string, unknown>;
  }[];
}

const ACTIONS: Record<string, string> = {
  "billing.plan_changed": "Changement de formule",
  "admin.impersonated": "Accès de l'équipe lemlearn",
  "file.created": "Organisation créée",
};

export default async function OrgPage({ params }: PageProps<"/admin/[orgId]">) {
  const { orgId } = await params;

  let detail: Detail;
  try {
    detail = await apiFetch(`/v1/admin/orgs/${orgId}`);
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) notFound();
    throw error;
  }

  const usage = detail.usage;

  return (
    <>
      <header className="flex h-14 items-center gap-3 border-b border-line px-6">
        <Link href="/admin" className="text-xs text-ink-3 hover:text-ink">
          Organisations
        </Link>
        <span className="text-ink-3">/</span>
        <span className="truncate text-xs text-ink-2">{detail.org.name}</span>
        <span className="ml-auto font-mono text-2xs text-ink-3">
          client depuis le {new Date(detail.org.createdAt).toLocaleDateString("fr-FR")}
        </span>
      </header>

      <div className="mx-auto max-w-4xl space-y-6 px-6 py-6">
        <section className="surface-card p-5">
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div>
              <h1 className="text-xl font-semibold tracking-[-0.03em]">{detail.org.name}</h1>
              <p className="mt-1 font-mono text-2xs text-ink-3">
                {detail.plan.label} · {detail.plan.priceCents / 100} € / mois HT
                {detail.org.siret ? ` · SIRET ${detail.org.siret}` : ""}
              </p>
            </div>
            <OrgActions orgId={orgId} plan={detail.org.plan} plans={detail.plans} />
          </div>

          {usage && (
            <dl className="mt-5 grid grid-cols-2 gap-x-6 gap-y-3 border-t border-line pt-4 text-xs sm:grid-cols-3">
              <Metric label="Apprenants" value={usage.learners} max={detail.plan.maxLearners} />
              <Metric label="Dossiers" value={usage.files} />
              <Metric label="Sessions" value={usage.sessions} />
              <Metric label="Signatures ce mois" value={usage.signatures} max={detail.plan.maxSignatures} />
              <Metric
                label="Vidéo (heures)"
                value={Math.round(usage.videoMs / 3_600_000)}
                max={detail.plan.maxVideoHours}
              />
              <Metric
                label="Stockage (Go)"
                value={Math.round(usage.storageMb / 1024)}
                max={detail.plan.maxStorageGb}
              />
            </dl>
          )}

          {detail.overage && detail.overage.length > 0 && (
            <p className="mt-4 rounded-lg border border-warn/40 bg-warn/10 px-3 py-2 text-2xs text-warn">
              Dépasse sa formule : {detail.overage.join(", ")}. Rien n&apos;est bloqué —
              c&apos;est un sujet de conversation, pas une sanction.
            </p>
          )}
        </section>

        <section className="surface-card p-5">
          <h2 className="text-sm font-medium">Comptes</h2>
          {!detail.users || detail.users.length === 0 ? (
            <p className="mt-2 text-xs text-ink-3">Aucun compte.</p>
          ) : (
            <table className="mt-3 w-full text-left text-xs">
              <thead className="text-2xs tracking-wide text-ink-3 uppercase">
                <tr>
                  <th className="py-1.5 font-medium">Nom</th>
                  <th className="py-1.5 font-medium">Adresse</th>
                  <th className="py-1.5 font-medium">Rôle</th>
                </tr>
              </thead>
              <tbody>
                {detail.users.map((user) => (
                  <tr key={user.id} className="border-t border-line/60">
                    <td className="py-1.5">
                      {user.firstName} {user.lastName}
                    </td>
                    <td className="py-1.5 font-mono text-2xs text-ink-2">{user.email}</td>
                    <td className="py-1.5 text-ink-3">{user.role}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>

        <section className="surface-card p-5">
          <h2 className="text-sm font-medium">Journal de la relation</h2>
          <p className="mt-1.5 text-xs text-ink-2">
            {/* Ce journal est celui de l'organisation, pas d'un dossier : les
                deux ne se mélangent pas, sans quoi l'export d'un dossier
                probatoire porterait des faits commerciaux. */}
            Changements de formule et accès de l&apos;équipe lemlearn. Chaque entrée
            est chaînée à la précédente.
          </p>

          {!detail.timeline || detail.timeline.length === 0 ? (
            <p className="mt-3 text-xs text-ink-3">Rien à signaler.</p>
          ) : (
            <ol className="mt-4 space-y-2.5">
              {detail.timeline.map((event) => (
                <li key={event.seq} className="flex gap-3">
                  <span className="mt-1.5 size-1.5 shrink-0 rounded-full bg-ink-3" />
                  <div className="min-w-0">
                    <p className="text-xs">{ACTIONS[event.action] ?? event.action}</p>
                    <p className="mt-0.5 font-mono text-2xs text-ink-3">
                      {new Date(event.at).toLocaleString("fr-FR")} ·{" "}
                      {event.actor.label || event.actor.id}
                      {event.payload && Object.keys(event.payload).length > 0
                        ? ` · ${Object.entries(event.payload)
                            .map(([key, value]) => `${key} ${String(value)}`)
                            .join(", ")}`
                        : ""}
                    </p>
                  </div>
                </li>
              ))}
            </ol>
          )}
        </section>

        <section className="surface-card p-5">
          <h2 className="text-sm font-medium">Courriels envoyés pour ce client</h2>
          <p className="mt-1.5 text-xs text-ink-2">
            <Link href={`/admin/emails?orgId=${orgId}`} className="underline hover:text-ink">
              Ouvrir le journal des envois filtré sur cette organisation
            </Link>
          </p>
        </section>
      </div>
    </>
  );
}

function Metric({ label, value, max }: { label: string; value: number; max?: number }) {
  const over = max !== undefined && max > 0 && value > max;
  return (
    <div>
      <dt className="text-ink-3">{label}</dt>
      <dd className={`mt-0.5 ${over ? "text-warn" : "text-ink"}`} data-numeric>
        {value}
        {max ? <span className="text-ink-3"> / {max}</span> : null}
      </dd>
    </div>
  );
}
