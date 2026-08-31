"use client";

import { useCallback, useState } from "react";

/**
 * Chargement de la suite d'une liste, par curseur.
 *
 * Pas de numéros de page, et ce n'est pas un renoncement : DynamoDB ne sait pas
 * sauter au millième élément sans lire les neuf cent quatre-vingt-dix-neuf
 * premiers. Un écran qui promettrait « page 7 sur 12 » paierait ce parcours à
 * chaque clic, et le total lui-même ne serait pas connu sans tout compter.
 *
 * Le composant ne sait rien de ce qu'il charge : il appelle une adresse, reçoit
 * une tranche et un curseur, et rend les éléments avec la fonction qu'on lui
 * donne. Une même mécanique pour les contacts, les sessions, le journal des
 * envois — et une seule à corriger le jour où elle se trompe.
 */
export function LoadMore<T>({
  endpoint,
  field,
  initialCursor,
  render,
  label = "Charger la suite",
}: {
  /** Adresse du relais, sans paramètre de pagination. */
  endpoint: string;
  /** Nom du tableau dans la réponse : « contacts », « sessions »… */
  field: string;
  /** Curseur rendu par le premier chargement, fait côté serveur. */
  initialCursor?: string;
  render: (items: T[]) => React.ReactNode;
  label?: string;
}) {
  const [items, setItems] = useState<T[]>([]);
  const [cursor, setCursor] = useState(initialCursor ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setError(null);
    setBusy(true);
    try {
      const url = new URL(endpoint, window.location.origin);
      url.searchParams.set("curseur", cursor);
      const response = await fetch(url);
      const body = (await response.json()) as Record<string, unknown>;
      if (!response.ok) throw new Error(String(body.error ?? "chargement impossible"));

      const batch = (body[field] as T[] | null) ?? [];
      setItems((precedents) => [...precedents, ...batch]);
      // Le curseur est le seul signal de fin fiable : une tranche plus courte
      // que la limite n'en est pas un, DynamoDB bornant aussi par la taille
      // lue.
      setCursor(String(body.cursor ?? ""));
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "chargement impossible");
    } finally {
      setBusy(false);
    }
  }, [cursor, endpoint, field]);

  return (
    <>
      {items.length > 0 && render(items)}

      {error && <p className="mt-3 text-xs text-danger">{error}</p>}

      {cursor && (
        <div className="mt-4 flex justify-center">
          <button type="button" className="btn-secondary" disabled={busy} onClick={load}>
            {busy ? "Chargement…" : label}
          </button>
        </div>
      )}
    </>
  );
}
