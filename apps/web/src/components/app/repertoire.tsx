import Link from "next/link";
import { ContactsTable } from "@/components/app/contacts-table";
import { PlatformRows } from "@/components/app/platform-rows";
import { CreatePanel, Field, Select } from "@/components/app/form";
import { createContact } from "@/app/actions/crm";
import { apiFetch, type Contact, type Me } from "@/lib/api";

/**
 * Le répertoire d'un organisme, vu par nature.
 *
 * Il n'y a pas de mot juste pour couvrir les trois à la fois : « contacts » est
 * du jargon de CRM qui ne dit rien du métier. On les nomme donc séparément —
 * stagiaires, entreprises, financeurs — et chaque écran porte le nom de ce
 * qu'il contient. Le code du travail dit « stagiaire », pas « apprenant » ni
 * « contact » : autant employer le mot que lira un contrôleur.
 *
 * Une seule implémentation les sert tous : ce sont les mêmes fiches, seule la
 * nature change.
 */
export const NATURES = {
  learner: {
    href: "/stagiaires",
    titre: "Stagiaires",
    singulier: "Stagiaire",
    vide: "Aucun stagiaire enregistré.",
    aide: "Ils s'ajoutent ici, puis s'inscrivent à une session depuis le catalogue.",
  },
  company: {
    href: "/entreprises",
    titre: "Entreprises",
    singulier: "Entreprise cliente",
    vide: "Aucune entreprise enregistrée.",
    aide: "L'entreprise est la partie qui signe la convention quand elle envoie ses salariés.",
  },
  funder: {
    href: "/financeurs",
    titre: "Financeurs",
    singulier: "Financeur",
    vide: "Aucun financeur enregistré.",
    aide: "OPCO, France Travail, Caisse des Dépôts — celui qui prend la formation en charge.",
  },
} as const;

export type Nature = keyof typeof NATURES;

export async function Repertoire({ nature }: { nature: Nature }) {
  // L'équipe lemlearn n'a pas de stagiaires à elle : son organisme est vide, et
  // lui montrer son propre répertoire n'aurait aucun sens. Ces trois écrans lui
  // servent donc toute la plateforme, chaque ligne nommant l'organisme dont
  // elle relève.
  const me = await apiFetch<Me>("/v1/me");
  const equipe = me.user.role === "superadmin" && !me.impersonatedBy;

  const { contacts, cursor } = equipe
    ? { contacts: null, cursor: undefined }
    : await apiFetch<{ contacts: Contact[] | null; cursor?: string }>(
        `/v1/contacts?kind=${nature}`,
      );
  const rows = contacts ?? [];

  const courant = NATURES[nature];

  return (
    <>
      <header className="flex h-14 items-center gap-1 border-b border-line px-6">
        <h1 className="mr-3 text-sm font-medium">{courant.titre}</h1>
        {(Object.keys(NATURES) as Nature[]).map((clef) => (
          <Link
            key={clef}
            href={NATURES[clef].href}
            aria-current={clef === nature ? "page" : undefined}
            className={`rounded-md px-2.5 py-1 text-xs transition-colors duration-[120ms] ${
              clef === nature
                ? "bg-surface-2 text-ink"
                : "text-ink-3 hover:bg-surface-2 hover:text-ink"
            }`}
          >
            {NATURES[clef].titre}
          </Link>
        ))}
        {equipe ? (
          <span className="ml-auto text-2xs text-ink-3">
            Toute la plateforme
          </span>
        ) : (
          <span className="ml-auto font-mono text-2xs text-ink-3" data-numeric>
            {rows.length}
          </span>
        )}
        <div className={equipe ? "hidden" : "ml-3"}>
          <CreatePanel label="Nouveau" title={courant.singulier} action={createContact}>
            <Select
              label="Nature"
              name="kind"
              defaultValue={nature}
              options={[
                { value: "learner", label: "Stagiaire" },
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

      {equipe ? (
        <PlatformRows vue={VUE_PLATEFORME[nature]} />
      ) : rows.length === 0 ? (
        <div className="mx-auto max-w-lg px-6 py-16 text-center">
          <p className="text-sm text-ink-2">{courant.vide}</p>
          <p className="mt-3 text-xs text-ink-3">{courant.aide}</p>
        </div>
      ) : (
        <ContactsTable kind={nature} initial={rows} initialCursor={cursor} />
      )}
    </>
  );
}

/** Le nom que porte la même nature dans la vue plateforme. */
const VUE_PLATEFORME = {
  learner: "stagiaires",
  company: "entreprises",
  funder: "financeurs",
} as const;
