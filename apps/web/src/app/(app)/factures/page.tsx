import type { Metadata } from "next";
import { Factures } from "@/components/app/factures";
import { apiFetch, contactName, type Contact, type Me } from "@/lib/api";

export const metadata: Metadata = { title: "Factures" };

/**
 * La facturation de l'organisme à ses clients.
 *
 * À ne pas confondre avec l'abonnement, qui est ce que l'organisme nous paie à
 * nous et vit dans un autre écran.
 */
export default async function FacturesPage() {
  const me = await apiFetch<Me>("/v1/me");

  // Les trois natures peuvent être facturées : un stagiaire à ses propres
  // frais, l'entreprise qui envoie ses salariés, l'OPCO qui prend en charge.
  const listes = await Promise.all(
    (["learner", "company", "funder"] as const).map((kind) =>
      apiFetch<{ contacts: Contact[] | null }>(`/v1/contacts?kind=${kind}`).catch(() => ({
        contacts: null,
      })),
    ),
  );
  const clients = listes
    .flatMap((liste) => liste.contacts ?? [])
    .map((contact) => ({ id: contact.id, nom: contactName(contact) }));

  return (
    <>
      <header className="flex h-14 items-center gap-3 border-b border-line px-6">
        <h1 className="text-sm font-medium">Factures</h1>
        <p className="ml-3 truncate text-2xs text-ink-3">
          Ce que {me.org.name} facture à ses clients.
        </p>
      </header>

      <div className="mx-auto max-w-4xl space-y-6 px-6 py-6">
        <Factures clients={clients} vatExempt={me.org.vatExempt ?? false} />
      </div>
    </>
  );
}
