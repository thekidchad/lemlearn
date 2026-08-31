import type { Metadata } from "next";
import { ConsoleRows } from "@/components/app/console-rows";
import { ConsoleShell } from "@/components/app/console-shell";

export const metadata: Metadata = { title: "Stagiaires" };

/** Une nature, sur toute la plateforme, regroupée par organisme. */
export default async function Page() {
  return (
    <ConsoleShell
      courant="/admin/stagiaires"
      chapeau="Les personnes formées chez nos clients. Le code du travail dit « stagiaire » : autant employer le mot que lira un contrôleur."
    >
      <ConsoleRows vue="stagiaires" aide="Les personnes formées chez nos clients. Le code du travail dit « stagiaire » : autant employer le mot que lira un contrôleur." />
    </ConsoleShell>
  );
}
