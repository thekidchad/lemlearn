import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ContactActions } from "@/components/app/contact-actions";
import { apiFetch, ApiError, contactName, type Contact, type FileRecord } from "@/lib/api";

export const metadata: Metadata = { title: "Fiche" };

interface Fiche {
  contact: Contact;
  org: { id: string; name: string; plan: string };
  compte?: { email: string; role: string; disabled: boolean };
  dossiers: FileRecord[] | null;
  inscriptions?: {
    sessionId: string;
    sessionTitle?: string;
    startsAt?: string;
    status: string;
    fileId?: string;
    finalPassed: boolean;
    finalPercent: number;
  }[];
}

const NATURES: Record<string, string> = {
  learner: "Stagiaire",
  company: "Entreprise",
  funder: "Financeur",
};

const ETAPES: Record<string, string> = {
  prospect: "Prospect",
  quote: "Devis",
  agreement: "Convention",
  training: "En formation",
  closed: "Clôturé",
};

const STATUTS: Record<string, string> = {
  enrolled: "Inscrit",
  in_progress: "En cours",
  completed: "Terminé",
  abandoned: "Abandonné",
};

/**
 * La fiche d'une personne, vue par l'équipe.
 *
 * La recherche menait jusqu'ici à la fiche de l'organisme : on trouvait la
 * bonne maison, pas la bonne personne. Cet écran répond aux questions qu'on se
 * pose en décrochant — qui est-ce, chez qui, inscrite à quoi, où en est son
 * dossier, a-t-elle un compte — et pose les deux gestes qui suivent : entrer
 * dans son espace, sortir son dossier.
 *
 * En lecture seule, délibérément : corriger une donnée chez un client se fait
 * dans son espace, sous son identité, pas depuis la console. Sinon plus
 * personne ne sait qui a écrit quoi.
 */
