"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";

/**
 * Ce qui manque à l'identité juridique de l'organisme.
 *
 * L'avertissement plutôt que le blocage : un organisme qui vient d'ouvrir son
 * espace a le droit de le regarder avant de saisir son numéro de déclaration
 * d'activité. Mais il doit savoir tout de suite ce qui l'attend, parce que ces
 * mentions ne sont pas des préférences — elles doivent figurer sur chaque
 * convention, et une convention à laquelle il en manque une n'est pas
 * opposable. S'en apercevoir au contrôle, c'est trop tard pour la refaire.
 *
 * Il ne se referme pas définitivement : replier vaut pour la visite en cours,
 * et il reparaît à la suivante. Un bandeau qu'on peut faire taire pour toujours
 * finit par ne plus rien dire.
 */
interface Manque {
  champ: string;
  label: string;
  pourquoi: string;
}

export function LegalWarning({ missing }: { missing: Manque[] }) {
  const pathname = usePathname();
  const [replie, setReplie] = useState(true);

  // Inutile de rappeler ce qui manque sur l'écran où on le remplit.
  if (missing.length === 0 || pathname === "/organisme") return null;

  return (
    <div className="border-b border-warn/30 bg-warn/10 px-6 py-2.5">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
        <span aria-hidden className="text-warn">
          ▲
        </span>
        <p className="text-xs text-warn">
          L&apos;identité juridique de votre organisme est incomplète —{" "}
          {missing.length} mention{missing.length > 1 ? "s" : ""} manquante
          {missing.length > 1 ? "s" : ""}. Vos conventions et attestations
          partiraient sans elle{missing.length > 1 ? "s" : ""}.
        </p>

        <button
          type="button"
          onClick={() => setReplie((etat) => !etat)}
          className="text-2xs text-warn underline hover:text-ink"
        >
          {replie ? "Voir le détail" : "Replier"}
        </button>

        <Link href="/organisme" className="btn-secondary ml-auto">
          Compléter
        </Link>
      </div>

      {!replie && (
        <ul className="mt-2.5 space-y-1 pl-6">
          {missing.map((manque) => (
            <li key={manque.champ} className="text-2xs text-ink-2">
              <span className="font-medium text-warn">{manque.label}</span> —{" "}
              {manque.pourquoi}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
