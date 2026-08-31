import type { Metadata } from "next";
import { ConsoleRows } from "@/components/app/console-rows";
import { ConsoleShell } from "@/components/app/console-shell";

export const metadata: Metadata = { title: "Entreprises" };

/** Une nature, sur toute la plateforme, regroupée par organisme. */
export default async function Page() {
  return (
    <ConsoleShell
      courant="/admin/entreprises"
      chapeau="L'entreprise est la partie qui signe la convention quand elle envoie ses salariés en formation."
    >
      <ConsoleRows vue="entreprises" aide="L'entreprise est la partie qui signe la convention quand elle envoie ses salariés en formation." />
    </ConsoleShell>
  );
}
