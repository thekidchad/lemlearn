import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { ConsoleShell } from "@/components/app/console-shell";
import { OpenOrg } from "@/components/app/open-org";
import { OrgRow } from "@/components/app/org-row";
import { apiFetch, ApiError } from "@/lib/api";
import type { Org, Plan } from "@/app/(app)/admin/page";

export const metadata: Metadata = { title: "Organismes" };

/**
 * Les organismes clients, et l'accès à leur espace.
 *
 * Cette liste vivait au bas du tableau de bord, après quatre graphiques :
 * c'est-à-dire nulle part. C'est pourtant l'écran le plus utilisé de la
 * console — on y entre chez un client, et c'est de là que part tout le reste.
 */
export default async function OrganismesPage() {
  let data: { orgs: Org[]; mrrCents: number; plans: Plan[] };
  try {
    data = await apiFetch("/v1/admin/orgs");
  } catch (error) {
    // Un compte qui n'est pas de l'équipe ne doit pas apprendre que cet écran
    // existe : 403 se rend en 404.
    if (error instanceof ApiError && (error.status === 403 || error.status === 404)) notFound();
    throw error;
  }

  return (
    <ConsoleShell
      courant="/admin/organismes"
      chapeau={`${data.orgs.length} organisme${data.orgs.length > 1 ? "s" : ""} sur la plateforme. Ouvrir une session chez l'un d'eux est tracé dans son journal et visible de lui.`}
      action={<OpenOrg />}
    >
      <div className="px-8 py-6">
        <div className="space-y-px overflow-hidden rounded-xl border border-line bg-line">
          {data.orgs.map((org) => (
            <OrgRow key={org.orgId} org={org} plans={data.plans} />
          ))}
        </div>
      </div>
    </ConsoleShell>
  );
}
