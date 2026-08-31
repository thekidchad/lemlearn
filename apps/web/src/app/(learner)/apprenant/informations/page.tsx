import type { Metadata } from "next";
import Link from "next/link";
import { apiFetch, ApiError, contactName, type Contact } from "@/lib/api";

export const metadata: Metadata = { title: "Mes informations" };

interface Organisme {
  id: string;
  name: string;
  legalForm?: string;
  siret?: string;
  ndaNumber?: string;
  ndaRegion?: string;
  rcs?: string;
  vatNumber?: string;
  capital?: string;
  repName?: string;
  repRole?: string;
  email?: string;
  phone?: string;
  address?: {
    line1?: string;
    postalCode?: string;
    city?: string;
    country?: string;
  };
  qualiopi?: { number?: string; body?: string; expiresOn?: string };
}

interface Moi {
  contact: Contact;
  organisme: Organisme;
}

/**
 * Ce que l'organisme a écrit sur soi, et qui il est.
 *
 * Un stagiaire ne demandait pas ces informations parce qu'il s'en désintéresse,
 * mais parce qu'il n'avait aucun moyen de les voir : il fallait écrire au
 * secrétariat pour savoir quelle date de naissance figurera sur l'attestation.
 * Une donnée fausse se corrige mieux avant la signature qu'après.
 *
 * La correction ne se fait pas ici : c'est l'organisme qui tient le registre,
 * et une fiche modifiable des deux côtés sans trace ne prouve plus rien. On
 * dit donc à qui s'adresser, ce qui est la vraie réponse.
 */
export default async function InformationsPage() {
  let moi: Moi;
  try {
    moi = await apiFetch<Moi>("/v1/learn/moi");
  } catch (error) {
    if (error instanceof ApiError && error.status === 403) {
      return <PasDeFiche vers="/pipeline" quoi="Mes informations" />;
    }
    throw error;
  }
  const { contact, organisme } = moi;

  return (
    <div className="mx-auto max-w-2xl px-5 py-12 sm:px-8 sm:py-16">
      <h1 className="learner-title">Mes informations</h1>
      <p className="learner-body mt-3">
        Ce que {organisme.name} a enregistré vous concernant. Ces mentions sont
        celles qui figureront sur votre convention et sur votre attestation.
      </p>

      <section className="mt-10">
        <h2 className="learner-heading">Mon identité</h2>
        <dl className="mt-4 divide-y divide-line/70 rounded-xl border border-line">
          <Ligne label="Nom" valeur={contactName(contact)} />
          <Ligne label="Courriel" valeur={contact.email} />
          <Ligne label="Téléphone" valeur={contact.phone} />
          <Ligne
            label="Naissance"
            valeur={[contact.birthDate, contact.birthPlace].filter(Boolean).join(" à ")}
            aide="Exigée sur l'attestation de fin de formation."
          />
          <Ligne
            label="Adresse"
            valeur={[
              contact.address?.line1,
              [contact.address?.postalCode, contact.address?.city].filter(Boolean).join(" "),
            ]
              .filter(Boolean)
              .join(", ")}
          />
        </dl>
        <p className="mt-3 text-sm text-ink-3">
          Une erreur ? Signalez-la à {organisme.name}
          {organisme.email ? ` — ${organisme.email}` : ""}. C&apos;est
          l&apos;organisme qui tient le registre : le corriger des deux côtés
          sans trace enlèverait à vos pièces leur valeur de preuve.
        </p>
      </section>

      <section className="mt-10">
        <h2 className="learner-heading">Mon organisme de formation</h2>
        <dl className="mt-4 divide-y divide-line/70 rounded-xl border border-line">
          <Ligne label="Raison sociale" valeur={organisme.name} />
          <Ligne label="Forme juridique" valeur={organisme.legalForm} />
          <Ligne
            label="Adresse"
            valeur={[
              organisme.address?.line1,
              [organisme.address?.postalCode, organisme.address?.city].filter(Boolean).join(" "),
            ]
              .filter(Boolean)
              .join(", ")}
          />
          <Ligne label="SIRET" valeur={organisme.siret} />
          <Ligne
            label="Déclaration d'activité"
            valeur={
              organisme.ndaNumber
                ? `${organisme.ndaNumber}${organisme.ndaRegion ? ` — préfet de région ${organisme.ndaRegion}` : ""}`
                : ""
            }
            aide="Cet enregistrement ne vaut pas agrément de l'État (art. L.6352-12 du code du travail)."
          />
          <Ligne
            label="Certification Qualiopi"
            valeur={
              organisme.qualiopi?.number
                ? `${organisme.qualiopi.number}${organisme.qualiopi.body ? ` — ${organisme.qualiopi.body}` : ""}`
                : ""
            }
          />
          <Ligne label="Représentant" valeur={[organisme.repName, organisme.repRole].filter(Boolean).join(", ")} />
          <Ligne label="Contact" valeur={[organisme.email, organisme.phone].filter(Boolean).join(" · ")} />
        </dl>
      </section>

      <section className="mt-10">
        <h2 className="learner-heading">Mes données</h2>
        <p className="learner-body mt-3">
          Vous pouvez emporter l&apos;intégralité de ce qui vous concerne : fiche,
          inscriptions, résultats, horodatages. C&apos;est un fichier lisible,
          pas une capture d&apos;écran.
        </p>
        <a
          href={`/api/contacts/${contact.id}/donnees`}
          className="mt-4 inline-flex h-11 items-center rounded-xl border border-line px-5 text-sm hover:border-accent"
        >
          Télécharger mes données
        </a>
      </section>
    </div>
  );
}

function Ligne({
  label,
  valeur,
  aide,
}: {
  label: string;
  valeur?: string;
  aide?: string;
}) {
  return (
    <div className="px-5 py-4">
      <dt className="text-xs text-ink-3">{label}</dt>
      <dd className="mt-1 text-base">
        {valeur?.trim() ? valeur : <span className="text-ink-3">non renseigné</span>}
      </dd>
      {aide && <p className="mt-1 text-xs text-ink-3">{aide}</p>}
    </div>
  );
}

/**
 * Ce qu'on affiche à un compte qui n'est pas celui d'un stagiaire.
 *
 * L'espace apprenant est ouvert à l'équipe de l'organisme — elle y consulte le
 * parcours de quelqu'un — mais ces deux écrans-ci parlent de soi, et un compte
 * d'administration n'a pas de fiche à lui. Le dire vaut mieux que de laisser
 * l'appel échouer : une page qui ne charge pas ressemble à une panne.
 */
function PasDeFiche({ vers, quoi }: { vers: string; quoi: string }) {
  return (
    <div className="mx-auto max-w-2xl px-5 py-12 sm:px-8 sm:py-16">
      <h1 className="learner-title">{quoi}</h1>
      <p className="learner-body mt-3">
        Votre compte n&apos;est pas rattaché à une fiche de stagiaire : cet écran
        montre ce qu&apos;un organisme a enregistré sur la personne connectée, et
        il n&apos;y a donc rien à montrer ici.
      </p>
      <Link
        href={vers}
        className="mt-6 inline-flex h-11 items-center rounded-xl border border-line px-5 text-sm hover:border-accent"
      >
        Retour à l&apos;espace de l&apos;organisme
      </Link>
    </div>
  );
}
