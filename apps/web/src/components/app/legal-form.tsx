"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

export interface OrgLegal {
  name: string;
  legalForm?: string;
  capital?: string;
  rcs?: string;
  siret?: string;
  vatNumber?: string;
  vatExempt: boolean;
  nda?: string;
  ndaRegion?: string;
  repName?: string;
  repRole?: string;
  address?: string;
  postalCode?: string;
  city?: string;
  qualiopiCertified: boolean;
  qualiopiNumber?: string;
  qualiopiBody?: string;
  qualiopiExpiresOn?: string;
}

/**
 * Identité juridique de l'organisme.
 *
 * Ces champs ne sont pas de l'administratif décoratif : ils sont repris sur
 * chaque convention, chaque contrat et chaque attestation, et deux d'entre eux
 * sont imposés par la réglementation. Les saisir une fois vaut mieux que les
 * ressaisir à chaque document — c'est aussi ce qui garantit qu'ils y sont
 * tous, et identiques.
 */
export function LegalForm({ initial }: { initial: OrgLegal }) {
  const router = useRouter();
  const [org, setOrg] = useState<OrgLegal>(initial);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  const set = (champ: keyof OrgLegal, valeur: string | boolean) => {
    setOrg((precedent) => ({ ...precedent, [champ]: valeur }));
    setSaved(false);
  };

  const save = async () => {
    setError(null);
    setBusy(true);
    try {
      const response = await fetch("/api/organisme", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(org),
      });
      const body = (await response.json()) as { error?: string };
      if (!response.ok) throw new Error(body.error ?? "enregistrement refusé");
      setSaved(true);
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "enregistrement impossible");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-5">
      <section className="surface-card p-5">
        <h2 className="text-sm font-medium">Identité de la structure</h2>
        <p className="mt-1.5 text-xs text-ink-2">
          Reprise en pied de vos conventions, contrats et attestations.
        </p>

        <div className="mt-5 grid gap-4 sm:grid-cols-2">
          <Champ label="Raison sociale" value={org.name} onChange={(v) => set("name", v)} />
          <Champ
            label="Forme juridique"
            value={org.legalForm ?? ""}
            placeholder="SAS, SARL, association…"
            onChange={(v) => set("legalForm", v)}
          />
          <Champ
            label="Capital social"
            value={org.capital ?? ""}
            placeholder="10 000 €"
            onChange={(v) => set("capital", v)}
          />
          <Champ
            label="RCS"
            value={org.rcs ?? ""}
            placeholder="Paris B 842 917 365"
            onChange={(v) => set("rcs", v)}
          />
          <Champ
            label="SIRET"
            value={org.siret ?? ""}
            placeholder="14 chiffres"
            aide="Le SIREN en est les neuf premiers chiffres."
            onChange={(v) => set("siret", v)}
          />
          <Champ
            label="TVA intracommunautaire"
            value={org.vatNumber ?? ""}
            placeholder="FR12842917365"
            onChange={(v) => set("vatNumber", v)}
          />
          <Champ label="Adresse" value={org.address ?? ""} onChange={(v) => set("address", v)} />
          <div className="grid grid-cols-[7rem_1fr] gap-3">
            <Champ
              label="Code postal"
              value={org.postalCode ?? ""}
              onChange={(v) => set("postalCode", v)}
            />
            <Champ label="Ville" value={org.city ?? ""} onChange={(v) => set("city", v)} />
          </div>
        </div>
      </section>

      <section className="surface-card p-5">
        <h2 className="text-sm font-medium">Déclaration d&apos;activité</h2>
        <p className="mt-1.5 text-xs text-ink-2">
          Le code du travail impose la mention de votre numéro de déclaration sur
          les conventions, dans une forme précise. Sans le numéro et la région, la
          mention est incomplète — et une mention incomplète ne vaut pas.
        </p>

        <div className="mt-5 grid gap-4 sm:grid-cols-2">
          <Champ
            label="Numéro de déclaration d'activité"
            value={org.nda ?? ""}
            placeholder="11756789012"
            onChange={(v) => set("nda", v)}
          />
          <Champ
            label="Préfecture de région"
            value={org.ndaRegion ?? ""}
            placeholder="Île-de-France"
            onChange={(v) => set("ndaRegion", v)}
          />
        </div>

        {org.nda && (
          <p className="mt-4 rounded-lg border border-line bg-surface-2/50 px-3 py-2.5 text-2xs text-ink-2">
            Sera imprimé : « déclaration d&apos;activité enregistrée sous le numéro{" "}
            {org.nda}
            {org.ndaRegion ? ` auprès du préfet de région ${org.ndaRegion}` : ""}. Cet
            enregistrement ne vaut pas agrément de l&apos;État. »
          </p>
        )}

        <label className="mt-5 flex items-start gap-3">
          <input
            type="checkbox"
            checked={org.vatExempt}
            onChange={(event) => set("vatExempt", event.target.checked)}
            className="mt-0.5 size-4"
          />
          <span>
            <span className="block text-sm">Exonéré de TVA</span>
            <span className="block text-2xs text-ink-3">
              Article 261-4-4° a du CGI. Vos documents porteront la mention et
              n&apos;afficheront aucune TVA.
            </span>
          </span>
        </label>
      </section>

      <section className="surface-card p-5">
        <h2 className="text-sm font-medium">Représentant légal</h2>
        <p className="mt-1.5 text-xs text-ink-2">
          C&apos;est cette personne qui engage l&apos;organisme en signant une
          convention. Un document signé par personne de nommé se conteste.
        </p>
        <div className="mt-5 grid gap-4 sm:grid-cols-2">
          <Champ label="Nom" value={org.repName ?? ""} onChange={(v) => set("repName", v)} />
          <Champ
            label="Qualité"
            value={org.repRole ?? ""}
            placeholder="présidente, gérant…"
            onChange={(v) => set("repRole", v)}
          />
        </div>
      </section>

      <section className="surface-card p-5">
        <h2 className="text-sm font-medium">Certification Qualiopi</h2>
        <p className="mt-1.5 text-xs text-ink-2">
          Un financeur ne se contente pas de savoir que vous êtes certifié : il
          vérifie le numéro, l&apos;organisme certificateur et l&apos;échéance.
        </p>

        <label className="mt-5 flex items-center gap-3">
          <input
            type="checkbox"
            checked={org.qualiopiCertified}
            onChange={(event) => set("qualiopiCertified", event.target.checked)}
            className="size-4"
          />
          <span className="text-sm">Organisme certifié Qualiopi</span>
        </label>

        {org.qualiopiCertified && (
          <div className="mt-5 grid gap-4 sm:grid-cols-3">
            <Champ
              label="Numéro de certificat"
              value={org.qualiopiNumber ?? ""}
              onChange={(v) => set("qualiopiNumber", v)}
            />
            <Champ
              label="Certificateur"
              value={org.qualiopiBody ?? ""}
              placeholder="AFNOR, Bureau Veritas…"
              onChange={(v) => set("qualiopiBody", v)}
            />
            <Champ
              label="Valable jusqu'au"
              value={org.qualiopiExpiresOn ?? ""}
              placeholder="30/06/2028"
              onChange={(v) => set("qualiopiExpiresOn", v)}
            />
          </div>
        )}
      </section>

      {error && <p className="text-xs text-danger">{error}</p>}

      <div className="flex items-center gap-3">
        <button type="button" className="btn-primary" disabled={busy} onClick={save}>
          {busy ? "Enregistrement…" : "Enregistrer"}
        </button>
        {saved && !busy && (
          <span className="text-xs text-ok">
            Enregistré. Vos prochains documents le reprendront.
          </span>
        )}
      </div>
    </div>
  );
}

function Champ({
  label,
  value,
  placeholder,
  aide,
  onChange,
}: {
  label: string;
  value: string;
  placeholder?: string;
  aide?: string;
  onChange: (valeur: string) => void;
}) {
  return (
    <label className="block">
      <span className="eyebrow">{label}</span>
      <input
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        className="field mt-1.5"
      />
      {aide && <span className="mt-1 block text-2xs text-ink-3">{aide}</span>}
    </label>
  );
}
