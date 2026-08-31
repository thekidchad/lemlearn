import type { Metadata } from "next";
import { ConsoleRows } from "@/components/app/console-rows";
import { ConsoleShell } from "@/components/app/console-shell";

export const metadata: Metadata = { title: "Financeurs" };

/** Une nature, sur toute la plateforme, regroupée par organisme. */
export default async function Page() {
  return (
    <ConsoleShell
      courant="/admin/financeurs"
      chapeau="OPCO, France Travail, Caisse des Dépôts — celui qui prend la formation en charge."
    >
      <ConsoleRows vue="financeurs" aide="OPCO, France Travail, Caisse des Dépôts — celui qui prend la formation en charge." />
    </ConsoleShell>
  );
}
