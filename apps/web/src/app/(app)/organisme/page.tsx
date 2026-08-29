import type { Metadata } from "next";
import { BrandForm } from "@/components/app/brand-form";
import { LegalForm, type OrgLegal } from "@/components/app/legal-form";
import { apiFetch, type Brand } from "@/lib/api";

export const metadata: Metadata = { title: "Votre organisme" };

interface BrandState {
  brand: { name?: string; logoKey?: string; accent?: string; supportEmail?: string };
  resolved: Brand;
  orgName: string;
}

/**
 * Réglages de l'organisme.
 *
 * Un seul écran pour l'instant, celui de l'identité visible — c'est le premier
 * geste d'un organisme qui ouvre son compte, et celui qui fait disparaître
 * notre nom de tout ce que voient ses apprenants.
 */
export default async function OrganismePage() {
  const state = await apiFetch<BrandState>("/v1/marque");
  const { org } = await apiFetch<{ org: OrgLegal }>("/v1/organisme");

  return (
    <div className="mx-auto max-w-4xl px-6 py-8">
      <p className="eyebrow">Votre organisme</p>
      <h1 className="mt-1 text-xl font-semibold tracking-[-0.03em]">Identité visible</h1>
      <p className="mt-2 max-w-2xl text-sm text-ink-2">
        Vos apprenants et vos signataires ne voient que vous. Ce réglage
        s&apos;applique immédiatement, partout : espace apprenant, pages de
        signature, courriels d&apos;invitation et de satisfaction.
      </p>

      <div className="mt-6">
        <BrandForm base="/api/marque" initial={state} />
      </div>

      <h2 className="learner-heading mt-12">Informations légales</h2>
      <p className="mt-2 max-w-2xl text-sm text-ink-2">
        Reprises sur chaque convention, contrat et attestation. Deux d&apos;entre
        elles sont imposées par le code du travail : sans elles, vos documents ne
        sont pas conformes.
      </p>

      <div className="mt-6">
        <LegalForm initial={org} />
      </div>
    </div>
  );
}
