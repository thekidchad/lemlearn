import type { Metadata } from "next";
import { TeamTable, type Membre } from "@/components/app/team-table";
import { apiFetch, type Me } from "@/lib/api";

export const metadata: Metadata = { title: "Équipe" };

/**
 * Qui travaille dans l'espace de l'organisme.
 *
 * L'écran manquait entièrement : un organisme était un compte unique, et tout
 * ce qui suppose plusieurs personnes — assigner un dossier, faire contresigner
 * un émargement par le formateur — n'avait donc aucun sens.
 */
export default async function EquipePage() {
  const [{ membres }, me] = await Promise.all([
    apiFetch<{ membres: Membre[] | null }>("/v1/equipe"),
    apiFetch<Me>("/v1/me"),
  ]);

  const peutGerer = me.user.role === "owner" || me.user.role === "admin";

  return (
    <>
      <header className="flex h-14 items-center gap-3 border-b border-line px-6">
        <h1 className="text-sm font-medium">Équipe</h1>
        <p className="ml-3 truncate text-2xs text-ink-3">
          Les personnes qui travaillent dans l&apos;espace de {me.org.name}.
        </p>
      </header>

      <div className="mx-auto max-w-3xl space-y-6 px-6 py-6">
        <TeamTable membres={membres ?? []} moi={me.user.id} peutGerer={peutGerer} />

        {!peutGerer && (
          <p className="text-2xs text-ink-3">
            Seuls le propriétaire et les administrateurs ouvrent ou retirent un
            accès.
          </p>
        )}
      </div>
    </>
  );
}
