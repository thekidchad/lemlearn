import type { Metadata } from "next";
import Link from "next/link";
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

  const { contacts } = await apiFetch<{ contacts: Contact[] | null }>(
    `/v1/contacts?kind=${encodeURIComponent(kind)}`,
  );
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
      </header>

      {rows.length === 0 ? (
        <p className="px-6 py-16 text-center text-xs text-ink-3">
          Aucun contact de ce type pour l&apos;instant.
        </p>
      ) : (
        <table className="w-full text-left">
          <thead>
            <tr className="border-b border-line text-2xs tracking-wide text-ink-3 uppercase">
              <th className="px-6 py-2.5 font-medium">Nom</th>
              <th className="px-6 py-2.5 font-medium">Contact</th>
              {kind === "learner" && <th className="px-6 py-2.5 font-medium">Naissance</th>}
              {kind !== "learner" && <th className="px-6 py-2.5 font-medium">SIRET</th>}
              <th className="px-6 py-2.5 font-medium">Ville</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((contact) => (
              <tr
                key={contact.id}
                className="border-b border-line/60 text-sm transition-colors duration-[120ms] hover:bg-surface-1"
              >
                <td className="px-6 py-2.5">{displayName(contact)}</td>
                <td className="px-6 py-2.5 text-xs text-ink-2">
                  {contact.email ?? "—"}
                  {contact.phone ? ` · ${contact.phone}` : ""}
                </td>
                {kind === "learner" && (
                  <td className="px-6 py-2.5 font-mono text-xs text-ink-2">
                    {contact.birthDate ?? "—"}
                  </td>
                )}
                {kind !== "learner" && (
                  <td className="px-6 py-2.5 font-mono text-xs text-ink-2">
                    {(contact as { siret?: string }).siret ?? "—"}
                  </td>
                )}
                <td className="px-6 py-2.5 text-xs text-ink-2">
                  {contact.address?.city ?? "—"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  );
}

function displayName(contact: Contact): string {
  if (contact.companyName) return contact.companyName;
  return `${contact.firstName ?? ""} ${contact.lastName ?? ""}`.trim() || "—";
}
