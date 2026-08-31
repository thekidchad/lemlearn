import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ContactForm } from "@/components/app/contact-form";
import { IdentityDoc } from "@/components/app/identity-doc";
import { LearnerAccess } from "@/components/app/learner-access";
import { apiFetch, ApiError, contactName, type Contact, type FileRecord } from "@/lib/api";

export const metadata: Metadata = { title: "Fiche" };

const KINDS: Record<string, string> = {
  learner: "Apprenant",
  company: "Entreprise cliente",
  funder: "Financeur",
};

export default async function ContactPage({ params }: PageProps<"/stagiaires/[contactId]">) {
  const { contactId } = await params;

  let contact: Contact;
  try {
    contact = await apiFetch(`/v1/contacts/${contactId}`);
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) notFound();
    throw error;
  }

  // Les dossiers où l'apprenant figure : c'est par là qu'on remonte à sa
  // formation, et c'est la question qu'on se pose en ouvrant une fiche.
  const { pipeline } = await apiFetch<{ pipeline: Record<string, FileRecord[]> }>(
    "/v1/files",
  ).catch(() => ({ pipeline: {} as Record<string, FileRecord[]> }));
  const files = Object.values(pipeline)
    .flat()
    .filter((file) => file.learnerId === contactId);

  return (
    <>
      <header className="flex h-14 items-center gap-3 border-b border-line px-6">
        <Link
          href={
            contact.kind === "company"
              ? "/entreprises"
              : contact.kind === "funder"
                ? "/financeurs"
                : "/stagiaires"
          }
          className="text-xs text-ink-3 hover:text-ink"
        >
          Contacts
        </Link>
        <span className="text-ink-3">/</span>
        <span className="truncate text-xs text-ink-2">{contactName(contact)}</span>
        <span className="ml-auto font-mono text-2xs text-ink-3">
          {KINDS[contact.kind] ?? contact.kind}
        </span>
      </header>

      <div className="mx-auto max-w-3xl space-y-6 px-6 py-6">
        {contact.anonymized && (
          <p className="rounded-lg border border-warn/40 bg-warn/10 px-3 py-2 text-xs text-warn">
            Cette fiche a été anonymisée à la demande de la personne. Les pièces
            probatoires qui lui étaient rattachées survivent sous pseudonyme :
            les effacer priverait l&apos;organisme de la preuve d&apos;une
            formation réellement dispensée.
          </p>
        )}

        <ContactForm contact={contact} />

        {contact.kind === "learner" && !contact.anonymized && (
          <>
            <LearnerAccess contactId={contactId} email={contact.email} />
            <IdentityDoc contactId={contactId} present={Boolean(contact.identityDocKey)} />
          </>
        )}

        <section className="surface-card p-5">
          <h2 className="text-sm font-medium">Dossiers</h2>
          {files.length === 0 ? (
            <p className="mt-2 text-xs text-ink-3">
              Aucun dossier. C&apos;est le dossier qui porte la chaîne de preuve :
              sans lui, une inscription n&apos;alimente rien.
            </p>
          ) : (
            <ul className="mt-3 space-y-1.5">
              {files.map((file) => (
                <li key={file.id}>
                  <Link
                    href={`/dossiers/${file.id}`}
                    className="flex items-center justify-between rounded-md px-2 py-1.5 text-xs text-ink-2 hover:bg-surface-2 hover:text-ink"
                  >
                    <span className="truncate">
                      <span className="mr-2 font-mono text-2xs text-ink-3">
                        {file.reference}
                      </span>
                      {file.title}
                    </span>
                    <span className="shrink-0 font-mono text-2xs text-ink-3" data-numeric>
                      {file.proof.present}/{file.proof.expected}
                    </span>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>
    </>
  );
}
