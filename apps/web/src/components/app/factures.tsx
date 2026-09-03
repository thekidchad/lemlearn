"use client";

import { useCallback, useEffect, useState } from "react";

/**
 * La facturation d'un organisme à ses clients.
 *
 * L'écran fait respecter la même asymétrie que l'API, et pour la même raison :
 * un brouillon se modifie et se supprime, une facture émise ni l'un ni l'autre.
 * C'est ce qui sépare une comptabilité d'un tableur — la numérotation doit
 * rester continue, et une erreur se corrige par un avoir, pas par une gomme.
 *
 * Le brouillon se compose en PDF lui aussi : l'émission étant irréversible,
 * s'en priver reviendrait à demander de signer sans lire.
 */
interface Ligne {
  label: string;
  quantity: number;
  unitPriceHT: number;
  vatRate: number;
}

interface Facture {
  id: string;
  number?: string;
  status: "brouillon" | "emise" | "payee" | "annulee";
  client: { name: string };
  clientId?: string;
  fileReference?: string;
  lines: Ligne[] | null;
  vatExempt: boolean;
  totalHT: number;
  totalVAT: number;
  totalTTC: number;
  issuedOn?: string;
  dueOn?: string;
  paidAt?: string;
  creditNoteFor?: string;
  cancelledBy?: string;
  notes?: string;
}

const ETATS: Record<Facture["status"], { label: string; classe: string }> = {
  brouillon: { label: "Brouillon", classe: "border border-line-strong text-ink-3" },
  emise: { label: "Émise", classe: "bg-warn/15 text-warn" },
  payee: { label: "Payée", classe: "bg-ok/15 text-ok" },
  annulee: { label: "Annulée", classe: "bg-danger/15 text-danger" },
};

