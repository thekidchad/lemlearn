import type { Metadata } from "next";
import { ProfilCompte, type Compte } from "@/components/app/profil-compte";
import { apiFetch } from "@/lib/api";

export const metadata: Metadata = { title: "Mon compte" };

/**
 * Le compte d'un stagiaire.
 *
 * Le même écran que celui de l'organisme : c'est la même chose qu'on y fait.
 * Ce qui change, ce sont les pièces qu'on retrouve à côté — les siennes.
 */
export default async function ProfilApprenantPage() {
  const data = await apiFetch<{
    compte: Compte;
    org: { id: string; name: string };
    impersonatedBy?: string;
  }>("/v1/profil");

  return (
    <div className="mx-auto max-w-2xl px-5 py-12 sm:px-8 sm:py-16">
      <h1 className="learner-title">Mon compte</h1>
      <p className="learner-body mt-3">
        Votre photo, votre nom et votre mot de passe. Vos pièces sont juste en
        dessous.
      </p>

      <div className="mt-10">
        <ProfilCompte
          compte={data.compte}
          orgName={data.org.name}
          impersonated={Boolean(data.impersonatedBy)}
          liens={[
            {
              href: "/apprenant/documents",
              label: "Mes documents",
              aide: "Les pièces que vous avez signées, avec leur empreinte.",
            },
            {
              href: "/apprenant/informations",
              label: "Mes informations",
              aide: "Ce que votre organisme a enregistré vous concernant.",
            },
            {
              href: "/apprenant",
              label: "Mon parcours",
              aide: "Vos formations et votre progression.",
            },
          ]}
        />
      </div>
    </div>
  );
}
