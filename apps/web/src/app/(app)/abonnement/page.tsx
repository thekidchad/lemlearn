import type { Metadata } from "next";
import { SubscriptionPlans } from "@/components/app/subscription-plans";
import { apiFetch } from "@/lib/api";
import type { Plan, Usage } from "@/app/(app)/admin/page";

export const metadata: Metadata = { title: "Abonnement" };

interface Subscription {
  org: { id: string; name: string; plan: string; createdAt: string };
  plan: Plan;
  plans: Plan[];
  usage?: Usage;
  overage?: string[];
  selfServe: boolean;
}

export default async function AbonnementPage({
  searchParams,
}: PageProps<"/abonnement">) {
  const query = await searchParams;
  const data = await apiFetch<Subscription>("/v1/abonnement");

  return (
    <>
      <header className="flex h-14 items-center border-b border-line px-6">
        <h1 className="text-sm font-medium">Abonnement</h1>
      </header>

      <div className="mx-auto max-w-4xl px-6 py-6">
        {query.paye === "1" && (
          <p className="mb-5 rounded-lg border border-ok/40 bg-ok/10 px-3 py-2 text-xs text-ok">
            Paiement enregistré. La formule s&apos;applique dès que notre
            prestataire nous le confirme — quelques secondes en général.
          </p>
        )}

        <div className="surface-card p-5">
          <p className="font-mono text-2xs tracking-wide text-ink-3 uppercase">
            Formule en cours
          </p>
          <p className="mt-1.5 text-xl font-semibold tracking-[-0.03em]">
            {data.plan.label}
            {data.plan.priceCents > 0 && (
              <span className="ml-2 text-sm font-normal text-ink-3" data-numeric>
                {data.plan.priceCents / 100} € / mois HT
              </span>
            )}
          </p>
          <p className="mt-1.5 text-xs text-ink-2">{data.plan.description}</p>

          {data.usage && (
            <dl className="mt-5 grid grid-cols-2 gap-x-6 gap-y-2 border-t border-line pt-4 text-xs sm:grid-cols-3">
              <Line label="Apprenants" value={data.usage.learners} max={data.plan.maxLearners} />
              <Line label="Dossiers" value={data.usage.files} />
              <Line label="Sessions" value={data.usage.sessions} />
              <Line
                label="Signatures ce mois"
                value={data.usage.signatures}
                max={data.plan.maxSignatures}
              />
              <Line
                label="Vidéo (heures)"
                value={Math.round(data.usage.videoMs / 3_600_000)}
                max={data.plan.maxVideoHours}
              />
              <Line
                label="Stockage (Go)"
                value={Math.round(data.usage.storageMb / 1024)}
                max={data.plan.maxStorageGb}
              />
            </dl>
          )}

          {data.overage && data.overage.length > 0 && (
            <p className="mt-4 rounded-lg border border-warn/40 bg-warn/10 px-3 py-2 text-2xs text-warn">
              {/* Un dépassement ne coupe rien : le dire évite l'inquiétude
                  autant que la mauvaise surprise. */}
              Vous dépassez votre formule ({data.overage.join(", ")}). Rien n&apos;est
              bloqué — vos sessions continuent — mais le palier suivant vous
              coûtera moins cher que le dépassement.
            </p>
          )}
        </div>

        <SubscriptionPlans
          plans={data.plans}
          current={data.plan.code}
          selfServe={data.selfServe}
        />

        <p className="mt-6 text-2xs text-ink-3">
          Résilier ne supprime rien : vos documents scellés restent archivés et
          consultables, et l&apos;export complet de chaque dossier reste
          disponible. La réversibilité n&apos;est pas une option de la formule.
        </p>
      </div>
    </>
  );
}

function Line({ label, value, max }: { label: string; value: number; max?: number }) {
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
