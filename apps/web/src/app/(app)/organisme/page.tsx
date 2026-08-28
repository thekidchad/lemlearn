import type { Metadata } from "next";
import { BrandForm } from "@/components/app/brand-form";
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
    </div>
  );
}