export default async function FichePage({
  params,
}: PageProps<"/admin/[orgId]/contacts/[contactId]">) {
  const { orgId, contactId } = await params;

  let fiche: Fiche;
  try {
    fiche = await apiFetch(`/v1/admin/orgs/${orgId}/contacts/${contactId}`);
  } catch (error) {
    if (error instanceof ApiError && (error.status === 404 || error.status === 403)) notFound();
    throw error;
  }

  const { contact, org, compte } = fiche;
  const dossiers = fiche.dossiers ?? [];
  const inscriptions = fiche.inscriptions ?? [];
  const dossier = dossiers[0];

  return (
    <>
      <header className="border-b border-line px-8 pt-6 pb-5">
        <nav className="flex items-center gap-2 text-2xs text-ink-3">
          <Link href="/admin/organismes" className="hover:text-ink">
            Plateforme
          </Link>
          <span>/</span>
          <Link href={`/admin/${org.id}`} className="hover:text-ink">
            {org.name}
          </Link>
          <span>/</span>
          <span>{NATURES[contact.kind] ?? contact.kind}</span>
        </nav>

        <div className="mt-2 flex flex-wrap items-end gap-4">
          <div className="min-w-0 flex-1">
            <h1 className="text-xl font-medium tracking-tight">{contactName(contact)}</h1>
            <p className="mt-1 font-mono text-2xs text-ink-3">
              {[contact.email, contact.phone].filter(Boolean).join(" · ") || "sans contact"}
            </p>
          </div>
          <ContactActions
            orgId={org.id}
            contactId={contact.id}
            hasAccount={Boolean(compte && !compte.disabled)}
            accountHint={
              !compte
                ? "Aucun compte : la personne n'a pas encore été invitée."
                : compte.disabled
                  ? "Compte créé mais mot de passe pas encore choisi."
                  : undefined
            }
            fileId={dossier?.id}
            fileReference={dossier?.reference}
          />
        </div>
      </header>

      <div className="grid gap-6 px-8 py-6 lg:grid-cols-[1fr_20rem]">
        <div className="space-y-6">
          <section className="surface-card overflow-hidden">
            <h2 className="border-b border-line px-5 py-3 text-sm font-medium">Identité</h2>
            <dl className="divide-y divide-line/60">
              <Ligne label="Nature" valeur={NATURES[contact.kind] ?? contact.kind} />
              <Ligne label="Nom" valeur={contactName(contact)} />
              <Ligne label="Raison sociale" valeur={contact.companyName} />
              <Ligne label="Courriel" valeur={contact.email} mono />
              <Ligne label="Téléphone" valeur={contact.phone} mono />
              <Ligne label="Naissance" valeur={dateEtLieu(contact)} />
              <Ligne label="SIRET" valeur={contact.siret} mono />
              <Ligne label="Adresse" valeur={adresse(contact)} />
            </dl>
          </section>

          {contact.kind === "learner" && (
            <section className="surface-card overflow-hidden">
              <h2 className="border-b border-line px-5 py-3 text-sm font-medium">
                Inscriptions
              </h2>
              {inscriptions.length === 0 ? (
                <p className="px-5 py-8 text-center text-xs text-ink-3">
                  Aucune inscription. La fiche existe, la formation n&apos;a pas
                  encore commencé.
                </p>
              ) : (
                <ul className="divide-y divide-line/60">
                  {inscriptions.map((ligne) => (
                    <li key={ligne.sessionId} className="flex items-baseline gap-3 px-5 py-3">
                      <span className="min-w-0 flex-1 truncate text-sm">
                        {ligne.sessionTitle || ligne.sessionId}
                      </span>
                      <span className="shrink-0 font-mono text-2xs text-ink-3">
                        {ligne.startsAt ?? ""}
                      </span>
                      <span className="shrink-0 text-2xs text-ink-2">
                        {STATUTS[ligne.status] ?? ligne.status}
                      </span>
                      {ligne.finalPassed && (
                        <span className="shrink-0 text-2xs text-success" data-numeric>
                          final {ligne.finalPercent} %
                        </span>
                      )}
                    </li>
                  ))}
                </ul>
              )}
            </section>
          )}

          <section className="surface-card overflow-hidden">
            <h2 className="border-b border-line px-5 py-3 text-sm font-medium">Dossiers</h2>
            {dossiers.length === 0 ? (
              <p className="px-5 py-8 text-center text-xs text-ink-3">
                Aucun dossier. C&apos;est le dossier qui porte la convention, les
                preuves et l&apos;archive : sans lui, il n&apos;y a rien à exporter.
              </p>
            ) : (
              <ul className="divide-y divide-line/60">
                {dossiers.map((file) => (
                  <li key={file.id} className="flex items-baseline gap-3 px-5 py-3">
                    <span className="min-w-0 flex-1 truncate text-sm">{file.title}</span>
                    <span className="shrink-0 font-mono text-2xs text-ink-3">
                      {file.reference}
                    </span>
                    <span className="shrink-0 text-2xs text-ink-2">
                      {ETAPES[file.stage] ?? file.stage}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </section>
        </div>

        <aside className="space-y-4">
          <section className="surface-card p-5">
            <h2 className="text-sm font-medium">Compte</h2>
            {compte ? (
              <dl className="mt-3 space-y-2 text-xs">
                <div className="flex justify-between gap-3">
                  <dt className="text-ink-3">Adresse</dt>
                  <dd className="truncate font-mono text-2xs">{compte.email}</dd>
                </div>
                <div className="flex justify-between gap-3">
                  <dt className="text-ink-3">Rôle</dt>
                  <dd>{compte.role}</dd>
                </div>
                <div className="flex justify-between gap-3">
                  <dt className="text-ink-3">État</dt>
                  <dd className={compte.disabled ? "text-warn" : "text-success"}>
                    {compte.disabled ? "en attente de mot de passe" : "actif"}
                  </dd>
                </div>
              </dl>
            ) : (
              <p className="mt-2 text-xs text-ink-3">
                Aucun compte rattaché à cette fiche. L&apos;organisme l&apos;invite
                depuis son propre espace.
              </p>
            )}
          </section>

          <section className="surface-card p-5">
            <h2 className="text-sm font-medium">Organisme</h2>
            <Link
              href={`/admin/${org.id}`}
              className="mt-2 block text-xs hover:text-accent-ink hover:underline"
            >
              {org.name}
            </Link>
            <p className="mt-1 text-2xs text-ink-3">Formule {org.plan}</p>
          </section>

          <p className="px-1 text-2xs text-ink-3">
            Cette consultation est inscrite au journal de {org.name}. Modifier une
            donnée se fait dans son espace, sous son identité — sinon plus personne
            ne sait qui a écrit quoi.
          </p>
        </aside>
      </div>
    </>
  );
}

function Ligne({
  label,
  valeur,
  mono,
}: {
  label: string;
  valeur?: string;
  mono?: boolean;
}) {
  return (
    <div className="flex items-baseline gap-4 px-5 py-2.5">
      <dt className="w-32 shrink-0 text-2xs text-ink-3">{label}</dt>
      <dd className={`min-w-0 flex-1 text-xs ${mono ? "font-mono" : ""}`}>
        {valeur?.trim() ? valeur : <span className="text-ink-3">—</span>}
      </dd>
    </div>
  );
}

/** Date et lieu de naissance : les deux mentions qu'exige l'attestation. */
function dateEtLieu(contact: Contact): string {
  return [contact.birthDate, contact.birthPlace].filter(Boolean).join(" à ");
}

function adresse(contact: Contact): string {
  const address = contact.address;
  if (!address) return "";
  return [
    address.line1,
    [address.postalCode, address.city].filter(Boolean).join(" "),
    address.country,
  ]
    .filter(Boolean)
    .join(", ");
}