export function Factures({
  clients,
  vatExempt,
}: {
  clients: { id: string; nom: string }[];
  /** Le régime de l'organisme, pour ne pas proposer un taux à un exonéré. */
  vatExempt: boolean;
}) {
  const [factures, setFactures] = useState<Facture[]>([]);
  const [pret, setPret] = useState(false);
  const [ouvert, setOuvert] = useState(false);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [lignes, setLignes] = useState<Ligne[]>([
    { label: "", quantity: 1, unitPriceHT: 0, vatRate: 20 },
  ]);

  const recharger = useCallback(async () => {
    const response = await fetch("/api/factures");
    const body = (await response.json()) as { factures?: Facture[] | null; error?: string };
    if (!response.ok) throw new Error(body.error ?? "lecture impossible");
    setFactures(body.factures ?? []);
  }, []);

  useEffect(() => {
    (async () => {
      try {
        await recharger();
      } catch (failure) {
        setError(failure instanceof Error ? failure.message : "lecture impossible");
      } finally {
        setPret(true);
      }
    })();
  }, [recharger]);

  const agir = async (clef: string, action: () => Promise<Response>) => {
    setError(null);
    setBusy(clef);
    try {
      const response = await action();
      if (!response.ok) {
        const body = (await response.json().catch(() => ({}))) as { error?: string };
        throw new Error(body.error ?? "action refusée");
      }
      await recharger();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "action refusée");
    } finally {
      setBusy(null);
    }
  };

  const creer = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    void agir("creation", async () => {
      const response = await fetch("/api/factures", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          clientId: String(data.get("clientId") ?? ""),
          fileId: String(data.get("fileId") ?? ""),
          dueOn: String(data.get("dueOn") ?? ""),
          paymentTerms: String(data.get("paymentTerms") ?? ""),
          notes: String(data.get("notes") ?? ""),
          lines: lignes.filter((ligne) => ligne.label.trim() !== ""),
        }),
      });
      if (response.ok) {
        form.reset();
        setLignes([{ label: "", quantity: 1, unitPriceHT: 0, vatRate: vatExempt ? 0 : 20 }]);
        setOuvert(false);
      }
      return response;
    });
  };

  const total = lignes.reduce((somme, ligne) => somme + ligne.quantity * ligne.unitPriceHT, 0);

  return (
    <section className="surface-card overflow-hidden">
      <header className="flex flex-wrap items-center gap-3 border-b border-line px-5 py-3">
        <h2 className="text-sm font-medium">Factures</h2>
        <span className="font-mono text-2xs text-ink-3" data-numeric>
          {factures.filter((facture) => facture.status === "emise").length} en attente de
          règlement
        </span>
        <button
          type="button"
          className="btn-primary ml-auto"
          onClick={() => setOuvert((etat) => !etat)}
        >
          {ouvert ? "Annuler" : "Nouvelle facture"}
        </button>
      </header>

      {error && <p className="px-5 py-2 text-xs text-danger">{error}</p>}

      {ouvert && (
        <form onSubmit={creer} className="border-b border-line bg-surface-2 px-5 py-4">
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="block">
              <span className="eyebrow">Client</span>
              <select name="clientId" required className="field mt-1.5">
                {clients.map((client) => (
                  <option key={client.id} value={client.id}>
                    {client.nom}
                  </option>
                ))}
              </select>
            </label>
            <label className="block">
              <span className="eyebrow">Échéance</span>
              <input name="dueOn" type="date" className="field mt-1.5" />
              <span className="mt-1 block text-2xs text-ink-3">
                Trente jours à défaut, comme le prévoit la loi.
              </span>
            </label>
            <label className="block sm:col-span-2">
              <span className="eyebrow">Dossier (facultatif)</span>
              <input name="fileId" className="field mt-1.5 font-mono text-xs" />
            </label>
          </div>

          <p className="eyebrow mt-4">Détail</p>
          <div className="mt-1.5 space-y-2">
            {lignes.map((ligne, index) => (
              <div key={index} className="grid grid-cols-12 gap-2">
                <input
                  aria-label="Prestation"
                  placeholder="Prestation"
                  value={ligne.label}
                  onChange={(event) =>
                    setLignes((etat) =>
                      etat.map((l, i) => (i === index ? { ...l, label: event.target.value } : l)),
                    )
                  }
                  className="field col-span-6 h-8 text-xs"
                />
                <input
                  aria-label="Quantité"
                  type="number"
                  step="0.5"
                  value={ligne.quantity}
                  onChange={(event) =>
                    setLignes((etat) =>
                      etat.map((l, i) =>
                        i === index ? { ...l, quantity: Number(event.target.value) } : l,
                      ),
                    )
                  }
                  className="field col-span-2 h-8 text-xs"
                />
                <input
                  aria-label="Prix unitaire HT"
                  type="number"
                  step="0.01"
                  value={ligne.unitPriceHT}
                  onChange={(event) =>
                    setLignes((etat) =>
                      etat.map((l, i) =>
                        i === index ? { ...l, unitPriceHT: Number(event.target.value) } : l,
                      ),
                    )
                  }
                  className="field col-span-2 h-8 text-xs"
                />
                <input
                  aria-label="Taux de TVA"
                  type="number"
                  step="0.1"
                  disabled={vatExempt}
                  value={vatExempt ? 0 : ligne.vatRate}
                  onChange={(event) =>
                    setLignes((etat) =>
                      etat.map((l, i) =>
                        i === index ? { ...l, vatRate: Number(event.target.value) } : l,
                      ),
                    )
                  }
                  className="field col-span-2 h-8 text-xs"
                />
              </div>
            ))}
          </div>

          <div className="mt-2 flex items-center gap-3">
            <button
              type="button"
              className="btn-ghost"
              onClick={() =>
                setLignes((etat) => [
                  ...etat,
                  { label: "", quantity: 1, unitPriceHT: 0, vatRate: vatExempt ? 0 : 20 },
                ])
              }
            >
              Ajouter une ligne
            </button>
            <span className="ml-auto text-xs" data-numeric>
              Total HT {euros(total)}
            </span>
          </div>

          {vatExempt && (
            <p className="mt-2 text-2xs text-ink-3">
              Votre organisme est exonéré au titre de l&apos;article 261-4-4° a du
              CGI : la facture ne portera aucune taxe, et la mention le dira.
            </p>
          )}

          <label className="mt-3 block">
            <span className="eyebrow">Conditions de règlement</span>
            <input
              name="paymentTerms"
              placeholder="Virement à 30 jours"
              className="field mt-1.5"
            />
          </label>

          <button type="submit" className="btn-primary mt-4" disabled={busy === "creation"}>
            {busy === "creation" ? "Enregistrement…" : "Créer le brouillon"}
          </button>
        </form>
      )}

      {!pret && <p className="px-5 py-8 text-center text-xs text-ink-3">Lecture…</p>}

      {pret && factures.length === 0 && (
        <p className="px-5 py-10 text-center text-xs text-ink-3">
          Aucune facture. Le premier numéro sera attribué à la première
          émission — pas à la création, pour qu&apos;un brouillon abandonné ne
          laisse pas de trou dans la numérotation.
        </p>
      )}

      {pret && factures.length > 0 && (
        <ul className="divide-y divide-line/60">
          {factures.map((facture) => (
            <li key={facture.id} className="px-5 py-3">
              <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
                <span className="font-mono text-2xs text-ink-3">
                  {facture.number || "sans numéro"}
                </span>
                <span className="min-w-0 flex-1 truncate text-sm">{facture.client.name}</span>
                <span className="text-sm" data-numeric>
                  {euros(facture.totalTTC)}
                </span>
                <span
                  className={`rounded px-1.5 py-0.5 text-2xs ${ETATS[facture.status].classe}`}
                >
                  {facture.creditNoteFor ? "Avoir" : ETATS[facture.status].label}
                </span>
              </div>

              <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1">
                <span className="font-mono text-2xs text-ink-3">
                  {facture.issuedOn ? `émise le ${frDate(facture.issuedOn)}` : "non émise"}
                  {facture.dueOn ? ` · échéance ${frDate(facture.dueOn)}` : ""}
                  {facture.fileReference ? ` · ${facture.fileReference}` : ""}
                  {facture.creditNoteFor ? ` · corrige ${facture.creditNoteFor}` : ""}
                </span>

                <div className="ml-auto flex flex-wrap items-center gap-2">
                  <a
                    href={`/api/factures/${facture.id}/pdf`}
                    target="_blank"
                    rel="noopener"
                    className="btn-ghost"
                  >
                    PDF
                  </a>

                  {facture.status === "brouillon" && (
                    <>
                      <button
                        type="button"
                        className="btn-secondary"
                        disabled={busy !== null}
                        onClick={() =>
                          agir(facture.id, () =>
                            fetch(`/api/factures/${facture.id}/emission`, { method: "POST" }),
                          )
                        }
                      >
                        Émettre
                      </button>
                      <button
                        type="button"
                        className="btn-ghost"
                        disabled={busy !== null}
                        onClick={() =>
                          agir(facture.id, () =>
                            fetch(`/api/factures/${facture.id}`, { method: "DELETE" }),
                          )
                        }
                      >
                        Supprimer
                      </button>
                    </>
                  )}

                  {facture.status === "emise" && (
                    <>
                      <button
                        type="button"
                        className="btn-secondary"
                        disabled={busy !== null}
                        onClick={() =>
                          agir(facture.id, () =>
                            fetch(`/api/factures/${facture.id}/paiement`, {
                              method: "POST",
                              headers: { "Content-Type": "application/json" },
                              body: JSON.stringify({ paid: true, way: "virement" }),
                            }),
                          )
                        }
                      >
                        Marquer payée
                      </button>
                      <button
                        type="button"
                        className="btn-ghost"
                        disabled={busy !== null}
                        onClick={() =>
                          agir(facture.id, () =>
                            fetch(`/api/factures/${facture.id}/avoir`, {
                              method: "POST",
                              headers: { "Content-Type": "application/json" },
                              body: JSON.stringify({ motif: "Annulation" }),
                            }),
                          )
                        }
                      >
                        Établir un avoir
                      </button>
                    </>
                  )}

                  {facture.status === "payee" && (
                    <span className="text-2xs text-ink-3">
                      encaissée le {facture.paidAt ? frDate(facture.paidAt) : ""}
                    </span>
                  )}
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}

      <p className="border-t border-line px-5 py-3 text-2xs text-ink-3">
        Une facture émise ne se modifie ni ne se supprime : la numérotation doit
        rester continue, et une erreur se corrige par un avoir qui la référence.
        C&apos;est ce qu&apos;un contrôle vérifie en premier.
      </p>
    </section>
  );
}

function euros(montant: number): string {
  return new Intl.NumberFormat("fr-FR", { style: "currency", currency: "EUR" }).format(montant);
}

function frDate(jour: string): string {
  return new Date(jour).toLocaleDateString("fr-FR");
}
