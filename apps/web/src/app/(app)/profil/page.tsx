import type { Metadata } from "next";
import { ProfilCompte, type Compte } from "@/components/app/profil-compte";
import { apiFetch } from "@/lib/api";

export const metadata: Metadata = { title: "Mon compte" };

/**
 * Le compte, dans l'espace de l'organisme et dans celui de l'équipe.
 *
 * Les deux partagent la même coque, donc le même écran. Ce qui change est ce
 * qu'on retrouve depuis là : un administrateur va vers son organisme et son
 * équipe, l'équipe lemlearn vers sa console.
 */
export default async function ProfilPage() {
  const data = await apiFetch<{
    compte: Compte;
    org: { id: string; name: string };
    impersonatedBy?: string;
  }>("/v1/profil");

  const equipe = data.compte.role === "superadmin" && !data.impersonatedBy;

  const liens = equipe
    ? [
        {
          href: "/admin/organismes",
          label: "Organismes",
          aide: "Les clients de la plateforme et l'accès à leur espace.",
        },
        {
          href: "/admin/journal",
          label: "Journal",
          aide: "Connexions, accès, signatures — dont les vôtres.",
        },
      ]
    : [
        {
          href: "/organisme",
          label: "Mon organisme",
          aide: "Identité juridique, mentions légales, marque.",
        },
        {
          href: "/equipe",
          label: "Équipe",
          aide: "Qui a un accès à cet espace, et ce qu'il peut faire.",
        },
        {
          href: "/factures",
          label: "Factures",
          aide: "Ce que votre organisme facture à ses clients.",
        },
      ];

  return (
    <>
      <header className="flex h-14 items-center gap-3 border-b border-line px-6">
        <h1 className="text-sm font-medium">Mon compte</h1>
      </header>

      <div className="mx-auto max-w-3xl px-6 py-6">
        <ProfilCompte
          compte={data.compte}
          orgName={data.org.name}
          impersonated={Boolean(data.impersonatedBy)}
          liens={liens}
        />
      </div>
    </>
  );
}
