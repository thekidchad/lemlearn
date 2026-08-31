import type { Metadata } from "next";
import Link from "next/link";
import { ContactsTable } from "@/components/app/contacts-table";
import { CreatePanel, Field, Select } from "@/components/app/form";
import { createContact } from "@/app/actions/crm";
import { apiFetch, type Contact } from "@/lib/api";

export const metadata: Metadata = { title: "Contacts" };

const KINDS = [
  { key: "learner", label: "Apprenants" },
  { key: "company", label: "Entreprises" },
  { key: "funder", label: "Financeurs" },
] as const;

export default async function ContactsPage({ searchParams }: PageProps<"/contacts">) {
  const params = await searchParams;
  const kind = typeof params.kind === "string" ? params.kind : "learner";

  const { contacts, cursor } = await apiFetch<{
    contacts: Contact[] | null;
    cursor?: string;
  }>(`/v1/contacts?kind=${encodeURIComponent(kind)}`);
  const rows = contacts ?? [];

  return (
    <>
      <header className="flex h-14 items-center gap-1 border-b border-line px-6">
        <h1 className="mr-3 text-sm font-medium">Contacts</h1>
        {KINDS.map((entry) => (
          <Link
            key={entry.key}
            href={`/contacts?kind=${entry.key}`}
            aria-current={entry.key === kind ? "page" : undefined}
            className={`rounded-md px-2.5 py-1 text-xs transition-colors duration-[120ms] ${
              entry.key === kind
                ? "bg-surface-2 text-ink"
                : "text-ink-3 hover:bg-surface-2 hover:text-ink"
            }`}
          >
            {entry.label}
          </Link>
        ))}
        <span className="ml-auto font-mono text-2xs text-ink-3" data-numeric>
          {rows.length}
        </span>
        <div className="ml-3">
          <CreatePanel label="Nouveau" title="Nouveau contact" action={createContact}>
            <Select
              label="Nature"
              name="kind"
              defaultValue={kind}
              options={[
                { value: "learner", label: "Apprenant" },
                { value: "company", label: "Entreprise cliente" },
                { value: "funder", label: "Financeur (OPCO)" },
              ]}
            />
            <div className="grid grid-cols-2 gap-3">
              <Field label="Prénom" name="firstName" />
              <Field label="Nom" name="lastName" />
            </div>
            <Field
              label="Raison sociale"
              name="companyName"
              hint="Pour une entreprise ou un financeur."
            />
            <div className="grid grid-cols-2 gap-3">
              <Field label="Courriel" name="email" type="email" />
              <Field label="Téléphone" name="phone" />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <Field
                label="Date de naissance"
                name="birthDate"
                placeholder="1988-04-12"
                hint="Exigée sur l'attestation."
              />
              <Field label="Lieu de naissance" name="birthPlace" />
            </div>
            <Field label="SIRET" name="siret" />
            <Field label="Adresse" name="line1" />
            <div className="grid grid-cols-2 gap-3">
              <Field label="Code postal" name="postalCode" />
              <Field label="Ville" name="city" />
            </div>
          </CreatePanel>
        </div>
      </header>

      <ContactsTable kind={kind} initial={rows} initialCursor={cursor} />
    </>
  );
}

