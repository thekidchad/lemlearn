"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

/**
 * Sortie d'une impersonation.
 *
 * Entrer dans un organisme remplace le cookie de session : sans cette porte,
 * on n'en ressort qu'en se déconnectant, ce qui n'est pas une porte mais une
 * fenêtre. Le bouton rend la main au compte de l'équipe qui l'avait prise.
 *
 * Une session ouverte avant l'existence de cette sortie n'a pas retenu
 * l'adresse de son auteur : on ne peut alors que déconnecter, et on le dit
 * plutôt que d'afficher une erreur sans issue.
 */
export function LeaveImpersonation() {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [fallback, setFallback] = useState(false);

  const leave = async () => {
    setBusy(true);
    try {
      const response = await fetch("/api/impersonation/fin", { method: "POST" });
      if (response.status === 409) {
        setFallback(true);
        return;
      }
      if (!response.ok) throw new Error();
      router.push("/admin");
      router.refresh();
    } catch {
      setFallback(true);
    } finally {
      setBusy(false);
    }
  };

  if (fallback) {
    // La déconnexion est une action serveur, dans la colonne juste en dessous :
    // on y renvoie plutôt que de dupliquer un formulaire ici.
    return (
      <span className="mt-1.5 block text-2xs">
        Session ouverte avant l&apos;existence de ce retour : déconnectez-vous pour
        revenir à l&apos;équipe.
      </span>
    );
  }

  return (
    <button
      type="button"
      onClick={leave}
      disabled={busy}
      className="mt-1.5 text-2xs underline underline-offset-2 hover:no-underline"
    >
      {busy ? "Retour…" : "Revenir à l'équipe"}
    </button>
  );
}
