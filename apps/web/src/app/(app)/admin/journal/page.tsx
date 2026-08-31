import type { Metadata } from "next";
import { JournalRows } from "@/components/app/journal-rows";
import { JournalShell } from "@/components/app/journal-shell";

export const metadata: Metadata = { title: "Journal" };

/**
 * Tout ce qui se passe sur la plateforme, dans l'ordre du temps.
 *
 * Le journal d'audit est rangé par sujet — c'est ce qui permet de prouver
 * qu'une chaîne n'a pas été altérée. Mais ce n'est pas la question qu'on pose
 * quand quelque chose cloche : on demande « qu'est-il arrivé aujourd'hui ».
 */
export default async function JournalPage() {
  return (
    <JournalShell
      courant="/admin/journal"
      chapeau="Connexions, accès de l'équipe, signatures, exports — tous organismes confondus, du plus récent au plus ancien. Chaque ligne porte l'adresse depuis laquelle l'action a été faite."
    >
      <JournalRows />
    </JournalShell>
  );
}
